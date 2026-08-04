#!/usr/bin/env python3
"""
现金流数据回填 — 为已有财务数据但缺少现金流的股票补充 llb 数据

数据源: 新浪财经 API (llb=现金流量表)
策略: 按市值降序处理，断点续传（已补的跳过），分批提交

用法:
  python3 backfill_cashflow.py                  # 首批 200 只
  python3 backfill_cashflow.py --batch 500      # 500 只
  python3 backfill_cashflow.py --all            # 全部（慎用，约5000只）
"""

import os, sys, time, argparse
import psycopg2, requests

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"


def log(msg):
    print(msg, flush=True)


def sina_financial_report(code, report_type="llb", num=8):
    """从新浪财经 API 拉取财务报表。"""
    prefix = "sh" if code.startswith("6") else "sz"
    paper_code = f"{prefix}{code}"
    url = "https://quotes.sina.cn/cn/api/openapi.php/CompanyFinanceService.getFinanceReport2022"
    params = {
        "paperCode": paper_code,
        "source": report_type, "type": "0", "page": "1", "num": str(num),
    }
    try:
        r = requests.get(url, params=params, headers={"User-Agent": UA}, timeout=15)
        report_list = r.json().get("result", {}).get("data", {}).get("report_list", {}) or {}
    except Exception:
        return {}

    result = {}
    for period in sorted(report_list.keys(), reverse=True)[:num]:
        obj = report_list[period]
        period_str = f"{period[:4]}-{period[4:6]}-{period[6:8]}"
        items = {}
        for it in obj.get("data", []) or []:
            title = it.get("item_title", "")
            val_str = it.get("item_value", "")
            if not title or val_str is None:
                continue
            try:
                items[title] = float(val_str)
            except Exception:
                items[title] = 0
        result[period_str] = items
    return result


def extract_cashflow(llb_data):
    """从现金流量表提取关键指标。返回 {report_date: {operating_cf, investing_cf, financing_cf, net_cash_flow}}"""
    result = {}
    for report_date, items in llb_data.items():
        cf = {
            'operating_cf': 0, 'investing_cf': 0,
            'financing_cf': 0, 'net_cash_flow': 0,
        }
        for key, val in items.items():
            v = float(val) if val else 0
            if '经营活动' in key and '现金流量净额' in key:
                cf['operating_cf'] = v
            elif '投资活动' in key and '现金流量净额' in key:
                cf['investing_cf'] = v
            elif '筹资活动' in key and '现金流量净额' in key:
                cf['financing_cf'] = v
            elif ('现金及现金等价物' in key or '现金及' in key) and '净增加额' in key:
                cf['net_cash_flow'] = v

        if cf['operating_cf'] != 0 or cf['investing_cf'] != 0:
            result[report_date] = cf
    return result


def main():
    parser = argparse.ArgumentParser(description='现金流数据回填')
    parser.add_argument('--batch', type=int, default=200, help='每批处理数量(默认200)')
    parser.add_argument('--all', action='store_true', help='全部(慎用)')
    args = parser.parse_args()

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # 查询缺少现金流数据的股票，按市值降序
    cur.execute("""
        SELECT f.code, COALESCE(i.mcap, 0) as mcap
        FROM (
            SELECT DISTINCT code FROM stock_financials
        ) f
        LEFT JOIN (
            SELECT code, MAX(total_market_cap) as mcap
            FROM stocks_daily_indicator
            WHERE total_market_cap > 0
            GROUP BY code
        ) i ON f.code = i.code
        WHERE f.code NOT IN (
            SELECT DISTINCT code FROM stock_financials
            WHERE operating_cf IS NOT NULL AND operating_cf != 0
        )
        ORDER BY i.mcap DESC NULLS LAST
    """)
    all_codes = [r[0] for r in cur.fetchall()]
    log(f"📊 缺现金流股票: {len(all_codes)} 只")

    if not all_codes:
        log("✅ 所有股票已有现金流数据")
        cur.close(); conn.close()
        return

    codes = all_codes if args.all else all_codes[:args.batch]
    log(f"📋 本次处理: {len(codes)} 只 (共 {len(all_codes)} 只)")
    log(f"🚀 开始采集...")

    done = 0
    total_updated = 0
    start = time.time()

    for i, code in enumerate(codes):
        try:
            llb_data = sina_financial_report(code, "llb", 8)

            cf_metrics = extract_cashflow(llb_data)

            if (i + 1) <= 3:
                log(f"  🔍 {code}: llb {len(llb_data)} periods, cf {len(cf_metrics)} metrics")

            if cf_metrics:
                updated = 0
                for report_date, cf in cf_metrics.items():
                    # 计算衍生指标
                    free_cf = round(cf['operating_cf'] + cf['investing_cf'], 2)

                    # 获取净利润以计算 cf_ratio
                    cur.execute("""
                        SELECT net_profit FROM stock_financials
                        WHERE code = %s AND report_date = %s
                    """, (code, report_date))
                    np_row = cur.fetchone()
                    net_profit = float(np_row[0]) if np_row else 0
                    cf_ratio = round(cf['operating_cf'] / net_profit * 100, 2) if net_profit != 0 else 0

                    cur.execute("""
                        UPDATE stock_financials SET
                            operating_cf = %s, investing_cf = %s, financing_cf = %s,
                            net_cash_flow = %s, free_cf = %s, cf_ratio = %s
                        WHERE code = %s AND report_date = %s
                    """, (cf['operating_cf'], cf['investing_cf'], cf['financing_cf'],
                          cf['net_cash_flow'], free_cf, cf_ratio,
                          code, report_date))
                    updated += 1

                total_updated += updated
                if updated > 0:
                    done += 1

        except Exception as e:
            if (i + 1) <= 3:
                log(f"  ⚠️ {code} error: {e}")

        if (i + 1) % 20 == 0:
            conn.commit()
            elapsed = time.time() - start
            rate = (i + 1) / elapsed if elapsed > 0 else 0
            eta = (len(codes) - i - 1) / rate if rate > 0 else 0
            log(f"  📊 {i+1}/{len(codes)} | 成功 {done} 只 | {total_updated} 期 | {elapsed:.0f}s | ETA {eta:.0f}s")
            log(f"PROGRESS:{i+1}/{len(codes)}")
        time.sleep(0.1)

    conn.commit()

    # 统计
    cur.execute("""
        SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE operating_cf IS NOT NULL AND operating_cf != 0) as has_cf
        FROM stock_financials
    """)
    total, has_cf = cur.fetchone()

    cur.execute("""
        SELECT COUNT(DISTINCT code) FROM stock_financials
        WHERE operating_cf IS NOT NULL AND operating_cf != 0
    """)
    codes_with_cf = cur.fetchone()[0]

    cur.close()
    conn.close()
    elapsed = time.time() - start
    print(f"\n{'─'*50}", flush=True)
    print(f"✅ 现金流回填完成", flush=True)
    print(f"   📥 本次更新: {done} 只 / {total_updated} 期", flush=True)
    print(f"   📊 现金流覆盖率: {has_cf}/{total} 行 ({has_cf/total*100:.1f}%), {codes_with_cf} 只", flush=True)
    print(f"   ⏱️  总耗时: {elapsed:.0f}s", flush=True)
    print(f"{'─'*50}", flush=True)


if __name__ == "__main__":
    main()
