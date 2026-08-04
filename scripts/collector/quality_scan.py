#!/usr/bin/env python3
"""
数据质量扫描 — 对 stocks_daily_k 逐行校验，标记 data_quality

校验规则:
  1. 价格异常: close < 0.01 或 close > 10000
  2. 涨跌幅超限: |change_pct| > 21 (超过A股涨跌停限制)
  3. OHLC 逻辑: low > min(open, close) 或 high < max(open, close)
  4. 昨收跳空: |close_t / close_{t-1} - 1| > 0.11 (单日跳空超11%)
  5. 零成交量: volume = 0 且非停牌

标记策略:
  ok      → 全部通过
  suspect → 1 条触警
  bad     → 2+ 条触警

用法:
  python3 quality_scan.py                    # 最近5天增量
  python3 quality_scan.py --days 30          # 最近30天
  python3 quality_scan.py --full             # 全量扫描
  python3 quality_scan.py --code 600519      # 单只股票
  python3 quality_scan.py --dry-run          # 仅报告不写入
"""

import os, sys, argparse, time
from collections import defaultdict

import psycopg2

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")


def log(msg):
    print(msg, flush=True)


def load_kline_data(cur, codes=None, recent_days=None):
    """加载K线数据，返回 [(code, trade_date, open, high, low, close, pre_close, change_pct, volume, is_paused), ...]"""
    sql = """
        SELECT code, trade_date, open, high, low, close,
               COALESCE(pre_close, 0), COALESCE(change_pct, 0),
               volume, COALESCE(is_paused, false)
        FROM stocks_daily_k
        WHERE code !~ '^IDX'
    """
    params = []
    if codes:
        sql += " AND code = ANY(%s)"
        params.append(codes)
    if recent_days:
        sql += f" AND trade_date >= CURRENT_DATE - INTERVAL '{recent_days} days'"

    sql += " ORDER BY code, trade_date"

    cur.execute(sql, params)
    return cur.fetchall()


def validate_rows(rows):
    """对每行数据执行5条校验规则，返回违规统计。

    Returns:
      stats: {code: {trade_date: {'violations': [rule_names], 'close': float}}}
      summary: {rule_name: count}
    """
    # 按股票分组以便做昨收校验（规则4需要前一日close）
    by_stock = defaultdict(list)
    for row in rows:
        code, td, o, h, l, c, pre_close, chg_pct, vol, is_paused = row
        by_stock[code].append({
            'trade_date': str(td),
            'open': float(o or 0),
            'high': float(h or 0),
            'low': float(l or 0),
            'close': float(c or 0),
            'pre_close': float(pre_close or 0),
            'change_pct': float(chg_pct or 0),
            'volume': int(vol or 0),
            'is_paused': bool(is_paused),
        })

    stats = {}
    summary = defaultdict(int)

    for code, klines in by_stock.items():
        # 按日期排序用于昨收校验
        klines.sort(key=lambda x: x['trade_date'])
        prev_close = None

        for i, k in enumerate(klines):
            violations = []
            td = k['trade_date']
            close = k['close']

            # 规则1: 价格异常
            if close < 0.01 or close > 10000:
                violations.append('价格异常')
                summary['价格异常'] += 1

            # 规则2: 涨跌幅超限（停牌日跳过）
            if not k['is_paused'] and abs(k['change_pct']) > 21:
                violations.append('涨跌幅超限')
                summary['涨跌幅超限'] += 1

            # 规则3: OHLC 逻辑
            min_oc = min(k['open'], close)
            max_oc = max(k['open'], close)
            ohlc_ok = (k['low'] <= min_oc or abs(k['low'] - min_oc) < 0.001) and \
                      (k['high'] >= max_oc or abs(k['high'] - max_oc) < 0.001)
            if not ohlc_ok and k['open'] > 0 and k['high'] > 0:
                violations.append('OHLC异常')
                summary['OHLC异常'] += 1

            # 规则4: 昨收跳空
            if prev_close is not None and prev_close > 0 and close > 0:
                jump = abs(close / prev_close - 1)
                if jump > 0.11 and not k['is_paused']:
                    violations.append('昨收跳空')
                    summary['昨收跳空'] += 1

            # 规则5: 零成交量（非停牌）
            if k['volume'] == 0 and not k['is_paused']:
                violations.append('零成交量')
                summary['零成交量'] += 1

            prev_close = close

            if violations:
                stats.setdefault(code, {})[td] = {
                    'violations': violations,
                    'close': close,
                }

    return stats, dict(summary)


def apply_quality(cur, stats, dry_run=False):
    """根据违规数量标记 data_quality（批量写入）。

    标记策略:
      0 条违规 → 'ok'
      1 条违规 → 'suspect'
      2+ 条违规 → 'bad'
    """
    from psycopg2.extras import execute_values

    updates = {'ok': 0, 'suspect': 0, 'bad': 0}
    rows = []

    for code, dates in stats.items():
        for td, info in dates.items():
            n = len(info['violations'])
            if n >= 2:
                quality = 'bad'
            elif n == 1:
                quality = 'suspect'
            else:
                quality = 'ok'

            updates[quality] += 1
            rows.append((quality, code, td))

    if rows and not dry_run:
        execute_values(cur, """
            UPDATE stocks_daily_k AS t SET data_quality = v.quality::varchar
            FROM (VALUES %s) AS v(quality, code, trade_date)
            WHERE t.code = v.code AND t.trade_date = v.trade_date::date
        """, rows, page_size=500)

    return updates


def main():
    parser = argparse.ArgumentParser(description='数据质量扫描')
    parser.add_argument('--days', type=int, default=5, help='扫描最近N天(默认5)')
    parser.add_argument('--full', action='store_true', help='全量扫描所有历史')
    parser.add_argument('--code', type=str, help='单只股票代码')
    parser.add_argument('--dry-run', action='store_true', help='仅报告不写入')
    args = parser.parse_args()

    codes = [args.code] if args.code else None
    recent_days = None if args.full else args.days
    label = f"({args.code})" if args.code else ("全量" if args.full else f"最近{args.days}天")

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Step 1: 加载数据
    t0 = time.time()
    log(f"[quality] 加载K线数据... {label}")
    rows = load_kline_data(cur, codes, recent_days)
    log(f"[quality] 加载 {len(rows)} 行 ({time.time()-t0:.1f}s)")

    # Step 2: 逐行校验
    t0 = time.time()
    stats, summary = validate_rows(rows)
    log(f"[quality] 校验完成 ({time.time()-t0:.1f}s)")

    # Step 3: 输出违规统计
    total_bad = sum(len(dates) for dates in stats.values())
    print(f"\n{'─'*50}")
    print(f"📋 校验结果摘要 ({label})")
    print(f"   总行数: {len(rows):,}")
    print(f"   违规行数: {total_bad:,} ({total_bad/len(rows)*100:.2f}%)" if len(rows) > 0 else "   违规行数: 0")

    if summary:
        print(f"\n   违规类型分布:")
        for rule, cnt in sorted(summary.items(), key=lambda x: -x[1]):
            print(f"     - {rule}: {cnt:,}")
    else:
        print(f"   ✅ 所有数据通过校验")

    # Step 4: 显示 Top 违规股票
    if stats:
        # 按违规行数排序
        code_violations = sorted(stats.items(), key=lambda x: -len(x[1]))
        print(f"\n   Top 10 违规股票:")
        for i, (code, dates) in enumerate(code_violations[:10]):
            total_v = sum(len(info['violations']) for info in dates.values())
            sample = list(dates.values())[0]
            print(f"   {i+1}. {code}: {len(dates)} 行违规 ({total_v} 条), "
                  f"例: {', '.join(sample['violations'][:3])}")

    # Step 5: 标记
    if not args.dry_run:
        t0 = time.time()
        updates = apply_quality(cur, stats, dry_run=False)

        conn.commit()
        log(f"\n[quality] 标记完成 ({time.time()-t0:.1f}s)")
        log(f"   ok: {updates['ok']:,}  suspect: {updates['suspect']:,}  bad: {updates['bad']:,}")
    else:
        print(f"\n⚠️  DRY-RUN 模式，未写入数据库")

    cur.close()
    conn.close()
    print(f"{'─'*50}")
    print(f"✅ 质量扫描完成")


if __name__ == "__main__":
    main()
