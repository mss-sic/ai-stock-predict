#!/usr/bin/env python3
"""
交易日历同步 — 从 Tushare trade_cal API 全量同步

数据源: Tushare pro.trade_cal() 接口
目标表: trade_calendar (PG)
降级策略: 如果 Tushare 不可用, 从 stocks_daily_k 去重聚合推断交易日

用法:
  python3 sync_trade_calendar.py                  # 全量同步所有年份
  python3 sync_trade_calendar.py --year 2026       # 仅同步指定年份
  python3 sync_trade_calendar.py --infer           # 降级: 从 stocks_daily_k 推断
"""
import os
import sys
import argparse
import time
import psycopg2
from psycopg2.extras import execute_values
from datetime import date, datetime

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
TUSHARE_TOKEN = os.environ.get("TUSHARE_TOKEN", "")

def log(msg):
    print(msg, flush=True)


def sync_from_tushare(year=None):
    """从 Tushare trade_cal API 同步交易日历。"""
    if not TUSHARE_TOKEN:
        log("[交易日历] TUSHARE_TOKEN 未配置, 降级到 infer 模式")
        return sync_from_db_infer()

    import tushare as ts
    pro = ts.pro_api(TUSHARE_TOKEN)

    if year:
        start = f"{year}0101"
        end   = f"{year}1231"
    else:
        start = "19900101"
        end   = f"{date.today().year + 1}1231"

    log(f"[交易日历] 从 Tushare 拉取 {start}-{end} ...")
    t0 = time.time()

    try:
        df = pro.trade_cal(exchange="SSE", start_date=start, end_date=end,
                          is_open="", fields="cal_date,is_open,pretrade_date")
    except Exception as e:
        log(f"[交易日历] Tushare 拉取失败: {e}, 降级到 infer 模式")
        return sync_from_db_infer()

    elapsed = time.time() - t0
    log(f"[交易日历] Tushare 返回 {len(df)} 天 ({elapsed:.1f}s)")

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    rows = []
    for _, r in df.iterrows():
        cal_date = str(r["cal_date"])
        is_open = int(r["is_open"])
        # 格式化日期 YYYYMMDD → YYYY-MM-DD
        trade_date = f"{cal_date[:4]}-{cal_date[4:6]}-{cal_date[6:8]}"
        rows.append((trade_date, bool(is_open == 1), None, "tushare"))

    if rows:
        upsert_sql = """
            INSERT INTO trade_calendar (trade_date, is_trading_day, holiday_name, data_source, updated_at)
            VALUES %s
            ON CONFLICT (trade_date) DO UPDATE SET
                is_trading_day = EXCLUDED.is_trading_day,
                data_source = EXCLUDED.data_source,
                updated_at = NOW()
        """
        execute_values(cur, upsert_sql, rows, page_size=500)
        conn.commit()

    cur.execute("SELECT COUNT(*) FROM trade_calendar WHERE is_trading_day = true")
    total_trading = cur.fetchone()[0]
    cur.execute("SELECT COUNT(*) FROM trade_calendar WHERE is_trading_day = false")
    total_holiday = cur.fetchone()[0]
    cur.close()
    conn.close()

    log(f"[交易日历] ✅ 同步完成: {total_trading} 个交易日, {total_holiday} 个假日")
    log(f"STAT:records_new={len(rows)},records_skip=0,records_err=0,trading_days={total_trading}")


def sync_from_db_infer():
    """降级方案: 从 stocks_daily_k 中推断交易日。"""
    log("[交易日历] 降级: 从 stocks_daily_k 推断交易日...")
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 从 stocks_daily_k 获取所有有数据的日期作为交易日
    cur.execute("""
        INSERT INTO trade_calendar (trade_date, is_trading_day, data_source, updated_at)
        SELECT DISTINCT trade_date, true, 'inferred', NOW()
        FROM stocks_daily_k
        WHERE trade_date IS NOT NULL
        ON CONFLICT (trade_date) DO UPDATE SET
            is_trading_day = true,
            data_source = CASE WHEN trade_calendar.data_source = 'tushare' THEN 'tushare' ELSE 'inferred' END,
            updated_at = NOW()
    """)
    inserted = cur.rowcount
    conn.commit()

    cur.execute("SELECT COUNT(*) FROM trade_calendar WHERE is_trading_day = true")
    total = cur.fetchone()[0]
    cur.close()
    conn.close()

    log(f"[交易日历] ✅ 从 K线数据推断: {total} 个交易日 (新增/更新 {inserted})")
    log(f"STAT:records_new={inserted},trading_days={total}")


def main():
    parser = argparse.ArgumentParser(description="交易日历同步")
    parser.add_argument("--year", type=int, default=None, help="指定年份 (如 2026)")
    parser.add_argument("--infer", action="store_true", help="从 stocks_daily_k 推断交易日")
    args = parser.parse_args()

    if args.infer or "--infer" in sys.argv:
        sync_from_db_infer()
    else:
        sync_from_tushare(args.year)


if __name__ == "__main__":
    main()
