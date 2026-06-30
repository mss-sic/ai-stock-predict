#!/usr/bin/env python3
"""Import fund_flow data dump into PostgreSQL via psycopg2."""
import os, sys, gzip, psycopg2

DUMP = os.path.join(os.path.dirname(__file__), "fund_flow_dump.sql.gz")
PG_DSN = os.environ.get("PG_DSN", "host=localhost dbname=stock_predict user=stock password=stock123")

if not os.path.exists(DUMP):
    print(f"ERROR: dump not found: {DUMP}")
    sys.exit(1)

conn = psycopg2.connect(PG_DSN)
cur = conn.cursor()

# Truncate existing data
cur.execute("TRUNCATE stock_fund_flow")
print("Truncated stock_fund_flow")

# Read and execute SQL batches
with gzip.open(DUMP, 'rt') as f:
    sql = f.read()

# Split into statements (skip comments, first is TRUNCATE which we already did)
statements = []
buf = []
for line in sql.split('\n'):
    stripped = line.strip()
    if stripped.startswith('--') or stripped == '':
        continue
    buf.append(line)
    if stripped.endswith(';'):
        s = '\n'.join(buf)
        if not s.strip().upper().startswith('TRUNCATE'):
            statements.append(s)
        buf = []

print(f"Executing {len(statements)} INSERT batches...")
for i, stmt in enumerate(statements):
    cur.execute(stmt)
    if (i + 1) % 100 == 0:
        conn.commit()
        print(f"  {i+1}/{len(statements)} batches")
conn.commit()

cur.execute("SELECT COUNT(*) FROM stock_fund_flow")
count = cur.fetchone()[0]
print(f"Done: {count} rows imported")
cur.close(); conn.close()
