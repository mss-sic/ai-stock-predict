#!/usr/bin/env python3
"""
PE/PB/PS 历史回填脚本
从 stocks_daily_k (历史K线) × stock_financials (财报) 计算历史 PE/PB/PS
批量写入 stocks_daily_indicator

策略：对于每个交易日，使用该日期之前最近一期财报的 EPS/BPS 计算估值
"""
import os, sys, time, psycopg2, psycopg2.extras
from datetime import date, datetime

os.environ['PYTHONUNBUFFERED'] = '1'

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def main():
    start_date = sys.argv[1] if len(sys.argv) > 1 else '2024-01-01'
    end_date = sys.argv[2] if len(sys.argv) > 2 else date.today().isoformat()
    batch_size = int(sys.argv[3]) if len(sys.argv) > 3 else 5000
    
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    print(f"回填区间: {start_date} ~ {end_date}")
    
    # Step 1: Count stocks with financial data
    cur.execute("SELECT COUNT(DISTINCT code) FROM stock_financials WHERE eps > 0 OR bps > 0")
    fin_count = cur.fetchone()[0]
    print(f"有财报数据的股票: {fin_count} 只")
    
    if fin_count == 0:
        print("❌ 请先运行 financial_collect.py 采集财报数据")
        cur.close(); conn.close()
        return
    
    # Step 2: Count K-line days in range
    cur.execute("""
        SELECT COUNT(DISTINCT trade_date) FROM stocks_daily_k 
        WHERE trade_date >= %s AND trade_date <= %s
    """, (start_date, end_date))
    total_days = cur.fetchone()[0]
    print(f"K线交易日: {total_days} 天")
    
    # Step 3: Bulk compute and insert using SQL
    # For each stock-day, find latest financial report <= that day, compute PE/PB/PS/market_cap
    print("正在计算历史 PE/PB/PS...")
    start = time.time()
    
    # Count existing records to avoid re-computing
    cur.execute("""
        SELECT COUNT(*) FROM stocks_daily_indicator 
        WHERE trade_date >= %s AND trade_date <= %s AND pe > 0
    """, (start_date, end_date))
    existing_count = cur.fetchone()[0]
    print(f"已有PE数据: {existing_count} 条")
    
    # Use a single giant INSERT ... ON CONFLICT query
    # This is much faster than row-by-row
    sql = """
    INSERT INTO stocks_daily_indicator (code, trade_date, pe, pb, ps, total_market_cap, circulating_market_cap)
    SELECT 
        k.code,
        k.trade_date,
        CASE WHEN fin.eps > 0 THEN ROUND((k.close / fin.eps)::numeric, 2) ELSE 0 END as pe,
        CASE WHEN fin.bps > 0 THEN ROUND((k.close / fin.bps)::numeric, 2) ELSE 0 END as pb,
        CASE WHEN fin.revenue_per_share > 0 THEN ROUND((k.close / fin.revenue_per_share)::numeric, 2) ELSE 0 END as ps,
        CASE WHEN fin.eps > 0 AND fin.net_profit > 0 
             THEN ROUND((k.close * (fin.net_profit / fin.eps))::numeric, 2) 
             ELSE 0 END as total_market_cap,
        0 as circulating_market_cap
    FROM stocks_daily_k k
    JOIN LATERAL (
        SELECT 
            f.eps,
            f.bps,
            f.total_revenue,
            f.net_profit,
            f.net_assets,
            CASE WHEN f.total_revenue > 0 AND f.net_assets > 0 AND f.bps > 0
                 THEN f.total_revenue / (f.net_assets / f.bps)
                 ELSE 0 END as revenue_per_share
        FROM stock_financials f
        WHERE f.code = k.code 
          AND f.report_date <= k.trade_date::text
          AND (f.eps > 0 OR f.bps > 0)
        ORDER BY f.report_date DESC 
        LIMIT 1
    ) fin ON true
    WHERE k.trade_date >= %s AND k.trade_date <= %s
    ON CONFLICT (code, trade_date) DO UPDATE SET
        pe = EXCLUDED.pe,
        pb = EXCLUDED.pb,
        ps = EXCLUDED.ps,
        total_market_cap = EXCLUDED.total_market_cap
    """
    
    cur.execute(sql, (start_date, end_date))
    inserted = cur.rowcount
    conn.commit()
    
    elapsed = time.time() - start
    print(f"✅ 回填完成: {inserted} 条 | {elapsed:.0f}s")
    
    # Verify
    cur.execute("""
        SELECT COUNT(*), MIN(trade_date), MAX(trade_date)
        FROM stocks_daily_indicator 
        WHERE pe > 0 AND trade_date >= %s AND trade_date <= %s
    """, (start_date, end_date))
    cnt, min_d, max_d = cur.fetchone()
    print(f"验证: {cnt} 条 PE>0 的记录 | {min_d} ~ {max_d}")
    
    cur.execute("""
        SELECT COUNT(DISTINCT code) FROM stocks_daily_indicator 
        WHERE pe > 0 AND trade_date >= %s AND trade_date <= %s
    """, (start_date, end_date))
    stock_cnt = cur.fetchone()[0]
    print(f"覆盖股票: {stock_cnt} 只")
    
    cur.close()
    conn.close()

if __name__ == "__main__":
    main()
