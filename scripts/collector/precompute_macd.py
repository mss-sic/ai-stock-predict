#!/usr/bin/env python3
"""
技术指标预计算 — 全市场 MACD (EMA12/26/9) 批量计算并写入 stocks_daily_k

算法:
  EMA12 = 2/13 × close + 11/13 × prev_ema12
  EMA26 = 2/27 × close + 25/27 × prev_ema26
  MACD DIF = EMA12 - EMA26
  DEA = 0.2 × DIF + 0.8 × prev_DEA
  MACD Bar = 2 × (DIF - DEA)

用法:
  python3 precompute_indicators.py                 # 全市场增量(最近60天)
  python3 precompute_indicators.py --full          # 全量重算(所有历史)
  python3 precompute_indicators.py --code 600519   # 单只股票
  python3 precompute_indicators.py --codes 600519,000001  # 多只
"""

import os, sys, argparse
from collections import defaultdict

import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

ALPHA12 = 2.0 / 13.0  # EMA(12)
ALPHA26 = 2.0 / 27.0  # EMA(26)
ALPHA9  = 0.2          # DEA(9)


def log(msg):
    print(msg, flush=True)


def load_close_data(cur, codes=None, recent_days=None):
    """加载股票收盘价数据，返回 {code: [(trade_date, close), ...]} 按日期升序。"""
    sql = """
        SELECT code, trade_date, close
        FROM stocks_daily_k
        WHERE code !~ '^IDX'
          AND close > 0
    """
    params = []
    if codes:
        sql += " AND code = ANY(%s)"
        params.append(codes)
    if recent_days:
        sql += f" AND trade_date >= CURRENT_DATE - INTERVAL '{recent_days} days'"

    sql += " ORDER BY code, trade_date"

    cur.execute(sql, params)
    data = defaultdict(list)
    for code, td, close in cur.fetchall():
        data[code].append((td, float(close)))
    return data


def compute_macd_batch(close_data):
    """批量计算 MACD 指标。

    Returns:
      {code: [(trade_date, ema12, ema26, macd_dif, dea, macd_bar), ...]}
    """
    results = {}
    for code, rows in close_data.items():
        if len(rows) < 26:
            continue

        indicators = []
        ema12 = ema26 = dea = 0.0
        first = True

        for td, close in rows:
            if first:
                ema12 = close
                ema26 = close
                dea = 0.0
                first = False
            else:
                ema12 = ALPHA12 * close + (1 - ALPHA12) * ema12
                ema26 = ALPHA26 * close + (1 - ALPHA26) * ema26

            dif = ema12 - ema26
            dea = ALPHA9 * dif + (1 - ALPHA9) * dea
            bar = 2.0 * (dif - dea)

            indicators.append((td, round(ema12, 4), round(ema26, 4),
                               round(dif, 4), round(dea, 4), round(bar, 4)))

        if indicators:
            results[code] = indicators

    return results


def upsert_indicators(cur, code, indicators):
    """批量写入技术指标到 stocks_daily_k。"""
    if not indicators:
        return 0

    rows = [(code, td, e12, e26, dif, dea, bar)
            for td, e12, e26, dif, dea, bar in indicators]

    sql = """
        INSERT INTO stocks_daily_k (code, trade_date, ema12, ema26, macd_dif, dea, macd_bar)
        VALUES %s
        ON CONFLICT (code, trade_date) DO UPDATE SET
            ema12   = EXCLUDED.ema12,
            ema26   = EXCLUDED.ema26,
            macd_dif = EXCLUDED.macd_dif,
            dea      = EXCLUDED.dea,
            macd_bar = EXCLUDED.macd_bar
    """
    execute_values(cur, sql, rows, page_size=500)
    return len(rows)


def main():
    parser = argparse.ArgumentParser(description="MACD 技术指标预计算")
    parser.add_argument('--code', help='单只股票代码')
    parser.add_argument('--codes', help='多只股票代码,逗号分隔')
    parser.add_argument('--full', action='store_true', help='全量重算(默认增量最近60天)')
    parser.add_argument('--days', type=int, default=60, help='增量天数(默认60)')
    args = parser.parse_args()

    codes = None
    if args.code:
        codes = [args.code]
    elif args.codes:
        codes = [c.strip() for c in args.codes.split(',') if c.strip()]

    label = f"({len(codes)}只)" if codes else ("全量" if args.full else f"增量(最近{args.days}天)")

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    log(f"[indicator] 加载K线数据... {label}")
    recent_days = None if args.full else args.days
    close_data = load_close_data(cur, codes, recent_days)
    log(f"[indicator] 加载 {len(close_data)} 只股票")

    log(f"[indicator] 计算MACD...")
    indicators = compute_macd_batch(close_data)
    log(f"[indicator] 计算完成 {len(indicators)} 只")

    log(f"[indicator] 写入数据库...")
    total = 0
    for i, (code, inds) in enumerate(sorted(indicators.items())):
        n = upsert_indicators(cur, code, inds)
        total += n
        if (i + 1) % 200 == 0:
            conn.commit()
            log(f"[indicator] {i+1}/{len(indicators)} 只, {total} 条 | PROGRESS:{i+1}/{len(indicators)}")

    conn.commit()
    log(f"STAT:records_new={total},records_skip=0,stocks={len(indicators)}")
    log(f"[indicator] ✅ 完成: {len(indicators)} 只, {total} 条 MACD")

    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
