#!/usr/bin/env python3
"""
填充 stock_concepts — 东财 stock/get API (Limited / 有限覆盖)
============================================================================
⚠️  DEPRECATED (降级标记, 2026-06-29):
  本脚本使用东财 push2 stock/get API 的 f129 字段获取概念标签，该接口仅返回
  每只股票 5-10 个"热门"概念标签，远不如 slist API (spt=3) 完整。
  
  对比 (以 300502 新易盛 为例):
    - f129 接口: 6 个概念 (CPO概念, 算力概念, 光通信模块, ...)
    - slist 接口: 28 个概念 (含行业/地域/指数归属等全部板块)
    - slist 比 f129 多 4-5 倍覆盖

  推荐使用: rebuild_concepts.py 或 populate_concept_stocks_sina.py (均已升级为 slist)

  如确需使用本脚本: 仅作为概念标签的快速补充，不应作为主要数据源。

用法: python3 populate_concept_stocks.py
 (f129 field).
Maps each stock's concept tags to 新浪 concept boards and inserts into stock_concepts.
"""
import os, sys, time, ssl, urllib.request, json
import psycopg2
from psycopg2.extras import execute_values

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
EM_BASE = "http://push2.eastmoney.com/api/qt/stock/get"
BATCH_SIZE = 100
DELAY = 0.15  # seconds between requests

ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

def fetch_stock_concepts(code):
    """Fetch concept tags for a single stock from 东方财富."""
    prefix = "1" if code.startswith(("6", "9")) else "0"
    secid = f"{prefix}.{code}"
    url = f"{EM_BASE}?secid={secid}&fields=f57,f58,f129"
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=8, context=ctx) as resp:
            data = json.loads(resp.read().decode("utf-8")).get("data", {})
        concepts_str = data.get("f129", "")
        if not concepts_str or concepts_str == "-":
            return []
        return [c.strip() for c in concepts_str.split(",") if c.strip()]
    except Exception:
        return []

def main():
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()

    # Get stocks without concept mappings
    cur.execute("""
        SELECT DISTINCT b.code, b.name
        FROM stocks_basic b
        WHERE b.code NOT IN (SELECT DISTINCT code FROM stock_concepts)
        ORDER BY b.code
    """)
    pending = cur.fetchall()
    if not pending:
        print("所有股票已有概念数据", flush=True)
        return

    # Get existing concept boards for name matching
    cur.execute("SELECT concept_code, concept_name FROM concept_boards WHERE concept_type = 'concept'")
    board_map = {row[1]: row[0] for row in cur.fetchall()}
    print(f"概念板块: {len(board_map)} 个，待处理股票: {len(pending)} 只", flush=True)

    # Also collect stock names from pending for matching
    pending_names = {name for _, name in pending}

    total_mapped = 0
    insert_batch = []
    errors = 0

    for i, (code, name) in enumerate(pending):
        concepts = fetch_stock_concepts(code)
        if not concepts:
            continue

        for concept_name in concepts:
            # Try exact match first
            if concept_name in board_map:
                insert_batch.append((code, board_map[concept_name], concept_name, "concept", name))
            # Try fuzzy: concept_name + "概念"
            elif concept_name + "概念" in board_map:
                insert_batch.append((code, board_map[concept_name + "概念"], concept_name + "概念", "concept", name))

        # Flush batch periodically
        if len(insert_batch) >= 200:
            try:
                execute_values(cur, """
                    INSERT INTO stock_concepts (code, concept_code, concept_name, concept_type, stock_name, updated_at)
                    VALUES %s
                    ON CONFLICT (code, concept_code) DO NOTHING
                """, [(r[0], r[1], r[2], r[3], r[4], "NOW()") for r in insert_batch], page_size=200)
                conn.commit()
                total_mapped += len(insert_batch)
                insert_batch = []
            except Exception as e:
                conn.rollback()
                errors += 1
                insert_batch = []
                print(f"  ❌ batch insert error: {e}", flush=True)

        if (i + 1) % 200 == 0:
            print(f"  进度: {i+1}/{len(pending)} ({100*(i+1)//len(pending)}%) | 已映射 {total_mapped} 条 | 错误 {errors}", flush=True)

        time.sleep(DELAY)

    # Final flush
    if insert_batch:
        try:
            execute_values(cur, """
                INSERT INTO stock_concepts (code, concept_code, concept_name, concept_type, stock_name, updated_at)
                VALUES %s
                ON CONFLICT (code, concept_code) DO NOTHING
            """, [(r[0], r[1], r[2], r[3], r[4], "NOW()") for r in insert_batch], page_size=200)
            conn.commit()
            total_mapped += len(insert_batch)
        except Exception as e:
            conn.rollback()
            print(f"  ❌ final batch error: {e}", flush=True)

    cur.execute("SELECT COUNT(*) FROM stock_concepts")
    final_count = cur.fetchone()[0]
    cur.close()
    conn.close()

    print(f"\n✅ 完成: 新增 {total_mapped} 条关联 | 总关联数 {final_count} | 错误 {errors}", flush=True)

if __name__ == "__main__":
    main()
