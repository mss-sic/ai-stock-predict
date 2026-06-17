#!/usr/bin/env python3
"""Populate stock_concepts using 东方财富 stock/get API (f129 field).
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
