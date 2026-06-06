#!/usr/bin/env python3
"""实时行情同步 — mootdx批量采集 → stock_quotes 表（快速版：纯批量行情，不逐股查finance）"""
import os, psycopg2, time
from datetime import datetime
os.environ['NO_PROXY'] = '*'

from mootdx.quotes import Quotes

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"

def main():
    client = Quotes.factory(market='std')
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("SELECT code FROM stocks_basic ORDER BY code")
    codes = [r[0] for r in cur.fetchall()]
    print(f"共 {len(codes)} 只股票")

    # 获取总股本用于计算换手率
    float_shares = {}
    try:
        cur.execute("SELECT code, total_shares FROM stocks_basic WHERE total_shares > 0")
        for code, ts in cur.fetchall():
            float_shares[code] = float(ts) if ts else 0
    except:
        pass

    now = datetime.now()
    total = 0
    start = time.time()

    for i in range(0, len(codes), 80):
        batch = codes[i:i+80]
        try:
            df = client.quotes(symbol=batch)
            for _, row in df.iterrows():
                code = str(row.get('code', '')).zfill(6)
                if code not in batch:
                    continue

                price = float(row.get('price', 0) or 0)
                open_p = float(row.get('open', 0) or 0)
                high = float(row.get('high', 0) or 0)
                low = float(row.get('low', 0) or 0)
                vol = int(float(row.get('volume', 0) or 0))
                amt = float(row.get('amount', 0) or 0)
                b_vol = int(float(row.get('b_vol', 0) or 0))
                s_vol = int(float(row.get('s_vol', 0) or 0))

                # Compute turnover from vol / float_shares
                turnover = 0.0
                if vol > 0 and code in float_shares and float_shares[code] > 0:
                    turnover = round(vol * 10000.0 / float_shares[code], 4)
                
                cur.execute("""
                    INSERT INTO stock_quotes (code, price, open, high, low, volume, amount, bid_vol, ask_vol, turnover, updated_at)
                    VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                    ON CONFLICT (code) DO UPDATE SET
                        price=EXCLUDED.price, open=EXCLUDED.open,
                        high=EXCLUDED.high, low=EXCLUDED.low,
                        volume=EXCLUDED.volume, amount=EXCLUDED.amount,
                        bid_vol=EXCLUDED.bid_vol, ask_vol=EXCLUDED.ask_vol,
                        turnover=EXCLUDED.turnover,
                        updated_at=EXCLUDED.updated_at
                """, (code, price, open_p, high, low, vol, amt, b_vol, s_vol, turnover, now))
                total += 1
        except Exception as e:
            print(f"  batch {i}: {e}")

        if (i + 80) % 500 == 0:
            print(f"  {min(i+80, len(codes))}/{len(codes)} | {total} 已写入 | {time.time()-start:.0f}s")

    conn.commit()

    cur.execute("SELECT COUNT(*) FROM stock_quotes WHERE volume > 0")
    active = cur.fetchone()[0]
    cur.execute("SELECT COUNT(*) FROM stock_quotes")
    total_all = cur.fetchone()[0]
    cur.close()
    conn.close()

    elapsed = time.time() - start
    print(f"✅ 行情同步完成: {total_all} 只 | 有成交: {active} | {elapsed:.0f}s")

if __name__ == "__main__":
    main()
