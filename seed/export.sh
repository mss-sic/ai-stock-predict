#!/bin/bash
# 智策投研 — 种子数据导出（排除时序表，部署后实时采集）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SEED_DIR="$SCRIPT_DIR/data"
mkdir -p "$SEED_DIR"

PG_CONTAINER="${PG_CONTAINER:-docker-postgres-1}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-mysql-1}"

PG_USER="${PG_USER:-stock}"
PG_DB="${PG_DB:-stock_predict}"
MYSQL_USER="${MYSQL_USER:-stock}"
MYSQL_PASS="${MYSQL_PASS:-stock123}"
MYSQL_DB="${MYSQL_DB:-stock_predict}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PG_OUT="$SEED_DIR/pg_dump_${TIMESTAMP}.sql"
MYSQL_OUT="$SEED_DIR/mysql_dump_${TIMESTAMP}.sql"

echo "============================================"
echo "智策投研 — 种子数据导出"
echo "时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "============================================"

# ── PostgreSQL：只导出非时序表 ──
# stocks_daily_k 和 stocks_daily_indicator 是 TimescaleDB 时序表，部署后实时采集
# 排除它们避免 chunk 名不匹配
echo "【PostgreSQL】导出中（排除 K线/指标时序表）..."
docker exec "$PG_CONTAINER" pg_dump \
  -U "$PG_USER" -d "$PG_DB" \
  --no-owner --no-acl \
  --inserts --rows-per-insert=100 \
  --data-only \
  --exclude-table-data='stocks_daily_k' \
  --exclude-table-data='stocks_daily_indicator' \
  --exclude-table-data='_timescaledb_internal.*' \
  | sed '/^\\restrict /d' > "$PG_OUT" 2>/dev/null

PG_SIZE=$(du -h "$PG_OUT" | cut -f1)
echo "  ✓ PostgreSQL → $PG_OUT ($PG_SIZE)"

# ── MySQL 导出 ──
echo "【MySQL】导出中..."
docker exec "$MYSQL_CONTAINER" mysqldump \
  -u "$MYSQL_USER" -p"$MYSQL_PASS" \
  --no-tablespaces --no-create-info --complete-insert \
  --skip-triggers --skip-add-locks --skip-lock-tables \
  "$MYSQL_DB" \
  > "$MYSQL_OUT" 2>/dev/null

MYSQL_SIZE=$(du -h "$MYSQL_OUT" | cut -f1)
echo "  ✓ MySQL    → $MYSQL_OUT ($MYSQL_SIZE)"

# 清理旧导出（保留最近5个）
ls -t "$SEED_DIR"/pg_dump_*.sql 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true
ls -t "$SEED_DIR"/mysql_dump_*.sql 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true

cat > "$SEED_DIR/manifest.json" << MANIFEST
{
  "exported_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "pg_dump": "$(basename "$PG_OUT")",
  "pg_size": "$PG_SIZE",
  "mysql_dump": "$(basename "$MYSQL_OUT")",
  "mysql_size": "$MYSQL_SIZE",
  "note": "K线和指标数据未导出，部署后请在采集控制台拉取"
}
MANIFEST

echo ""
echo "导出完成 (K线/指标需部署后采集)"
echo "  PostgreSQL: $PG_OUT ($PG_SIZE)"
echo "  MySQL:      $MYSQL_OUT ($MYSQL_SIZE)"
