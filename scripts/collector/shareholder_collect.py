#!/usr/bin/env python3
"""股东数据采集 — 多期股东户数 + 十大股东（按报告期分组，含环比对比）"""
import os, sys, json, time, random, psycopg2, requests
from datetime import datetime, timedelta
from collections import defaultdict

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
DATACENTER_URL = "https://datacenter-web.eastmoney.com/api/data/v1/get"

EM_SESSION = requests.Session()
EM_SESSION.headers.update({"User-Agent": UA})
EM_MIN_INTERVAL = 1.0
_em_last_call = [0.0]

def em_get(url, params=None, headers=None, timeout=15, **kwargs):
    wait = EM_MIN_INTERVAL - (time.time() - _em_last_call[0])
    if wait > 0:
        time.sleep(wait + random.uniform(0.1, 0.5))
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

def fetch_holder_history(code, periods=12):
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
            })
    except Exception as e:
        print(f"  holder history {code} error: {e}", flush=True)
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
            page_size=120,
            sort_columns="END_DATE,HOLDER_RANK",
            sort_types="-1,1"
        )
        # Group by report date
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
                'state': row.get('HOLDER_STATE', '') or '',
            })
        return by_date
    except Exception as e:
        print(f"  top holders {code} error: {e}", flush=True)
        return {}

def compare_holders(current, previous):
    """Add comparison info: 新进/增持/减持/不变, and find 退出 holders"""
    prev_names = {h['name']: h for h in previous}
    prev_set = set(prev_names.keys())
    curr_set = {h['name'] for h in current}
    
    enriched = []
    exited = []
    
    for h in current:
        entry = dict(h)
        if h['name'] not in prev_set:
            entry['trend'] = '新进'
        elif h['change'] == '不变':
            entry['trend'] = '不变'
        elif h['change'] == '新进':
            entry['trend'] = '新进'
        else:
            # Numeric change - check direction
            try:
                chg = float(str(h['change']).replace(',', ''))
                if chg > 0:
                    entry['trend'] = '增持'
                elif chg < 0:
                    entry['trend'] = '减持'
                else:
                    entry['trend'] = '不变'
            except:
                entry['trend'] = '不变'
        enriched.append(entry)
    
    # Find exited holders
    for name, h in prev_names.items():
        if name not in curr_set:
            exited.append(dict(h, trend='退出'))
    
    return enriched, exited

def store_data(conn, cur, code, history, top_holders_by_date, exited_by_date):
    """Store holder count history and top10 per period"""
    total_rows = 0
    for h in history:
        dt = h['reportDate']
        top10 = top_holders_by_date.get(dt, [])
        exited = exited_by_date.get(dt, [])
        
        cur.execute("""
            INSERT INTO stock_shareholders (code, report_date, total_holders, holder_change, top10_holders, top10_float, inst_hold_ratio, avg_holding)
            VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
            ON CONFLICT (code, report_date) DO UPDATE SET
                total_holders=EXCLUDED.total_holders, holder_change=EXCLUDED.holder_change,
                avg_holding=EXCLUDED.avg_holding,
                top10_holders=EXCLUDED.top10_holders
        """, (code, dt, h['totalHolders'], h['holderChange'],
              json.dumps(top10, ensure_ascii=False), json.dumps(exited, ensure_ascii=False), 0, h['avgHolding']))
        total_rows += 1
    return total_rows

def main():
    codes_arg = sys.argv[1] if len(sys.argv) > 1 else ''
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    if codes_arg:
        codes = [c.strip() for c in codes_arg.split(',') if c.strip()]
    else:
        # 增量策略: 优先无数据股票，再拉取报告期超3个月的
        cur.execute("""
            SELECT b.code FROM stocks_basic b
            LEFT JOIN stock_shareholders s ON b.code = s.code
            WHERE s.code IS NULL
            ORDER BY b.code LIMIT 300
        """)
        codes_new = [r[0] for r in cur.fetchall()]

        cur.execute("""
            SELECT b.code FROM stocks_basic b
            INNER JOIN (
                SELECT code, MAX(report_date) as latest FROM stock_shareholders GROUP BY code
            ) s ON b.code = s.code
            WHERE s.latest < TO_CHAR(CURRENT_DATE - INTERVAL '3 months', 'YYYY-MM-DD')
            ORDER BY s.latest ASC
            LIMIT 200
        """)
        codes_stale = [r[0] for r in cur.fetchall()]

        codes = list(dict.fromkeys(codes_new + codes_stale))
    
    if not codes:
        print("股东数据已是最新", flush=True)
        return
    
    print(f"采集股东数据: {len(codes)} 只", flush=True)
    done, total_rows = 0, 0
    start = time.time()
    
    for i, code in enumerate(codes):
        try:
            history = fetch_holder_history(code, 12)
            if not history:
                continue
            
            # Get the report dates from history
            report_dates = [h['reportDate'] for h in history]
            
            # Fetch all top holders
            all_top = fetch_top_holders_all(code)
            
            # Match top holders only to quarterly dates (avoid duplicating across nearby dates)
            top_by_date = {}
            exited_by_date = {}
            sorted_td = sorted(all_top.keys())
            
            if sorted_td:
                # Build quarterly top holders with comparison
                for idx, td in enumerate(sorted_td):
                    prev_q = sorted_td[idx - 1] if idx > 0 else None
                    prev_holders = all_top.get(prev_q, [])
                    enriched, exited = compare_holders(all_top[td], prev_holders)
                    top_by_date[td] = enriched
                    exited_by_date[td] = exited
                
                # Only attach top10 to history dates that match quarterly reports
                # A date "matches" if it's within 45 days after the quarterly date
                for rd in report_dates:
                    for td in sorted_td:
                        try:
                            dt_rd = datetime.strptime(rd, '%Y-%m-%d')
                            dt_td = datetime.strptime(td, '%Y-%m-%d')
                            if dt_td <= dt_rd <= dt_td + timedelta(days=15):
                                top_by_date[rd] = top_by_date.get(td, [])
                                exited_by_date[rd] = exited_by_date.get(td, [])
                                break
                        except:
                            pass
            
            total_rows += store_data(conn, cur, code, history, top_by_date, exited_by_date)
            done += 1
        except Exception as e:
            print(f"  {code} error: {e}", flush=True)
        
        if (i + 1) % 50 == 0:
            conn.commit()
            elapsed = time.time() - start
            print(f"  {i+1}/{len(codes)} | {done} stocks | {total_rows} rows | {elapsed:.0f}s", flush=True)
        time.sleep(0.15)
    
    conn.commit()
    cur.close()
    conn.close()
    print(f"✅ 股东数据: {done} stocks, {total_rows} rows | {time.time()-start:.0f}s")

if __name__ == "__main__":
    main()
