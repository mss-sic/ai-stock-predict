#!/usr/bin/env python3
"""
市场情绪指数计算引擎 (v4 — reads from pre-computed market_daily_agg)
从 stocks_daily_k + stocks_basic + northbound_flow + stock_capital_flow + market_daily_agg 计算11个情绪子指标

依赖: 先运行 precompute_aggs.py 生成 market_daily_agg

用法:
  python3 compute_sentiment.py                    # 最新交易日
  python3 compute_sentiment.py 2026-06-18         # 指定日期
  python3 compute_sentiment.py --last 60          # 最近60天
"""
import os, sys, math, time
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
LIMIT_PCTS = {"sh": 0.10, "sz": 0.10, "kc": 0.20, "cy": 0.20, "bj": 0.30}
ST_LIMIT_PCT = 0.05
BOND_ETF = ["511010", "511090", "511520"]

INSERT_SQL = """
    INSERT INTO market_sentiment (
        trade_date, market_breadth, breadth_score, style_risk_pref, style_risk_score,
        trade_activity, activity_score, profit_effect, profit_score,
        volatility, vol_score, price_strength, strength_score,
        risk_appetite, risk_app_score, limit_sentiment, limit_score,
        sector_diffusion, sector_score, northbound_net, northbound_score,
        capital_flow_net, capital_flow_score, composite_score,
        up_count, down_count, limit_up_count, limit_down_count, board_break_count, total_stocks
    ) VALUES %s
    ON CONFLICT (trade_date) DO UPDATE SET
        market_breadth = EXCLUDED.market_breadth, breadth_score = EXCLUDED.breadth_score,
        style_risk_pref = EXCLUDED.style_risk_pref, style_risk_score = EXCLUDED.style_risk_score,
        trade_activity = EXCLUDED.trade_activity, activity_score = EXCLUDED.activity_score,
        profit_effect = EXCLUDED.profit_effect, profit_score = EXCLUDED.profit_score,
        volatility = EXCLUDED.volatility, vol_score = EXCLUDED.vol_score,
        price_strength = EXCLUDED.price_strength, strength_score = EXCLUDED.strength_score,
        risk_appetite = EXCLUDED.risk_appetite, risk_app_score = EXCLUDED.risk_app_score,
        limit_sentiment = EXCLUDED.limit_sentiment, limit_score = EXCLUDED.limit_score,
        sector_diffusion = EXCLUDED.sector_diffusion, sector_score = EXCLUDED.sector_score,
        northbound_net = EXCLUDED.northbound_net, northbound_score = EXCLUDED.northbound_score,
        capital_flow_net = EXCLUDED.capital_flow_net, capital_flow_score = EXCLUDED.capital_flow_score,
        composite_score = EXCLUDED.composite_score,
        up_count = EXCLUDED.up_count, down_count = EXCLUDED.down_count,
        limit_up_count = EXCLUDED.limit_up_count, limit_down_count = EXCLUDED.limit_down_count,
        board_break_count = EXCLUDED.board_break_count, total_stocks = EXCLUDED.total_stocks,
        created_at = NOW()
"""

def percentile_rank(values, target):
    if not values: return 0.5
    return (sum(1 for v in sorted(values) if v <= target) + sum(1 for v in sorted(values) if v < target)) / (2 * len(values))

def rolling_pcts(series, window=252):
    result = {}
    for i, (d, v) in enumerate(series):
        start = max(0, i - window + 1)
        result[d] = percentile_rank([x[1] for x in series[start:i+1]], v)
    return result

def load_index_ret(cur, code, td, days):
    cur.execute("SELECT close FROM stocks_daily_k WHERE code=%s AND trade_date<=%s ORDER BY trade_date DESC LIMIT %s", (code, td, days+1))
    rows = cur.fetchall()
    if len(rows) >= days+1:
        c, p = float(rows[0][0]), float(rows[days][0])
        return (c-p)/p if p > 0 else 0
    return 0

def load_index_vol(cur, code, td, days=20):
    cur.execute("SELECT close FROM stocks_daily_k WHERE code=%s AND trade_date<=%s ORDER BY trade_date DESC LIMIT %s", (code, td, days+1))
    rows = cur.fetchall()
    if len(rows) < 2: return 0
    closes = [float(r[0]) for r in reversed(rows)]
    rets = [(closes[i]-closes[i-1])/closes[i-1] for i in range(1, len(closes))]
    if not rets: return 0
    mean = sum(rets)/len(rets)
    return math.sqrt(sum((r-mean)**2 for r in rets)/len(rets))

def load_nb(cur, td):
    cur.execute("SELECT COALESCE(total_net,0) FROM northbound_daily_view WHERE trade_date=%s", (td,))
    r = cur.fetchone()
    return float(r[0]) if r else 0

def load_capflow(cur, td):
    cur.execute("SELECT COALESCE(SUM(main_net),0) FROM stock_capital_flow WHERE trade_date=%s", (td,))
    r = cur.fetchone()
    return float(r[0]) if r else 0

def limit_prices(prev_close, bt, is_st):
    pct = ST_LIMIT_PCT if is_st else LIMIT_PCTS.get(bt, 0.10)
    return round(prev_close*(1+pct), 2), round(max(0.01, prev_close*(1-pct)), 2)

def load_agg(cur, td):
    cur.execute("SELECT * FROM market_daily_agg WHERE trade_date=%s", (td,))
    r = cur.fetchone()
    if not r: return None
    return {
        "up": int(r[1] or 0), "down": int(r[2] or 0), "total": int(r[3] or 0),
        "ma20": int(r[4] or 0), "n52": int(r[5] or 0), "n60": int(r[6] or 0),
        "amt": float(r[7] or 0), "up5": int(r[8] or 0), "up5_total": int(r[9] or 0),
    }

def avg20_amt(cur, td):
    cur.execute("SELECT AVG(total_amount) FROM (SELECT total_amount FROM market_daily_agg WHERE trade_date < %s ORDER BY trade_date DESC LIMIT 20) t", (td,))
    r = cur.fetchone()
    return float(r[0]) if r and r[0] else 0

def compute_one(cur, td_str, stocks, stock_lookup):
    a = load_agg(cur, td_str)
    if not a:
        return None
    total = max(a["total"], 1)

    # 1. Breadth
    breadth = 0.5 * (a["up"] / total) + 0.5 * (a["ma20"] / total)

    # 2. Style Risk
    r5_1000 = load_index_ret(cur, "IDX000852", td_str, 5)
    r5_300 = load_index_ret(cur, "IDX000300", td_str, 5)
    r20_1000 = load_index_ret(cur, "IDX000852", td_str, 20)
    r20_300 = load_index_ret(cur, "IDX000300", td_str, 20)
    style_risk = 0.5*(r5_1000-r5_300) + 0.5*(r20_1000-r20_300)

    # 3. Activity
    avg20 = avg20_amt(cur, td_str)
    activity = a["amt"] / avg20 if avg20 > 0 else 1.0

    # 4. Profit
    up5_ratio = a["up5"] / max(a["up5_total"], 1)
    n60_ratio = a["n60"] / total
    profit = 0.6 * up5_ratio + 0.4 * (1 - n60_ratio)

    # 5. Volatility
    vol = load_index_vol(cur, "IDX000300", td_str, 20)
    annual_vol = vol * math.sqrt(252)
    ret20 = load_index_ret(cur, "IDX000300", td_str, 20)
    direction = 1 if ret20 > 0 else -1
    vol_adj = annual_vol * (1 + 0.2 * direction * min(abs(ret20)*10, 1))

    # 6. Price Strength
    strength = a["n52"] / total

    # 7. Risk Appetite
    ret300 = load_index_ret(cur, "IDX000300", td_str, 20)
    bond_rets = [load_index_ret(cur, c, td_str, 20) for c in BOND_ETF]
    risk_app = ret300 - (sum(bond_rets)/len(bond_rets) if bond_rets else 0)

    # 8. Limit Sentiment
    cur.execute("""
        SELECT k.code, k.close, k.high, k.low, kp.close as prev_close
        FROM stocks_daily_k k
        JOIN LATERAL (SELECT k2.close FROM stocks_daily_k k2 WHERE k2.code=k.code AND k2.trade_date<%s ORDER BY k2.trade_date DESC LIMIT 1) kp ON TRUE
        WHERE k.trade_date=%s
    """, (td_str, td_str))
    daily = {r[0]: {"close": float(r[1] or 0), "high": float(r[2] or 0), "low": float(r[3] or 0), "prev_close": float(r[4] or 0)} for r in cur.fetchall()}

    lu_set, ld_cnt, bb_cnt = set(), 0, 0
    for code, d in daily.items():
        if d["prev_close"] <= 0: continue
        st = stock_lookup.get(code)
        if not st: continue
        lu, ld = limit_prices(d["prev_close"], st[0], st[1])
        if d["high"] >= lu:
            if d["close"] >= lu: lu_set.add(code)
            else: bb_cnt += 1
        if ld > 0 and d["close"] <= ld: ld_cnt += 1

    lu_ratio = len(lu_set) / total
    touched = len(lu_set) + bb_cnt
    anti_break = 1 - (bb_cnt / max(touched, 1))

    # 昨涨停收益
    cur.execute("SELECT trade_date FROM stocks_daily_k WHERE trade_date < %s ORDER BY trade_date DESC LIMIT 1", (td_str,))
    r = cur.fetchone()
    yest_ret = 0
    if r:
        yest_date = str(r[0])
        cur.execute("""
            SELECT k.code, k.close, kp.close as prev_close
            FROM stocks_daily_k k
            JOIN LATERAL (SELECT k2.close FROM stocks_daily_k k2 WHERE k2.code=k.code AND k2.trade_date<%s ORDER BY k2.trade_date DESC LIMIT 1) kp ON TRUE
            WHERE k.trade_date=%s
        """, (yest_date, yest_date))
        yest_daily = {r2[0]: {"close": float(r2[1] or 0), "prev_close": float(r2[2] or 0)} for r2 in cur.fetchall()}
        yest_up = set()
        for code, yd in yest_daily.items():
            if yd["prev_close"] <= 0: continue
            st = stock_lookup.get(code)
            if not st: continue
            lu, _ = limit_prices(yd["prev_close"], st[0], st[1])
            if yd["close"] >= lu: yest_up.add(code)
        rets = []
        for code in yest_up:
            d = daily.get(code)
            if d and d["prev_close"] > 0:
                rets.append((d["close"]-d["prev_close"])/d["prev_close"])
        yest_ret = sum(rets)/len(rets) if rets else 0
    yest_score = max(0, min(1, (yest_ret + 0.05) / 0.10))
    limit_sent = 0.4 * lu_ratio + 0.3 * anti_break + 0.3 * yest_score

    # 9. Sector Diffusion
    cur.execute("""
        WITH ind_chg AS (
            SELECT cb.concept_name, AVG((k.close - kc.prev_c) / NULLIF(kc.prev_c, 0)) as avg_chg
            FROM stocks_daily_k k
            JOIN stock_concepts sc ON sc.code = k.code
            JOIN concept_boards cb ON sc.concept_code = cb.concept_code AND cb.concept_type = 'industry'
            CROSS JOIN LATERAL (SELECT k2.close as prev_c FROM stocks_daily_k k2 WHERE k2.code=k.code AND k2.trade_date<%s ORDER BY k2.trade_date DESC LIMIT 1) kc
            WHERE k.trade_date = %s GROUP BY cb.concept_name
        ) SELECT COUNT(*) FROM ind_chg WHERE avg_chg > 0
    """, (td_str, td_str))
    up_ind = cur.fetchone()[0]
    cur.execute("SELECT COUNT(*) FROM concept_boards WHERE concept_type='industry'")
    total_ind = cur.fetchone()[0]
    sector_diff = up_ind / max(total_ind, 1)

    # 10 & 11
    nb = load_nb(cur, td_str)
    cf = load_capflow(cur, td_str)

    return {
        "trade_date": td_str,
        "breadth": breadth, "style_risk": style_risk, "activity": activity,
        "profit": profit, "volatility": vol_adj, "strength": strength,
        "risk_appetite": risk_app, "limit_sent": limit_sent, "sector_diff": sector_diff,
        "northbound": nb, "capital_flow": cf,
        "up": a["up"], "down": a["down"],
        "lu": len(lu_set), "ld": ld_cnt, "bb": bb_cnt,
        "total": total,
    }

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("SELECT code, COALESCE(board_type,'sh'), COALESCE(is_st,false) FROM stocks_basic ORDER BY code")
    stocks = [(r[0], r[1], r[2]) for r in cur.fetchall()]
    stock_lookup = {s[0]: (s[1], s[2]) for s in stocks}

    cur.execute("SELECT trade_date FROM market_daily_agg ORDER BY trade_date")
    all_dates = [str(r[0]) for r in cur.fetchall()]

    if "--last" in sys.argv:
        idx = sys.argv.index("--last")
        n = int(sys.argv[idx+1]) if idx+1 < len(sys.argv) else 60
        dates = all_dates[-n:]
    elif len(sys.argv) > 1 and sys.argv[1].startswith("202"):
        dates = [d for d in all_dates if d == sys.argv[1]]
    else:
        dates = all_dates[-1:] if all_dates else []

    if not dates:
        print("❌ 请先运行 precompute_aggs.py 生成 market_daily_agg", flush=True)
        return

    print(f"股票: {len(stocks)} | 日期: {len(dates)}", flush=True)

    raw = []
    ts = time.time()
    for i, td in enumerate(dates):
        d = compute_one(cur, td, stocks, stock_lookup)
        if d:
            raw.append(d)
        if (i+1) % 10 == 0 or i == len(dates)-1:
            print(f"  {i+1}/{len(dates)} ({time.time()-ts:.0f}s)", flush=True)

    keys = ["breadth","style_risk","activity","profit","volatility","strength",
            "risk_appetite","limit_sent","sector_diff","northbound","capital_flow"]
    # Optimized weights (backtested, 2026-06-20) — northbound/capital_flow excluded
    _w = {
        "breadth": 0.183, "style_risk": 0.175, "activity": 0.049,
        "profit": 0.060, "volatility": 0.179, "strength": 0.049,
        "risk_appetite": 0.049, "limit_sent": 0.090, "sector_diff": 0.168,
        "northbound": 0, "capital_flow": 0,
    }
    score_maps = {}
    for k in keys:
        series = [(d["trade_date"], d[k]) for d in raw]
        score_maps[k] = rolling_pcts(series)

    rows = []
    for d in raw:
        td = d["trade_date"]
        composite = 0
        row = [td]
        for k in keys:
            pct = score_maps[k].get(td, 0.5)
            score = round(pct * 100, 2)
            row.append(d[k])
            row.append(score)
            composite += score * _w.get(k, 0)
        composite = round(composite, 2)
        row.append(composite)
        row.extend([d["up"], d["down"], d["lu"], d["ld"], d["bb"], d["total"]])
        rows.append(tuple(row))

    execute_values(cur, INSERT_SQL, rows, page_size=50)
    conn.commit()

    if raw:
        latest = raw[-1]
        print(f"\n✅ {len(raw)} days ({time.time()-ts:.0f}s)", flush=True)
        print(f"  Latest {latest['trade_date']}: composite={round(sum(score_maps[k].get(latest['trade_date'],0.5)*100*_w.get(k,0) for k in keys),2)}", flush=True)

    cur.close(); conn.close()

if __name__ == "__main__":
    main()
