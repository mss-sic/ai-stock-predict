#!/bin/bash
# 智策投研 — 种子数据导出
# 通过 Docker 容器导出 PostgreSQL + MySQL 为 SQL 文件
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SEED_DIR="$SCRIPT_DIR/data"
mkdir -p "$SEED_DIR"

# 容器名称
PG_CONTAINER="${PG_CONTAINER:-docker-postgres-1}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-mysql-1}"

# 数据库连接
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
echo ""

# ── PostgreSQL 导出 ──
echo "【PostgreSQL】导出中..."
docker exec "$PG_CONTAINER" pg_dump \
  -U "$PG_USER" -d "$PG_DB" \
  --no-owner --no-acl \
  --inserts --rows-per-insert=100 \
  --data-only \
  | sed "/^\\\estrict /d" > "$PG_OUT" 2>/dev/null

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

# ── 删除旧的导出文件（保留最近 5 个） ──
echo ""
echo "清理旧导出..."
ls -t "$SEED_DIR"/pg_dump_*.sql 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true
ls -t "$SEED_DIR"/mysql_dump_*.sql 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null || true

# ── 创建 manifest ──
cat > "$SEED_DIR/manifest.json" << MANIFEST
{
  "exported_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "pg_dump": "$(basename "$PG_OUT")",
  "pg_size": "$PG_SIZE",
  "mysql_dump": "$(basename "$MYSQL_OUT")",
  "mysql_size": "$MYSQL_SIZE"
}
MANIFEST

echo ""
echo "============================================"
echo "导出完成!"
echo "  PostgreSQL: $PG_OUT ($PG_SIZE)"
echo "  MySQL:      $MYSQL_OUT ($MYSQL_SIZE)"
echo "============================================"
