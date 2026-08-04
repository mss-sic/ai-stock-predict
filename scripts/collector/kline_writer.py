#!/usr/bin/env python3
"""
统一K线写入器 — 优先级感知 UPSERT

数据源优先级（越大越优先）:
  100: tencent  — 腾讯财经 (前复权, 主力数据源)
  80:  tushare  — Tushare Pro (未复权, 官方认证)
  60:  mootdx    — 通达信 TCP (未复权, 不封IP)
  40:  youzi     — 柚子API (备用)
  0:   fallback  — 默认/降级

UPSERT 规则: 仅当新数据源的 priority >= 已有数据的 priority 时才覆盖。
           首次写入 (source_priority IS NULL) 直接接受所有数据。

用法:
  from kline_writer import upsert_kline, SOURCE_PRIORITY
  upsert_kline(cur, k_rows, source='tencent')
"""

import os
import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

SOURCE_PRIORITY = {
    'tencent': 100,
    'tushare': 80,
    'mootdx': 60,
    'youzi': 40,
    'fallback': 0,
}

# ── K-line UPSERT with priority guard ───────────────────────────
UPSERT_KLINE = """
    INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close,
        pre_close, change_amount, volume, amount, turnover_rate, buy_vol, sell_vol, change_pct, amplitude,
        volume_ratio, source_priority, data_source)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        open           = EXCLUDED.open,
        high           = EXCLUDED.high,
        low            = EXCLUDED.low,
        close          = EXCLUDED.close,
        pre_close      = CASE WHEN EXCLUDED.pre_close > 0 THEN EXCLUDED.pre_close
                              ELSE stocks_daily_k.pre_close END,
        change_amount  = EXCLUDED.change_amount,
        volume         = EXCLUDED.volume,
        amount         = EXCLUDED.amount,
        turnover_rate  = CASE WHEN EXCLUDED.turnover_rate > 0 THEN EXCLUDED.turnover_rate
                              ELSE stocks_daily_k.turnover_rate END,
        buy_vol        = EXCLUDED.buy_vol,
        sell_vol       = EXCLUDED.sell_vol,
        change_pct     = EXCLUDED.change_pct,
        amplitude      = EXCLUDED.amplitude,
        volume_ratio   = EXCLUDED.volume_ratio,
        source_priority = EXCLUDED.source_priority,
        data_source    = EXCLUDED.data_source
    WHERE stocks_daily_k.source_priority IS NULL
       OR EXCLUDED.source_priority >= stocks_daily_k.source_priority
"""

# ── Indicator UPSERT (PE/PB) ─────────────────────────────────────
UPSERT_INDICATOR = """
    INSERT INTO stocks_daily_indicator (code, trade_date, pe, pb, ps,
        total_market_cap, circulating_market_cap)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        pe = EXCLUDED.pe, pb = EXCLUDED.pb, ps = EXCLUDED.ps,
        total_market_cap = EXCLUDED.total_market_cap,
        circulating_market_cap = EXCLUDED.circulating_market_cap
"""


def upsert_kline(cur, rows, source='tencent', source_priority=None):
    """
    批量写入K线数据，带优先级保护。

    Args:
      cur: psycopg2 cursor
      rows: [(code, trade_date, open, high, low, close, pre_close, change_amount, volume, amount,
              turnover_rate, buy_vol, sell_vol, change_pct, amplitude, volume_ratio), ...]
      source: 数据源名称 (tencent/tushare/mootdx/youzi)
      source_priority: 手动指定优先级 (默认从 SOURCE_PRIORITY 查表)

    Returns:
      int: 写入行数
    """
    if not rows:
        return 0

    priority = source_priority if source_priority is not None else SOURCE_PRIORITY.get(source, 0)

    # Append source_priority and data_source to each row
    wrap_rows = [tuple(row) + (priority, source) for row in rows]

    execute_values(cur, UPSERT_KLINE, wrap_rows, page_size=500)
    return len(wrap_rows)


def upsert_indicator(cur, rows):
    """批量写入估值指标 (PE/PB等)。"""
    if not rows:
        return 0
    execute_values(cur, UPSERT_INDICATOR, rows, page_size=200)
    return len(rows)


def get_conn():
    """获取数据库连接。"""
    return psycopg2.connect(PG_DSN)


# ── Self-test ────────────────────────────────────────────────────
if __name__ == "__main__":
    conn = get_conn()
    cur = conn.cursor()

    # Verify the priority logic by checking existing data
    cur.execute("""
        SELECT code, trade_date, close, source_priority, data_source
        FROM stocks_daily_k
        WHERE code = '600519'
        ORDER BY trade_date DESC LIMIT 5
    """)
    print("茅台最近5条数据源:")
    for row in cur.fetchall():
        print(f"  {row[0]} {row[1]} close={row[2]} priority={row[3]} source={row[4]}")

    cur.execute("""
        SELECT data_source, source_priority, COUNT(*) as cnt
        FROM stocks_daily_k
        WHERE source_priority IS NOT NULL
        GROUP BY data_source, source_priority
        ORDER BY source_priority DESC
    """)
    print("\nsource_priority 分布:")
    for row in cur.fetchall():
        print(f"  {row[0]}: priority={row[1]}, count={row[2]}")

    cur.close()
    conn.close()
