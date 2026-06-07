#!/usr/bin/env python3
"""
股东数据全量回填脚本
从东方财富API采集所有A股的股东户数+十大股东历史数据
支持断点续传
"""
import os, sys, json, time, random, psycopg2, requests
from datetime import datetime, timedelta
from collections import defaultdict

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
DATACENTER_URL = "https://datacenter-web.eastmoney.com/api/data/v1/get"

EM_SESSION = requests.Session()
EM_SESSION.headers.update({"User-Agent": UA})
EM_MIN_INTERVAL = 0.8
_em_last_call = [0.0]

def em_get(url, params=None, headers=None, timeout=15, **kwargs):
    wait = EM_MIN_INTERVAL - (time.time() - _em_last_call[0])
    if wait > 0:
        time.sleep(wait + random.uniform(0.05, 0.3))
    try:
        return EM_SESSION.get(url, params=params, headers=headers, timeout=timeout, **kwargs)
    finally:
        _em_last_call[0] = time.time()

def eastmoney_datacenter(report_name, columns="ALL", filter_str="",
                          page_size=50, sort_columns="", sort_types="-1"):
    params = {
        "reportName": report_name, "columns": columns,
        "filter": filter_str, "pageNumber": "1", "pageSize": str(page_size),
        "sortColumns": sort_columns, "sortTypes": sort_types,
        "source": "WEB", "client": "WEB",
    }
    r = em_get(DATACENTER_URL, params=params, timeout=15)
    d = r.json()
    if d.get("result") and d["result"].get("data"):
        return d["result"]["data"]
    return []

def fetch_holder_history(code, periods=20):
    """Fetch multi-period shareholder count history"""
    records = []
    try:
        rows = eastmoney_datacenter(
            "RPT_F10_EH_HOLDERNUM",
            filter_str=f'(SECURITY_CODE="{code}")',
            page_size=periods,
            sort_columns="END_DATE", sort_types="-1"
        )
        for row in rows:
            end_date = str(row.get("END_DATE", ""))[:10]
            if not end_date:
                continue
            records.append({
                'reportDate': end_date,
                'totalHolders': int(row.get('HOLDER_TOTAL_NUM', 0) or 0),
                'holderChange': 0,
                'avgHolding': int(float(row.get('AVG_FREE_SHARES', 0) or 0)),
                'avgHoldingValue': float(row.get('AVG_HOLD_MARKET_CAP', 0) or 0),
            })
    except Exception as e:
        pass
    
    # Compute holder change
    for i in range(len(records)):
        if i + 1 < len(records) and records[i+1]['totalHolders'] > 0:
            records[i]['holderChange'] = round(
                (records[i]['totalHolders'] - records[i+1]['totalHolders']) 
                / records[i+1]['totalHolders'] * 100, 2
            )
    return records

def fetch_top_holders_all(code):
    """Fetch top 10 holders for ALL available quarters"""
    try:
        rows = eastmoney_datacenter(
            "RPT_F10_EH_HOLDERS",
            filter_str=f'(SECURITY_CODE="{code}")',
            page_size=200,
            sort_columns="END_DATE,HOLDER_RANK",
            sort_types="-1,1"
        )
        by_date = defaultdict(list)
        for row in rows:
            dt = str(row.get("END_DATE", ""))[:10]
            if not dt:
                continue
            by_date[dt].append({
                'rank': int(row.get('HOLDER_RANK', 99)),
                'name': row.get('HOLDER_NAME', ''),
                'shares': int(row.get('HOLD_NUM', 0) or 0),
                'ratio': float(row.get('HOLD_NUM_RATIO', 0) or 0),
                'change': row.get('HOLD_NUM_CHANGE', '') or '',
            })
        return by_date
    except:
        return {}

def compute_inst_ratio(top10):
    """Compute institutional holding ratio from top 10"""
    inst_types = ['基金', '证券', '保险', '信托', 'QFII', '社保', '银行', '资管', '私募']
    inst_ratio = 0.0
    for h in top10:
        name = h.get('name', '')
        if any(t in name for t in inst_types):
            inst_ratio += h.get('ratio', 0)
    return round(inst_ratio, 2)

def main():
    codes_arg = sys.argv[1] if len(sys.argv) > 1 else ''
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    if codes_arg:
        codes = [c.strip() for c in codes_arg.split(',') if c.strip()]
    else:
        cur.execute("""
            SELECT b.code FROM stocks_basic b
            ORDER BY b.code
        """)
        codes = [r[0] for r in cur.fetchall()]
    
    if not codes:
        print("没有需要采集的股票", flush=True)
        return
    
    cur.execute("SELECT COUNT(DISTINCT code) FROM stock_shareholders")
    existing = cur.fetchone()[0]
    
    print(f"全量股东回填: {len(codes)} 只 (已有 {existing} 只)", flush=True)
    done, new_stocks, total_rows = 0, 0, 0
    start = time.time()
    
    for i, code in enumerate(codes):
        try:
            # Check if already has data
            cur.execute("SELECT COUNT(*) FROM stock_shareholders WHERE code = %s", (code,))
            had_data = cur.fetchone()[0] > 0
            
            history = fetch_holder_history(code, 20)
            if not history:
                continue
            
            all_top = fetch_top_holders_all(code)
            
            if not had_data:
                new_stocks += 1
            
            for h in history:
                dt = h['reportDate']
                top10 = all_top.get(dt, [])
                inst_ratio = compute_inst_ratio(top10)
                
                cur.execute("""
                    INSERT INTO stock_shareholders (code, report_date, total_holders, holder_change, 
                        top10_holders, top10_float, inst_hold_ratio, avg_holding)
                    VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
                    ON CONFLICT (code, report_date) DO UPDATE SET
                        total_holders=EXCLUDED.total_holders, 
                        holder_change=EXCLUDED.holder_change,
                        avg_holding=EXCLUDED.avg_holding,
                        top10_holders=EXCLUDED.top10_holders,
                        inst_hold_ratio=EXCLUDED.inst_hold_ratio
                """, (code, dt, h['totalHolders'], h['holderChange'],
                      json.dumps(top10, ensure_ascii=False), 
                      json.dumps([], ensure_ascii=False), 
                      inst_ratio, h['avgHolding']))
                total_rows += 1
            
            done += 1
        except Exception as e:
            pass
        
        if (i + 1) % 100 == 0:
            conn.commit()
            elapsed = time.time() - start
            rate = (i+1) / elapsed if elapsed > 0 else 0
            eta = (len(codes) - i - 1) / rate if rate > 0 else 0
            eta_h = eta / 3600
            print(f"  {i+1}/{len(codes)} | 完成 {done} | 新增 {new_stocks} | {total_rows}条 | {elapsed:.0f}s | ETA {eta_h:.1f}h", flush=True)
        time.sleep(0.06)
    
    conn.commit()
    
    cur.execute("SELECT COUNT(DISTINCT code), COUNT(*) FROM stock_shareholders")
    total_codes, total_rows_final = cur.fetchone()
    
    cur.close()
    conn.close()
    elapsed = time.time() - start
    elapsed_h = elapsed / 3600
    print(f"✅ 股东回填完成: {total_codes} stocks, {total_rows_final} rows | {elapsed_h:.1f}h")

if __name__ == "__main__":
    main()
