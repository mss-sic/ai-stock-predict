#!/usr/bin/env python3
"""
财务数据全量回填脚本
从新浪财经API采集所有A股的利润表+资产负债表
支持断点续传（跳过已有数据的股票）
"""
import os, sys, time, psycopg2, requests
from datetime import date

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

def sina_financial_report(code, report_type="lrb", num=8):
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
    except:
        return {}
    
    result = {}
    for period in sorted(report_list.keys(), reverse=True)[:num]:
        obj = report_list[period]
        period_str = f"{period[:4]}-{period[6:8]}-{period[4:6]}" if len(period) >= 8 else period
        # Fix: period format is YYYYMMDD -> YYYY-MM-DD
        if len(period) == 8:
            period_str = f"{period[:4]}-{period[4:6]}-{period[6:8]}"
        items = {}
        for it in obj.get("data", []) or []:
            title = it.get("item_title", "")
            val_str = it.get("item_value", "")
            if not title or val_str is None:
                continue
            try:
                items[title] = float(val_str)
            except:
                items[title] = 0
        result[period_str] = items
    return result

def extract_metrics(code, lrb_data, fzb_data):
    all_dates = sorted(set(list(lrb_data.keys()) + list(fzb_data.keys())), reverse=True)
    metrics = {}
    
    for report_date in all_dates:
        lrb = lrb_data.get(report_date, {})
        fzb = fzb_data.get(report_date, {})
        if not lrb and not fzb:
            continue
        
        m = {
            'reportDate': report_date,
            'totalRevenue': 0, 'netProfit': 0,
            'totalAssets': 0, 'totalLiabilities': 0, 'netAssets': 0,
            'roe': 0, 'eps': 0, 'bps': 0,
            'grossMargin': 0, 'netMargin': 0, 'debtRatio': 0,
        }
        
        for key, val in lrb.items():
            v = float(val) if val else 0
            if key in ('营业总收入', '营业收入') and not m['totalRevenue']:
                m['totalRevenue'] = v
            elif '净利润' in key and '归属于' in key and '少数' not in key:
                m['netProfit'] = v
            elif key == '净利润' and not m['netProfit']:
                m['netProfit'] = v
            elif key in ('基本每股收益', '稀释每股收益'):
                if not m['eps']: m['eps'] = v
            elif key in ('营业成本', '营业总成本') or ('营业成本' in key):
                if m['totalRevenue'] > 0:
                    m['grossMargin'] = round((m['totalRevenue'] - v) / m['totalRevenue'] * 100, 2)
        
        for key, val in fzb.items():
            v = float(val) if val else 0
            if key == '资产总计':
                m['totalAssets'] = v
            elif key == '负债合计':
                m['totalLiabilities'] = v
            elif ('归属于母公司' in key or '归属母公司' in key) and '权益' in key and '少数' not in key:
                m['netAssets'] = v
            elif ('所有者权益' in key or '股东权益' in key) and '负债' not in key and not m['netAssets']:
                m['netAssets'] = v
        
        if m['netAssets'] > 0 and m['netProfit'] != 0:
            m['roe'] = round(m['netProfit'] / m['netAssets'] * 100, 2)
        if m['totalRevenue'] > 0 and m['netProfit'] != 0:
            m['netMargin'] = round(m['netProfit'] / m['totalRevenue'] * 100, 2)
        if m['totalAssets'] > 0 and m['totalLiabilities'] > 0:
            m['debtRatio'] = round(m['totalLiabilities'] / m['totalAssets'] * 100, 2)
        if m['netAssets'] > 0 and m['eps'] > 0:
            m['bps'] = round(m['netAssets'] / (m['netProfit'] / m['eps']) if m['netProfit'] != 0 else 0, 2)
        
        if report_date.endswith('12-31'): m['reportType'] = '年报'
        elif report_date.endswith('03-31'): m['reportType'] = '一季报'
        elif report_date.endswith('06-30'): m['reportType'] = '中报'
        elif report_date.endswith('09-30'): m['reportType'] = '三季报'
        else: m['reportType'] = '其他'
        
        if m['totalRevenue'] > 0 or m['netProfit'] != 0:
            # Compute growth rates
            metrics[report_date] = m
    
    # Compute YoY growth
    sorted_dates = sorted(metrics.keys())
    for i, rd in enumerate(sorted_dates):
        m = metrics[rd]
        # Find same quarter last year
        year = int(rd[:4])
        quarter_end = rd[5:]
        prev_rd = f"{year-1}-{quarter_end}"
        if prev_rd in metrics:
            prev = metrics[prev_rd]
            if prev['totalRevenue'] != 0:
                m['revenueGrowth'] = round((m['totalRevenue'] - prev['totalRevenue']) / abs(prev['totalRevenue']) * 100, 2)
            if prev['netProfit'] != 0:
                m['profitGrowth'] = round((m['netProfit'] - prev['netProfit']) / abs(prev['netProfit']) * 100, 2)
    
    return metrics

def main():
    codes_arg = sys.argv[1] if len(sys.argv) > 1 else ''
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    if codes_arg:
        codes = [c.strip() for c in codes_arg.split(',') if c.strip()]
    else:
        # Get ALL stocks, prioritize those without financial data
        cur.execute("""
            SELECT b.code FROM stocks_basic b
            ORDER BY b.code
        """)
        codes = [r[0] for r in cur.fetchall()]
    
    if not codes:
        print("没有需要采集的股票", flush=True)
        return
    
    # Get counts of stocks that already have data
    cur.execute("SELECT COUNT(DISTINCT code) FROM stock_financials")
    existing = cur.fetchone()[0]
    
    print(f"全量财务回填: {len(codes)} 只 (已有 {existing} 只)", flush=True)
    done = 0
    new_stocks = 0
    total_periods = 0
    start = time.time()
    
    for i, code in enumerate(codes):
        try:
            lrb_data = sina_financial_report(code, "lrb", 8)
            time.sleep(0.12)
            fzb_data = sina_financial_report(code, "fzb", 8)
            
            metrics = extract_metrics(code, lrb_data, fzb_data)
            
            if metrics:
                had_data = False
                cur.execute("SELECT COUNT(*) FROM stock_financials WHERE code = %s", (code,))
                if cur.fetchone()[0] > 0:
                    had_data = True
                else:
                    new_stocks += 1
                
                for report_date, m in metrics.items():
                    try:
                        cur.execute("""
                            INSERT INTO stock_financials (code, report_date, report_type, 
                                total_revenue, net_profit, revenue_growth, profit_growth,
                                total_assets, total_liabilities, net_assets, 
                                roe, eps, bps, gross_margin, net_margin, debt_ratio)
                            VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                            ON CONFLICT (code, report_date) DO UPDATE SET
                                total_revenue=EXCLUDED.total_revenue, net_profit=EXCLUDED.net_profit,
                                revenue_growth=EXCLUDED.revenue_growth, profit_growth=EXCLUDED.profit_growth,
                                total_assets=EXCLUDED.total_assets, total_liabilities=EXCLUDED.total_liabilities,
                                net_assets=EXCLUDED.net_assets, roe=EXCLUDED.roe, eps=EXCLUDED.eps,
                                bps=EXCLUDED.bps, gross_margin=EXCLUDED.gross_margin,
                                net_margin=EXCLUDED.net_margin, debt_ratio=EXCLUDED.debt_ratio
                        """, (code, m['reportDate'], m['reportType'],
                              m['totalRevenue'], m['netProfit'],
                              m.get('revenueGrowth', 0), m.get('profitGrowth', 0),
                              m['totalAssets'], m['totalLiabilities'], m['netAssets'],
                              m['roe'], m['eps'], m['bps'],
                              m['grossMargin'], m['netMargin'], m['debtRatio']))
                        total_periods += 1
                    except Exception as e:
                        pass
                done += 1
        except Exception as e:
            pass
        
        if (i + 1) % 100 == 0:
            conn.commit()
            elapsed = time.time() - start
            rate = (i+1) / elapsed if elapsed > 0 else 0
            eta = (len(codes) - i - 1) / rate if rate > 0 else 0
            print(f"  {i+1}/{len(codes)} | 完成 {done} | 新增 {new_stocks} | {total_periods}期 | {elapsed:.0f}s | ETA {eta:.0f}s", flush=True)
        time.sleep(0.08)
    
    conn.commit()
    
    cur.execute("SELECT COUNT(DISTINCT code), COUNT(*) FROM stock_financials")
    total_codes, total_rows = cur.fetchone()
    
    cur.close()
    conn.close()
    elapsed = time.time() - start
    print(f"✅ 财务回填完成: {total_codes} stocks, {total_rows} rows | {elapsed:.0f}s | {elapsed/len(codes):.1f}s/只")

if __name__ == "__main__":
    main()
