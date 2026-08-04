#!/usr/bin/env python3
"""
增量采集日K线 + 估值指标 — 腾讯前复权 API (V4 unified)
单次请求同时获取 K线(qfqday) + 行情(qt: 换手率/PE/PB/市值/涨跌停)

数据标准化:
  volume → 股(×100 for 主板/创业板)
  amount → 元(close×volume_股)
  turnoverRate → 原始比率(0.0026=0.26%, 即 qt[38]/100)
  marketCap → 元(qt[44/45]原为亿,×1e8)
  PE/PB → 原始倍数

用法:
  python3 batch_collect.py              # 增量(各股按缺口)
  python3 batch_collect.py --last 10    # 强制拉最近10个日历日
"""
import os, re, sys, json, ssl, time, urllib.request
from datetime import date
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

import psycopg2

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
MAX_WORKERS = int(os.environ.get("COLLECTOR_WORKERS", "15"))
CHUNK_SIZE  = int(os.environ.get("COLLECTOR_CHUNK",  "200"))

# Boards where Tencent returns volume in 股 (not 手)
BOARDS_VOL_IN_GU = {"688", "8", "4", "92"}

progress_lock = Lock()
_fetch_errors = set()
_errors_lock = Lock()

# ── Tencent K-line API ──────────────────────────────────────────
def fetch_kline(code, days=365):
    """Fetch K-line + qt from Tencent. Returns (klines_list, qt_list_or_None)."""
    if not re.match(r'^[0-9]{6}$', code):
        return [], None
    p = "bj" if code.startswith(("92","8","4")) else ("sh" if code.startswith(("6","9")) else "sz")
    url = f"https://ifzq.gtimg.cn/appstock/app/fqkline/get?param={p}{code},day,,,{days},qfq"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    try:
        with urllib.request.urlopen(req, timeout=15, context=ssl.create_default_context()) as resp:
            data = json.loads(resp.read().decode())
    except Exception as e:
        with _errors_lock:
            if code not in _fetch_errors:
                _fetch_errors.add(code)
                print(f"  ⚠ fetch error {code}: {e}", flush=True)
        return [], None

    stock_data = data.get("data", {}).get(f"{p}{code}", {})
    if not stock_data:
        return [], None

    klines = stock_data.get("qfqday", []) or stock_data.get("day", [])
    # qt field contains 88-field real-time quote array
    qt = stock_data.get("qt", {}).get(f"{p}{code}", None)
    return klines, qt

# ── Unified row builder ──────────────────────────────────────────
def build_rows(code, klines, qt):
    """
    Build standardized rows for stocks_daily_k and stocks_daily_indicator.
    Returns (k_rows, ind_rows) — lists of tuples ready for execute_values.
    """
    k_rows = []
    ind_rows = []

    is_gu_board = any(code.startswith(p) for p in BOARDS_VOL_IN_GU)
    prev_close = 0.0

    for row in klines:
        if len(row) < 6:
            continue

        # Parse K-line fields: [date, open, close, high, low, volume(手/股)]
        td = row[0]
        open_p  = float(row[1])
        close_p = float(row[2])
        high_p  = float(row[3])
        low_p   = float(row[4])
        vol_shou = float(row[5])
        pre_close = prev_close
        change_amount = close_p - pre_close if pre_close > 0 else 0.0
        prev_close = close_p

        # Normalize volume → 股
        vol_gu = int(vol_shou) if is_gu_board else int(vol_shou * 100)
        # Amount → 元
        amt = round(close_p * float(vol_gu), 2)

        # Extract daily-frequency fields from qt.
        # IMPORTANT: qt is a real-time snapshot — only accurate for the latest trading day.
        # For historical rows in this batch, leave qt-derived fields as 0.
        buy_vol = 0
        sell_vol = 0
        change_pct = 0.0
        amplitude = 0.0
        volume_ratio = 0.0
        turnover = 0.0
        pe = 0.0
        pb = 0.0
        total_mcap = 0.0
        circ_mcap = 0.0
        amount_wan = 0.0
        is_latest = (td == klines[-1][0])  # only apply qt to the most recent date row

        if is_latest and qt and len(qt) > 51:
            try:
                buy_vol_raw = float(qt[7]) if qt[7] else 0.0
                sell_vol_raw = float(qt[8]) if qt[8] else 0.0
                buy_vol = int(buy_vol_raw) if is_gu_board else int(buy_vol_raw * 100)
                sell_vol = int(sell_vol_raw) if is_gu_board else int(sell_vol_raw * 100)
                change_pct = float(qt[32]) if qt[32] else 0.0
                amount_wan = float(qt[37]) if qt[37] else 0.0
                turnover = float(qt[38]) / 100.0 if qt[38] else 0.0
                pe = float(qt[39]) if qt[39] else 0.0
                amplitude = float(qt[43]) if qt[43] else 0.0
                pb = float(qt[46]) if qt[46] else 0.0
                volume_ratio = float(qt[49]) if qt[49] else 0.0
                circ_mcap  = float(qt[44]) if qt[44] else 0.0
                total_mcap = float(qt[45]) if qt[45] else 0.0
            except (ValueError, IndexError):
                pass

        final_amount = amount_wan * 1e4 if amount_wan > 0 else amt

        k_rows.append((code, td, open_p, high_p, low_p, close_p,
                       pre_close, change_amount, vol_gu, final_amount, turnover,
                       buy_vol, sell_vol, change_pct, amplitude, volume_ratio))

        if is_latest and (pe > 0 or pb > 0):
            ind_rows.append((code, td, pe, pb, 0.0, total_mcap, circ_mcap))

    return k_rows, ind_rows

# ── Unified writer (priority-aware) ──────────────────────────────
from kline_writer import upsert_kline, upsert_indicator

# ── Main ─────────────────────────────────────────────────────────
def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    force_days = None
    if "--last" in sys.argv:
        idx = sys.argv.index("--last")
        if idx + 1 < len(sys.argv):
            force_days = int(sys.argv[idx + 1])

    cur.execute("""
        SELECT b.code, MAX(k.trade_date) as latest
        FROM stocks_basic b
        LEFT JOIN stocks_daily_k k ON b.code = k.code
        WHERE b.code ~ '^[0-9]{6}$'
        GROUP BY b.code
        ORDER BY latest NULLS FIRST
    """)
    stocks = [(r[0], r[1]) for r in cur.fetchall()]
    today = date.today()
    has_k = sum(1 for _, d in stocks if d is not None)

    chunks = [stocks[i:i + CHUNK_SIZE] for i in range(0, len(stocks), CHUNK_SIZE)]
    mode_str = f"强制(最近{force_days}天)" if force_days else "增量(按缺口)"

    print(f"📊 数据源: 腾讯财经 ifzq.gtimg.cn | 前复权(qfq) | 统一单位(股/元/%)", flush=True)
    print(f"⚙️  并发: {MAX_WORKERS}线程 | {CHUNK_SIZE}只/批 | {len(chunks)}批", flush=True)
    print(f"📋 {len(stocks)}只 | 已有K线 {has_k}只 | 模式: {mode_str}", flush=True)
    print(f"🚀 开始采集...", flush=True)

    start = time.time()
    total_k_records = 0
    total_ind_records = 0
    total_new_stocks = 0

    for chunk_idx, chunk in enumerate(chunks):
        chunk_start = time.time()
        all_k_rows = []
        all_ind_rows = []
        new_count = 0

        with ThreadPoolExecutor(max_workers=MAX_WORKERS) as pool:
            def fetch_one(code, latest):
                if force_days is not None:
                    days = max(force_days, 10)
                elif latest is None:
                    days = 60
                else:
                    days = max(5, (today - latest).days + 5)
                klines, qt = fetch_kline(code, days=days)
                k_rows, ind_rows = build_rows(code, klines, qt)
                return code, k_rows, ind_rows, len(k_rows) > 0

            futures = {pool.submit(fetch_one, code, latest): code for code, latest in chunk}
            for fut in as_completed(futures):
                code, k_rows, ind_rows, is_new = fut.result()
                all_k_rows.extend(k_rows)
                all_ind_rows.extend(ind_rows)
                if is_new:
                    new_count += 1

        upserted_k = upsert_kline(cur, all_k_rows, source='tencent')
        upserted_ind = upsert_indicator(cur, all_ind_rows)
        conn.commit()

        total_k_records += upserted_k
        total_ind_records += upserted_ind
        total_new_stocks += new_count

        elapsed = time.time() - start
        processed = min((chunk_idx + 1) * CHUNK_SIZE, len(stocks))
        pct = processed * 100 // len(stocks)
        rate = processed / max(elapsed, 1)
        eta = max(0, (len(stocks) - processed) / max(rate, 0.01))
        eta_str = f"{int(eta//60)}分{int(eta%60)}秒" if eta < 3600 else f"{eta/3600:.1f}时"

        print(f"  📈 [{chunk_idx + 1}/{len(chunks)}] {processed}/{len(stocks)} ({pct}%)"
              f" | K线 {upserted_k}条 + 指标 {upserted_ind}条"
              f" | 新入库 {new_count}只 | 累计K {total_k_records}条 指标 {total_ind_records}条"
              f" | 剩余 {eta_str}", flush=True)
        if _fetch_errors:
            print(f"     ⚠️  {len(_fetch_errors)}只失败: {', '.join(sorted(_fetch_errors)[:5])}", flush=True)
        print(f"STAT:kline={upserted_k},indicator={upserted_ind},new={new_count}", flush=True)
        print(f"PROGRESS:{processed}/{len(stocks)}", flush=True)

    elapsed = time.time() - start
    failed = len(_fetch_errors)
    print(f"\n{'─'*50}", flush=True)
    print(f"✅ 采集完成", flush=True)
    print(f"   数据源: 腾讯财经 ifzq.gtimg.cn (K线 + qt行情)", flush=True)
    print(f"   K线入库: {total_k_records}条 | 指标入库: {total_ind_records}条", flush=True)
    print(f"   新增股票: {total_new_stocks}只 | 失败: {failed}只", flush=True)
    print(f"   耗时: {elapsed:.0f}s ({elapsed/60:.1f}分)", flush=True)
    print(f"{'─'*50}", flush=True)

    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
