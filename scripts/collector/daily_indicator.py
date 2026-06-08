#!/usr/bin/env python3
"""PE/PB指标采集 — mootdx(沪) + 新浪(深) 双源，只采集今日缺失"""
import os, sys, json, time, psycopg2, urllib.request, logging
from datetime import date

os.environ['PYTHONUNBUFFERED'] = '1'
os.environ['NO_PROXY'] = '*'

# Suppress mootdx/tqdm noise
logging.getLogger('tdxpy').setLevel(logging.CRITICAL)
logging.getLogger('mootdx').setLevel(logging.CRITICAL)

from mootdx.quotes import Quotes

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

def fetch_pe_pb_sina(codes):
    """Batch fetch PE/PB from新浪 for深圳 stocks"""
    headers = {'User-Agent': 'Mozilla/5.0', 'Referer': 'https://finance.sina.com.cn'}
    results = {}
    for i in range(0, len(codes), 80):
        batch = codes[i:i+80]
        symbols = ','.join(f'sz{c}' for c in batch)
        url = f'http://hq.sinajs.cn/list={symbols}'
        try:
            req = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(req, timeout=15) as resp:
                text = resp.read().decode('gbk', errors='replace')
            for line in text.strip().split('\n'):
                if '=\"\"' in line or '=\"' not in line:
                    continue
                try:
                    code_part = line.split('hq_str_')[1].split('=\"')[0]
                    if not code_part.startswith('sz'):
                        continue
                    code = code_part[2:]
                    fields = line.split('=\"')[1].split(',')
                    price = float(fields[3]) if fields[3] else 0
                    pe = float(fields[-8]) if len(fields) > 40 and fields[-8] else 0  # PE in新浪 format
                    if price > 0:
                        results[code] = {'price': price, 'pe': pe, 'pb': 0}
                except:
                    pass
        except Exception as e:
            pass
        if i % 500 == 0 and i > 0:
            print(f"  新浪进度: {min(i+80, len(codes))}/{len(codes)}", flush=True)
        time.sleep(0.2)
    return results

def main():
    client = Quotes.factory(market='std')
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    today = date.today()

    # Stocks missing today's PE
    cur.execute("""
        SELECT b.code FROM stocks_basic b
        LEFT JOIN stocks_daily_indicator i ON b.code = i.code AND i.trade_date = %s AND i.pe > 0
        WHERE i.code IS NULL
        ORDER BY b.code
    """, (today,))
    codes = [r[0] for r in cur.fetchall()]
    total = len(codes)
    print(f"共 {total} 只股票需采集PE/PB")

    if total == 0:
        print("今日PE/PB数据已完整，跳过")
        cur.close(); conn.close()
        return

    # Split: 沪 (mootdx finance) vs 深 (新浪)
    sh_codes = [c for c in codes if c.startswith(('60', '68'))]
    sz_codes = [c for c in codes if c.startswith(('00', '30'))]
    print(f"  沪市(mootdx): {len(sh_codes)} 只 | 深市(新浪): {len(sz_codes)} 只")

    # ---- 沪市: mootdx finance per stock ----
    if sh_codes:
        print("采集沪市PE/PB (mootdx)...")
        # Batch quotes for prices
        prices = {}
        for i in range(0, len(sh_codes), 80):
            batch = sh_codes[i:i+80]
            try:
                df = client.quotes(symbol=batch)
                for _, row in df.iterrows():
                    code = str(row.get('code', '')).zfill(6)
                    price = float(row.get('price', 0) or 0)
                    if code and price > 0:
                        prices[code] = price
            except:
                pass

        sh_done = 0
        start = time.time()
        for idx, code in enumerate(sh_codes):
            if code not in prices:
                continue
            try:
                fin = client.finance(code)
                if fin is None or len(fin) == 0:
                    continue
                row = fin.iloc[-1]
                zgb = float(row.get('zongguben', 0) or 0)
                jlr = float(row.get('jinglirun', 0) or 0)
                mgjzc = float(row.get('meigujingzichan', 0) or 0)
                price = prices[code]

                pe = pb = mcap = 0.0
                if zgb > 0 and price > 0:
                    mcap = price * zgb
                if jlr > 0 and zgb > 0:
                    eps = jlr / zgb
                    if eps > 0:
                        pe = round(price / eps, 2)
                if mgjzc > 0 and price > 0:
                    pb = round(price / mgjzc, 2)

                if pe > 0 or pb > 0:
                    cur.execute("""
                        INSERT INTO stocks_daily_indicator (code, trade_date, pe, pb, total_market_cap)
                        VALUES (%s,%s,%s,%s,%s)
                        ON CONFLICT (code, trade_date) DO UPDATE SET
                            pe=EXCLUDED.pe, pb=EXCLUDED.pb, total_market_cap=EXCLUDED.total_market_cap
                    """, (code, today, pe, pb, mcap))
                    sh_done += 1
            except:
                pass

            if (idx + 1) % 300 == 0:
                elapsed = time.time() - start
                conn.commit()
                print(f"  沪市: {idx+1}/{len(sh_codes)} | {sh_done}只 | {elapsed:.0f}s", flush=True)

        conn.commit()
        print(f"  沪市完成: {sh_done} 只", flush=True)

    # ---- 深市: 新浪 API ----
    if sz_codes:
        print(f"采集深市PE/PB (新浪)... {len(sz_codes)} 只")
        sz_data = fetch_pe_pb_sina(sz_codes)
        sz_done = 0
        for code, d in sz_data.items():
            if d['pe'] > 0 or d['price'] > 0:
                mcap = 0.0
                cur.execute("""
                    INSERT INTO stocks_daily_indicator (code, trade_date, pe, total_market_cap)
                    VALUES (%s,%s,%s,%s)
                    ON CONFLICT (code, trade_date) DO UPDATE SET
                        pe=EXCLUDED.pe, total_market_cap=EXCLUDED.total_market_cap
                """, (code, today, d['pe'], mcap))
                sz_done += 1
        conn.commit()
        print(f"  深市完成: {sz_done} 只", flush=True)

    cur.execute("SELECT COUNT(*) FROM stocks_daily_indicator WHERE trade_date = %s AND pe > 0", (today,))
    pe_cnt = cur.fetchone()[0]
    cur.close()
    conn.close()

    print(f"✅ PE/PB: {pe_cnt} 只")

if __name__ == "__main__":
    main()
