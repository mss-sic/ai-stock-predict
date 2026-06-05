#!/usr/bin/env python3
"""批量采集 — mootdx TCP直连 (绕过代理) + 腾讯K线"""
import sys, time, os, psycopg2, requests
from mootdx.quotes import Quotes
from datetime import date, timedelta

os.environ['NO_PROXY'] = '*'

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"
KSESSION = requests.Session()
KSESSION.trust_env = False
KSESSION.headers.update({"User-Agent": "Mozilla/5.0"})

def get_codes():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    cur.execute("SELECT code FROM stocks_basic ORDER BY code")
    codes = [r[0] for r in cur.fetchall()]
    cur.close(); conn.close()
    return codes

def fetch_quotes_batch(client, codes):
    """mootdx 批量获取行情（含PE等）"""
    results = {}
    batch_size = 80
    for i in range(0, len(codes), batch_size):
        batch = codes[i:i+batch_size]
        try:
            df = client.quotes(symbol=batch)
            for _, row in df.iterrows():
                code = str(row.get('code', '')).zfill(6)
                if code and code in batch:
                    results[code] = {
                        'name': str(row.get('name', '') or ''),
                        'price': float(row.get('price', 0) or 0),
                        'pe': float(row.get('pe', 0) or 0),
                        'open': float(row.get('open', 0) or 0),
                        'high': float(row.get('high', 0) or 0),
                        'low': float(row.get('low', 0) or 0),
                        'volume': int(float(row.get('volume', 0) or 0)),
                        'amount': float(row.get('amount', 0) or 0),
                    }
        except Exception as e:
            pass
        if (i+batch_size) % 400 == 0:
            print(f"  行情: {min(i+batch_size, len(codes))}/{len(codes)}")
    return results

def fetch_stock_info(codes):
    """从 mootdx 获取股票基础信息"""
    client = Quotes.factory(market='std')
    df = client.stocks()
    info = {}
    for _, row in df.iterrows():
        code = str(row.get('code', '')).zfill(6)
        if code in codes:
            info[code] = str(row.get('name', '') or '')
    return info

def fetch_kline_batch(codes, days=60):
    """腾讯API批量拉日K"""
    results = {}
    for i, code in enumerate(codes):
        prefix = "sh" if code.startswith(("6","9")) else "sz"
        url = f"https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param={prefix}{code},day,,,{days},qfq"
        try:
            r = KSESSION.get(url, timeout=20)
            data = r.json()
            klines = data.get("data", {}).get(f"{prefix}{code}", {}).get("qfqday", [])
            results[code] = klines
        except: pass
        if (i+1) % 200 == 0:
            print(f"  K线: {i+1}/{len(codes)}")
        time.sleep(0.02)
    return results

def main():
    print("📡 获取股票列表...")
    codes = get_codes()
    print(f"   {len(codes)} 只")

    # Phase 1: K-line
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM stocks_daily_k")
    existing_k = cur.fetchone()[0]
    cur.close(); conn.close()

    if existing_k == 0:
        print(f"\n📈 Step 1: 腾讯日K线 (~{len(codes)*0.02:.0f}秒)")
        kdata = fetch_kline_batch(codes, days=60)
        conn = psycopg2.connect(PG_DSN)
        cur = conn.cursor()
        total = 0
        for code, klines in kdata.items():
            for row in klines:
                if len(row) < 6: continue
                try:
                    cur.execute("""
                        INSERT INTO stocks_daily_k (code,trade_date,open,high,low,close,volume,amount)
                        VALUES (%s,%s,%s,%s,%s,%s,%s,%s)
                        ON CONFLICT (code,trade_date) DO NOTHING
                    """, (code, row[0], float(row[1]), float(row[3]), float(row[4]),
                          float(row[2]), int(float(row[5])), float(row[2])*float(row[5])/100))
                    total += 1
                except: pass
            if total % 10000 == 0:
                conn.commit()
        conn.commit()
        cur.close(); conn.close()
        print(f"   ✅ K线: {total} 条")
    else:
        print(f"   K线已有 {existing_k} 条，跳过")

    # Phase 2: Stock names & quotes
    print(f"\n📊 Step 2: mootdx 行情数据")
    client = Quotes.factory(market='std')
    quotes = fetch_quotes_batch(client, codes)
    print(f"   获取到 {len(quotes)} 只股票行情")

    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    today = date.today()
    updated = 0
    for code, q in quotes.items():
        try:
            cur.execute("""
                UPDATE stocks_basic SET name=%s, updated_at=NOW()
                WHERE code=%s AND (name='' OR name IS NULL)
            """, (q['name'], code))
            cur.execute("""
                INSERT INTO stocks_daily_indicator (code, trade_date, pe, total_market_cap)
                VALUES (%s, %s, %s, %s)
                ON CONFLICT (code, trade_date) DO UPDATE SET pe=EXCLUDED.pe
            """, (code, today, q['pe'], q['amount']))
            updated += 1
        except: pass
    conn.commit()
    cur.close(); conn.close()
    print(f"   ✅ 行情更新: {updated} 只")

    # Phase 3: Stock info (names from full stock list)
    print(f"\n📋 Step 3: 股票名称补全")
    info = fetch_stock_info(codes)
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    named = 0
    for code, name in info.items():
        if name and name != code:
            cur.execute("UPDATE stocks_basic SET name=%s WHERE code=%s AND (name='' OR name IS NULL)", (name, code))
            named += 1
    conn.commit()
    cur.close(); conn.close()
    print(f"   ✅ 名称补全: {named} 只")

    print(f"\n🎉 采集完成！")

if __name__ == "__main__":
    main()
