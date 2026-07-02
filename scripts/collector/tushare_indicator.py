#!/usr/bin/env python3
"""
Tushare 技术指标采集 — daily_basic 接口全市场单日批量更新

数据源: Tushare pro.daily_basic()
字段映射 & 单位转换:
  pe/pe_ttm → pe/pe_ttm (倍数, 原值)
  pb → pb (倍数, 原值)
  ps/ps_ttm → ps/ps_ttm (倍数, 原值)
  turnover_rate → turnover_rate (%, 原值如0.47=0.47%)
  turnover_rate_f → turnover_rate_f (自由流通换手率, %)
  volume_ratio → volume_ratio (量比, 原值)
  dv_ratio/dv_ttm → dv_ratio/dv_ttm (股息率, %)
  total_share → total_share (总股本, 万股)
  float_share → float_share (流通股本, 万股)
  free_share → free_share (自由流通股本, 万股)
  total_mv → total_market_cap (万元→元 ×10000)
  circ_mv → circulating_market_cap (万元→元 ×10000)

频率限制: 1次/小时 (免费token)

用法:
  python3 tushare_indicator.py                    # 默认最新交易日(从DB推断)
  python3 tushare_indicator.py --date 20260701    # 指定日期
"""
import os, sys, time, argparse
from datetime import date

import tushare as ts
import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
TUSHARE_TOKEN = os.environ.get("TUSHARE_TOKEN", "")

def log(msg):
    print(msg, flush=True)

def get_pro():
    if not TUSHARE_TOKEN:
        log("❌ 未配置 TUSHARE_TOKEN")
        sys.exit(1)
    return ts.pro_api(TUSHARE_TOKEN)

def get_latest_trade_date(cur):
    cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k")
    row = cur.fetchone()
    if row and row[0]:
        return row[0].strftime('%Y%m%d')
    return date.today().strftime('%Y%m%d')

# ── 入库 SQL ─────────────────────────────────────────────────────
UPSERT_SQL = """
    INSERT INTO stocks_daily_indicator (
        code, trade_date, pe, pe_ttm, pb, ps, ps_ttm,
        turnover_rate, turnover_rate_f, volume_ratio,
        dv_ratio, dv_ttm,
        total_share, float_share, free_share,
        total_market_cap, circulating_market_cap, data_source
    ) VALUES %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        pe = EXCLUDED.pe,
        pe_ttm = EXCLUDED.pe_ttm,
        pb = EXCLUDED.pb,
        ps = EXCLUDED.ps,
        ps_ttm = EXCLUDED.ps_ttm,
        turnover_rate = EXCLUDED.turnover_rate,
        turnover_rate_f = EXCLUDED.turnover_rate_f,
        volume_ratio = EXCLUDED.volume_ratio,
        dv_ratio = EXCLUDED.dv_ratio,
        dv_ttm = EXCLUDED.dv_ttm,
        total_share = EXCLUDED.total_share,
        float_share = EXCLUDED.float_share,
        free_share = EXCLUDED.free_share,
        total_market_cap = EXCLUDED.total_market_cap,
        circulating_market_cap = EXCLUDED.circulating_market_cap,
        data_source = EXCLUDED.data_source
"""

def or0(v):
    """None/NaN → 0"""
    try:
        f = float(v)
        return 0.0 if (f != f) else f  # NaN check
    except (TypeError, ValueError):
        return 0.0

def upsert_indicator(cur, df, trade_date):
    rows = []
    bad = 0
    for _, r in df.iterrows():
        raw = str(r['ts_code'])
        code = raw.split('.')[0] if '.' in raw else raw
        if len(code) != 6 or not code.isdigit():
            bad += 1
            continue

        # 市值: 万元 → 元
        total_mv = or0(r.get('total_mv')) * 10000
        circ_mv  = or0(r.get('circ_mv')) * 10000

        rows.append((
            code, trade_date,
            or0(r.get('pe')), or0(r.get('pe_ttm')),
            or0(r.get('pb')),
            or0(r.get('ps')), or0(r.get('ps_ttm')),
            or0(r.get('turnover_rate')), or0(r.get('turnover_rate_f')),
            or0(r.get('volume_ratio')),
            or0(r.get('dv_ratio')), or0(r.get('dv_ttm')),
            or0(r.get('total_share')), or0(r.get('float_share')), or0(r.get('free_share')),
            total_mv, circ_mv,
            'tushare',
        ))

    if rows:
        execute_values(cur, UPSERT_SQL, rows, page_size=200)
    return len(rows), bad

# ── 主流程 ───────────────────────────────────────────────────────
def main():
    parser = argparse.ArgumentParser(description='Tushare 技术指标采集')
    parser.add_argument('--date', type=str, default=None, help='交易日 YYYYMMDD')
    args = parser.parse_args()

    pro = get_pro()
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    trade_date = args.date or get_latest_trade_date(cur)
    log(f"📅 交易日: {trade_date}")

    # 拉取数据
    log(f"📡 正在从 Tushare daily_basic 拉取 {trade_date} 全市场指标...")
    t0 = time.time()
    df = pro.daily_basic(trade_date=trade_date)
    elapsed = time.time() - t0
    log(f"   返回 {len(df)} 条 ({elapsed:.1f}s)")

    if df.empty:
        log(f"⚠️  无数据返回，{trade_date} 可能非交易日")
        cur.close(); conn.close()
        sys.exit(0)

    # 入库
    t0 = time.time()
    n, bad = upsert_indicator(cur, df, trade_date)
    conn.commit()

    # 统计
    cur.execute("SELECT COUNT(*), COUNT(*) FILTER (WHERE pe > 0), COUNT(*) FILTER (WHERE pb > 0), COUNT(*) FILTER (WHERE dv_ratio > 0) FROM stocks_daily_indicator WHERE trade_date = %s AND data_source = 'tushare'", (trade_date,))
    total, with_pe, with_pb, with_dv = cur.fetchone()

    cur.close()
    conn.close()

    elapsed = time.time() - t0
    log(f"\n{'─'*50}")
    log(f"✅ Tushare 技术指标采集完成")
    log(f"   交易日: {trade_date}")
    log(f"   接口返回: {len(df)} 条 (过滤 {bad} 条)")
    log(f"   入库: {n} 条")
    log(f"   有PE: {with_pe} | 有PB: {with_pb} | 有股息率: {with_dv}")
    log(f"   耗时: {elapsed:.1f}s")
    log(f"{'─'*50}")
    print(f"STAT:indicator={n},with_pe={with_pe},with_dv={with_dv}", flush=True)

if __name__ == "__main__":
    main()
