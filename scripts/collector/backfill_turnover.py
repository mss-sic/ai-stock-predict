#!/usr/bin/env python3
"""回溯填充历史 K线换手率 = 成交量 / 总股本 × 100"""
import os, sys, time, psycopg2

os.environ['PYTHONUNBUFFERED'] = '1'
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 获取所有股票的总股本
    cur.execute("SELECT code, total_shares FROM stocks_basic WHERE total_shares > 0")
    shares = {r[0]: r[1] for r in cur.fetchall()}
    print(f"有总股本数据: {len(shares)} 只")

    # 更新 turnover_rate = volume / total_shares * 100
    updated = 0
    for code, ts in shares.items():
        cur.execute("""
            UPDATE stocks_daily_k 
            SET turnover_rate = ROUND(volume::numeric / %s * 100, 4)
            WHERE code = %s AND turnover_rate IS NULL AND volume > 0
        """, (ts, code))
        cnt = cur.rowcount
        if cnt > 0:
            updated += cnt

    conn.commit()
    print(f"✅ 已填充 {updated} 条历史换手率")

    # 验证
    cur.execute("SELECT COUNT(*) FROM stocks_daily_k WHERE turnover_rate IS NOT NULL")
    has = cur.fetchone()[0]
    cur.execute("SELECT COUNT(*) FROM stocks_daily_k")
    total = cur.fetchone()[0]
    print(f"换手率覆盖率: {has}/{total} ({100*has//total if total else 0}%)")

    # 采样
    cur.execute("""
        SELECT code, trade_date, volume, turnover_rate 
        FROM stocks_daily_k WHERE turnover_rate IS NOT NULL 
        ORDER BY trade_date DESC LIMIT 5
    """)
    for r in cur.fetchall():
        print(f"  {r[0]} {r[1]} vol={r[2]} turnover={r[3]:.2f}%")

    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
