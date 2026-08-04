#!/usr/bin/env python3
"""验证 P3/P5 结果：MACD 覆盖率 + 信号扫描"""
import psycopg2, os, time
from collections import defaultdict

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
conn = psycopg2.connect(PG_DSN)
cur = conn.cursor()

# ── 1. 验证 MACD 数据覆盖 ──
t0 = time.time()
cur.execute("""
    SELECT COUNT(*) as total,
           COUNT(*) FILTER (WHERE macd_dif IS NOT NULL) as has_macd,
           ROUND(COUNT(*) FILTER (WHERE macd_dif IS NOT NULL)::numeric / COUNT(*)::numeric * 100, 1) as pct
    FROM stocks_daily_k
""")
total, has_macd, pct = cur.fetchone()
print(f"MACD 覆盖率: {has_macd}/{total} = {pct}% ({time.time()-t0:.2f}s)")

# ── 2. 验证 MACD 金叉/死叉扫描（模拟 scanMACDCrossBatch）──
t0 = time.time()
cur.execute("""
    WITH ranked AS (
        SELECT code, trade_date, close, macd_dif, dea,
            ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) AS rn
        FROM stocks_daily_k
        WHERE macd_dif IS NOT NULL AND dea IS NOT NULL
    ),
    pivoted AS (
        SELECT code,
            MAX(CASE WHEN rn = 2 THEN trade_date END) AS prev_date,
            MAX(CASE WHEN rn = 2 THEN macd_dif END)    AS prev_dif,
            MAX(CASE WHEN rn = 2 THEN dea END)         AS prev_dea,
            MAX(CASE WHEN rn = 1 THEN trade_date END)  AS curr_date,
            MAX(CASE WHEN rn = 1 THEN close END)       AS curr_close,
            MAX(CASE WHEN rn = 1 THEN macd_dif END)    AS curr_dif,
            MAX(CASE WHEN rn = 1 THEN dea END)         AS curr_dea
        FROM ranked WHERE rn <= 2
        GROUP BY code HAVING COUNT(*) = 2
    )
    SELECT s.name, p.code, p.curr_date, ROUND(p.curr_close::numeric, 2) as close,
        ROUND(p.curr_dif::numeric, 4) as dif, ROUND(p.curr_dea::numeric, 4) as dea,
        CASE WHEN p.prev_dif <= p.prev_dea AND p.curr_dif > p.curr_dea THEN 'MACD金叉'
             WHEN p.prev_dif >= p.prev_dea AND p.curr_dif < p.curr_dea THEN 'MACD死叉'
        END as signal
    FROM pivoted p
    JOIN stocks_basic s ON s.code = p.code
    WHERE (p.prev_dif <= p.prev_dea AND p.curr_dif > p.curr_dea)
       OR (p.prev_dif >= p.prev_dea AND p.curr_dif < p.curr_dea)
    ORDER BY p.curr_date DESC
""")
macd_signals = cur.fetchall()
elapsed = time.time() - t0
print(f"MACD 交叉信号: {len(macd_signals)} 条 ({elapsed:.2f}s)")

golden = sum(1 for r in macd_signals if '金叉' in (r[6] or ''))
death = sum(1 for r in macd_signals if '死叉' in (r[6] or ''))
print(f"  金叉: {golden}, 死叉: {death}")

print("Top 10:")
for i, row in enumerate(macd_signals[:10]):
    name, code, dt, close, dif, dea, sig = row
    print(f"  {i+1}. {dt} {code} {name}: {sig} close={close} dif={dif:.4f} dea={dea:.4f}")

# ── 3. 验证 MA5×MA20 交叉（模拟 scanMACrossBatch）──
t0 = time.time()
cur.execute("""
    SELECT trade_date::text FROM trade_calendar
    WHERE trade_date <= CURRENT_DATE AND is_trading_day = true
    ORDER BY trade_date DESC LIMIT 2
""")
dates = cur.fetchall()
if len(dates) >= 2:
    curr_date = dates[0][0]
    print(f"\n最新交易日: {curr_date}, 上一交易日: {dates[1][0]}")

# Full MA scan with Go-equivalent logic
cur.execute("""
    SELECT code, trade_date::text, close FROM stocks_daily_k
    WHERE close > 0 AND trade_date > CURRENT_DATE - INTERVAL '90 days'
    ORDER BY code, trade_date ASC
""")
stock_data = defaultdict(list)
for code, td, close in cur.fetchall():
    stock_data[code].append(float(close))

ma_signals = []
for code, arr in stock_data.items():
    if len(arr) < 21:
        continue
    n = len(arr)
    prev_ma5 = sum(arr[n-7:n-2]) / 5
    prev_ma20 = sum(arr[n-21:n-1]) / 20
    curr_ma5 = sum(arr[n-6:n-1]) / 5
    curr_ma20 = sum(arr[n-20:]) / 20
    if prev_ma5 <= prev_ma20 and curr_ma5 > curr_ma20:
        ma_signals.append((code, 'MA5金叉MA20', 'bullish', arr[-1]))
    elif prev_ma5 >= prev_ma20 and curr_ma5 < curr_ma20:
        ma_signals.append((code, 'MA5死叉MA20', 'bearish', arr[-1]))

elapsed = time.time() - t0
print(f"MA 交叉信号: {len(ma_signals)} 条 ({elapsed:.2f}s)")
golden_ma = sum(1 for s in ma_signals if '金叉' in s[1])
death_ma = sum(1 for s in ma_signals if '死叉' in s[1])
print(f"  金叉: {golden_ma}, 死叉: {death_ma}")
print("Top 10:")
for i, (code, sig, direction, close) in enumerate(sorted(ma_signals, key=lambda x: -x[3])[:10]):
    print(f"  {i+1}. {code}: {sig} close={close:.2f}")

# ── 4. 数据源分布 ──
cur.execute("""
    SELECT data_source, source_priority, COUNT(*) as cnt
    FROM stocks_daily_k WHERE source_priority IS NOT NULL
    GROUP BY data_source, source_priority ORDER BY source_priority DESC
""")
print("\n数据源分布:")
for src, pri, cnt in cur.fetchall():
    print(f"  {src}: priority={pri}, count={cnt:,}")

cur.close()
conn.close()
print("\n验证完成")
