#!/usr/bin/env python3
"""
概念板块重建 — 东财 slist API 反向采集 (Phase 1:API扫描 + Phase 2:DB写入)
用法: python3 rebuild_concepts.py
监控: tail -f /tmp/rebuild_concepts.log
"""
import os, time, ssl, urllib.request, json, sys, traceback
import psycopg2
from psycopg2.extras import execute_values
from collections import defaultdict

LOG = '/tmp/rebuild_concepts.log'
def log(msg):
    with open(LOG, 'a') as f:
        f.write(f'{time.strftime("%H:%M:%S")} {msg}\n')
    print(msg, flush=True)

log('=== CONCEPT REBUILD START ===')
try:
    PG = os.environ.get('PG_DSN', 'host=localhost dbname=stock_predict user=stock password=stock123')
    conn = psycopg2.connect(PG)
    cur = conn.cursor()
    cur.execute("SELECT code, name FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
    stocks = cur.fetchall()
    cur.close()
    conn.close()
    total = len(stocks)
    log(f'STOCKS: {total} ~{total*0.4/60:.0f}min')
    
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    
    concept_map = {}
    stock_mappings = []
    concept_stock_count = defaultdict(int)
    errors = 0
    t_start = time.time()
    
    for i, (code, name) in enumerate(stocks):
        boards = []
        try:
            mkt = 1 if code.startswith('6') else 0
            url = f'https://push2.eastmoney.com/api/qt/slist/get?fltt=2&invt=2&secid={mkt}.{code}&spt=3&pi=0&pz=200&po=1&fields=f12,f14'
            req = urllib.request.Request(url, headers={'User-Agent':'Mozilla/5.0','Referer':'https://quote.eastmoney.com/'})
            with urllib.request.urlopen(req, timeout=8, context=ctx) as resp:
                r = json.loads(resp.read().decode('utf-8'))
            diff = (r.get('data') or {}).get('diff') or {}
            items = diff.values() if isinstance(diff, dict) else diff
            boards = [(it.get('f12',''), it.get('f14','')) for it in items if it.get('f12')]
        except:
            errors += 1
        
        for bk, bn in boards:
            if bk not in concept_map:
                concept_map[bk] = bn
            stock_mappings.append((bk, code, bn, name))
            concept_stock_count[bk] += 1
        
        if (i+1) % 400 == 0:
            e = time.time()-t_start
            r = (i+1)/max(e,0.1)
            log(f'PROG: {i+1}/{total} concepts={len(concept_map)} mappings={len(stock_mappings)} {r:.1f}/s ETA:{(total-i-1)/max(r,0.01)/60:.0f}m')
        time.sleep(0.35)
    
    elapsed = time.time()-t_start
    log(f'SCAN DONE: {elapsed/60:.1f}min concepts={len(concept_map)} mappings={len(stock_mappings)} errors={errors}')
    
    # Phase 2: DB
    log('DB: clearing old data...')
    conn = psycopg2.connect(PG)
    cur = conn.cursor()
    cur.execute("DELETE FROM stock_concepts WHERE concept_type='concept'")
    cur.execute("DELETE FROM concept_boards WHERE concept_type='concept'")
    conn.commit()
    log('DB: cleared')
    
    log(f'DB: inserting {len(stock_mappings)} stock_concepts...')
    B = 500
    for s in range(0, len(stock_mappings), B):
        batch = [(c, bk, bn, 'concept', n) for bk, c, bn, n in stock_mappings[s:s+B]]
        execute_values(cur, "INSERT INTO stock_concepts (code,concept_code,concept_name,concept_type,stock_name) VALUES %s ON CONFLICT (code,concept_code) DO UPDATE SET concept_name=EXCLUDED.concept_name,stock_name=EXCLUDED.stock_name,updated_at=NOW()", batch, page_size=200)
        conn.commit()
    log(f'DB: {len(stock_mappings)} mappings inserted')
    
    log(f'DB: inserting {len(concept_map)} concept_boards...')
    brds = [(bk, bn, 'concept', concept_stock_count.get(bk,0)) for bk, bn in concept_map.items()]
    execute_values(cur, "INSERT INTO concept_boards (concept_code,concept_name,concept_type,stock_count) VALUES %s ON CONFLICT (concept_code) DO UPDATE SET concept_name=EXCLUDED.concept_name,stock_count=EXCLUDED.stock_count,updated_at=NOW()", brds, page_size=200)
    conn.commit()
    
    cur.execute("SELECT COUNT(*), COUNT(DISTINCT code) FROM stock_concepts WHERE concept_type='concept'")
    rr = cur.fetchone()
    log(f'DONE: {len(concept_map)} concepts, {rr[0]} mappings, {rr[1]} stocks, {elapsed/60:.1f}min total')
    
    cur.execute("SELECT concept_name,stock_count FROM concept_boards WHERE concept_type='concept' ORDER BY stock_count DESC LIMIT 15")
    log('TOP CONCEPTS:')
    for row in cur.fetchall():
        log(f'  {row[0]}: {row[1]}')
    
    cur.close()
    conn.close()
    log('=== FINISHED ===')
except Exception as e:
    log(f'FATAL: {e}')
    log(traceback.format_exc())
