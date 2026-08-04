import psycopg2, json, os
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
conn = psycopg2.connect(PG_DSN)
cur = conn.cursor()

result = {}

# K-line summary
cur.execute("SELECT COUNT(*) as total_rows, COUNT(DISTINCT code) as total_stocks, MIN(trade_date)::text as min_date, MAX(trade_date)::text as max_date FROM stocks_daily_k")
row = cur.fetchone()
result["kline"] = {"totalRows": row[0], "totalStocks": row[1], "minDate": row[2], "maxDate": row[3]}

# Quality
cur.execute("SELECT data_quality, COUNT(*) FROM stocks_daily_k GROUP BY data_quality")
for q, cnt in cur.fetchall():
    if q == "ok": result["kline"]["qualityOk"] = cnt
    elif q == "suspect": result["kline"]["qualitySuspect"] = cnt
    elif q == "bad": result["kline"]["qualityBad"] = cnt

# Stale
cur.execute("SELECT COUNT(*) FROM (SELECT code, MAX(trade_date) as last_date FROM stocks_daily_k GROUP BY code) t WHERE last_date < (SELECT MAX(trade_date) FROM stocks_daily_k) - INTERVAL '3 days'")
result["kline"]["staleStocks"] = cur.fetchone()[0]

# Sparse
cur.execute("SELECT COUNT(*) FROM (SELECT code, COUNT(*) as cnt FROM stocks_daily_k GROUP BY code) t WHERE cnt < 100")
result["kline"]["sparseStocks"] = cur.fetchone()[0]

# Financials
cur.execute("SELECT COUNT(*) as total_rows, COUNT(DISTINCT code) as total_stocks, COUNT(*) FILTER (WHERE operating_cf IS NOT NULL AND operating_cf != 0) as has_cf, ROUND(COUNT(*) FILTER (WHERE operating_cf IS NOT NULL AND operating_cf != 0)::numeric / COUNT(*)::numeric * 100, 1) as cf_pct FROM stock_financials")
row = cur.fetchone()
result["financials"] = {"totalRows": row[0], "totalStocks": row[1], "hasCashFlow": row[2], "cashFlowPct": float(row[3]) if row[3] else 0}

cur.close(); conn.close()
print(json.dumps(result, ensure_ascii=False, indent=2))
