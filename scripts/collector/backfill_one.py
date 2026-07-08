import json, sys, psycopg2, urllib.request, ssl
from psycopg2.extras import execute_values

PG_DSN = "host=postgres dbname=stock_predict user=stock password=stock123"
conn = psycopg2.connect(PG_DSN)
cur = conn.cursor()

code = sys.argv[1] if len(sys.argv) > 1 else '301151'
prefix = "sh" if code.startswith(("6", "9")) else "sz"
url = f"https://ifzq.gtimg.cn/appstock/app/fqkline/get?param={prefix}{code},day,,,1100,qfq"
req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
ctx = ssl.create_default_context()
with urllib.request.urlopen(req, timeout=30, context=ctx) as resp:
    data = json.loads(resp.read().decode())
klines = data.get("data",{}).get(f"{prefix}{code}",{}).get("qfqday",[]) or \
         data.get("data",{}).get(f"{prefix}{code}",{}).get("day",[])
print(f"API: {len(klines)} records", flush=True)

rows = []
for row in klines:
    if len(row) >= 6:
        vol_shou = float(row[5]); vol_gu = int(vol_shou) if code.startswith("688") else int(vol_shou * 100); close_p = float(row[2])
        rows.append((code, row[0], float(row[1]), float(row[3]), float(row[4]),
                     close_p, vol_gu, close_p * float(vol_gu)))
execute_values(cur, "INSERT INTO stocks_daily_k (code,trade_date,open,high,low,close,volume,amount) VALUES %s ON CONFLICT (code, trade_date) DO UPDATE SET updated_at = NOW()", rows)
conn.commit()
cur.execute("SELECT COUNT(*), MIN(trade_date), MAX(trade_date) FROM stocks_daily_k WHERE code=%s", (code,))
c, mn, mx = cur.fetchone()
print(f"DB: {c} records, {mn} ~ {mx}")
