#!/usr/bin/env python3
"""
Tushare 日K线采集 — 全市场单日批量更新

数据源: Tushare pro.daily() 接口（未复权原始行情）
字段映射 & 单位转换:
  ts_code → code (去后缀)
  vol(手) → volume(股) ×100
  amount(千元) → amount(元) ×1000
  pct_chg → change_pct (%, 直接)
  pre_close → pre_close (新字段)
  change → change_amount (新字段, 涨跌额)

交易日判断: 从 stocks_daily_k 表取最新有数据的日期作为参考

用法:
  python3 tushare_kline.py                    # 默认最新交易日
  python3 tushare_kline.py --date 20260701    # 指定日期
  python3 tushare_kline.py --date 20260701 --code 600519  # 单只股票（6位代码）
"""
import os, sys, json, time, argparse
from datetime import date, datetime, timedelta

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
        log("❌ 未配置 TUSHARE_TOKEN 环境变量")
        sys.exit(1)
    return ts.pro_api(TUSHARE_TOKEN)

# ── 交易日解析（从 DB 已有数据推断） ─────────────────────────────
def get_latest_trade_date(cur):
    """从 stocks_daily_k 表获取最近一个交易日。"""
    cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k")
    row = cur.fetchone()
    if row and row[0]:
        return row[0].strftime('%Y%m%d')
    # 兜底：今天
    return date.today().strftime('%Y%m%d')

def find_prev_trade_date(cur, target_date_str):
    """从 DB 找 target_date 或之前最近一个有数据的交易日。"""
    cur.execute("SELECT MAX(trade_date) FROM stocks_daily_k WHERE trade_date <= %s", (target_date_str,))
    row = cur.fetchone()
    if row and row[0]:
        return row[0].strftime('%Y%m%d')
    return None

# ── 数据拉取 ─────────────────────────────────────────────────────
def fetch_daily(pro, trade_date, ts_code=None):
    """拉取指定交易日全市场(或单只)日K数据。"""
    target = ts_code if ts_code else "全市场"
    log(f"📡 正在从 Tushare 拉取 {trade_date} {target} 日K...")
    t0 = time.time()

    if ts_code:
        df = pro.daily(ts_code=ts_code, trade_date=trade_date)
    else:
        df = pro.daily(trade_date=trade_date)

    elapsed = time.time() - t0
    log(f"   返回 {len(df)} 条 ({elapsed:.1f}s)")
    return df

# ── 数据转换与入库 ───────────────────────────────────────────────
UPSERT_SQL = """
    INSERT INTO stocks_daily_k (code, trade_date, open, high, low, close, pre_close, change_amount,
                                change_pct, volume, amount, data_source)
    VALUES %s
    ON CONFLICT (code, trade_date) DO NOTHING
"""

def upsert_kline(cur, df, trade_date):
    """将 Tushare DataFrame 转换为标准格式并 UPSERT。返回入库条数。"""
    rows = []
    bad_codes = 0

    for _, r in df.iterrows():
        raw_code = str(r['ts_code'])
        code = raw_code.split('.')[0] if '.' in raw_code else raw_code

        # 只处理6位数字代码
        if len(code) != 6 or not code.isdigit():
            bad_codes += 1
            continue

        # 单位转换: vol(手→股×100), amount(千元→元×1000)
        vol_gu = int(float(r['vol']) * 100)
        amt_yuan = round(float(r['amount']) * 1000, 2)

        rows.append((
            code, trade_date,
            float(r['open']),
            float(r['high']),
            float(r['low']),
            float(r['close']),
            float(r['pre_close']),
            float(r['change']),
            float(r['pct_chg']),
            vol_gu,
            amt_yuan,
            'tushare',
        ))

    if rows:
        execute_values(cur, UPSERT_SQL, rows, page_size=200)
    return len(rows), bad_codes

# ── 主流程 ───────────────────────────────────────────────────────
def main():
    parser = argparse.ArgumentParser(description='Tushare 日K线采集')
    parser.add_argument('--date', type=str, default=None,
                        help='交易日 YYYYMMDD，默认最近一个交易日(从DB推断)')
    parser.add_argument('--code', type=str, default=None,
                        help='单只股票 6位代码 (如 600519)，默认全市场')
    args = parser.parse_args()

    pro = get_pro()
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 1. 解析交易日
    if args.date:
        trade_date = args.date
        # 校验: 从 DB 确认该日是否有数据（粗略判断是否交易日）
        prev = find_prev_trade_date(cur, trade_date)
        if prev is None:
            log(f"❌ 无法确认 {trade_date} 是否为交易日（DB无参考数据）")
            cur.close(); conn.close()
            sys.exit(1)
        if prev != trade_date:
            log(f"⚠️  DB中 {trade_date} 无K线记录，最近交易日为 {prev}")
            log(f"   将继续尝试拉取 {trade_date}（可能是新交易日）")
    else:
        trade_date = get_latest_trade_date(cur)
        log(f"📅 默认日期: {trade_date} (DB最近交易日)")

    # 2. 构造 ts_code
    ts_code = None
    if args.code:
        code = args.code.strip()
        if len(code) != 6 or not code.isdigit():
            log(f"❌ 无效股票代码: {code}")
            cur.close(); conn.close()
            sys.exit(1)
        if code.startswith(('6', '9')):
            ts_code = f"{code}.SH"
        else:
            ts_code = f"{code}.SZ"
        log(f"🎯 单只模式: {code} → {ts_code}")

    # 3. 拉取
    df = fetch_daily(pro, trade_date, ts_code)

    if df.empty:
        log(f"⚠️  Tushare 返回空数据，{trade_date} 可能非交易日")
        cur.close(); conn.close()
        sys.exit(0)

    # 4. 入库
    t0 = time.time()
    n_inserted, n_bad = upsert_kline(cur, df, trade_date)
    conn.commit()

    # 5. 统计（DO NOTHING 模式下实际新增=入库数，已存在被跳过）
    cur.execute("SELECT COUNT(*) FROM stocks_daily_k WHERE trade_date = %s AND data_source = 'tushare'", (trade_date,))
    ts_count = cur.fetchone()[0]

    cur.execute("SELECT COUNT(*) FROM stocks_daily_k WHERE trade_date = %s", (trade_date,))
    db_total = cur.fetchone()[0]

    # 检查数据源分布
    cur.execute("SELECT data_source, COUNT(*) FROM stocks_daily_k WHERE trade_date = %s GROUP BY data_source", (trade_date,))
    src_stats = cur.fetchall()

    cur.close()
    conn.close()

    elapsed = time.time() - t0
    log(f"\n{'─'*50}")
    log(f"✅ Tushare 日K采集完成")
    log(f"   交易日: {trade_date}")
    log(f"   接口返回: {len(df)} 条 (过滤非数字代码 {n_bad} 条)")
    log(f"   入库(Tushare): {n_inserted} 条 | 该日Tushare总计: {ts_count} 条")
    log(f"   DB该日总计(所有源): {db_total} 条")
    for src, cnt in src_stats:
        log(f"     - {src or 'NULL'}: {cnt} 条")
    log(f"   入库耗时: {elapsed:.1f}s")
    log(f"{'─'*50}")

    # STAT 行供 Go 引擎统计
    print(f"STAT:kline={n_inserted},total={db_total}", flush=True)

if __name__ == "__main__":
    main()
