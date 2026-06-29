#!/usr/bin/env python3
"""
个股资金流向采集 — 120日日级主力/大单/中单/小单净流入
数据源: 东财 push2his (零鉴权)
用法: python3 collect_fund_flow.py [--sample | CODE]  (默认: 全市场采集)
"""
import os, sys, time, json
import psycopg2
from psycopg2.extras import execute_values
import urllib.request

SAMPLE_CODES = ['600519','601318','688017','000001','002475','300750','600036','688981']
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

def fetch_fund_flow(code):
    market = 1 if code.startswith("6") else 0
    url = "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"
    params = {
        "secid": f"{market}.{code}",
        "fields1": "f1,f2,f3,f7",
        "fields2": "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65",
        "lmt": "120",
    }
    headers = {
        "User-Agent": UA,
        "Referer": "https://quote.eastmoney.com/",
        "Origin": "https://quote.eastmoney.com",
    }
    req = urllib.request.Request(f"{url}?{urllib.parse.urlencode(params)}", headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            d = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        return None

    klines = d.get("data", {}).get("klines", [])
    if not klines:
        return []

    rows = []
    for line in klines:
        parts = line.split(",")
        if len(parts) >= 7:
            rows.append((
                code,
                parts[0],  # date
                round(float(parts[1]) / 10000, 2) if parts[1] != "-" else 0,  # main_net (万元)
                round(float(parts[2]) / 10000, 2) if parts[2] != "-" else 0,  # small_net
                round(float(parts[3]) / 10000, 2) if parts[3] != "-" else 0,  # mid_net
                round(float(parts[4]) / 10000, 2) if parts[4] != "-" else 0,  # large_net
                round(float(parts[5]) / 10000, 2) if parts[5] != "-" else 0,  # super_net
            ))
    return rows

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[资金流] 样本模式: {len(codes)} 只")
    elif len(sys.argv) > 1:
        codes = [sys.argv[1]]
        print(f"[资金流] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[资金流] 全量模式: {len(codes)} 只")

    total, skip, errors = 0, 0, 0
    for i, code in enumerate(codes):
        if (i + 1) % 100 == 0:
            print(f"[资金流] 进度: {i+1}/{len(codes)} (新增 {total}, 跳过 {skip}, 错误 {errors})")
        try:
            rows = fetch_fund_flow(code)
            if not rows:
                skip += 1
                continue
            for r in rows:
                cur.execute("""
                    INSERT INTO stock_fund_flow (code, trade_date, main_net, small_net, mid_net, large_net, super_net)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (code, trade_date) DO UPDATE SET
                        main_net = EXCLUDED.main_net, small_net = EXCLUDED.small_net,
                        mid_net = EXCLUDED.mid_net, large_net = EXCLUDED.large_net,
                        super_net = EXCLUDED.super_net
                """, r)
            conn.commit()
            total += len(rows)
        except Exception as e:
            errors += 1
            conn.rollback()
            continue
        time.sleep(0.15)

    cur.close()
    conn.close()
    print(f"[资金流] 采集完成: 新增 {total} 条, 跳过 {skip} 只, 错误 {errors} 只")
    print(f"STAT:records_new={total},records_skip={skip},records_err={errors},fund_flow_new={total}", flush=True)

if __name__ == "__main__":
    main()
