#!/usr/bin/env python3
"""Populate stock_concepts using 新浪 concept stock list API.
Fetches stock lists for all concept boards and inserts into stock_concepts.
Data source: vip.stock.finance.sina.com.cn Market_Center.getHQNodeData
"""
import os, sys, time, ssl, urllib.request, json
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
SINA_API = "http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData"
DELAY = 0.3  # seconds between requests (be gentle)

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE
HEADERS = {"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"}


def fetch_concept_stocks(node_code):
    """Fetch stock list for a concept from Sina API."""
    url = f"{SINA_API}?page=1&num=500&sort=symbol&asc=1&node={node_code}"
    try:
        req = urllib.request.Request(url, headers=HEADERS)
        with urllib.request.urlopen(req, timeout=15, context=ctx) as resp:
            data = json.loads(resp.read().decode('utf-8'))
        if not isinstance(data, list):
            print(f"     ⚠️  非预期响应: {type(data)}")
            return []
        return [(item['code'], item['name']) for item in data if 'code' in item and 'name' in item]
    except Exception as e:
        print(f"     ❌ API错误: {e}")
        return []


def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Get all concept boards
    cur.execute("""
        SELECT concept_code, concept_name, stock_count
        FROM concept_boards WHERE concept_type = 'concept'
        ORDER BY stock_count DESC
    """)
    boards = cur.fetchall()
    print(f"📊 新浪概念板块成分股填充")
    print(f"📋 共 {len(boards)} 个概念板块")
    print(f"📡 数据源: {SINA_API}")
    print()

    total_stocks = 0
    total_inserted = 0
    success = 0
    failed = 0

    t_start = time.time()
    for idx, (code, name, expected_count) in enumerate(boards):
        elapsed = time.time() - t_start
        rate = (idx + 1) / max(elapsed, 0.1)
        eta = (len(boards) - idx - 1) / max(rate, 0.01)
        print(f"  [{idx+1}/{len(boards)}] {name} ({code}) | 预期 {expected_count}只 | 速度 {rate:.1f}/s | 预计剩余 {eta:.0f}s", flush=True, end=' ')
        
        stocks = fetch_concept_stocks(code)
        
        if not stocks:
            print(f"❌ 0只 (获取失败)")
            failed += 1
            time.sleep(DELAY)
            continue

        # Insert into stock_concepts
        rows = [(s[0], code, name, 'concept', s[1]) for s in stocks]
        try:
            execute_values(cur, """
                INSERT INTO stock_concepts (code, concept_code, concept_name, concept_type, stock_name)
                VALUES %s
                ON CONFLICT (code, concept_code) DO UPDATE SET
                    concept_name = EXCLUDED.concept_name,
                    stock_name = EXCLUDED.stock_name,
                    updated_at = NOW()
            """, rows, page_size=200)
            conn.commit()
        except Exception as e:
            print(f"❌ DB错误: {e}")
            conn.rollback()
            failed += 1
            time.sleep(DELAY)
            continue

        print(f"✅ {len(stocks)}只")
        print(f"STAT:concept_fetched={len(stocks)},concept_board={name}", flush=True)
        total_inserted += len(stocks)
        total_stocks += len(stocks)
        success += 1

        # Update board stock_count to actual
        if len(stocks) != expected_count:
            cur.execute("UPDATE concept_boards SET stock_count = %s WHERE concept_code = %s",
                       (len(stocks), code))
            conn.commit()

        time.sleep(DELAY)

    # Final stats
    cur.execute("""
        SELECT concept_type, COUNT(*), COUNT(DISTINCT code)
        FROM stock_concepts GROUP BY concept_type
    """)
    print(f"\n{'='*50}")
    print(f"✅ 填充完成")
    for r in cur.fetchall():
        print(f"   {r[0]}: {r[1]}条记录, {r[2]}只股票")
    print(f"   成功: {success} 板块 | 失败: {failed} 板块")
    print(f"   总插入: {total_inserted} 条")

    cur.close()
    conn.close()


if __name__ == "__main__":
    main()
