#!/usr/bin/env python3
"""
个股主力资金流采集 — 东财 push2his 日级资金流 API
数据源: https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get
存储: stock_capital_flow (PG)

返回字段: date, main_net(主力净流入), small_net, mid_net, large_net, super_net
单位: 元 → 存入PG时转换为万元

⚠️ 东财防封: 串行请求，每只股票间隔 ≥1.5s

用法:
  python3 collect_capital_flow.py              # 增量（最近120天，全量股票）
  python3 collect_capital_flow.py --days 60    # 最近60天
  python3 collect_capital_flow.py 600519       # 单只股票
"""
import os, sys, time, json
from datetime import date, timedelta
import requests
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
PUSH2_URL = "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"

# 东财防封: 请求最小间隔
MIN_INTERVAL = float(os.environ.get("EM_MIN_INTERVAL", "1.5"))

UPSERT_SQL = """
    INSERT INTO stock_capital_flow (code, trade_date, main_net, super_large_net, large_net, medium_net, small_net)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        main_net = EXCLUDED.main_net, super_large_net = EXCLUDED.super_large_net,
        large_net = EXCLUDED.large_net, medium_net = EXCLUDED.medium_net,
        small_net = EXCLUDED.small_net
"""

# Rate limiting
_last_request = 0

def rate_limit():
    global _last_request
    elapsed = time.time() - _last_request
    if elapsed < MIN_INTERVAL:
        time.sleep(MIN_INTERVAL - elapsed + (time.time() % 1) * 0.3)
    _last_request = time.time()

def fetch_fund_flow(code):
    """Fetch daily fund flow for a single stock. Returns list of dicts."""
    market_code = 1 if code.startswith("6") else 0
    params = {
        "secid": f"{market_code}.{code}",
        "fields1": "f1,f2,f3,f7",
        "fields2": "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65",
        "lmt": "120",
    }
    headers = {
        "User-Agent": UA,
        "Referer": "https://quote.eastmoney.com/",
        "Origin": "https://quote.eastmoney.com",
    }
    try:
        rate_limit()
        r = requests.get(PUSH2_URL, params=params, headers=headers, timeout=15)
        d = r.json()
        klines = d.get("data", {}).get("klines", []) or []
    except Exception as e:
        print(f"  ⚠️ push2his {code} 失败: {e}", flush=True)
        return []

    rows = []
    for line in klines:
        parts = line.split(",")
        if len(parts) >= 6:
            try:
                rows.append({
                    "date": parts[0],
                    "main_net": float(parts[1]) if parts[1] != "-" else 0,
                    "small_net": float(parts[2]) if parts[2] != "-" else 0,
                    "mid_net": float(parts[3]) if parts[3] != "-" else 0,
                    "large_net": float(parts[4]) if parts[4] != "-" else 0,
                    "super_net": float(parts[5]) if parts[5] != "-" else 0,
                })
            except (ValueError, IndexError):
                continue
    return rows

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Determine target codes
    if len(sys.argv) > 1 and sys.argv[1].isdigit() and len(sys.argv[1]) == 6:
        codes = [sys.argv[1]]
        print(f"单股模式: {codes[0]}", flush=True)
    else:
        cur.execute("SELECT code FROM stocks_basic ORDER BY code")
        codes = [r[0] for r in cur.fetchall()]
        print(f"全量模式: {len(codes)} 只股票", flush=True)

    total_new = 0
    start_ts = time.time()

    for idx, code in enumerate(codes):
        flows = fetch_fund_flow(code)
        if not flows:
            continue

        db_rows = []
        for f in flows:
            db_rows.append((
                code, f["date"],
                f["main_net"] / 1e4,       # 元→万元
                f["super_net"] / 1e4,
                f["large_net"] / 1e4,
                f["mid_net"] / 1e4,
                f["small_net"] / 1e4,
            ))

        if db_rows:
            execute_values(cur, UPSERT_SQL, db_rows, page_size=100)
            total_new += len(db_rows)

        if (idx + 1) % 100 == 0:
            conn.commit()
            elapsed = time.time() - start_ts
            print(f"  进度: {idx+1}/{len(codes)} | {total_new} records | {elapsed:.0f}s | {elapsed/(idx+1):.1f}s/stock", flush=True)

    conn.commit()

    # Stats
    cur.execute("SELECT COUNT(*), COUNT(DISTINCT code), MIN(trade_date), MAX(trade_date) FROM stock_capital_flow")
    cnt, stocks, dmin, dmax = cur.fetchone()
    cur.close()
    conn.close()

    elapsed = time.time() - start_ts
    print(f"\n✅ 资金流采集完成: {total_new} 条新增 | {elapsed:.0f}s", flush=True)
    print(f"   PG: {cnt} records, {stocks} stocks, {dmin} ~ {dmax}", flush=True)

if __name__ == "__main__":
    main()
