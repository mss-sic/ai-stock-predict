#!/usr/bin/env python3
"""
概念板块快速重建 — 先拉板块列表，再按板块拉成分股
比逐只股票反向采集快 10x+
"""
import os, time, json, sys, traceback, ssl
import urllib.request as ur
import psycopg2
from psycopg2.extras import execute_values
from collections import defaultdict

LOG = '/tmp/rebuild_concepts.log'
def log(msg):
    line = f'{time.strftime("%H:%M:%S")} {msg}'
    with open(LOG, 'a') as f:
        f.write(line + '\n')
    print(line, flush=True)

def classify(name, code):
    if name.endswith('Ⅲ'): return 'industry_l3'
    if name.endswith('Ⅱ'): return 'industry_l2'
    if any(kw in name for kw in ['预增','预减','扭亏','首亏']): return 'financial_report'
    return 'concept'

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

def api_get(url):
    req = ur.Request(url, headers={'User-Agent':'Mozilla/5.0','Referer':'https://quote.eastmoney.com/'})
    with ur.urlopen(req, timeout=10, context=ctx) as resp:
        return json.loads(resp.read().decode('utf-8'))

log('=== CONCEPT FAST REBUILD ===')

try:
    PG = os.environ.get('PG_DSN', 'host=localhost dbname=stock_predict user=stock password=stock123')
    conn = psycopg2.connect(PG)
    cur = conn.cursor()

    # Phase 0: Clear 东财 data
    log('Phase 0: clearing old 东财 data...')
    cur.execute("DELETE FROM stock_concepts WHERE concept_type IN ('concept','industry_l3')")
    cur.execute("DELETE FROM stock_concepts WHERE concept_type='industry_l2' AND (concept_code LIKE 'BK%%' OR concept_code LIKE 'gn_%%')")
    cur.execute("DELETE FROM concept_boards WHERE concept_type IN ('concept','industry_l3')")
    cur.execute("DELETE FROM concept_boards WHERE concept_type='industry_l2' AND (concept_code LIKE 'BK%%' OR concept_code LIKE 'gn_%%')")
    conn.commit()
    cur.execute("SELECT concept_type, COUNT(*) FROM concept_boards GROUP BY concept_type ORDER BY concept_type")
    for r in cur.fetchall(): log(f'  Remaining: {r[0]} = {r[1]}')

    # Phase 1: Fetch board lists
    log('Phase 1: fetching board lists...')
    boards = []

    # Industry boards (m:90+t:2)
    try:
        r = api_get('https://push2.eastmoney.com/api/qt/clist/get?fs=m:90+t:2&fid=f3&po=1&pz=500&np=1&fltt=2&invt=2&fields=f12,f14')
        for it in (r.get('data',{}).get('diff',[]) or []):
            boards.append((it['f12'], it['f14'], 'industry'))
        log(f'  Industry boards: {len(boards)}')
    except Exception as e:
        log(f'  Industry fetch failed: {e}')

    # Concept boards (m:90+t:3) - this is the big one
    n_concept = 0
    try:
        r = api_get('https://push2.eastmoney.com/api/qt/clist/get?fs=m:90+t:3&fid=f3&po=1&pz=2000&np=1&fltt=2&invt=2&fields=f12,f14')
        for it in (r.get('data',{}).get('diff',[]) or []):
            bk, bn = it['f12'], it['f14']
            ctype = classify(bn, bk)
            boards.append((bk, bn, ctype))
            if ctype == 'concept': n_concept += 1
        log(f'  Total boards: {len(boards)} (concepts: {n_concept})')
    except Exception as e:
        log(f'  Concept fetch failed: {e}')

    if not boards:
        log('FATAL: No boards fetched')
        sys.exit(1)

    # Phase 2: Fetch stocks for each board
    log(f'Phase 2: fetching stocks for {len(boards)} boards...')
    stock_mappings = []
    board_stock_counts = defaultdict(int)
    errors = 0
    t_start = time.time()

    for i, (bk, bn, ctype) in enumerate(boards):
        try:
            url = f'https://push2.eastmoney.com/api/qt/clist/get?fs=b:{bk}&fid=f3&po=1&pz=500&np=1&fltt=2&invt=2&fields=f12,f14'
            r = api_get(url)
            items = r.get('data',{}).get('diff',[]) or []
            for it in items:
                scode = it['f12']
                if len(scode) > 6 and scode[1] == '.':
                    scode = scode[2:]
                sname = it.get('f14','')
                stock_mappings.append((bk, scode, bn, sname, ctype))
                board_stock_counts[bk] += 1
        except Exception as e:
            errors += 1

        if (i+1) % 50 == 0 or i == len(boards)-1:
            e = time.time()-t_start
            r = (i+1)/max(e,0.01)
            eta = (len(boards)-i-1)/max(r,0.01)/60
            log(f'  [{i+1}/{len(boards)}] mappings={len(stock_mappings)} errs={errors} {r:.1f}/s ETA:{eta:.0f}m')
        time.sleep(0.15)

    elapsed = time.time()-t_start
    log(f'Phase 2 done: {elapsed/60:.1f}min mappings={len(stock_mappings)} errors={errors}')

    # Phase 3: Write to DB
    log(f'Phase 3: inserting {len(boards)} boards...')
    brds = [(bk, bn, ctype, board_stock_counts.get(bk,0)) for bk, bn, ctype in boards]
    execute_values(cur, """INSERT INTO concept_boards(concept_code,concept_name,concept_type,stock_count) VALUES %s ON CONFLICT(concept_code) DO UPDATE SET concept_name=EXCLUDED.concept_name,concept_type=EXCLUDED.concept_type,stock_count=EXCLUDED.stock_count,updated_at=NOW()""", brds, page_size=200)
    conn.commit(); log(f'  Boards done')

    counts = defaultdict(int)
    for ctype in set(c for _,_,_,_,c in stock_mappings):
        counts[ctype] = sum(1 for _,_,_,_,c in stock_mappings if c == ctype)
    log(f'  Mapping type distribution: {dict(counts)}')

    log(f'Phase 3: inserting {len(stock_mappings)} mappings...')
    B = 500
    for s in range(0, len(stock_mappings), B):
        batch = [(code, bk, bn, ctype, sname) for bk,code,bn,sname,ctype in stock_mappings[s:s+B]]
        execute_values(cur, """INSERT INTO stock_concepts(code,concept_code,concept_name,concept_type,stock_name) VALUES %s ON CONFLICT(code,concept_code) DO UPDATE SET concept_name=EXCLUDED.concept_name,concept_type=EXCLUDED.concept_type,stock_name=EXCLUDED.stock_name,updated_at=NOW()""", batch, page_size=200)
        conn.commit()
        if (s//B+1) % 20 == 0: log(f'  Mappings: {min(s+B,len(stock_mappings))}/{len(stock_mappings)}')
    log('  Mappings done')

    # Summary
    cur.execute("SELECT concept_type, COUNT(*) boards, SUM(stock_count) total FROM concept_boards GROUP BY concept_type ORDER BY COUNT(*) DESC")
    log('\n=== FINAL SUMMARY ===')
    log(f'{"TYPE":<20} {"BOARDS":>7} {"STOCKS":>10}')
    log('-' * 39)
    for row in cur.fetchall(): log(f'{row[0]:<20} {row[1]:>7} {row[2]:>10}')
    cur.execute("SELECT COUNT(*), COUNT(DISTINCT code) FROM stock_concepts")
    rr = cur.fetchone()
    log(f'\nTotal stock_concepts: {rr[0]} mappings, {rr[1]} unique stocks')
    cur.close(); conn.close()
    log(f'\n=== FINISHED in {elapsed/60:.1f}min ===')

except Exception as e:
    log(f'FATAL: {e}')
    log(traceback.format_exc())
    sys.exit(1)
