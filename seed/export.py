#!/usr/bin/env python3
"""导出 PostgreSQL 种子数据为 CSV 文件"""
import csv, gzip, json, os, sys
from datetime import datetime

PG_DSN = "host=localhost dbname=stock_predict user=stock password=stock123"
SEED_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data")
os.makedirs(SEED_DIR, exist_ok=True)

TABLES = [
    "stocks_basic",
    ("stocks_daily_k", True),  # (table_name, gzip)
    "stocks_daily_indicator",
    "stock_quotes",
    "stock_signals",
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

def export_table(cur, table, gz=False):
    cur.execute(f"SELECT * FROM {table}")
    rows = cur.fetchall()
    cols = [desc[0] for desc in cur.description]
    
    outfile = os.path.join(SEED_DIR, f"{table}.csv")
    if gz:
        f = gzip.open(outfile + ".gz", "wt", encoding="utf-8")
    else:
        f = open(outfile, "w", newline="", encoding="utf-8")
    
    w = csv.writer(f)
    w.writerow(cols)
    for row in rows:
        w.writerow([str(v) if v is not None else "" for v in row])
    f.close()
    
    size = os.path.getsize(outfile + (".gz" if gz else ""))
    print(f"  ✓ {table:30s} {len(rows):>8,} 行  {size:>10,} bytes")
    return rows

def main():
    import psycopg2
    conn = psycopg2.connect(PG_DSN)
    cur = conn.cursor()
    
    print("=" * 50)
    print("智策投研 — 种子数据导出")
    print(f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"目标: {SEED_DIR}")
    print("=" * 50)
    
    manifest_files = []
    for t in TABLES:
        if isinstance(t, tuple):
            table, gz = t
        else:
            table, gz = t, False
        try:
            rows = export_table(cur, table, gz)
            filename = f"{table}.csv" + (".gz" if gz else "")
            manifest_files.append({
                "table": table,
                "file": filename,
                "rows": len(rows),
                "size": os.path.getsize(os.path.join(SEED_DIR, filename))
            })
        except Exception as e:
            print(f"  ✗ {table:30s} ERROR: {e}")
    
    cur.close()
    conn.close()
    
    # Write manifest
    manifest = {
        "exported_at": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "total_tables": len(manifest_files),
        "total_rows": sum(f["rows"] for f in manifest_files),
        "total_size": sum(f["size"] for f in manifest_files),
        "files": manifest_files
    }
    with open(os.path.join(SEED_DIR, "manifest.json"), "w") as f:
        json.dump(manifest, f, indent=2, ensure_ascii=False)
    
    print()
    print(f"完成: {len(manifest_files)} 表, {manifest['total_rows']:,} 行, {manifest['total_size']:,} bytes")

if __name__ == "__main__":
    main()
