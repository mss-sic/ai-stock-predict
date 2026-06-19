#!/usr/bin/env python3
"""
北向资金采集 — 同花顺 hsgtApi (分钟级存储)
数据源：https://data.hexin.cn/market/hsgtApi/method/dayChart/
存储：northbound_minute (PG，分钟级累计净买入，亿)
日聚合：northbound_daily_view (VIEW，取收盘时刻)

策略：同花顺 API 始终返回最近交易日的分钟数据。
     非交易日调用 → 自动识别为上一交易日 → 回填历史缺口。

用法:
  python3 collect_northbound.py              # 拉取并自动识别交易日
"""
import os, sys, json
import requests
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

HSGT_HEADERS = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/117.0.0.0 Safari/537.36",
    "Host": "data.hexin.cn",
    "Referer": "https://data.hexin.cn/",
}
HSGT_URL = "https://data.hexin.cn/market/hsgtApi/method/dayChart/"

UPSERT_SQL = """
    INSERT INTO northbound_minute (trade_date, time, hgt_cumulative, sgt_cumulative)
    VALUES %s
    ON CONFLICT (trade_date, time) DO NOTHING
"""

def get_latest_trading_day(cur):
    """Get the most recent trading day from stocks_daily_k."""
    cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k")
    r = cur.fetchone()
    return str(r[0]) if r and r[0] else None

def fetch_minute_data():
    """Fetch minute-level northbound flow from 同花顺 API.
    Returns [(time, hgt_cumulative, sgt_cumulative), ...] or [].
    API always returns the latest trading day's data, even on non-trading days.
    """
    try:
        r = requests.get(HSGT_URL, headers=HSGT_HEADERS, timeout=10)
        d = r.json()
        times = d.get("time", [])
        hgt = d.get("hgt", [])
        sgt = d.get("sgt", [])
        
        if not times or not hgt or not sgt:
            return []
        
        rows = []
        n = len(times)
        for i in range(n):
            t = times[i] if isinstance(times[i], str) else str(times[i])
            h = float(hgt[i]) if i < len(hgt) and hgt[i] is not None else None
            s = float(sgt[i]) if i < len(sgt) and sgt[i] is not None else None
            if h is not None and s is not None:
                rows.append((t, h, s))
        return rows
    except Exception as e:
        print(f"  ⚠️ 同花顺北向API失败: {e}", flush=True)
    return []

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Determine actual trading date from DB
    trade_date = get_latest_trading_day(cur)
    if not trade_date:
        print("❌ stocks_daily_k 无交易日数据", flush=True)
        return

    # Check if we already have data for this date
    cur.execute("SELECT COUNT(*) FROM northbound_minute WHERE trade_date = %s", (trade_date,))
    existing = cur.fetchone()[0]
    if existing > 0:
        print(f"  ✅ {trade_date} 已有 {existing} 条分钟数据，跳过", flush=True)
        cur.close(); conn.close()
        return

    print(f"📡 拉取北向数据 → 最新交易日 {trade_date} ...", flush=True)
    rows = fetch_minute_data()

    if not rows:
        print(f"  ⚠️ API 无数据返回", flush=True)
        cur.close(); conn.close()
        return

    # Store under the correct trading date
    db_rows = [(trade_date, t, h, s) for t, h, s in rows]
    execute_values(cur, UPSERT_SQL, db_rows, page_size=200)
    conn.commit()

    last = db_rows[-1]
    print(f"  ✅ {trade_date}: {len(db_rows)} 分钟点, "
          f"收盘 沪={last[2]:.2f}亿 深={last[3]:.2f}亿 合计={last[2]+last[3]:.2f}亿", flush=True)

    # Report stats
    cur.execute("SELECT COUNT(*), COUNT(DISTINCT trade_date), MIN(trade_date), MAX(trade_date) FROM northbound_minute")
    cnt, days, dmin, dmax = cur.fetchone()
    print(f"  📊 northbound_minute: {cnt} rows, {days} days, {dmin} ~ {dmax}", flush=True)

    cur.close()
    conn.close()
    print("✅ 北向资金采集完成", flush=True)

if __name__ == "__main__":
    main()
