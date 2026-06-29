#!/usr/bin/env python3
"""
填充 stock_concepts — 东财 slist API (Authoritative / 权威数据源)
============================================================================
⚠️  数据源升级记录 (2026-06-29):
  原使用新浪 vip.stock.finance.sina.com.cn API 采集概念板块归属，因不稳定已
  升级为东财 push2 slist API (spt=3)，与 rebuild_concepts.py 统一数据源。
  - 当前覆盖: 1068 个概念板块, 覆盖 4,844 只股票
  - CPO概念 / 低空经济 / 人形机器人 / 可控核聚变 等前沿概念均已覆盖
  - 每次返回 20-50 个板块/股 (含行业+概念+地域三种类型)

⚠️  禁止降级回新浪 API。如遇数据问题请对照 rebuild_concepts.py 排查。

用法: python3 populate_concept_stocks_sina.py
 (spt=3).
Replaces old 新浪 concept API with more reliable 东财 slist endpoint.
Fetches concept/industry/area boards for each stock and inserts into stock_concepts.
"""
import os, sys, time, ssl, urllib.request, json
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
DELAY = 0.2

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE
HEADERS = {"User-Agent": "Mozilla/5.0"}

def fetch_stock_boards(code):
    """Fetch all concept/industry/area boards for a stock from 东财 slist API."""
    mkt = 1 if code.startswith("6") else 0
    url = f"https://push2.eastmoney.com/api/qt/slist/get?fltt=2&invt=2&secid={mkt}.{code}&spt=3&pi=0&pz=200&po=1&fields=f12,f14"
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0", "Referer": "https://quote.eastmoney.com/"})
        with urllib.request.urlopen(req, timeout=8, context=ctx) as resp:
            r = json.loads(resp.read().decode("utf-8"))
        diff = (r.get("data") or {}).get("diff") or {}
        items = diff.values() if isinstance(diff, dict) else diff
        return [(it.get("f12", ""), it.get("f14", "")) for it in items if it.get("f12")]
    except Exception as e:
        print(f"  ⚠️  {code} API 失败: {e}")
        return []

def main():
    print("=== 概念板块数据更新 (东财 slist) ===")
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    cur.execute("SELECT code, name FROM stocks_basic WHERE code IS NOT NULL AND code != '' ORDER BY code")
    stocks = cur.fetchall()
    total = len(stocks)
    print(f"共 {total} 只股票, 预计 {total * 0.4 / 60:.0f} 分钟")

    # 清空旧数据
    cur.execute("DELETE FROM stock_concepts")
    conn.commit()

    count = 0
    for i, (code, name) in enumerate(stocks):
        boards = fetch_stock_boards(code)
        if not boards:
            continue

        rows = [(code, bk_code, bk_name, "concept", name) for bk_code, bk_name in boards]
        sql = """
            INSERT INTO stock_concepts (code, concept_code, concept_name, concept_type, stock_name)
            VALUES %s
            ON CONFLICT (code, concept_code) DO UPDATE SET
                concept_name = EXCLUDED.concept_name, stock_name = EXCLUDED.stock_name
        """
        execute_values(cur, sql, rows, page_size=200)
        conn.commit()
        count += len(rows)

        if (i + 1) % 200 == 0:
            print(f"进度: {i+1}/{total} ({count} 条概念关联)")

        time.sleep(DELAY)

    cur.close()
    conn.close()
    print(f"完成! 共 {count} 条概念关联")

if __name__ == "__main__":
    main()
