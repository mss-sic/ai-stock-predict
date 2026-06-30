#!/usr/bin/env python3
"""
个股资金流向采集 — 120日日级主力/大单/中单/小单净流入
数据源: 东财 push2his (零鉴权)
用法:
  python3 collect_fund_flow.py                  # 全市场全量采集
  python3 collect_fund_flow.py --sample         # 8 只样本股
  python3 collect_fund_flow.py --last 5         # 全市场只保留最近 5 个交易日
  python3 collect_fund_flow.py --last 20 --sample  # 样本股最近 20 个交易日
  python3 collect_fund_flow.py 600519           # 单只股票
"""
import os, sys, time, json, random
from datetime import date, timedelta
import psycopg2
from psycopg2.extras import execute_values
import urllib.request

SAMPLE_CODES = ['600519', '601318', '688017', '000001', '002475', '300750', '600036', '688981']
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"


def fetch_fund_flow(code, retries=2):
    market = 1 if code.startswith("6") else 0
    url = "https://push2his.eastmoney.com/api/qt/stock/fflow/daykline/get"
    params = {
        "secid": f"{market}.{code}",
        "fields1": "f1,f2,f3,f7",
        "fields2": "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63,f64,f65",
        "lmt": "120",
    }
    headers = {
        "User-Agent": UA,
        "Referer": "https://quote.eastmoney.com/",
        "Origin": "https://quote.eastmoney.com",
    }
    last_err = None
    for attempt in range(retries + 1):
        try:
            req = urllib.request.Request(f"{url}?{urllib.parse.urlencode(params)}", headers=headers)
            with urllib.request.urlopen(req, timeout=15) as resp:
                d = json.loads(resp.read().decode("utf-8"))
            break  # success
        except Exception as e:
            last_err = str(e)
            if attempt < retries:
                time.sleep(1 + attempt)
    else:
        print(f"  ⚠️ {code} fetch failed after {retries+1} attempts: {last_err[:80]}", flush=True)
        return None

    klines = d.get("data", {}).get("klines", [])
    if not klines:
        return []

    rows = []
    for line in klines:
        parts = line.split(",")
        if len(parts) >= 7:
            rows.append((
                code,
                parts[0],  # date
                round(float(parts[1]) / 10000, 2) if parts[1] != "-" else 0,  # main_net (万元)
                round(float(parts[2]) / 10000, 2) if parts[2] != "-" else 0,  # small_net
                round(float(parts[3]) / 10000, 2) if parts[3] != "-" else 0,  # mid_net
                round(float(parts[4]) / 10000, 2) if parts[4] != "-" else 0,  # large_net
                round(float(parts[5]) / 10000, 2) if parts[5] != "-" else 0,  # super_net
            ))
    return rows


def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Parse --last N argument
    last_days = 0
    args = [a for a in sys.argv[1:] if not a.startswith('--') or a == '--sample']
    if '--last' in sys.argv:
        idx = sys.argv.index('--last')
        if idx + 1 < len(sys.argv):
            last_days = int(sys.argv[idx + 1])

    # Calculate cutoff date for --last mode
    cutoff_date = None
    if last_days > 0:
        cutoff_date = (date.today() - timedelta(days=last_days * 2)).isoformat()  # generous window

    if '--sample' in sys.argv:
        codes = SAMPLE_CODES
        print(f"[资金流] 样本模式: {len(codes)} 只" + (f", 最近 {last_days} 天" if last_days > 0 else ""))
    elif len(args) > 0 and args[0].isdigit() and len(args[0]) == 6:
        codes = [args[0]]
        print(f"[资金流] 单股模式: {codes[0]}")
    else:
        cur.execute("SELECT code FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
        codes = [row[0] for row in cur.fetchall()]
        print(f"[资金流] 全量模式: {len(codes)} 只" + (f", 最近 {last_days} 天" if last_days > 0 else ""))

    total, skip, api_err, empty = 0, 0, 0, 0
    t_start = time.time()
    for i, code in enumerate(codes):
        # Progress every 50 stocks with ETA
        if (i + 1) % 50 == 0:
            elapsed = time.time() - t_start
            rate = (i + 1) / elapsed if elapsed > 0 else 0
            eta_min = (len(codes) - i - 1) / (rate * 60) if rate > 0 else 0
            print(f"[资金流] {i+1}/{len(codes)} | 新增:{total} 跳过:{skip} API错误:{api_err} 空数据:{empty} | {rate:.1f}只/s | ETA:{eta_min:.0f}min", flush=True)
        try:
            rows = fetch_fund_flow(code)
            if rows is None:
                api_err += 1
                continue
            if not rows:
                empty += 1
                skip += 1
                continue

            # Filter by cutoff date if --last mode
            if cutoff_date:
                rows = [r for r in rows if r[1] >= cutoff_date]

            if not rows:
                empty += 1
                skip += 1
                continue

            for r in rows:
                cur.execute("""
                    INSERT INTO stock_fund_flow (code, trade_date, main_net, small_net, mid_net, large_net, super_net)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (code, trade_date) DO UPDATE SET
                        main_net = EXCLUDED.main_net, small_net = EXCLUDED.small_net,
                        mid_net = EXCLUDED.mid_net, large_net = EXCLUDED.large_net,
                        super_net = EXCLUDED.super_net
                """, r)
            conn.commit()
            total += len(rows)
        except Exception as e:
            errors += 1
            conn.rollback()
            continue
        time.sleep(0.3 + random.random() * 0.4)  # 0.3-0.7s to avoid rate limit

    cur.close()
    conn.close()
    elapsed = time.time() - t_start
    print(f"[资金流] 完成 | 耗时:{elapsed/60:.0f}min | 新增:{total}条 跳过:{skip} API错误:{api_err} 空数据:{empty}", flush=True)
    print(f"STAT:records_new={total},records_skip={skip},records_err={api_err},fund_flow_new={total}", flush=True)


if __name__ == "__main__":
    main()
