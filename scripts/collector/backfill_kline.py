#!/usr/bin/env python3
"""
K线数据回填 — 为缺失最近交易日的股票补全数据

诊断逻辑:
  1. 找出最新交易日与最晚日期差距 >=3 天的股票（数据滞后）
  2. 按市值降序处理，通过腾讯 API 拉取缺失日期的 K 线

用法:
  python3 backfill_kline.py                  # 诊断模式，仅显示缺口统计
  python3 backfill_kline.py --dry-run        # 同上
  python3 backfill_kline.py --run            # 执行回填
  python3 backfill_kline.py --run --limit 50 # 回填前50只
"""

import os, sys, re, time, argparse, json, ssl, urllib.request
from datetime import date, datetime, timedelta
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
MAX_WORKERS = int(os.environ.get("COLLECTOR_WORKERS", "8"))
UA = "Mozilla/5.0"

progress_lock = Lock()
fetch_errors = set()
errors_lock = Lock()


def log(msg):
    print(msg, flush=True)


# ── Tencent K-line API ──────────────────────────────────────────
BOARDS_VOL_IN_GU = {"688", "8", "4", "92"}


def fetch_tencent_kline(code, days=120):
    """从腾讯 API 拉取 K 线数据。返回 [(trade_date, open, high, low, close, volume, amount), ...] 或 []"""
    if not re.match(r'^[0-9]{6}$', code):
        return []
    # 确定市场前缀
    p = "bj" if code.startswith(("92","8","4")) else ("sh" if code.startswith(("6","9")) else "sz")
    url = f"https://ifzq.gtimg.cn/appstock/app/fqkline/get?param={p}{code},day,,,{days},qfq"
    try:
        req = urllib.request.Request(url, headers={"User-Agent": UA})
        ctx = ssl.create_default_context()
        with urllib.request.urlopen(req, timeout=15, context=ctx) as resp:
            data = json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        with errors_lock:
            fetch_errors.add(code)
        return []

    stock_data = data.get("data", {}).get(f"{p}{code}", {})
    if not stock_data:
        return []

    qfqday = stock_data.get("qfqday", []) or stock_data.get("day", []) or []
    if not qfqday:
        return []

    # Determine if volume is in 股 (need to divide by 100)
    is_gu = code.startswith("688") or code.startswith("8") or code.startswith("4") or code.startswith("92")

    result = []
    for row in qfqday:
        try:
            parts = row if isinstance(row, list) else row.split(",")
            if len(parts) < 6:
                continue
            td = str(parts[0])
            # Tencent API returns date as YYYY-MM-DD (10 chars) or YYYYMMDD (8 chars)
            if len(td) == 10 and '-' in td:
                td_str = td  # already YYYY-MM-DD
            elif len(td) == 8:
                td_str = f"{td[:4]}-{td[4:6]}-{td[6:8]}"
            else:
                continue
            o = float(parts[1])
            c = float(parts[2])
            h = float(parts[3])
            l = float(parts[4])
            vol = float(parts[5])
            if not is_gu:
                vol = vol * 100  # 手 → 股
            amt = round(c * vol, 2) if c > 0 and vol > 0 else 0
            result.append((td_str, o, h, l, c, int(vol), amt))
        except (ValueError, IndexError):
            continue
    return result


def upsert_stocks_daily_k(cur, code, klines):
    """批量写入 stocks_daily_k。返回写入条数。"""
    if not klines:
        return 0

    rows = []
    for td, o, h, l, c, vol, amt in klines:
        chg_pct = 0.0
        rows.append((code, td, o, h, l, c, vol, amt, 0.0, 0, 0, chg_pct, 0.0, 0.0, 100, 'tencent'))

    try:
        execute_values(cur, """
            INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close,
                volume, amount, turnover_rate, buy_vol, sell_vol, change_pct,
                amplitude, volume_ratio, source_priority, data_source)
            VALUES %s
            ON CONFLICT (code, trade_date) DO UPDATE SET
                open = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.open ELSE stocks_daily_k.open END,
                high = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.high ELSE stocks_daily_k.high END,
                low = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.low ELSE stocks_daily_k.low END,
                close = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.close ELSE stocks_daily_k.close END,
                volume = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.volume ELSE stocks_daily_k.volume END,
                amount = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.amount ELSE stocks_daily_k.amount END,
                change_pct = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.change_pct ELSE stocks_daily_k.change_pct END,
                turnover_rate = CASE WHEN stocks_daily_k.source_priority <= 100 THEN EXCLUDED.turnover_rate ELSE stocks_daily_k.turnover_rate END,
                source_priority = CASE WHEN EXCLUDED.source_priority >= stocks_daily_k.source_priority THEN EXCLUDED.source_priority ELSE stocks_daily_k.source_priority END,
                data_source = CASE WHEN EXCLUDED.source_priority >= stocks_daily_k.source_priority THEN EXCLUDED.data_source ELSE stocks_daily_k.data_source END
        """, rows, page_size=200)
    except Exception as e:
        log(f"  ⚠️ {code} UPSERT error: {e}")
        return 0

    return len(rows)


def diagnose_gaps(cur):
    """返回需要回填的股票列表（按市值降序）。"""
    cur.execute("""
        SELECT f.code, COALESCE(i.mcap, 0) as mcap, f.last_date::text
        FROM (
            SELECT code, MAX(trade_date) as last_date
            FROM stocks_daily_k GROUP BY code
        ) f
        LEFT JOIN (
            SELECT code, MAX(total_market_cap) as mcap
            FROM stocks_daily_indicator WHERE total_market_cap > 0 GROUP BY code
        ) i ON f.code = i.code
        WHERE f.last_date < (SELECT MAX(trade_date) FROM stocks_daily_k) - INTERVAL '3 days'
        ORDER BY i.mcap DESC NULLS LAST
    """)
    return [(r[0], float(r[1] or 0), r[2]) for r in cur.fetchall()]


def main():
    parser = argparse.ArgumentParser(description='K线数据回填')
    parser.add_argument('--run', action='store_true', help='执行回填（默认仅诊断）')
    parser.add_argument('--dry-run', action='store_true', help='诊断模式')
    parser.add_argument('--limit', type=int, default=0, help='限制处理数量')
    args = parser.parse_args()

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Step 1: 诊断缺口
    gap_stocks = diagnose_gaps(cur)
    log(f"📊 数据滞后股票: {len(gap_stocks)} 只")

    if not gap_stocks:
        log("✅ 所有股票数据已更新到最新")
        cur.close(); conn.close()
        return

    # 按延迟天数分组
    cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k")
    latest_date = cur.fetchone()[0]
    delay_buckets = {}
    for code, mcap, last_date in gap_stocks:
        delay = (latest_date - datetime.strptime(last_date, '%Y-%m-%d').date()).days
        bucket = delay_buckets.setdefault(delay // 7, [])
        bucket.append(code)

    log(f"   最新日期: {latest_date}")
    for weeks, codes in sorted(delay_buckets.items()):
        log(f"   延迟 {weeks * 7}~{(weeks + 1) * 7} 天: {len(codes)} 只")
    log(f"   Top 10 (市值最大):")
    for code, mcap, last_date in gap_stocks[:10]:
        delay = (latest_date - datetime.strptime(last_date, '%Y-%m-%d').date()).days
        log(f"     {code}: 最后 {last_date}, 延迟 {delay} 天, 市值 {mcap/1e8:.0f} 亿")

    if not args.run and not args.dry_run:
        log(f"\n💡 执行回填: python3 backfill_kline.py --run [--limit N]")
        cur.close(); conn.close()
        return

    if args.run:
        stocks = gap_stocks[:args.limit] if args.limit > 0 else gap_stocks
        log(f"\n🚀 开始回填 {len(stocks)} 只...")

        done = 0
        total_rows = 0
        start = time.time()

        with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
            futures = {}
            for code, mcap, last_date in stocks:
                futures[executor.submit(fetch_tencent_kline, code, 120)] = code

            for i, future in enumerate(as_completed(futures)):
                code = futures[future]
                try:
                    klines = future.result()
                except Exception as e:
                    with errors_lock:
                        fetch_errors.add(code)
                    klines = []

                if klines:
                    # 只写入缺失日期的数据（晚于 last_date 的）
                    from datetime import date as dt_date
                    code_last = dt_date.today()
                    for s in stocks:
                        if s[0] == code:
                            code_last = datetime.strptime(str(s[2]), '%Y-%m-%d').date()
                            break
                    new_klines = [k for k in klines if k[0] > str(code_last)]
                    n = upsert_stocks_daily_k(cur, code, new_klines)
                    total_rows += n
                    if n > 0:
                        done += 1

                if (i + 1) % 50 == 0:
                    conn.commit()
                    elapsed = time.time() - start
                    rate = (i + 1) / elapsed if elapsed > 0 else 0
                    eta = (len(stocks) - i - 1) / rate if rate > 0 else 0
                    log(f"  📊 {i+1}/{len(stocks)} | 更新 {done} 只/{total_rows} 行 | {elapsed:.0f}s | ETA {eta:.0f}s")

        conn.commit()

        # 统计
        cur.execute("SELECT COUNT(*) FROM stocks_daily_k WHERE trade_date = %s", (latest_date,))
        new_stocks = cur.fetchone()[0]
        elapsed = time.time() - start
        log(f"\n{'─'*50}")
        log(f"✅ K线回填完成")
        log(f"   📥 更新: {done} 只 / {total_rows} 行")
        log(f"   📅 最新日期覆盖: {new_stocks} 只")
        log(f"   ❌ 采集失败: {len(fetch_errors)} 只")
        log(f"   ⏱️  耗时: {elapsed:.0f}s")
        log(f"{'─'*50}")

    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
