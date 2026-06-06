#!/bin/bash
# ============================================================
# 智策投研 — 种子数据初始化脚本
# 部署时自动导入预采集的基础数据
# 用法: bash seed/init.sh
# ============================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SEED_DIR="$SCRIPT_DIR/data"

DB_HOST="${PG_HOST:-localhost}"
DB_NAME="${PG_DB:-stock_predict}"
DB_USER="${PG_USER:-stock}"
DB_PASS="${PG_PASS:-stock123}"

export PGPASSWORD="$DB_PASS"

echo "============================================"
echo "智策投研 — 种子数据初始化"
echo "数据库: $DB_HOST/$DB_NAME"
echo "============================================"

# 检查数据文件
if [ ! -f "$SEED_DIR/stocks_basic.csv" ]; then
    echo "❌ 未找到种子数据文件，请先运行 bash seed/export.sh 导出数据"
    exit 1
fi

# 解压大文件
if [ -f "$SEED_DIR/stocks_daily_k.csv.gz" ] && [ ! -f "$SEED_DIR/stocks_daily_k.csv" ]; then
    echo "→ 解压 K线数据..."
    gunzip -k "$SEED_DIR/stocks_daily_k.csv.gz"
fi

# 导入函数
import_table() {
    local table="$1"
    local file="$SEED_DIR/${table}.csv"
    if [ ! -f "$file" ]; then
        echo "⊘ 跳过 $table (文件不存在)"
        return
    fi
    local rows=$(($(wc -l < "$file") - 1))
    echo "→ 导入 $table ($rows 行) ..."
    psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c "TRUNCATE $table CASCADE" 2>/dev/null || true
    psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" \
        -c "\\COPY $table FROM '$file' WITH CSV HEADER" 2>&1 | grep -v "^COPY" || true
    echo "   ✓ $table"
}

# ============ 检查表是否存在 ============
echo ""
echo "【检查数据库表】"
TABLE_COUNT=$(psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'" 2>/dev/null | tr -d ' ')
echo "   当前表数: $TABLE_COUNT"

if [ "$TABLE_COUNT" -lt 5 ]; then
    echo "   表不存在，请先启动服务端完成 AutoMigrate (go run cmd/server/main.go)"
    echo "   或手动执行: psql -h $DB_HOST -U $DB_USER -d $DB_NAME -f db/schema.sql"
    exit 1
fi

# ============ 检查是否已有数据 ============
echo ""
EXISTING=$(psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT count(*) FROM stocks_basic" 2>/dev/null | tr -d ' ')
if [ "$EXISTING" -gt 0 ]; then
    echo "⚠️  数据库已有 $EXISTING 条股票基础数据"
    echo ""
    echo "选择操作:"
    echo "  1) 覆盖全部数据 (TRUNCATE + 重新导入)"
    echo "  2) 跳过已有表 (只导入空表)"
    echo "  3) 取消"
    read -p "请输入 (1/2/3): " choice
    case $choice in
        1) MODE="overwrite" ;;
        2) MODE="skip_existing" ;;
        *) echo "已取消"; exit 0 ;;
    esac
else
    MODE="overwrite"
fi

# ============ 导入数据 ============
echo ""
echo "【导入数据】(模式: $MODE)"

import_if_needed() {
    local table="$1"
    local file="$SEED_DIR/${table}.csv"
    if [ ! -f "$file" ]; then
        return
    fi
    if [ "$MODE" = "skip_existing" ]; then
        local cnt=$(psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT count(*) FROM $table" 2>/dev/null | tr -d ' ')
        if [ "$cnt" -gt 0 ]; then
            echo "⊘ 跳过 $table (已有 $cnt 行)"
            return
        fi
    fi
    import_table "$table"
}

# 按依赖顺序导入
import_if_needed "stocks_basic"
import_if_needed "stocks_daily_k"
import_if_needed "stocks_daily_indicator"
import_if_needed "stock_signals"
import_if_needed "stock_quotes"
import_if_needed "stock_financials"
import_if_needed "stock_shareholders"
import_if_needed "stock_news"
import_if_needed "stock_reports"
import_if_needed "algorithm_picks"
import_if_needed "algorithm_pick_details"
import_if_needed "predictions"
import_if_needed "ai_analyses"
import_if_needed "ai_stock_scores"
import_if_needed "ai_conversations"

# ============ 验证 ============
echo ""
echo "【验证导入结果】"
psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c "
SELECT 'stocks_basic' AS \"表\", count(*) AS \"行数\" FROM stocks_basic
UNION ALL SELECT 'stocks_daily_k', count(*) FROM stocks_daily_k
UNION ALL SELECT 'stock_financials', count(*) FROM stock_financials
UNION ALL SELECT 'stock_reports', count(*) FROM stock_reports
UNION ALL SELECT 'stock_shareholders', count(*) FROM stock_shareholders
UNION ALL SELECT 'algorithm_picks', count(*) FROM algorithm_picks
ORDER BY \"行数\" DESC;
" 2>/dev/null

echo ""
echo "============================================"
echo "种子数据初始化完成！"
echo "============================================"
