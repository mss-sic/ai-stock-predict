#!/usr/bin/env python3
"""增量采集日K线 — 腾讯前复权API，自动检测除权并修复历史数据"""
import os
import sys, time, os, json, psycopg2, urllib.request, ssl
from datetime import date, timedelta

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def fetch_kline(code, days=365):
    prefix = "sh" if code.startswith(("6", "9")) else "sz"
    url = f"https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param={prefix}{code},day,,,{days},qfq"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=15, context=ctx) as resp:
            data = json.loads(resp.read().decode())
        return data.get("data", {}).get(f"{prefix}{code}", {}).get("qfqday", []) or \
               data.get("data", {}).get(f"{prefix}{code}", {}).get("day", []) or []
    except:
        return []

def insert_kline(cur, code, row):
    """Insert or update a single kline record"""
    amt = float(row[2]) * float(row[5]) / 100
    cur.execute("""
        INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount)
        VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
        ON CONFLICT (code, trade_date) DO UPDATE SET
            open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
            close = EXCLUDED.close, volume = EXCLUDED.volume, amount = EXCLUDED.amount
    """, (code, row[0], float(row[1]), float(row[3]), float(row[4]),
          float(row[2]), int(float(row[5])), amt))
    return cur.rowcount

def detect_adjustment(cur, code):
    """Check if stock has had a 除权 event by comparing latest known close with API"""
    # Get last 3 days from DB
    cur.execute("""
        SELECT trade_date, close FROM stocks_daily_k 
        WHERE code = %s ORDER BY trade_date DESC LIMIT 3
    """, (code,))
    db_rows = {str(r[0]): float(r[1]) for r in cur.fetchall()}
    
    if len(db_rows) < 2:
        return False  # Not enough data to compare
    
    # Fetch last 5 days from API
    klines = fetch_kline(code, days=10)
    api_rows = {}
    for row in klines:
        if len(row) >= 3:
            api_rows[row[0]] = float(row[2])
    
    # Compare: if same date has different close → 除权 happened
    for db_date, db_close in db_rows.items():
        if db_date in api_rows:
            diff_pct = abs(api_rows[db_date] - db_close) / max(db_close, 0.01)
            if diff_pct > 0.005:  # >0.5% difference
                return True
    
    return False

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("""
        SELECT b.code, MAX(k.trade_date) as latest
        FROM stocks_basic b
        LEFT JOIN stocks_daily_k k ON b.code = k.code
        GROUP BY b.code
        ORDER BY latest NULLS FIRST
    """)
    stocks = [(r[0], r[1]) for r in cur.fetchall()]

    today = date.today()
    has_k = sum(1 for _, d in stocks if d is not None)
    need = len(stocks) - has_k
    print(f"📊 数据源: 腾讯财经 (web.ifzq.gtimg.cn) | 前复权(qfq)", flush=True)
    print(f"总计 {len(stocks)} 只 | 有K线 {has_k} 只 | 缺失 {need} 只", flush=True)

    start = time.time()
    total_records = 0
    total_new = 0
    total_adjusted = 0

    for i, (code, latest) in enumerate(stocks):
        if latest is None:
            days_to_fetch = 60
        else:
            missing = (today - latest.date()).days
            if missing <= 0:
                # Still check for 除权 adjustment every 7 days
                if i % 7 == 0 and detect_adjustment(cur, code):
                    # Full reload for adjusted stock
                    klines = fetch_kline(code, days=365)
                    updated = 0
                    for row in klines:
                        if len(row) >= 6:
                            insert_kline(cur, code, row)
                            updated += 1
                    total_adjusted += 1
                    if updated > 0:
                        total_records += updated
                    if total_adjusted % 10 == 0:
                        print(f"  [除权修复] {code} → 已更新 {updated} 条历史数据", flush=True)
                continue
            days_to_fetch = missing + 3

        klines = fetch_kline(code, days=days_to_fetch)
        new_for_stock = 0
        for row in klines:
            if len(row) < 6:
                continue
            insert_kline(cur, code, row)
            new_for_stock += 1
            total_records += 1

        if new_for_stock > 0:
            total_new += 1

        if (i + 1) % 200 == 0:
            elapsed = time.time() - start
            conn.commit()
            conn.commit()
            print(f"  📈 进度 {i+1}/{len(stocks)} | 更新{total_new}只股票 | 入库{total_records}条K线 | 除权修复{total_adjusted}只 | 耗时{elapsed:.0f}s", flush=True)

    conn.commit()
    elapsed = time.time() - start
    print(f"\n✅ K线采集完成: {total_new}只股票更新 | 共入库 {total_records} 条新数据 | 除权修复 {total_adjusted} 只 | 总耗时 {elapsed:.0f}s\n", flush=True)

    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
