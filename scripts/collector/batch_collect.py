#!/usr/bin/env python3
"""增量采集日K线 — 腾讯前复权API，并发拉取 + 批量upsert"""
import os, sys, time, json, ssl, urllib.request
from datetime import date
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

MAX_WORKERS = int(os.environ.get("COLLECTOR_WORKERS", "15"))
CHUNK_SIZE  = int(os.environ.get("COLLECTOR_CHUNK",  "200"))

# ── Global progress lock ──
progress_lock = Lock()
progress_done = 0
progress_new_stocks = 0
progress_records = 0

# ── Per-thread error tracking ──
_fetch_errors = set()   # codes that failed at least once
_errors_lock = Lock()

# ──────────────────────────────────────────────
#  fetch_kline — thread-safe, pure function
# ──────────────────────────────────────────────
def fetch_kline(code, days=365):
    prefix = "sh" if code.startswith(("6", "9")) else "sz"
    url = f"http://ifzq.gtimg.cn/appstock/app/fqkline/get?param={prefix}{code},day,,,{days},qfq"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=15, context=ctx) as resp:
            data = json.loads(resp.read().decode())
        return data.get("data", {}).get(f"{prefix}{code}", {}).get("qfqday", []) or \
               data.get("data", {}).get(f"{prefix}{code}", {}).get("day", []) or []
    except Exception as e:
        with _errors_lock:
            if code not in _fetch_errors:
                _fetch_errors.add(code)
                print(f"  ⚠ fetch_kline error for {code}: {e}", flush=True)
        return []


# ──────────────────────────────────────────────
#  fetch_one_stock — one unit of concurrent work
# ──────────────────────────────────────────────
def fetch_one_stock(code, latest, today):
    """Returns (code, rows_for_upsert, is_new_stock)"""
    if latest is None:
        days_to_fetch = 60
    else:
        missing = (today - latest).days
        days_to_fetch = max(5, missing + 5)

    klines = fetch_kline(code, days=days_to_fetch)
    rows = []
    for row in klines:
        if len(row) < 6:
            continue
        vol_shou = float(row[5])
        vol_gu = int(vol_shou * 100)
        close_p = float(row[2])
        amt = close_p * float(vol_gu)
        rows.append((
            code,
            row[0],           # trade_date
            float(row[1]),    # open
            float(row[3]),    # high
            float(row[4]),    # low
            close_p,          # close
            vol_gu,           # volume
            amt,              # amount
            0.0,              # turnover_rate (filled later by quote phase)
        ))
    is_new = len(rows) > 0
    return code, rows, is_new


# ──────────────────────────────────────────────
#  batch_upsert_chunk
# ──────────────────────────────────────────────
UPSERT_SQL = """
    INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, volume, amount, turnover_rate)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        open = EXCLUDED.open, high = EXCLUDED.high, low = EXCLUDED.low,
        close = EXCLUDED.close, volume = EXCLUDED.volume, amount = EXCLUDED.amount,
        turnover_rate = EXCLUDED.turnover_rate
"""

def batch_upsert_chunk(cur, all_rows):
    """Batch upsert all collected rows for a chunk."""
    if not all_rows:
        return 0
    execute_values(cur, UPSERT_SQL, all_rows, page_size=200)
    return len(all_rows)


# ──────────────────────────────────────────────
#  fetch_quote_batch (unchanged)
# ──────────────────────────────────────────────
def fetch_quote_batch(codes_batch):
    results = {}
    symbols = []
    for code in codes_batch:
        prefix = "sh" if code.startswith(("6", "9")) else "sz"
        symbols.append(f"{prefix}{code}")
    url = f"http://qt.gtimg.cn/q={','.join(symbols)}"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=10, context=ctx) as resp:
            text = resp.read().decode('gbk', errors='replace')
        for line in text.strip().split('\n'):
            if '="' not in line:
                continue
            try:
                code_part = line.split('_')[1].split('="')[0] if '_' in line else ''
                code = code_part[2:] if len(code_part) > 2 else code_part
                fields = line.split('="')[1].rstrip('";').split('~')
                if len(fields) > 45:
                    turnover = float(fields[38]) if fields[38] else 0
                    pe = float(fields[39]) if fields[39] else 0
                    mcap = float(fields[44]) if fields[44] else 0
                    cmcap = float(fields[45]) if fields[45] else 0
                    results[code] = {
                        'turnover': turnover,
                        'pe': pe,
                        'market_cap': mcap,
                        'circulating_market_cap': cmcap
                    }
            except:
                pass
    except:
        pass
    return results


# ══════════════════════════════════════════════
#  main
# ══════════════════════════════════════════════
def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("""
        SELECT b.code, MAX(k.trade_date) as latest
        FROM stocks_basic b
        LEFT JOIN stocks_daily_k k ON b.code = k.code
        WHERE b.code NOT LIKE '88%'
        GROUP BY b.code
        ORDER BY latest NULLS FIRST
    """)
    stocks = [(r[0], r[1]) for r in cur.fetchall()]
    codes_list = [s[0] for s in stocks]

    today = date.today()
    has_k = sum(1 for _, d in stocks if d is not None)
    need = len(stocks) - has_k
    # ── Split into chunks ──
    chunks = [stocks[i:i + CHUNK_SIZE] for i in range(0, len(stocks), CHUNK_SIZE)]
    total_chunks = len(chunks)

    print(f"📊 数据源: 腾讯财经 (ifzq.gtimg.cn) | 前复权(qfq)", flush=True)
    print(f"⚙️  并发: {MAX_WORKERS} 线程 | 批量: {CHUNK_SIZE} 只/chunk | 共 {total_chunks} 批次", flush=True)
    print(f"📋 总计 {len(stocks)} 只股票 | 已有K线 {has_k} 只 | 待采集 {need} 只", flush=True)
    stale_info = ""
    if has_k > 0 and need == 0:
        print(f"⚠️  K线数据可能过期，将重新拉取最新数据", flush=True)
        stale_info = ",stale=1"
    print(f"🚀 开始采集K线数据...", flush=True)

    start = time.time()
    total_new = 0
    total_records = 0

    for chunk_idx, chunk in enumerate(chunks):
        chunk_start = time.time()

        # ── Phase 1: concurrent fetch ──
        with ThreadPoolExecutor(max_workers=MAX_WORKERS) as pool:
            futures = {
                pool.submit(fetch_one_stock, code, latest, today): code
                for code, latest in chunk
            }
            chunk_rows = []
            chunk_new_count = 0
            for fut in as_completed(futures):
                code, rows, is_new = fut.result()
                if rows:
                    chunk_rows.extend(rows)
                if is_new:
                    chunk_new_count += 1

        # ── Phase 2: batch upsert ──
        upserted = batch_upsert_chunk(cur, chunk_rows)
        conn.commit()

        total_new += chunk_new_count
        total_records += upserted

        elapsed = time.time() - start
        chunk_time = time.time() - chunk_start
        processed = min((chunk_idx + 1) * CHUNK_SIZE, len(stocks))
        pct = processed * 100 // len(stocks)
        rate = processed / max(elapsed, 1)
        eta_sec = max(0, (len(stocks) - processed) / max(rate, 0.01))
        eta_str = f"{int(eta_sec//60)}分{int(eta_sec%60)}秒" if eta_sec < 3600 else f"{eta_sec/3600:.1f}时"
        # ── 计算本批日期范围 ──
        chunk_dates = set()
        chunk_new_codes = set()
        for row in chunk_rows:
            chunk_dates.add(row[1])  # trade_date is second field
        date_str = ""
        if chunk_dates:
            sorted_dates = sorted(chunk_dates)
            date_str = f"{sorted_dates[0]}~{sorted_dates[-1]}" if len(sorted_dates) > 1 else sorted_dates[0]
        new_records = len(chunk_rows)
        updated_records = sum(1 for c, _ in chunk if c not in [_fetch_errors])

        print(f"  📈 腾讯K线 [{chunk_idx + 1}/{total_chunks}] 已处理 {processed}/{len(stocks)} ({pct}%) "
              f"| 日期 {date_str} "
              f"| 本批拉取 {new_records} 条，新入库 {chunk_new_count} 只 "
              f"| 累计入库 {total_records} 条，更新 {total_new} 只 "
              f"| 预计剩余 {eta_str}", flush=True)
        if date_str:
            print(f"     📊 关键行为: 通过腾讯财经(ifzq.gtimg.cn)获取 {date_str} 日K线 {new_records} 条, "
                  f"本批新增 {chunk_new_count} 只股票K线", flush=True)
        if _fetch_errors:
            print(f"     ⚠️  累积 {len(_fetch_errors)} 只获取失败: {', '.join(sorted(_fetch_errors)[:5])}", flush=True)
        print(f"STAT:kline_fetched={len(chunk_rows)},kline_new={chunk_new_count},kline_upserted={upserted}", flush=True)
        print(f"PROGRESS:{processed}/{len(stocks)}", flush=True)

    # ─── 历史换手率回填 (跳过，实时行情阶段处理) ───
    print(f"\n━━━ 阶段 2/2: 行情数据采集 ━━━", flush=True)
    print(f"📊 通过腾讯行情接口 (qt.gtimg.cn) 采集换手率/PE/总市值/流通市值", flush=True)
    t0 = time.time()
    turnover_updated = 0
    indicator_updated = 0
    for i in range(0, len(codes_list), 80):
        batch = codes_list[i:i+80]
        quotes = fetch_quote_batch(batch)
        for code, q in quotes.items():
            if q['turnover'] > 0:
                cur.execute("""
                    UPDATE stocks_daily_k SET turnover_rate = %s
                    WHERE code = %s AND trade_date = (
                        SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = %s
                    )
                """, (q['turnover'] / 100, code, code))
                if cur.rowcount > 0:
                    turnover_updated += 1
            if q['pe'] > 0 or q['market_cap'] > 0:
                # Guard: skip codes with no kline data to avoid NULL trade_date
                cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = %s", (code,))
                latest_td = cur.fetchone()[0]
                if latest_td is None:
                    continue
                try:
                    cur.execute("""
                        INSERT INTO stocks_daily_indicator (code, trade_date, pe, total_market_cap, circulating_market_cap)
                        VALUES (%s, %s, %s, %s, %s)
                        ON CONFLICT (code, trade_date) DO UPDATE SET
                            pe = EXCLUDED.pe,
                            total_market_cap = EXCLUDED.total_market_cap,
                            circulating_market_cap = EXCLUDED.circulating_market_cap
                    """, (code, latest_td, q['pe'], q['market_cap'], q['circulating_market_cap']))
                    if cur.rowcount > 0:
                        indicator_updated += 1
                except Exception as e:
                    print(f"     ⚠️  指标写入失败 {code}: {e}", flush=True)
        if (i + 80) % 400 == 0 or i == 0:
            pct_q = min(i+80, len(codes_list)) * 100 // len(codes_list)
            print(f"  📊 行情采集 {min(i+80, len(codes_list))}/{len(codes_list)} ({pct_q}%) | 换手率更新 {turnover_updated} 只 | PE/市值更新 {indicator_updated} 只", flush=True)
            print(f"STAT:turnover_updated={turnover_updated},indicator_updated={indicator_updated}", flush=True)
            print(f"PROGRESS:{min(i+80, len(codes_list))}/{len(codes_list)}", flush=True)
    conn.commit()
    quote_elapsed = time.time()-t0
    print(f"  ✅ 行情采集完成: 换手率 {turnover_updated} 只 | PE/市值 {indicator_updated} 只 | 耗时 {quote_elapsed:.0f}s", flush=True)

    elapsed = time.time() - start
    failed = len(_fetch_errors)
    print(f"\n{'─'*50}", flush=True)
    print(f"✅ 日K线采集完成", flush=True)
    print(f"   数据源: 腾讯财经 (ifzq.gtimg.cn) 前复权", flush=True)
    print(f"   处理股票: {len(stocks)} 只，其中 {total_new} 只有新数据", flush=True)
    print(f"   入库记录: {total_records} 条 (含新增+更新)", flush=True)
    print(f"   换手率回填: {turnover_updated} 只", flush=True)
    print(f"   PE/市值更新: {indicator_updated} 只", flush=True)
    if failed > 0:
        print(f"   失败: {failed} 只", flush=True)
    print(f"   总耗时: {elapsed:.0f}s ({elapsed/60:.1f}分)", flush=True)
    print(f"{'─'*50}\n", flush=True)

    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
