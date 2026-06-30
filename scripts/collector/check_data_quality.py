#!/usr/bin/env python3
"""数据质量检查 — 近N日每日成交额汇总 + 缺失检测
用法: python3 check_data_quality.py [--days 60]
输出: 每日 stocks 数量 / 成交额汇总 / 换手率覆盖率 / 异常标记
"""
import os, sys, argparse, psycopg2

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def main():
    parser = argparse.ArgumentParser(description="数据质量检查")
    parser.add_argument("--days", type=int, default=60, help="检查天数 (默认 60)")
    args = parser.parse_args()

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 1. Daily stats
    cur.execute("""
        SELECT trade_date,
               COUNT(*) as stock_count,
               SUM(amount) as total_amount,
               SUM(CASE WHEN turnover_rate > 0 THEN 1 ELSE 0 END) as has_turnover,
               SUM(CASE WHEN amount > 0 THEN 1 ELSE 0 END) as has_amount,
               SUM(CASE WHEN amount IS NULL OR amount = 0 THEN 1 ELSE 0 END) as missing_amount,
               SUM(CASE WHEN turnover_rate IS NULL OR turnover_rate = 0 THEN 1 ELSE 0 END) as missing_turnover
        FROM stocks_daily_k
        WHERE trade_date >= CURRENT_DATE - INTERVAL '%s days'
        GROUP BY trade_date
        ORDER BY trade_date DESC
    """ % args.days)
    rows = cur.fetchall()

    if not rows:
        print("⚠️  无数据")
        cur.close(); conn.close()
        return

    # Header
    print(f"\n{'='*95}")
    print(f"  数据质量报告 — 近 {args.days} 日 (共 {len(rows)} 个交易日)")
    print(f"{'='*95}")
    print(f"{'日期':<12} {'股票数':>6} {'成交额(亿)':>12} {'有换手':>6} {'额缺失':>6} {'换手缺失':>8} {'状态':>6}")
    print(f"{'-'*95}")

    anomalies = []
    for r in rows:
        td, cnt, amt, has_to, has_amt, miss_amt, miss_to = r
        amt_yi = float(amt or 0) / 1e8
        status = ""
        if cnt < 4000:
            status = "⚠️ 数量少"
            anomalies.append((td, f"仅 {cnt} 只股票"))
        if miss_amt > cnt * 0.1:
            status = "⚠️ 额缺失"
            anomalies.append((td, f"成交额缺失 {miss_amt}/{cnt}"))
        if miss_to > cnt * 0.3:
            if status:
                status += "+换手缺"
            else:
                status = "⚠️ 换手缺"
            anomalies.append((td, f"换手率缺失 {miss_to}/{cnt}"))
        if not status:
            status = "✓"

        print(f"{str(td):<12} {cnt:>6} {amt_yi:>12.1f} {has_to:>6} {miss_amt:>6} {miss_to:>8} {status:>6}")

    print(f"{'-'*95}")

    # 2. Summary anomalies
    if anomalies:
        print(f"\n🔴 可疑日期 ({len(anomalies)} 个):")
        for td, reason in anomalies:
            print(f"   {td} — {reason}")
    else:
        print(f"\n✅ 近 {args.days} 日数据正常")

    # 3. Board breakdown for latest date
    cur.execute("""
        SELECT trade_date,
               SUM(CASE WHEN code ~ '^6' THEN 1 ELSE 0 END) as sh,
               SUM(CASE WHEN code ~ '^[03]' THEN 1 ELSE 0 END) as sz,
               SUM(CASE WHEN code ~ '^[84]|^92' THEN 1 ELSE 0 END) as bj,
               SUM(amount) as total_amount
        FROM stocks_daily_k
        WHERE trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k)
        GROUP BY trade_date
    """)
    br = cur.fetchone()
    if br:
        td, sh, sz, bj, amt = br
        print(f"\n📊 最新交易日 {td} 板块分布:")
        print(f"   沪市: {sh} 只 | 深市: {sz} 只 | 北交所: {bj} 只")
        print(f"   成交额: {float(amt or 0)/1e8:.1f} 亿")

    # 4. Market cap check
    cur.execute("""
        SELECT COUNT(*) as total,
               SUM(CASE WHEN circulating_market_cap > 0 THEN 1 ELSE 0 END) as has_cap,
               SUM(CASE WHEN pe > 0 THEN 1 ELSE 0 END) as has_pe
        FROM stocks_daily_indicator
        WHERE trade_date = (SELECT MAX(trade_date) FROM stocks_daily_indicator)
    """)
    ir = cur.fetchone()
    if ir:
        total, has_cap, has_pe = ir
        print(f"\n📊 指标覆盖 (最新):")
        print(f"   总计: {total} | 有市值: {has_cap} | 有PE: {has_pe}")
        if total > 0 and has_cap < total * 0.5:
            print(f"   ⚠️ 流通市值覆盖率过低: {has_cap}/{total}")

    cur.close()
    conn.close()
    print()

if __name__ == "__main__":
    main()
