#!/usr/bin/env python3
"""种子数据初始化 — 将预采集的 CSV 数据导入 PostgreSQL"""
import csv, gzip, json, os, sys
from datetime import datetime

PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")
SEED_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data")

# 导入顺序 (先导基础表, 再导依赖表)
TABLES = [
    "stocks_basic",
    ("stocks_daily_k", True),
    "stocks_daily_indicator",
    "stock_signals",
    "stock_quotes",
    "stock_financials",
    "stock_shareholders",
    "stock_news",
    "stock_reports",
    "algorithm_picks",
    "algorithm_pick_details",
    "predictions",
    "ai_analyses",
    "ai_stock_scores",
    "ai_conversations",
]

def check_seed_exists():
    manifest = os.path.join(SEED_DIR, "manifest.json")
    if not os.path.exists(manifest):
        print("❌ 未找到种子数据文件")
        print("   请先运行: python3 seed/export.py")
        return False
    with open(manifest) as f:
        m = json.load(f)
    print(f"种子数据: {m['total_tables']} 表, {m['total_rows']:,} 行, {m['total_size']:,} bytes")
    print(f"导出时间: {m['exported_at']}")
    return True

def import_table(cur, table, gz=False):
    filename = f"{table}.csv" + (".gz" if gz else "")
    filepath = os.path.join(SEED_DIR, filename)
    if not os.path.exists(filepath):
        print(f"  ⊘ {table}: 文件不存在")
        return 0
    
    if gz:
        f = gzip.open(filepath, "rt", encoding="utf-8")
    else:
        f = open(filepath, "r", encoding="utf-8")
    
    reader = csv.reader(f)
    header = next(reader)
    cols = ", ".join(f'"{c}"' for c in header)
    placeholders = ", ".join(["%s"] * len(header))
    
    rows = list(reader)
    f.close()
    
    if not rows:
        print(f"  ⊘ {table}: 空文件")
        return 0
    
    # TRUNCATE + INSERT
    cur.execute(f'TRUNCATE "{table}" CASCADE')
    
    sql = f'INSERT INTO "{table}" ({cols}) VALUES ({placeholders})'
    for row in rows:
        values = [v if v != "" else None for v in row]
        try:
            cur.execute(sql, values)
        except Exception as e:
            print(f"  ✗ {table} row error: {e} | row[0]={row[0][:50] if row else '?'}")
            raise
    
    print(f"  ✓ {table:30s} {len(rows):>8,} 行")
    return len(rows)

def main():
    import psycopg2
    
    print("=" * 50)
    print("智策投研 — 种子数据初始化")
    print(f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 50)
    
    if not check_seed_exists():
        sys.exit(1)
    
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    # 检查是否已有数据
    cur.execute("SELECT count(*) FROM stocks_basic")
    existing = cur.fetchone()[0]
    if existing > 0:
        print(f"\n⚠️  数据库已有 {existing} 条股票基础数据")
        answer = input("覆盖全部数据? (y/N): ").strip().lower()
        if answer != "y":
            print("已取消")
            conn.close()
            return
    
    print()
    total = 0
    for t in TABLES:
        if isinstance(t, tuple):
            table, gz = t
        else:
            table, gz = t, False
        try:
            n = import_table(cur, table, gz)
            total += n
        except Exception as e:
            print(f"  ✗ {table}: {e}")
            conn.rollback()
    
    conn.commit()
    
    # 验证
    print()
    print("【验证】")
    cur.execute("SELECT count(*) FROM stocks_basic")
    print(f"  stocks_basic: {cur.fetchone()[0]:,} 行")
    cur.execute("SELECT count(*) FROM stocks_daily_k")
    print(f"  stocks_daily_k: {cur.fetchone()[0]:,} 行")
    cur.execute("SELECT count(*) FROM stock_reports")
    print(f"  stock_reports: {cur.fetchone()[0]:,} 行")
    
    cur.close()
    conn.close()
    
    print()
    print(f"完成: 导入 {total:,} 行")
    print("=" * 50)

if __name__ == "__main__":
    main()
