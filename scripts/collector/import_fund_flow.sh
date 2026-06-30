#!/bin/bash
# Import fund_flow data dump into PostgreSQL
# Usage: bash import_fund_flow.sh

set -e

DUMP="$(dirname "$0")/fund_flow_dump.sql.gz"
PG_DSN="${PG_DSN:-host=localhost dbname=stock_predict user=stock password=stock123}"

# Parse PG_DSN into psql args
HOST=$(echo "$PG_DSN" | grep -oP 'host=\K\S+')
DB=$(echo "$PG_DSN" | grep -oP 'dbname=\K\S+')
USER=$(echo "$PG_DSN" | grep -oP 'user=\K\S+')

echo "Importing fund_flow data into $DB @ $HOST ..."
zcat "$DUMP" | psql -h "$HOST" -U "$USER" -d "$DB" -q

echo "Done: $(psql -h "$HOST" -U "$USER" -d "$DB" -t -c 'SELECT COUNT(*) FROM stock_fund_flow') rows"
