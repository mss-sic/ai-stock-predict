#!/usr/bin/env python3
"""
Pre-compute daily market aggregates for sentiment calculation.
Loads all stocks_daily_k data into Python, computes rolling metrics,
stores in market_daily_agg table.

This avoids expensive PostgreSQL window functions.

用法:
  python3 precompute_aggs.py                    # Compute latest date only
  python3 precompute_aggs.py --last 60          # Last 60 trading days
  python3 precompute_aggs.py --all              # All trading days
"""
import os, sys, time
from collections import defaultdict
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

CREATE_TABLE = """
CREATE TABLE IF NOT EXISTS market_daily_agg (
    trade_date DATE PRIMARY KEY,
    up_count INT, down_count INT, total_stocks INT,
    ma20_count INT, n52_high_count INT, n60_low_count INT,
    total_amount NUMERIC(20,2),
    up5_count INT, up5_total INT,
    created_at TIMESTAMPTZ DEFAULT NOW()
)
"""

INSERT_SQL = """
    INSERT INTO market_daily_agg (trade_date, up_count, down_count, total_stocks,
        ma20_count, n52_high_count, n60_low_count, total_amount, up5_count, up5_total)
    VALUES %s
    ON CONFLICT (trade_date) DO UPDATE SET
        up_count=EXCLUDED.up_count, down_count=EXCLUDED.down_count,
        total_stocks=EXCLUDED.total_stocks, ma20_count=EXCLUDED.ma20_count,
        n52_high_count=EXCLUDED.n52_high_count, n60_low_count=EXCLUDED.n60_low_count,
        total_amount=EXCLUDED.total_amount, up5_count=EXCLUDED.up5_count,
        up5_total=EXCLUDED.up5_total, created_at=NOW()
"""

def load_data(cur, min_date):
    """Load all K-line data from min_date onwards into a dict by code."""
    print(f"  Loading K-line data from {min_date}...", flush=True)
    ts = time.time()
    cur.execute("""
        SELECT code, trade_date, close, high, low, amount,
            LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
        FROM stocks_daily_k
        WHERE trade_date >= %s AND code !~ '^IDX'
        ORDER BY code, trade_date
    """, (min_date,))
    
    data = defaultdict(list)  # code -> [(trade_date, close, high, low, amount, prev_close), ...]
    dates_set = set()
    for r in cur.fetchall():
        code = r[0]
        data[code].append((
            str(r[1]), float(r[2] or 0), float(r[3] or 0),
            float(r[4] or 0), float(r[5] or 0), float(r[6] or 0),
        ))
        dates_set.add(str(r[1]))
    
    all_dates = sorted(dates_set)
    print(f"  Loaded {sum(len(v) for v in data.values())} rows for {len(data)} stocks, {len(all_dates)} dates ({time.time()-ts:.1f}s)", flush=True)
    return data, all_dates

def compute_aggs(data, all_dates, target_dates):
    """Compute daily aggregates for target dates."""
    results = {}
    ts = time.time()
    
    for td in target_dates:
        up = down = total = ma20 = n52 = n60 = 0
        total_amt = 0.0
        up5_cnt = up5_total = 0
        
        for code, rows in data.items():
            # Find the index of td in this stock's data
            idx = None
            for i, r in enumerate(rows):
                if r[0] == td:
                    idx = i
                    break
            if idx is None:
                continue
            
            close, high, low, amount, prev_close = rows[idx][1], rows[idx][2], rows[idx][3], rows[idx][4], rows[idx][5]
            if close <= 0:
                continue
            
            total += 1
            total_amt += amount
            
            if prev_close > 0:
                if close > prev_close:
                    up += 1
                elif close < prev_close:
                    down += 1
            
            # MA20: average of last 20 closes (including today)
            start = max(0, idx - 19)
            ma20_closes = [rows[j][1] for j in range(start, idx + 1) if rows[j][1] > 0]
            if ma20_closes and close > sum(ma20_closes) / len(ma20_closes):
                ma20 += 1
            
            # 52-week high: max high of previous 252 days
            prev_252_start = max(0, idx - 252)
            prev_highs = [rows[j][2] for j in range(prev_252_start, idx) if rows[j][2] > 0]
            if prev_highs and high >= max(prev_highs):
                n52 += 1
            
            # 60-day low: min low of previous 60 days
            prev_60_start = max(0, idx - 60)
            prev_lows = [rows[j][3] for j in range(prev_60_start, idx) if rows[j][3] > 0]
            if prev_lows and low <= min(prev_lows):
                n60 += 1
            
            # Up 5 days
            if idx >= 5:
                close5 = rows[idx - 5][1]
                if close5 > 0:
                    up5_total += 1
                    if close > close5:
                        up5_cnt += 1
        
        results[td] = (up, down, total, ma20, n52, n60, total_amt, up5_cnt, up5_total)
    
    print(f"  Computed {len(target_dates)} dates ({time.time()-ts:.1f}s)", flush=True)
    return results

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    cur.execute(CREATE_TABLE)
    conn.commit()
    
    # Determine target dates
    cur.execute("SELECT DISTINCT trade_date FROM stocks_daily_k WHERE code LIKE '0%' OR code LIKE '3%' OR code LIKE '6%' ORDER BY trade_date")
    all_dates = [str(r[0]) for r in cur.fetchall()]
    
    if "--all" in sys.argv:
        target = all_dates
    elif "--last" in sys.argv:
        idx = sys.argv.index("--last")
        n = int(sys.argv[idx + 1]) if idx + 1 < len(sys.argv) else 60
        target = all_dates[-n:]
    elif len(sys.argv) > 1 and sys.argv[1].startswith("202"):
        target = [d for d in all_dates if d == sys.argv[1]]
    else:
        target = all_dates[-1:]
    
    print(f"Target: {len(target)} dates ({target[0]} ~ {target[-1]})", flush=True)
    
    # Data window: need 400 days before first target date for 252-day lookback
    from datetime import date, timedelta
    first_date = date.fromisoformat(target[0])
    min_date = (first_date - timedelta(days=400)).isoformat()
    
    data, _ = load_data(cur, min_date)
    aggs = compute_aggs(data, all_dates, target)
    
    # Save
    rows = [(td,) + aggs[td] for td in target if td in aggs]
    execute_values(cur, INSERT_SQL, rows, page_size=50)
    conn.commit()
    
    cur.execute("SELECT COUNT(*), MIN(trade_date), MAX(trade_date) FROM market_daily_agg")
    r = cur.fetchone()
    print(f"✅ market_daily_agg: {r[0]} rows, {r[1]} ~ {r[2]}", flush=True)
    
    cur.close(); conn.close()

if __name__ == "__main__":
    main()
