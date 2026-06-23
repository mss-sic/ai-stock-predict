#!/usr/bin/env python3
"""
backfill_macd.py — 回填 stocks_daily_k 的 EMA12/EMA26/MACD_DIF/MACD_DEA 列

数据源: stocks_daily_k 的 close 价格
目标表: stocks_daily_k (PG)
使用真正的 EMA 递归算法（非 SMA）计算：

  EMA(12): α = 2/(12+1) = 2/13 ≈ 0.153846
  EMA(26): α = 2/(26+1) = 2/27 ≈ 0.074074
  DIF = EMA(12) - EMA(26)
  DEA = EMA(DIF, 9): α = 2/(9+1) = 0.2

用法:
  python3 backfill_macd.py                          # 处理所有股票
  python3 backfill_macd.py 000001 000002            # 处理指定股票
  python3 backfill_macd.py --chunk 1000             # 每批处理1000只股票
  python3 backfill_macd.py --since 2024-01-01       # 仅回填2024年至今的数据
"""

import os
import sys
import argparse
import psycopg2
import psycopg2.extras
import logging

logging.basicConfig(level=logging.INFO, format='%(asctime)s [backfill_macd] %(message)s')
log = logging.getLogger(__name__)

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
EMA12_ALPHA = 2.0 / 13.0   # ≈ 0.153846
EMA26_ALPHA = 2.0 / 27.0   # ≈ 0.074074
DEA_ALPHA   = 2.0 / 10.0   # = 0.2


def get_conn():
    return psycopg2.connect(PG_DSN)


def backfill_stock(conn, code, since=None):
    """Backfill EMA12/EMA26/MACD_DIF/MACD_DEA for a single stock."""
    cur = conn.cursor()
    query = """
        SELECT trade_date, close FROM stocks_daily_k
        WHERE code = %s
    """
    params = [code]
    if since:
        query += " AND trade_date >= %s"
        params.append(since)
    query += " ORDER BY trade_date ASC"

    cur.execute(query, params)
    rows = cur.fetchall()
    if len(rows) < 2:
        cur.close()
        return 0

    # Compute EMA recursively
    ema12 = float(rows[0][1])  # first close as seed
    ema26 = float(rows[0][1])
    dif = 0.0
    dea = 0.0
    is_first = True

    updates = []
    for trade_date, close_raw in rows:
        close = float(close_raw)
        if is_first:
            is_first = False
            dif = ema12 - ema26
            dea = dif
        else:
            ema12 = EMA12_ALPHA * close + (1 - EMA12_ALPHA) * ema12
            ema26 = EMA26_ALPHA * close + (1 - EMA26_ALPHA) * ema26
            dif = ema12 - ema26
            dea = DEA_ALPHA * dif + (1 - DEA_ALPHA) * dea
        updates.append((round(ema12, 4), round(ema26, 4), round(dif, 4), round(dea, 4), code, trade_date))

    # Batch update
    psycopg2.extras.execute_values(cur, """
        UPDATE stocks_daily_k SET
            ema12 = data.v1, ema26 = data.v2, macd_dif = data.v3, macd_dea = data.v4
        FROM (VALUES %s) AS data(v1, v2, v3, v4, code, trade_date)
        WHERE stocks_daily_k.code = data.code AND stocks_daily_k.trade_date = data.trade_date
    """, updates, page_size=1000)

    conn.commit()
    cur.close()
    return len(updates)


def main():
    parser = argparse.ArgumentParser(description='Backfill MACD EMA columns on stocks_daily_k')
    parser.add_argument('codes', nargs='*', help='Stock codes to process (default: all)')
    parser.add_argument('--chunk', type=int, default=1000, help='Commit every N stocks')
    parser.add_argument('--since', type=str, default=None, help='Only backfill data since date (YYYY-MM-DD)')
    args = parser.parse_args()

    conn = get_conn()

    if args.codes:
        codes = args.codes
    else:
        cur = conn.cursor()
        cur.execute("SELECT DISTINCT code FROM stocks_daily_k ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        cur.close()

    log.info(f"Processing {len(codes)} stocks, chunk={args.chunk}, since={args.since}")

    total_updated = 0
    for i, code in enumerate(codes):
        try:
            n = backfill_stock(conn, code, args.since)
            total_updated += n
        except Exception as e:
            log.error(f"Failed for {code}: {e}")
            conn.rollback()
            conn = get_conn()

        if (i + 1) % args.chunk == 0:
            log.info(f"Progress: {i+1}/{len(codes)} stocks, {total_updated} rows updated")

    conn.close()
    log.info(f"Done. {len(codes)} stocks, {total_updated} rows updated.")


if __name__ == '__main__':
    main()
