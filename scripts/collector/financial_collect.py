#!/usr/bin/env python3
"""财务数据采集 — 利润表/资产负债表，来源: 新浪 finance"""
import os, sys, json, time, psycopg2, requests
from datetime import date

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

def sina_financial_report(code, report_type="lrb", num=8):
    """
    新浪财报: "lrb"(利润表) / "fzb"(资产负债表) / "llb"(现金流量表)
    返回: {report_date: {item_title: value}}
    """
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
    except Exception as e:
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
            except:
                items[title] = 0
        result[period_str] = items
    return result

def extract_metrics(code, lrb_data, fzb_data):
    """Extract key financial metrics from combined reports — flexible field matching"""
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
        
        # 利润表 items — use flexible matching
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
            elif key in ('营业成本',):
                if m['totalRevenue'] > 0:
                    m['grossMargin'] = round((m['totalRevenue'] - v) / m['totalRevenue'] * 100, 2)
        
        # 资产负债表 items — use flexible matching
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
        
        # Derived metrics — handle negative values
        if m['netAssets'] > 0 and m['netProfit'] != 0:
            m['roe'] = round(m['netProfit'] / m['netAssets'] * 100, 2)
        if m['totalRevenue'] > 0 and m['netProfit'] != 0:
            m['netMargin'] = round(m['netProfit'] / m['totalRevenue'] * 100, 2)
        if m['totalAssets'] > 0 and m['totalLiabilities'] > 0:
            m['debtRatio'] = round(m['totalLiabilities'] / m['totalAssets'] * 100, 2)
        
        # Report type
        if report_date.endswith('12-31'): m['reportType'] = '年报'
        elif report_date.endswith('03-31'): m['reportType'] = '一季报'
        elif report_date.endswith('06-30'): m['reportType'] = '中报'
        elif report_date.endswith('09-30'): m['reportType'] = '三季报'
        else: m['reportType'] = '其他'
        
        if m['totalRevenue'] > 0 or m['netProfit'] != 0:
            metrics[report_date] = m
    
    return metrics

def main():
    codes_arg = sys.argv[1] if len(sys.argv) > 1 else ''
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    if codes_arg:
        codes = [c.strip() for c in codes_arg.split(',') if c.strip()]
    else:
        cur.execute("""
            SELECT b.code FROM stocks_basic b
            LEFT JOIN stock_financials f ON b.code = f.code
            WHERE f.code IS NULL
            ORDER BY b.code
            LIMIT 200
        """)
        codes = [r[0] for r in cur.fetchall()]
    
    if not codes:
        print("没有需要采集的股票", flush=True)
        return
    
    print(f"采集财务数据: {len(codes)} 只", flush=True)
    done = 0
    total_periods = 0
    start = time.time()
    
    for i, code in enumerate(codes):
        try:
            lrb_data = sina_financial_report(code, "lrb", 8)
            time.sleep(0.15)
            fzb_data = sina_financial_report(code, "fzb", 8)
            
            metrics = extract_metrics(code, lrb_data, fzb_data)
            
            if metrics:
                done += 1
                for report_date, m in metrics.items():
                    try:
                        cur.execute("""
                            INSERT INTO stock_financials (code, report_date, report_type, total_revenue, net_profit,
                                total_assets, total_liabilities, net_assets, roe, eps, bps, gross_margin, net_margin, debt_ratio)
                            VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                            ON CONFLICT (code, report_date) DO UPDATE SET
                                total_revenue=EXCLUDED.total_revenue, net_profit=EXCLUDED.net_profit,
                                total_assets=EXCLUDED.total_assets, total_liabilities=EXCLUDED.total_liabilities,
                                net_assets=EXCLUDED.net_assets, roe=EXCLUDED.roe, eps=EXCLUDED.eps,
                                bps=EXCLUDED.bps, gross_margin=EXCLUDED.gross_margin,
                                net_margin=EXCLUDED.net_margin, debt_ratio=EXCLUDED.debt_ratio
                        """, (code, m['reportDate'], m['reportType'],
                              m['totalRevenue'], m['netProfit'],
                              m['totalAssets'], m['totalLiabilities'], m['netAssets'],
                              m['roe'], m['eps'], m['bps'],
                              m['grossMargin'], m['netMargin'], m['debtRatio']))
                        total_periods += 1
                    except Exception as e:
                        pass
        except Exception as e:
            pass
        
        if (i + 1) % 20 == 0:
            conn.commit()
            elapsed = time.time() - start
            print(f"  {i+1}/{len(codes)} | {done} stocks | {total_periods} periods | {elapsed:.0f}s", flush=True)
        time.sleep(0.2)
    
    conn.commit()
    cur.close()
    conn.close()
    print(f"✅ 财务数据: {done} stocks, {total_periods} periods | {time.time()-start:.0f}s")

if __name__ == "__main__":
    main()
