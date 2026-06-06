#!/usr/bin/env python3
"""增量采集日K线 — 腾讯API，仅拉取每个股票缺失的日期"""
import sys, time, os, json, psycopg2, urllib.request, ssl
from datetime import date, timedelta

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"

def fetch_kline(code, days=365):
    """Fetch K-line from腾讯"""
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

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Get all stocks with their latest K-line date
    cur.execute("""
        SELECT b.code, MAX(k.trade_date) as latest
        FROM stocks_basic b
        LEFT JOIN stocks_daily_k k ON b.code = k.code
        GROUP BY b.code
        ORDER BY latest NULLS FIRST
    """)
    stocks = [(r[0], r[1]) for r in cur.fetchall()]

    # Determine how many days to fetch
    today = date.today()
    has_k = sum(1 for _, d in stocks if d is not None)
    need = len(stocks) - has_k
    print(f"总计 {len(stocks)} 只 | 有K线 {has_k} 只 | 缺失 {need} 只")

    if need == 0:
        print("K线数据完整，跳过")
        cur.close()
        conn.close()
        return

    start = time.time()
    total_records = 0
    total_new = 0

    for i, (code, latest) in enumerate(stocks):
        if latest is None:
            # No data at all: fetch 60 days
            days_to_fetch = 60
        else:
            # Has data: only fetch newer days + some buffer
            missing_days = (today - latest.date()).days
            if missing_days <= 1:
                continue  # Already up to date
            days_to_fetch = missing_days + 5  # slight buffer

        klines = fetch_kline(code, days=days_to_fetch)
        new_for_stock = 0
        for row in klines:
            if len(row) < 6:
                continue
            try:
                # Compute amount (万元) and turnover_rate
                amt = float(row[2]) * float(row[5]) / 100  # 万元
                cur.execute("""
                    INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount)
                    VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
                    ON CONFLICT (code, trade_date) DO NOTHING
                """, (
                    code,
                    row[0],
                    float(row[1]),
                    float(row[3]),
                    float(row[4]),
                    float(row[2]),
                    int(float(row[5])),
                    amt,
                ))
                # Update turnover_rate if we have total_shares
                if cur.rowcount > 0:
                    try:
                        cur.execute("SELECT total_shares FROM stocks_basic WHERE code = %s", (code,))
                        ts = cur.fetchone()
                        if ts and ts[0] and ts[0] > 0:
                            vol = int(float(row[5]))
                            turnover = round(vol * 10000.0 / float(ts[0]), 4)
                            cur.execute("UPDATE stocks_daily_k SET turnover_rate = %s WHERE code = %s AND trade_date = %s", (turnover, code, row[0]))
                    except:
                        pass
                if cur.rowcount > 0:
                    new_for_stock += 1
                    total_records += 1
            except:
                pass

        if new_for_stock > 0:
            total_new += 1

        if (i + 1) % 100 == 0:
            elapsed = time.time() - start
            rate = (i + 1) / elapsed if elapsed > 0 else 0
            eta = (need - total_new - 1) / max(rate, 0.01)
            conn.commit()
            print(f"  {i+1}/{len(stocks)} | 新增{total_new}只 {total_records}条 | {rate:.1f}只/s | 剩余{eta:.0f}s", flush=True)

    conn.commit()
    cur.close()
    conn.close()

    elapsed = time.time() - start
    print(f"✅ K线: 新增{total_new}只 | {total_records}条 | {elapsed:.0f}s", flush=True)

if __name__ == "__main__":
    main()
