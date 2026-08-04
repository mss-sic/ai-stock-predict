#!/usr/bin/env python3
"""
复权因子计算器 — 基于 dividend_history 表计算前复权因子(adj_factor)

数据源: dividend_history 表 (由 collect_dividend.py 从东财 datacenter 采集)
输出: 直接更新 stocks_daily_k.adj_factor 列 (需要 P3 v099 迁移先创建该列)

复权因子定义:
  adj_factor = 从该日期到最新日期的所有除权除息调整的累乘
  - 最新交易日 adj_factor = 1.0（当前价格无需调整）
  - 每往前遇到一个除权除息日: adj_factor *= (1 + bonus_ratio + transfer_ratio)
  - 前复权价格 = 不复权价格 × adj_factor

计算流程:
  1. 读取 dividend_history，按股票+除权日排序
  2. 读取 stocks_daily_k 中该股票的全部交易日
  3. 从最新交易日往前遍历，遇到除权日则更新累计调整因子
  4. 批量 UPSERT 到 stocks_daily_k.adj_factor

用法:
  python3 adj_factor_calculator.py                    # 全市场计算
  python3 adj_factor_calculator.py --code 600519      # 单只股票
  python3 adj_factor_calculator.py --codes 600519,000001  # 多只(逗号分隔)
  python3 adj_factor_calculator.py --dry-run          # 仅计算不写入
  python3 adj_factor_calculator.py --export /tmp/adj.csv  # 导出CSV
"""

import os, sys, argparse, csv
from collections import defaultdict
from datetime import date

import psycopg2
from psycopg2.extras import execute_values

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")


def log(msg):
    print(msg, flush=True)


def load_dividends(cur, codes=None):
    """加载分红记录，返回 {code: [(ex_date, bonus_rmb, bonus_ratio, transfer_ratio), ...]} 按日期升序。
    包含所有'实施分配'状态的记录（包括纯派息、送转股）。
    """
    sql = """
        SELECT code, ex_dividend_date, bonus_rmb, bonus_ratio, transfer_ratio
        FROM dividend_history
        WHERE progress = '实施分配'
    """
    params = []
    if codes:
        sql += " AND code = ANY(%s)"
        params.append(codes)

    sql += " ORDER BY code, ex_dividend_date"

    cur.execute(sql, params)
    divs = defaultdict(list)
    for code, ex_date, bonus_rmb, bonus_ratio, transfer_ratio in cur.fetchall():
        b_rmb = float(bonus_rmb or 0)
        b_ratio = float(bonus_ratio or 0)
        t_ratio = float(transfer_ratio or 0)
        # 只保留有实质影响的记录
        if b_rmb > 0.001 or b_ratio > 0 or t_ratio > 0:
            divs[code].append((ex_date, b_rmb, b_ratio, t_ratio))
    return divs


def load_trade_dates_with_close(cur, codes=None):
    """加载每个股票的交易日列表及收盘价，返回 {code: [(date, close), ...]} 按日期升序。"""
    sql = """
        SELECT code, trade_date, close
        FROM stocks_daily_k
        WHERE code !~ '^IDX'
    """
    params = []
    if codes:
        sql += " AND code = ANY(%s)"
        params.append(codes)
    sql += " ORDER BY code, trade_date"

    cur.execute(sql, params)
    dates = defaultdict(list)
    for code, td, close in cur.fetchall():
        dates[code].append((td, float(close or 0)))
    return dates


def compute_adj_factors(trade_data, dividends):
    """
    为每个交易日计算前复权因子 adj_factor。

    前复权逻辑：
      - 最新交易日 adj_factor = 1.0
      - 往前遇到除权日 E，所有 E 之前的交易日乘以调整系数:
        adj *= (1 - bonus_rmb / close_before_ex) * (1 + bonus_ratio + transfer_ratio)
        其中 close_before_ex 是除权日前一个交易日的收盘价

    Args:
      trade_data: [(date, close), ...] 按日期升序
      dividends: [(ex_date, bonus_rmb, bonus_ratio, transfer_ratio), ...] 按日期升序

    Returns:
      {date: adj_factor, ...}
    """
    if not trade_data:
        return {}

    # 构建 date → close 的映射，用于快速查找除权日前一日的收盘价
    date_to_close = {td: cl for td, cl in trade_data}

    result = {}
    div_idx = len(dividends) - 1
    adj = 1.0

    for td, close in reversed(trade_data):
        # 处理所有在 td 日期当天或之后生效的除权日
        while div_idx >= 0 and dividends[div_idx][0] >= td:
            ex_date, bonus_rmb, bonus_ratio, transfer_ratio = dividends[div_idx]
            # 计算除权日前一个交易日的收盘价
            close_before = _find_close_before(date_to_close, ex_date)
            ratio = _calc_adjust_ratio(bonus_rmb, bonus_ratio, transfer_ratio, close_before)
            adj *= ratio
            div_idx -= 1
        result[td] = round(adj, 10)

    return result


def _find_close_before(date_to_close, ex_date):
    """找到除权日前最近一个交易日的收盘价。"""
    # date_to_close 的 key 是 date 类型
    candidates = [d for d in date_to_close if d < ex_date]
    if not candidates:
        return 0
    return date_to_close[max(candidates)]


def _calc_adjust_ratio(bonus_rmb, bonus_ratio, transfer_ratio, close_before):
    """
    计算单次除权的调整系数（前复权）。

    前复权：历史价格向下调整。
      - 股本变动: new_shares = old_shares * (1 + bonus_ratio + transfer_ratio)
        对应价格调整: adj *= 1 / (1 + bonus_ratio + transfer_ratio) 即价格除以扩股比例
        反过来说，历史价格要乘以: 1 / (1 + bonus_ratio + transfer_ratio)

    实际上前复权的公式是：
      除权前价格' = (除权前价格 × (1 + bonus + transfer) + bonus_rmb) / (1 + bonus + transfer)
      → 除权前价格' = 除权前价格 + bonus_rmb / (1 + bonus + transfer)

    复权因子含义：不复权价格 × adj_factor = 前复权价格
    adj_factor = 除权前价格' / 除权前价格

    推导：
      前复权调整系数 = (close_before - bonus_rmb) / (close_before * (1 + bonus_ratio + transfer_ratio))
      但这是对于除权前一日价格而言的。

    简化（业界常用）：
      adj *= (close_before - bonus_rmb) / (close_before * (1 + bonus_ratio + transfer_ratio))
      当 close_before 未知时:
      adj *= 1 / (1 + bonus_ratio + transfer_ratio)  # 仅股本调整
    """
    if close_before <= 0:
        # 无收盘价参考，仅做股本调整
        denom = 1.0 + bonus_ratio + transfer_ratio
        return 1.0 / denom if denom > 0 else 1.0

    # 含现金分红的完整调整
    denom = close_before * (1.0 + bonus_ratio + transfer_ratio)
    if denom <= 0:
        return 1.0
    return (close_before - bonus_rmb) / denom


def upsert_adj_factors(cur, code, factors):
    """批量写入 adj_factor 到 stocks_daily_k。"""
    if not factors:
        return 0

    rows = [(code, td, adj) for td, adj in factors.items()]
    sql = """
        INSERT INTO stocks_daily_k (code, trade_date, adj_factor)
        VALUES %s
        ON CONFLICT (code, trade_date) DO UPDATE SET
            adj_factor = EXCLUDED.adj_factor
    """
    execute_values(cur, sql, rows, page_size=500)
    return len(rows)


def export_csv(path, all_factors):
    """导出所有 adj_factor 到 CSV。"""
    with open(path, 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['code', 'trade_date', 'adj_factor'])
        for code, factors in all_factors.items():
            for td, adj in sorted(factors.items()):
                writer.writerow([code, td.strftime('%Y-%m-%d'), adj])
    log(f"[adj_factor] 导出 {sum(len(v) for v in all_factors.values())} 条到 {path}")


def main():
    parser = argparse.ArgumentParser(description="复权因子计算器")
    parser.add_argument('--code', help='单只股票代码')
    parser.add_argument('--codes', help='多只股票代码,逗号分隔')
    parser.add_argument('--dry-run', action='store_true', help='仅计算不写入数据库')
    parser.add_argument('--export', help='导出CSV路径(不写入数据库)')
    args = parser.parse_args()

    codes = None
    if args.code:
        codes = [args.code]
    elif args.codes:
        codes = [c.strip() for c in args.codes.split(',') if c.strip()]

    label = f"({len(codes)}只: {','.join(codes[:5])}...)" if codes else "(全市场)"

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 1. 加载分红数据
    log(f"[adj_factor] 加载分红数据... {label}")
    dividends = load_dividends(cur, codes)
    stocks_with_div = len(dividends)
    total_divs = sum(len(v) for v in dividends.values())
    log(f"[adj_factor] 有分红记录的股票: {stocks_with_div} 只, 共 {total_divs} 条")

    # 2. 加载交易日及收盘价
    log(f"[adj_factor] 加载交易日...")
    trade_data = load_trade_dates_with_close(cur, codes)
    log(f"[adj_factor] 加载 {len(trade_data)} 只股票的交易日期")

    # 3. 计算 adj_factor
    log(f"[adj_factor] 计算复权因子...")
    all_factors = {}
    total_updates = 0

    for code, td_close_list in sorted(trade_data.items()):
        divs = dividends.get(code, [])
        factors = compute_adj_factors(td_close_list, divs)
        if factors:
            all_factors[code] = factors

            if not args.dry_run and not args.export:
                n = upsert_adj_factors(cur, code, factors)
                total_updates += n

            # 进度输出 (每100只)
            if len(all_factors) % 100 == 0:
                progress_msg = f"[adj_factor] 已处理 {len(all_factors)}/{len(trade_data)} 只"
                if not args.dry_run and not args.export:
                    progress_msg += f", 写入 {total_updates} 条"
                log(progress_msg)

    # 4. 提交或导出
    if args.export:
        export_csv(args.export, all_factors)
    elif args.dry_run:
        total = sum(len(v) for v in all_factors.values())
        log(f"[adj_factor] DRY RUN: 将为 {len(all_factors)} 只股票计算 {total} 条 adj_factor")
        # 打印示例
        for code in sorted(all_factors.keys())[:3]:
            factors = all_factors[code]
            # 取最大和最小的 adj_factor
            items = sorted(factors.items(), key=lambda x: x[0])
            if items:
                log(f"  {code}: {len(items)} 条, "
                    f"最早={items[0][0]} adj={items[0][1]:.6f}, "
                    f"最新={items[-1][0]} adj={items[-1][1]:.6f}")
    else:
        conn.commit()
        total = sum(len(v) for v in all_factors.values())
        div_stocks = len([c for c in all_factors if dividends.get(c)])
        log(f"STAT:records_new={total_updates},records_skip=0,records_err=0,"
            f"stocks_with_div={div_stocks},stocks_total={len(all_factors)}")
        log(f"[adj_factor] ✅ 完成: {len(all_factors)} 只, {total_updates} 条 adj_factor "
            f"({div_stocks} 只有分红调整)")

    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
