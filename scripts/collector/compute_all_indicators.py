#!/usr/bin/env python3
"""
全量技术指标预计算 — 每日盘后批量计算 84 个指标，写入 stock_daily_indicators JSONB 缓存表

数据源:
  stocks_daily_k      — 日K线 (55个技术指标)
  stock_financials    — 财务报表 (7个基本面指标)
  ai_stock_scores     — AI六维评分 (7个)
  algorithm_pick_details — 榜单数据 (4个)
  stock_shareholders  — 股东数据 (2个)
  ai_stock_predictions — 预测数据 (2个)

用法:
  python3 compute_all_indicators.py                 # 增量(最近5天)
  python3 compute_all_indicators.py --days 30       # 最近30天
  python3 compute_all_indicators.py --code 600519   # 单只股票
  python3 compute_all_indicators.py --full          # 全量(谨慎! 耗时数小时)
"""

import os, sys, argparse, json, time, math
from collections import defaultdict
from datetime import date, timedelta

import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

# Hot columns written as standalone SQL columns
HOT_KEYS = {'daily_change', 'pe', 'pb', 'rsi', 'volume_ratio', 'turnover_rate', 'total_market_cap', 'algo_score'}

def log(msg):
    print(msg, flush=True)

def get_conn():
    return psycopg2.connect(PG_DSN)

# ═══════════════════════════════════════════════════════════════
# K-line loading
# ═══════════════════════════════════════════════════════════════

def load_kline(cur, codes=None, recent_days=None, lookback=120):
    """Load K-line data for indicator computation. Returns {code: [(date, o, h, l, c, vol, amt, turnover, pre_close, change_pct), ...]}"""
    sql = """
        SELECT code, trade_date::text, open, high, low, close, volume, amount,
               COALESCE(turnover_rate,0), COALESCE(pre_close,0), COALESCE(change_pct,0),
               COALESCE(adj_factor, 1.0)
        FROM stocks_daily_k
        WHERE close > 0 AND code !~ '^IDX'
    """
    params = []
    if codes:
        sql += " AND code = ANY(%s)"
        params.append(codes)
    if recent_days:
        sql += f" AND trade_date >= CURRENT_DATE - INTERVAL '{recent_days} days'"
    else:
        sql += f" AND trade_date >= CURRENT_DATE - INTERVAL '{lookback} days'"

    sql += " ORDER BY code, trade_date"

    cur.execute(sql, params)
    data = defaultdict(list)
    for row in cur.fetchall():
        code, td, o, h, l, c, vol, amt, turnover, pre_close, chg_pct, adj_f = row
        data[code].append((td, float(o or 0), float(h or 0), float(l or 0),
                          float(c or 0), int(vol or 0), float(amt or 0),
                          float(turnover or 0), float(pre_close or 0), float(chg_pct or 0),
                          float(adj_f or 1.0)))
    return data


# ═══════════════════════════════════════════════════════════════
# Technical Indicator Computations
# ═══════════════════════════════════════════════════════════════

def calc_sma(closes, period):
    """Simple Moving Average for last N closes."""
    if len(closes) < period:
        return 0.0
    return round(sum(closes[-period:]) / period, 4)

def calc_ema(closes, period):
    """Exponential Moving Average."""
    if len(closes) < period:
        return 0.0
    alpha = 2.0 / (period + 1)
    ema = closes[0]
    for c in closes[1:]:
        ema = alpha * c + (1 - alpha) * ema
    return round(ema, 4)

def calc_rsi(closes, period=14):
    """Relative Strength Index."""
    if len(closes) < period + 1:
        return 50.0
    gains = losses = 0.0
    for i in range(len(closes)-period, len(closes)):
        chg = closes[i] - closes[i-1]
        if chg > 0:
            gains += chg
        else:
            losses -= chg
    if losses == 0:
        return 100.0
    rs = gains / losses
    return round(100.0 - 100.0 / (1.0 + rs), 2)

def calc_macd(closes):
    """MACD (12,26,9). Returns (dif, dea, macd_hist)."""
    if len(closes) < 26:
        return (0, 0, 0)
    alpha12, alpha26, alpha9 = 2.0/13.0, 2.0/27.0, 2.0/10.0
    ema12 = ema26 = closes[0]
    dea = 0.0
    for c in closes[1:]:
        ema12 = alpha12 * c + (1-alpha12) * ema12
        ema26 = alpha26 * c + (1-alpha26) * ema26
    dif = ema12 - ema26
    # DEA needs more history; approximate with first DIF
    dea = dif * alpha9
    for _ in range(8):
        dea = alpha9 * dif + (1-alpha9) * dea
    macd_hist = 2.0 * (dif - dea)
    return (round(dif, 4), round(dea, 4), round(macd_hist, 4))

def calc_kdj(highs, lows, closes, n=9):
    """KDJ (9,3,3). Returns (k, d, j)."""
    if len(closes) < n:
        return (50.0, 50.0, 50.0)
    window_h = highs[-n:]
    window_l = lows[-n:]
    window_c = closes[-n:]
    hn = max(window_h)
    ln = min(window_l)
    rsv = 50.0
    if hn != ln:
        rsv = (closes[-1] - ln) / (hn - ln) * 100.0
    # Simplified: use RSV as K, D = K MA3, J = 3K-2D
    k = round(rsv, 2)
    d = round(rsv * 0.8 + 50 * 0.2, 2)  # approximate
    j = round(3 * k - 2 * d, 2)
    return (k, d, j)

def calc_boll(closes, period=20):
    """Bollinger Bands. Returns (upper, mid, lower, position, width)."""
    if len(closes) < period:
        return (0, 0, 0, 50, 0)
    window = closes[-period:]
    mid = sum(window) / period
    variance = sum((x - mid) ** 2 for x in window) / period
    std = math.sqrt(variance)
    upper = mid + 2 * std
    lower = mid - 2 * std
    # Position: where is current price within the band (0-100)
    band_width = upper - lower
    if band_width > 0:
        position = (closes[-1] - lower) / band_width * 100
    else:
        position = 50
    width_pct = (band_width / mid * 100) if mid > 0 else 0
    return (round(upper, 4), round(mid, 4), round(lower, 4),
            round(position, 2), round(width_pct, 2))

def calc_atr(highs, lows, closes, period=14):
    """Average True Range."""
    if len(closes) < period + 1:
        return 0.0
    tr_sum = 0.0
    for i in range(len(closes)-period+1, len(closes)):
        h, l, pc = highs[i], lows[i], closes[i-1]
        tr = max(h-l, abs(h-pc), abs(l-pc))
        tr_sum += tr
    atr_val = tr_sum / period
    atr_pct = (atr_val / closes[-1] * 100) if closes[-1] > 0 else 0
    return (round(atr_val, 4), round(atr_pct, 2))

def calc_cci(highs, lows, closes, period=20):
    """Commodity Channel Index."""
    if len(closes) < period:
        return 0.0
    tp = [(highs[i] + lows[i] + closes[i]) / 3.0 for i in range(-period, 0)]
    ma_tp = sum(tp) / period
    md = sum(abs(x - ma_tp) for x in tp) / period
    if md == 0:
        return 0.0
    return round((tp[-1] - ma_tp) / (0.015 * md), 2)

def calc_williams_r(highs, lows, closes, period=14):
    """Williams %R."""
    if len(closes) < period:
        return -50.0
    hh = max(highs[-period:])
    ll = min(lows[-period:])
    if hh == ll:
        return -50.0
    return round((hh - closes[-1]) / (hh - ll) * -100, 2)

def calc_mfi(highs, lows, closes, volumes, period=14):
    """Money Flow Index."""
    if len(closes) < period + 1 or len(volumes) < period + 1:
        return 50.0
    pos_mf = neg_mf = 0.0
    for i in range(len(closes)-period, len(closes)):
        tp = (highs[i] + lows[i] + closes[i]) / 3.0
        prev_tp = (highs[i-1] + lows[i-1] + closes[i-1]) / 3.0
        mf = tp * (volumes[i] or 1)
        if tp > prev_tp:
            pos_mf += mf
        else:
            neg_mf += mf
    if neg_mf == 0:
        return 100.0
    mfr = pos_mf / neg_mf
    return round(100.0 - 100.0 / (1.0 + mfr), 2)

def calc_adx(highs, lows, closes, period=14):
    """Average Directional Index."""
    if len(closes) < period * 2:
        return (0, 0, 0)
    tr_sum = pdm_sum = ndm_sum = 0.0
    for i in range(len(closes)-period*2, len(closes)):
        h, l, ph, pl = highs[i], lows[i], highs[i-1], lows[i-1]
        tr = max(h-l, abs(h-ph), abs(l-pl))
        up = h - ph
        dn = pl - l
        pdm = up if up > dn and up > 0 else 0
        ndm = dn if dn > up and dn > 0 else 0
        tr_sum += tr
        pdm_sum += pdm
        ndm_sum += ndm
    if tr_sum == 0:
        return (0, 0, 0)
    pdi = pdm_sum / tr_sum * 100
    ndi = ndm_sum / tr_sum * 100
    dx_sum = 0.0
    if pdi + ndi > 0:
        dx_sum = abs(pdi - ndi) / (pdi + ndi) * 100
    adx_val = dx_sum  # simplified
    return (round(pdi, 2), round(ndi, 2), round(adx_val, 2))

def calc_psy(closes, period=12):
    """Psychological Line — % of up days."""
    if len(closes) < period:
        return 50.0
    ups = sum(1 for i in range(len(closes)-period, len(closes)) if closes[i] > closes[i-1])
    return round(ups / period * 100, 2)

def calc_momentum(closes, period):
    """Price momentum over period."""
    if len(closes) < period + 1:
        return 0.0
    return round((closes[-1] - closes[-period-1]) / closes[-period-1] * 100, 2)

def calc_max_drawdown(closes, period=20):
    """Maximum drawdown over period."""
    if len(closes) < period:
        return 0.0
    peak = closes[-period]
    max_dd = 0.0
    for c in closes[-period:]:
        if c > peak:
            peak = c
        dd = (peak - c) / peak * 100
        if dd > max_dd:
            max_dd = dd
    return round(max_dd, 2)

def calc_price_position(closes, period):
    """Current price position within period range (0-100)."""
    if len(closes) < period:
        return 50.0
    window = closes[-period:]
    hh, ll = max(window), min(window)
    if hh == ll:
        return 50.0
    return round((closes[-1] - ll) / (hh - ll) * 100, 2)


# ═══════════════════════════════════════════════════════════════
# Main computation engine
# ═══════════════════════════════════════════════════════════════

def compute_indicators(kline_data, write_days=5):
    """Compute all technical indicators for each stock. Returns {code: {date: {indicators}}}."""
    results = defaultdict(lambda: defaultdict(dict))
    total = len(kline_data)

    for idx, (code, rows) in enumerate(sorted(kline_data.items())):
        if len(rows) < 30:
            continue

        # Extract arrays for computation
        dates    = [r[0] for r in rows]
        opens    = [r[1] for r in rows]
        highs    = [r[2] for r in rows]
        lows     = [r[3] for r in rows]
        closes   = [r[4] for r in rows]
        volumes  = [r[5] for r in rows]
        amounts  = [r[6] for r in rows]
        turnovers= [r[7] for r in rows]
        pre_closes=[r[8] for r in rows]
        chg_pcts = [r[9] for r in rows]
        adj_factors=[r[10] for r in rows]

        n = len(closes)

        for i in range(max(20, n-write_days), n):  # Compute for last N days (needs 20+ history)
            dt = dates[i]
            w_closes   = closes[:i+1]
            w_highs    = highs[:i+1]
            w_lows     = lows[:i+1]
            w_volumes  = volumes[:i+1]
            w_turnovers= turnovers[:i+1]

            indicators = {}

            # -- Moving Averages --
            indicators['ma_5']  = calc_sma(w_closes, 5)
            indicators['ma_10'] = calc_sma(w_closes, 10)
            indicators['ma_20'] = calc_sma(w_closes, 20)
            indicators['ma_30'] = calc_sma(w_closes, 30)
            indicators['ma_60'] = calc_sma(w_closes, 60)

            # -- Daily change --
            indicators['daily_change'] = round(chg_pcts[i], 4)

            # -- Momentum --
            indicators['momentum_5']  = calc_momentum(w_closes, 5)
            indicators['momentum_20'] = calc_momentum(w_closes, 20)

            # -- MA deviation --
            ma20v = indicators.get('ma_20', 0)
            if ma20v > 0:
                indicators['ma_deviation'] = round((closes[i] - ma20v) / ma20v * 100, 2)

            # -- MACD --
            dif, dea_v, macd_h = calc_macd(w_closes)
            indicators['macd_dif'] = dif
            indicators['macd_dea'] = dea_v
            indicators['macd'] = macd_h

            # -- RSI --
            indicators['rsi']    = calc_rsi(w_closes, 14)
            indicators['rsi_6']  = calc_rsi(w_closes, 6)
            indicators['rsi_12'] = calc_rsi(w_closes, 12)
            indicators['rsi_24'] = calc_rsi(w_closes, 24)

            # -- KDJ --
            k, d, j = calc_kdj(w_highs, w_lows, w_closes)
            indicators['kdj_k'] = k
            indicators['kdj_d'] = d
            indicators['kdj_j'] = j

            # -- Bollinger Bands --
            upper, mid, lower, bpos, bwidth = calc_boll(w_closes)
            indicators['boll_upper'] = upper
            indicators['boll_middle'] = mid
            indicators['boll_lower'] = lower
            indicators['boll_position'] = bpos
            indicators['boll_width'] = bwidth

            # -- ATR --
            atr_v, atr_p = calc_atr(w_highs, w_lows, w_closes)
            indicators['atr'] = atr_v
            indicators['atr_pct'] = atr_p

            # -- CCI / Williams / MFI --
            indicators['cci'] = calc_cci(w_highs, w_lows, w_closes)
            indicators['williams_r'] = calc_williams_r(w_highs, w_lows, w_closes)
            indicators['mfi'] = calc_mfi(w_highs, w_lows, w_closes, w_volumes)

            # -- Volume --
            indicators['volume_ratio'] = round(w_volumes[-1] / (sum(w_volumes[-6:-1]) / 5.0), 2) if sum(w_volumes[-6:-1]) > 0 else 0
            indicators['volume_ma_ratio'] = round(w_volumes[-1] / (sum(w_volumes[-21:-1]) / 20.0), 2) if sum(w_volumes[-21:-1]) > 0 else 0
            indicators['turnover_rate'] = round(turnovers[i], 4) if i < len(turnovers) else 0

            # -- ADX / DMI --
            pdi, ndi, adv = calc_adx(w_highs, w_lows, w_closes)
            indicators['adx'] = adv
            indicators['dmi_plus'] = pdi
            indicators['dmi_minus'] = ndi

            # -- PSY --
            psy12 = calc_psy(w_closes, 12)
            indicators['psy_12'] = psy12
            # psy_ma: rough 6-period avg of PSY
            indicators['psy_ma'] = round(psy12 * 0.8 + 50 * 0.2, 2)

            # -- Drawdown / Position --
            indicators['drawdown_20'] = calc_max_drawdown(w_closes, 20)
            indicators['price_position_20'] = calc_price_position(w_closes, 20)
            indicators['price_position_60'] = calc_price_position(w_closes, 60)

            # -- Trend strength --
            if ma20v > 0 and indicators.get('ma_60', 0) > 0:
                indicators['trend_strength'] = round(abs(ma20v - indicators['ma_60']) / indicators['ma_60'], 4)

            # -- MA convergence (how close are the MAs) --
            mas_present = [indicators.get(f'ma_{p}', 0) for p in [5,10,20,30,60] if indicators.get(f'ma_{p}', 0) > 0]
            if len(mas_present) >= 3:
                indicators['ma_convergence'] = round(max(mas_present) - min(mas_present), 4)

            # -- Up days ratio --
            ups = sum(1 for j in range(max(0, i-20), i) if closes[j] > closes[j-1]) if i >= 1 else 0
            indicators['up_days_ratio'] = round(ups / min(20, i) * 100, 2) if i > 0 else 50

            # -- New high 20 --
            if i >= 20:
                recent_high = max(closes[i-20:i])
                indicators['new_high_20'] = 1 if closes[i] >= recent_high else 0

            # -- Gap --
            if i > 0 and pre_closes[i] > 0:
                indicators['gap_pct'] = round((opens[i] - pre_closes[i]) / pre_closes[i] * 100, 2)

            # -- High-low range --
            if pre_closes[i] > 0:
                indicators['high_low_range'] = round((highs[i] - lows[i]) / pre_closes[i] * 100, 2)

            # -- VWAP deviation --
            if amounts[i] > 0 and volumes[i] > 0:
                vwap = amounts[i] / volumes[i]
                if vwap > 0:
                    indicators['vwap_deviation'] = round((closes[i] - vwap) / vwap * 100, 2)

            # -- Volume trend (volume direction over 5 days) --
            if i >= 5:
                v_avg_old = sum(volumes[max(0,i-10):max(0,i-5)]) / max(1, min(5, i))
                v_avg_new = sum(volumes[max(0,i-5):i+1]) / max(1, min(5, i+1))
                if v_avg_old > 0:
                    indicators['volume_trend'] = round((v_avg_new - v_avg_old) / v_avg_old, 2)

            # -- Index relative (approx: use change_pct as proxy) --
            indicators['index_relative'] = round(chg_pcts[i], 4)  # placeholder

            # -- Consecutive days (simplified: 1 if up, 0 if down) --
            if i > 0 and closes[i] > closes[i-1]:
                indicators['consecutive_days'] = 1

            # -- MA cross (simplified: 1 if ma5>ma20, -1 otherwise) --
            ma5v  = indicators.get('ma_5', 0)
            ma20v2 = indicators.get('ma_20', 0)
            indicators['ma_cross'] = 1 if ma5v > 0 and ma20v2 > 0 and ma5v > ma20v2 else 0
            indicators['ema_cross'] = indicators['ma_cross']  # same pattern

            # -- Bollinger squeeze (100-day) --
            indicators['boll_squeeze'] = 0  # needs 100 days history

            # -- streak_count, signal_value, net_flow_ratio, buy_sell_ratio --
            # These come from other tables, set to 0 here
            indicators['streak_count'] = 0
            indicators['signal_value'] = 0

            # Store adj_factor
            if i < len(adj_factors):
                indicators['adj_factor'] = round(adj_factors[i], 8)

            # Store result
            results[code][dt] = indicators

        if (idx + 1) % 500 == 0:
            log(f"[indicator] computed {idx+1}/{total} stocks | PROGRESS:{idx+1}/{total}")

    return results


# ═══════════════════════════════════════════════════════════════
# Fundamental / AI / Pick data loaders
# ═══════════════════════════════════════════════════════════════

def load_financials(cur, codes=None):
    """Load latest financial data per stock. Returns {code: {field: value}}."""
    sql = """
        SELECT DISTINCT ON (code) code, roe, eps, revenue_growth, profit_growth,
               gross_margin, net_margin, debt_ratio
        FROM stock_financials
        WHERE report_date::date <= CURRENT_DATE
        ORDER BY code, report_date DESC
    """
    cur.execute(sql)
    result = {}
    for row in cur.fetchall():
        code = row[0]
        result[code] = {
            'roe': float(row[1] or 0), 'eps': float(row[2] or 0),
            'revenue_growth': float(row[3] or 0), 'profit_growth': float(row[4] or 0),
            'gross_margin': float(row[5] or 0), 'net_margin': float(row[6] or 0),
            'debt_ratio': float(row[7] or 0),
        }
    return result

def load_daily_indicators(cur, date_strs, codes=None):
    """Load PE/PB/PS/市值 for all target dates in one query."""
    sql = """
        SELECT code, trade_date::text, pe, pb, ps, total_market_cap
        FROM stocks_daily_indicator
        WHERE trade_date = ANY(%s::date[])
    """
    if codes:
        sql += " AND code = ANY(%s)"
        cur.execute(sql, (date_strs, codes))
    else:
        cur.execute(sql, (date_strs,))
    result = {}
    for row in cur.fetchall():
        result[(row[0], row[1])] = {
            'pe': float(row[2] or 0), 'pb': float(row[3] or 0),
            'ps': float(row[4] or 0), 'total_market_cap': float(row[5] or 0),
        }
    return result


def load_ai_scores(cur, date_str):
    """Load AI scores for a specific date. Returns {code: {field: value}}."""
    cur.execute("""
        SELECT code, composite_score, fundamental_score, technical_score,
               valuation_score, growth_score, industry_score, capital_score
        FROM ai_stock_scores
        WHERE analyzed_at::date = %s::date
    """, (date_str,))
    result = {}
    for row in cur.fetchall():
        result[row[0]] = {
            'ai_score': float(row[1] or 0), 'ai_fundamental': float(row[2] or 0),
            'ai_technical': float(row[3] or 0), 'ai_valuation': float(row[4] or 0),
            'ai_growth': float(row[5] or 0), 'ai_industry': float(row[6] or 0),
            'ai_capital': float(row[7] or 0),
        }
    return result

def load_pick_counts(cur, date_str, days=20):
    """Load pick counts for last N days. Returns {code: {pick_count_5d, pick_count_20d, algo_score}}."""
    cur.execute("""
        SELECT stock_code,
               COUNT(*) FILTER (WHERE pick_date >= %s::date - INTERVAL '4 days') as cnt_5d,
               COUNT(*) FILTER (WHERE pick_date >= %s::date - INTERVAL '19 days') as cnt_20d,
               AVG(COALESCE(score, 0)) as avg_score
        FROM algorithm_pick_details
        WHERE pick_date >= %s::date - INTERVAL '19 days' AND pick_date <= %s::date
        GROUP BY stock_code
    """, (date_str, date_str, date_str, date_str))
    result = {}
    for row in cur.fetchall():
        result[row[0]] = {
            'pick_count_5d': int(row[1] or 0),
            'pick_count_20d': int(row[2] or 0),
            'algo_score': round(float(row[3] or 0), 2),
        }
    return result

def load_shareholders(cur):
    """Load latest shareholder data. Returns {code: {shareholder_change, inst_hold_ratio}}."""
    cur.execute("""
        SELECT DISTINCT ON (code) code,
               COALESCE(holder_change, 0), COALESCE(inst_hold_ratio, 0)
        FROM stock_shareholders
        ORDER BY code, report_date DESC
    """)
    result = {}
    for row in cur.fetchall():
        result[row[0]] = {'shareholder_change': float(row[1] or 0), 'inst_hold_ratio': float(row[2] or 0)}
    return result


# ═══════════════════════════════════════════════════════════════
# UPSERT
# ═══════════════════════════════════════════════════════════════

def build_indicator_row(code, date_str, indicators, financials, daily_indicators, ai_scores, picks, shareholders):
    """Merge all data sources and return one batch-upsert row."""
    merged = dict(indicators)

    # Merge daily indicators (PE/PB/PS/市值 from stocks_daily_indicator — per-date)
    daily = daily_indicators.get((code, date_str), {})
    for key in ('pe', 'pb', 'ps', 'total_market_cap'):
        value = daily.get(key, 0)
        if value and merged.get(key, 0) == 0:
            merged[key] = value

    # Merge financials (closest report before date)
    if code in financials:
        for k, v in financials[code].items():
            if v and k not in merged:
                merged[k] = v

    # Merge AI scores
    if code in ai_scores:
        for k, v in ai_scores[code].items():
            if v:
                merged[k] = v

    # Merge pick counts
    if code in picks:
        for k, v in picks[code].items():
            merged[k] = v

    # Merge shareholders
    if code in shareholders:
        for k, v in shareholders[code].items():
            if v:
                merged[k] = v

    # Extract hot columns
    daily_change = merged.get('daily_change', 0)
    pe = merged.get('pe', 0)
    pb = merged.get('pb', 0)
    rsi = merged.get('rsi', 0)
    volume_ratio = merged.get('volume_ratio', 0)
    turnover_rate = merged.get('turnover_rate', 0)
    total_market_cap = merged.get('total_market_cap', 0)
    algo_score = merged.get('algo_score', 0)

    indicators_json = json.dumps(merged)

    return (code, date_str, daily_change, pe, pb, rsi, volume_ratio,
            turnover_rate, total_market_cap, algo_score, indicators_json)


UPSERT_INDICATORS_SQL = """
    INSERT INTO stock_daily_indicators (code, trade_date, daily_change, pe, pb, rsi,
        volume_ratio, turnover_rate, total_market_cap, algo_score, indicators)
    VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        daily_change = EXCLUDED.daily_change,
        pe = EXCLUDED.pe, pb = EXCLUDED.pb, rsi = EXCLUDED.rsi,
        volume_ratio = EXCLUDED.volume_ratio,
        turnover_rate = EXCLUDED.turnover_rate,
        total_market_cap = EXCLUDED.total_market_cap,
        algo_score = EXCLUDED.algo_score,
        indicators = EXCLUDED.indicators,
        computed_at = NOW()
"""


# ═══════════════════════════════════════════════════════════════
# Main
# ═══════════════════════════════════════════════════════════════

def main():
    parser = argparse.ArgumentParser(description="全量技术指标预计算")
    parser.add_argument('--code', help='单只股票')
    parser.add_argument('--days', type=int, default=5, help='增量天数(默认5)')
    parser.add_argument('--full', action='store_true', help='全量计算')
    args = parser.parse_args()

    codes = [args.code] if args.code else None
    recent_days = None if (args.full or args.code) else args.days
    label = f"({args.code})" if args.code else (f"全量" if args.full else f"增量(最近{args.days}天)")

    conn = get_conn()
    cur = conn.cursor()

    t0 = time.time()

    # 1. Load K-line
    log(f"[indicator] 加载K线数据... {label}")
    kline_data = load_kline(cur, codes, None, lookback=250)
    log(f"[indicator] 加载 {len(kline_data)} 只股票 ({time.time()-t0:.1f}s)")

    # 2. Compute technical indicators
    log(f"[indicator] 计算技术指标...")
    results = compute_indicators(kline_data, write_days=args.days)
    log(f"[indicator] 计算完成 {len(results)} 只 ({time.time()-t0:.1f}s)")

    # 3. Load auxiliary data
    target_dates = sorted({dt for dates in results.values() for dt in dates})
    target_date = target_dates[-1] if target_dates else date.today().isoformat()
    log(f"[indicator] 加载辅助数据 (target={target_date})...")
    financials = load_financials(cur)
    daily_inds = load_daily_indicators(cur, target_dates, codes) if target_dates else {}
    ai_scores = load_ai_scores(cur, target_date)
    picks = load_pick_counts(cur, target_date)
    shareholders = load_shareholders(cur)
    log(f"[indicator] 辅助: {len(financials)}财务 {len(daily_inds)}日指标 {len(ai_scores)}AI {len(picks)}榜单 {len(shareholders)}股东")

    # 4. Write to DB
    log(f"[indicator] 写入缓存表...")
    total_rows = 0
    stock_count = 0
    pending_rows = []
    for idx, (code, dates) in enumerate(sorted(results.items())):
        stock_count += 1
        for dt_str in sorted(dates.keys()):
            pending_rows.append(build_indicator_row(
                code, dt_str, dates[dt_str], financials, daily_inds,
                ai_scores, picks, shareholders))

        if (idx + 1) % 200 == 0:
            try:
                execute_values(cur, UPSERT_INDICATORS_SQL, pending_rows, page_size=1000)
                conn.commit()
                total_rows += len(pending_rows)
                pending_rows.clear()
            except Exception as e:
                conn.rollback()
                log(f"[indicator] 批量写入失败 ({idx-199}-{idx}): {e}")
                raise
            log(f"[indicator] {idx+1}/{len(results)} 只, {total_rows} 行 | PROGRESS:{idx+1}/{len(results)}")

    if pending_rows:
        try:
            execute_values(cur, UPSERT_INDICATORS_SQL, pending_rows, page_size=1000)
            conn.commit()
            total_rows += len(pending_rows)
        except Exception as e:
            conn.rollback()
            log(f"[indicator] 最终批次写入失败: {e}")
            raise

    elapsed = time.time() - t0
    log(f"STAT:records_new={total_rows},stocks={stock_count},elapsed={elapsed:.0f}")
    log(f"[indicator] ✅ 完成: {stock_count} 只, {total_rows} 行指标 ({elapsed:.0f}s)")

    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
