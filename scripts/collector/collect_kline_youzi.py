#!/usr/bin/env python3
"""柚子大数据 K线采集 — 全市日K线一次拉取，写入 stocks_daily_k
数据源: http://39.98.238.239/api_stock_kline_daily_th/
覆盖: 沪深京三市全量个股 (含北交所)
字段: code, open, high, low, close, volume, amount, turnover_rate,
      high_limit, low_limit, avg_price, prev_close, is_paused, is_st, factor
"""
import os, sys, time, requests, psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

YOUZI_KEY = os.environ.get("YOUZI_API_KEY", "")
YOUZI_BASE = os.environ.get("YOUZI_BASE_URL", "http://39.98.238.239")
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

UPSERT_SQL = """
    INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount, turnover_rate, data_source)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
        close = EXCLUDED.close, volume = EXCLUDED.volume, amount = EXCLUDED.amount,
        turnover_rate = CASE WHEN stocks_daily_k.turnover_rate = 0 THEN EXCLUDED.turnover_rate
                             ELSE stocks_daily_k.turnover_rate END,
        data_source = 'youzi'
"""

def main():
    if not YOUZI_KEY:
        print("ERROR: YOUZI_API_KEY not set", flush=True)
        sys.exit(1)

    date_arg = sys.argv[1] if len(sys.argv) > 1 else None
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Determine date: CLI arg > yesterday
    if date_arg:
        fetch_date = date_arg
    else:
        from datetime import date, timedelta
        fetch_date = (date.today() - timedelta(days=1)).isoformat()

    print(f"[youzi_kline] fetching {fetch_date} ...", flush=True)

    t0 = time.time()
    params = {"key": YOUZI_KEY, "date": fetch_date}
    try:
        resp = requests.get(f"{YOUZI_BASE}/api_stock_kline_daily_th/", params=params, timeout=120)
        data = resp.json()
    except Exception as e:
        print(f"[youzi_kline] API error: {e}", flush=True)
        cur.close(); conn.close()
        sys.exit(1)

    if data.get("status") != "成功":
        print(f"[youzi_kline] API failed: {data}", flush=True)
        cur.close(); conn.close()
        sys.exit(1)

    rows_raw = data.get("datas") or []
    columns = data.get("columns") or []
    if not rows_raw:
        print(f"[youzi_kline] 0 records for {fetch_date}", flush=True)
        cur.close(); conn.close()
        sys.exit(0)

    # Build column index map
    ci = {c: i for i, c in enumerate(columns)}
    rows = []
    skipped = 0
    for row in rows_raw:
        code = str(row[ci['code']])
        # Skip non-stock codes (index, ETF, bond)
        if not code.isdigit() or len(code) != 6:
            skipped += 1
            continue

        try:
            trade_date = str(row[ci.get('date', ci.get('trade_date', ''))])[:10]
            open_p = float(row[ci['open']])
            high_p = float(row[ci['high']])
            low_p = float(row[ci['low']])
            close_p = float(row[ci['close']])
            volume = int(float(row[ci['volume']]))
            amount = float(row[ci['amount']])
            turnover = float(row[ci.get('turnover_rate', 0)] or 0)
        except (ValueError, KeyError, IndexError) as e:
            skipped += 1
            continue

        rows.append((code, trade_date, open_p, high_p, low_p, close_p, volume, amount, turnover))

    if not rows:
        print(f"[youzi_kline] all {len(rows_raw)} rows skipped", flush=True)
        cur.close(); conn.close()
        sys.exit(0)

    execute_values(cur, UPSERT_SQL, rows, page_size=500)
    conn.commit()

    elapsed = time.time() - t0
    # Count by board
    bj = sum(1 for r in rows if r[0].startswith(('8', '92')))
    sh = sum(1 for r in rows if r[0].startswith('6'))
    sz = sum(1 for r in rows if r[0].startswith(('0', '3')))
    print(f"[youzi_kline] {fetch_date}: {len(rows)} stocks (sh:{sh} sz:{sz} bj:{bj}) | {elapsed:.1f}s", flush=True)
    print(f"STAT:youzi_kline_records={len(rows)},youzi_kline_bj={bj}", flush=True)

    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
