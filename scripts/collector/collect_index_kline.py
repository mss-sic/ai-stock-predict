#!/usr/bin/env python3
"""
大盘指数 + 国债ETF 日K线采集 — 腾讯前复权API
索引用 IDX 前缀避免与个股代码冲突 (000001=平安银行 vs IDX000001=上证指数)

目标: IDX000001/IDX000300/IDX000852/IDX399001/IDX399006
国债: 511010/511090/511520 (正常代码，无冲突)

用法:
  python3 collect_index_kline.py              # 增量
  python3 collect_index_kline.py --backfill   # 回填3年
"""
import os, sys, json, ssl, urllib.request
from datetime import date
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

INDEX_CODES = {
    "IDX000001": "上证指数", "IDX000300": "沪深300", "IDX000852": "中证1000",
    "IDX399001": "深证成指", "IDX399006": "创业板指",
}
# Map prefixed codes to Tencent API parameters
CODE_TO_API = {
    "IDX000001": "sh000001", "IDX000300": "sh000300",
    "IDX000852": "sh000852", "IDX399001": "sz399001", "IDX399006": "sz399006",
}
BOND_ETF = {
    "511010": "国债ETF", "511090": "30年国债ETF", "511520": "政金债ETF",
}
ALL_CODES = {**INDEX_CODES, **BOND_ETF}

UPSERT_SQL = """
    INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount, turnover_rate)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
        close = EXCLUDED.close, volume = EXCLUDED.volume, amount = EXCLUDED.amount,
        turnover_rate = EXCLUDED.turnover_rate
"""

def fetch_kline(api_code, days=750):
    url = f"http://ifzq.gtimg.cn/appstock/app/fqkline/get?param={api_code},day,,,{days},qfq"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=15, context=ctx) as resp:
            data = json.loads(resp.read().decode())
        result = data.get("data", {})
        return result.get(api_code, {}).get("qfqday", []) or \
               result.get(api_code, {}).get("day", []) or []
    except Exception as e:
        print(f"  ⚠️ fetch error for {api_code}: {e}", flush=True)
        return []

def main():
    backfill = "--backfill" in sys.argv
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    for code, name in ALL_CODES.items():
        api_code = CODE_TO_API.get(code, code)  # bond ETFs use raw code
        # For bond ETFs, determine prefix
        if code.startswith("5"):
            api_code = f"sh{code}" if code.startswith(("6","9","5")) else f"sz{code}"
        
        if backfill:
            days = 750
        else:
            cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = %s", (code,))
            row = cur.fetchone()
            if row and row[0]:
                missing = (date.today() - row[0]).days
                days = max(5, missing + 10)
            else:
                days = 750

        print(f"采集 {name}({code}) ... api={api_code} days={days}", flush=True)
        klines = fetch_kline(api_code, days=days)

        rows = []
        for row_data in klines:
            if len(row_data) < 6:
                continue
            vol_shou = float(row_data[5])
            vol_gu = int(vol_shou * 100)
            close_p = float(row_data[2])
            amt = close_p * float(vol_gu)
            rows.append((
                code,
                row_data[0],
                float(row_data[1]), float(row_data[3]), float(row_data[4]),
                close_p, vol_gu, amt, 0.0,
            ))

        if rows:
            execute_values(cur, UPSERT_SQL, rows, page_size=200)
            conn.commit()
            print(f"  ✅ {name}: {len(rows)} records", flush=True)
        else:
            print(f"  ⚠️ {name}: no data", flush=True)

    cur.close(); conn.close()
    print("✅ index/bond K-line collection complete", flush=True)

if __name__ == "__main__":
    main()
