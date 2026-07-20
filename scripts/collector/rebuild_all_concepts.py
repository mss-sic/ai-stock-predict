#!/usr/bin/env python3
"""
概念板块 & 行业数据全量重建 (v2)
===================================
数据源: 东财 push2 slist API (按个股反向采集)
分类规则:
  - Ⅲ后缀 → industry_l3 / Ⅱ后缀 → industry_l2
  - 预增/预减/扭亏/首亏 → financial_report
  - 其他 → concept
"""
import os, time, ssl, json, sys, traceback
import urllib.request as ur
import psycopg2
from psycopg2.extras import execute_values
from collections import defaultdict

LOG = '/tmp/rebuild_concepts.log'
FL = open(LOG, 'w')
def log(msg):
    line = f'{time.strftime("%H:%M:%S")} {msg}'
    FL.write(line + '\n')
    FL.flush()
    print(line, flush=True)

def classify_board(name, code):
    if name.endswith('Ⅲ'): return 'industry_l3'
    if name.endswith('Ⅱ'): return 'industry_l2'
    if any(kw in name for kw in ['预增','预减','扭亏','首亏']): return 'financial_report'
    return 'concept'

log('=== CONCEPT & INDUSTRY FULL REBUILD v2 ===')

try:
    PG = os.environ.get('PG_DSN', 'host=localhost dbname=stock_predict user=stock password=stock123')
    conn = psycopg2.connect(PG)
    cur = conn.cursor()

    # Phase 0: Clear 东财 data (keep 申万 industry/industry_l2)
    log('Phase 0: clearing old 东财 data...')
    cur.execute("DELETE FROM stock_concepts WHERE concept_type IN ('concept','industry_l3')")
    cur.execute("DELETE FROM stock_concepts WHERE concept_type='industry_l2' AND (concept_code LIKE 'BK%%' OR concept_code LIKE 'gn_%%')")
    cur.execute("DELETE FROM concept_boards WHERE concept_type IN ('concept','industry_l3')")
    cur.execute("DELETE FROM concept_boards WHERE concept_type='industry_l2' AND (concept_code LIKE 'BK%%' OR concept_code LIKE 'gn_%%')")
    conn.commit()
    cur.execute("SELECT concept_type, COUNT(*) FROM concept_boards GROUP BY concept_type ORDER BY concept_type")
    for r in cur.fetchall(): log(f'  Remaining: {r[0]} = {r[1]}')
    log('Phase 0 done')

    # Phase 1: Scan
    cur.execute("SELECT code, name FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
    stocks = cur.fetchall()
    total = len(stocks)
    log(f'Phase 1: scanning {total} stocks (~{total*0.35/60:.0f}min)')
    cur.close(); conn.close()

    # SSL context
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE

    concept_map = {}
    stock_mappings = []
    concept_stock_count = defaultdict(int)
    errors = 0
    t_start = time.time()

    for i, (code, name) in enumerate(stocks):
        try:
            mkt = 1 if code.startswith('6') else 0
            url = f'https://push2.eastmoney.com/api/qt/slist/get?fltt=2&invt=2&secid={mkt}.{code}&spt=3&pi=0&pz=200&po=1&fields=f12,f14'
            req = ur.Request(url, headers={'User-Agent':'Mozilla/5.0','Referer':'https://quote.eastmoney.com/'})
            with ur.urlopen(req, timeout=5, context=ctx) as resp:
                r = json.loads(resp.read().decode('utf-8'))
            diff = (r.get('data') or {}).get('diff') or {}
            items = diff.values() if isinstance(diff, dict) else diff
            for it in items:
                bk = it.get('f12',''); bn = it.get('f14','')
                if not bk: continue
                if bk not in concept_map:
                    concept_map[bk] = (bn, classify_board(bn, bk))
                stock_mappings.append((bk, code, bn, name))
                concept_stock_count[bk] += 1
        except: errors += 1

        if (i+1) % 200 == 0:
            e = time.time()-t_start
            r = (i+1)/max(e,0.01)
            eta = (total-i-1)/max(r,0.01)/60
            log(f'  [{i+1}/{total}] boards={len(concept_map)} errs={errors} {r:.1f}/s ETA:{eta:.0f}m')
        time.sleep(0.25)

    elapsed = time.time()-t_start
    log(f'Phase 1 done: {elapsed/60:.1f}min boards={len(concept_map)} mappings={len(stock_mappings)} errors={errors}')

    # Phase 2: DB
    conn = psycopg2.connect(PG)
    cur = conn.cursor()

    log(f'Phase 2: inserting {len(concept_map)} boards...')
    brds = [(bk, bn, ctype, concept_stock_count.get(bk,0)) for bk,(bn,ctype) in concept_map.items()]
    execute_values(cur, """INSERT INTO concept_boards(concept_code,concept_name,concept_type,stock_count) VALUES %s ON CONFLICT(concept_code) DO UPDATE SET concept_name=EXCLUDED.concept_name,concept_type=EXCLUDED.concept_type,stock_count=EXCLUDED.stock_count,updated_at=NOW()""", brds, page_size=200)
    conn.commit(); log('  Boards done')

    log(f'Phase 2: inserting {len(stock_mappings)} mappings...')
    B = 500
    for s in range(0, len(stock_mappings), B):
        batch = [(code, bk, bn, concept_map.get(bk,(bn,'concept'))[1], sname) for bk,code,bn,sname in stock_mappings[s:s+B]]
        execute_values(cur, """INSERT INTO stock_concepts(code,concept_code,concept_name,concept_type,stock_name) VALUES %s ON CONFLICT(code,concept_code) DO UPDATE SET concept_name=EXCLUDED.concept_name,concept_type=EXCLUDED.concept_type,stock_name=EXCLUDED.stock_name,updated_at=NOW()""", batch, page_size=200)
        conn.commit()
        if (s//B+1) % 20 == 0: log(f'  Mappings: {min(s+B,len(stock_mappings))}/{len(stock_mappings)}')
    log('  Mappings done')

    # Summary
    cur.execute("SELECT concept_type, COUNT(*) boards, SUM(stock_count) total FROM concept_boards GROUP BY concept_type ORDER BY COUNT(*) DESC")
    log('\n=== FINAL SUMMARY ===')
    for row in cur.fetchall(): log(f'  {row[0]:<20} boards={row[1]:>5}  stocks={row[2]:>8}')
    cur.execute("SELECT COUNT(*), COUNT(DISTINCT code) FROM stock_concepts")
    rr = cur.fetchone()
    log(f'  Total stock_concepts: {rr[0]} mappings, {rr[1]} stocks')
    cur.close(); conn.close()
    FL.close()
    log(f'\n=== FINISHED in {elapsed/60:.1f}min ===')
except Exception as e:
    log(f'FATAL: {e}')
    log(traceback.format_exc())
    FL.close()
    sys.exit(1)
