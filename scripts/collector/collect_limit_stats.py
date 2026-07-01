#!/usr/bin/env python3
"""预计算每日涨跌停统计，写入 limit_stats_daily 表。
数据源: stocks_daily_k + stocks_basic (PostgreSQL)
用法:
  增量: python3 collect_limit_stats.py              # 计算最新交易日
  回填: python3 collect_limit_stats.py --backfill    # 回填所有历史日期
  指定: python3 collect_limit_stats.py --date 2026-06-30
"""
import os, sys, argparse
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

SQL_COMPUTE_ONE_DAY = """
    WITH prices AS (
        SELECT k.code, b.board_type, COALESCE(b.is_st, false) as is_st,
            kp.close as prev_close, k.close as close_price, k.high,
            CASE
                WHEN b.board_type IN ('kc','cy') THEN ROUND(kp.close * 1.20::numeric, 2)
                WHEN b.board_type = 'bj' THEN ROUND(kp.close * 1.30::numeric, 2)
                WHEN COALESCE(b.is_st, false) THEN ROUND(kp.close * 1.05::numeric, 2)
                ELSE ROUND(kp.close * 1.10::numeric, 2)
            END as limit_up_price,
            CASE
                WHEN b.board_type IN ('kc','cy') THEN ROUND(kp.close * 0.80::numeric, 2)
                WHEN b.board_type = 'bj' THEN ROUND(kp.close * 0.70::numeric, 2)
                WHEN COALESCE(b.is_st, false) THEN ROUND(kp.close * 0.95::numeric, 2)
                ELSE ROUND(kp.close * 0.90::numeric, 2)
            END as limit_down_price
        FROM stocks_daily_k k
        JOIN LATERAL (
            SELECT k2.close FROM stocks_daily_k k2
            WHERE k2.code = k.code AND k2.trade_date < k.trade_date
            ORDER BY k2.trade_date DESC LIMIT 1
        ) kp ON TRUE
        JOIN stocks_basic b ON b.code = k.code
        WHERE k.trade_date = %s
          AND k.close > 0 AND kp.close > 0
    )
    SELECT
        COUNT(*) FILTER (WHERE close_price = limit_up_price) as up_count,
        COUNT(*) FILTER (WHERE close_price = limit_down_price) as down_count,
        COUNT(*) FILTER (WHERE close_price > prev_close) as rise_count,
        COUNT(*) FILTER (WHERE close_price < prev_close) as fall_count,
        SUM(
            CASE
                WHEN close_price = limit_up_price
                    AND (high - close_price) / NULLIF(high, 0) > 0.02 THEN 1
                ELSE 0
            END
        ) as board_break,
        COUNT(*) as total_stocks
    FROM prices
"""

SQL_UPSERT = """
    INSERT INTO limit_stats_daily (trade_date, up_count, down_count, rise_count, fall_count, board_break, total_stocks)
    VALUES (%s, %s, %s, %s, %s, %s, %s)
    ON CONFLICT (trade_date) DO UPDATE SET
        up_count = EXCLUDED.up_count,
        down_count = EXCLUDED.down_count,
        rise_count = EXCLUDED.rise_count,
        fall_count = EXCLUDED.fall_count,
        board_break = EXCLUDED.board_break,
        total_stocks = EXCLUDED.total_stocks,
        created_at = now()
"""

SQL_GET_DATES = """
    SELECT trade_date FROM market_daily_agg ORDER BY trade_date DESC LIMIT %s
"""


def compute_and_store(conn, trade_date):
    """Compute limit stats for one trading day and upsert into limit_stats_daily."""
    cur = conn.cursor()
    cur.execute(SQL_COMPUTE_ONE_DAY, (trade_date,))
    row = cur.fetchone()
    if not row or row[5] == 0:  # total_stocks == 0 means no data
        print(f"  [{trade_date}] 无数据，跳过")
        return False

    up_count, down_count, rise_count, fall_count, board_break, total_stocks = row
    cur.execute(SQL_UPSERT, (trade_date, up_count, down_count, rise_count, fall_count, board_break, total_stocks))
    conn.commit()
    print(f"  [{trade_date}] 涨停={up_count} 跌停={down_count} 涨={rise_count} 跌={fall_count} 炸板={board_break} 总数={total_stocks}")
    return True


def main():
    parser = argparse.ArgumentParser(description="预计算每日涨跌停统计")
    parser.add_argument("--backfill", action="store_true", help="回填所有历史日期")
    parser.add_argument("--date", type=str, help="指定日期 YYYY-MM-DD")
    parser.add_argument("--days", type=int, default=365, help="backfill 模式最大天数 (默认365)")
    parser.add_argument("--repair", action="store_true", help="修复模式（配合 --from/--to/--all）")
    parser.add_argument("--from", type=str, dest="from_date", help="修复起始日期 YYYY-MM-DD")
    parser.add_argument("--to", type=str, dest="to_date", help="修复结束日期 YYYY-MM-DD")
    parser.add_argument("--all", action="store_true", help="修复全部历史")
    args = parser.parse_args()

    conn = psycopg2.connect(PG_DSN)
    conn.autocommit = False

    if args.repair:
        if args.all:
            cur = conn.cursor()
            cur.execute("SELECT trade_date FROM market_daily_agg ORDER BY trade_date")
            dates = [r[0].strftime("%Y-%m-%d") for r in cur.fetchall()]
            print(f"修复全部: {len(dates)} 个交易日")
        elif args.from_date and args.to_date:
            cur = conn.cursor()
            cur.execute(
                "SELECT trade_date FROM market_daily_agg WHERE trade_date >= %s AND trade_date <= %s ORDER BY trade_date",
                (args.from_date, args.to_date)
            )
            dates = [r[0].strftime("%Y-%m-%d") for r in cur.fetchall()]
            print(f"修复区间: {args.from_date} ~ {args.to_date}, {len(dates)} 天")
        else:
            # Repair without range = repair last 60 days
            cur = conn.cursor()
            cur.execute(
                "SELECT trade_date FROM market_daily_agg ORDER BY trade_date DESC LIMIT 60"
            )
            dates = [r[0].strftime("%Y-%m-%d") for r in cur.fetchall()]
            print(f"修复最近: {len(dates)} 个交易日")
    elif args.date:
        dates = [args.date]
        print(f"计算指定日期: {args.date}")
    elif args.backfill:
        cur = conn.cursor()
        cur.execute(SQL_GET_DATES, (args.days,))
        dates = [r[0].strftime("%Y-%m-%d") for r in cur.fetchall()]
        print(f"回填模式: 最近 {len(dates)} 个交易日")
    else:
        # Default: latest trading day only
        cur = conn.cursor()
        cur.execute("SELECT MAX(trade_date) FROM market_daily_agg")
        latest = cur.fetchone()[0]
        dates = [latest.strftime("%Y-%m-%d")]
        print(f"增量模式: 最新交易日 {dates[0]}")

    success = 0
    for i, d in enumerate(dates):
        try:
            if compute_and_store(conn, d):
                success += 1
        except Exception as e:
            conn.rollback()
            print(f"  [{d}] 错误: {e}")
        if (i + 1) % 10 == 0:
            print(f"  进度: {i+1}/{len(dates)}, 成功={success}")

    conn.close()
    print(f"\n完成: {success}/{len(dates)} 天写入成功")


if __name__ == "__main__":
    main()
