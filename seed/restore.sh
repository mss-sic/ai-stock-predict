#!/bin/bash
# 智策投研 — 种子数据恢复
# 通过 Docker 容器将 SQL 种子文件导入到目标数据库
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SEED_DIR="$SCRIPT_DIR/data"

# 容器名称
PG_CONTAINER="${PG_CONTAINER:-docker-postgres-1}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-mysql-1}"

# 数据库连接
PG_USER="${PG_USER:-stock}"
PG_DB="${PG_DB:-stock_predict}"

MYSQL_USER="${MYSQL_USER:-stock}"
MYSQL_PASS="${MYSQL_PASS:-stock123}"
MYSQL_DB="${MYSQL_DB:-stock_predict}"

echo "============================================"
echo "智策投研 — 种子数据恢复"
echo "时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "============================================"
echo ""

# ── 查找最新的 SQL 导出文件 ──
PG_FILE=$(ls -t "$SEED_DIR"/pg_dump_*.sql 2>/dev/null | head -1)
MYSQL_FILE=$(ls -t "$SEED_DIR"/mysql_dump_*.sql 2>/dev/null | head -1)

if [ -z "$PG_FILE" ] && [ -z "$MYSQL_FILE" ]; then
  echo "❌ 未找到种子数据文件"
  echo "   请先在有数据的机器上运行: bash seed/export.sh"
  echo "   然后将 seed/data/ 目录复制到目标机器"
  exit 1
fi

# ── 检查目标数据库是否已有数据 ──
echo "检查目标数据库..."
EXISTING=$(docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -t -c "SELECT count(*) FROM stocks_basic" 2>/dev/null | tr -d ' ' || echo "0")
if [ "$EXISTING" -gt 0 ] 2>/dev/null; then
  echo "⚠️  目标数据库已有 $EXISTING 条股票基础数据"
  read -r -p "覆盖全部数据? [y/N]: " REPLY
  if [ "${REPLY,,}" != "y" ]; then
    echo "已取消"
    exit 0
  fi
fi

# ── PostgreSQL 恢复 ──
if [ -n "$PG_FILE" ]; then
  PG_SIZE=$(du -h "$PG_FILE" | cut -f1)
  echo ""
  echo "【PostgreSQL】导入 $PG_FILE ($PG_SIZE) ..."

  # 清空现有数据（保留表结构）
  echo "  清空现有数据..."
  docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -c "
    DO \$\$ DECLARE r RECORD;
    BEGIN
      FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename NOT IN ('spatial_ref_sys'))
      LOOP
        EXECUTE 'TRUNCATE TABLE ' || quote_ident(r.tablename) || ' CASCADE';
      END LOOP;
    END \$\$;
  " 2>/dev/null

  # 导入数据（通过 stdin 传给容器内的 psql）
  echo "  导入数据..."
  docker exec -i "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" < "$PG_FILE" 2>/dev/null

  # 验证
  PG_COUNT=$(docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -t -c "SELECT count(*) FROM stocks_basic" 2>/dev/null | tr -d ' ')
  echo "  ✓ PostgreSQL 恢复完成 ($PG_COUNT 条)"
fi

# ── MySQL 恢复 ──
if [ -n "$MYSQL_FILE" ]; then
  MYSQL_SIZE=$(du -h "$MYSQL_FILE" | cut -f1)
  echo ""
  echo "【MySQL】导入 $MYSQL_FILE ($MYSQL_SIZE) ..."

  # 清空现有数据
  echo "  清空现有数据..."
  TABLES=$(docker exec "$MYSQL_CONTAINER" mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" -N -e "SELECT table_name FROM information_schema.tables WHERE table_schema='$MYSQL_DB' AND table_type='BASE TABLE'" 2>/dev/null)
  if [ -n "$TABLES" ]; then
    docker exec "$MYSQL_CONTAINER" mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "SET FOREIGN_KEY_CHECKS=0;" 2>/dev/null
    for t in $TABLES; do
      docker exec "$MYSQL_CONTAINER" mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "TRUNCATE TABLE \`$t\`" "$MYSQL_DB" 2>/dev/null
    done
    docker exec "$MYSQL_CONTAINER" mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "SET FOREIGN_KEY_CHECKS=1;" 2>/dev/null
  fi

  # 导入数据
  echo "  导入数据..."
  docker exec -i "$MYSQL_CONTAINER" mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" "$MYSQL_DB" < "$MYSQL_FILE" 2>/dev/null

  echo "  ✓ MySQL 恢复完成"
fi

echo ""
echo "============================================"
echo "恢复完成!"
echo "============================================"
echo ""
echo "默认管理员账号: admin / admin123"
