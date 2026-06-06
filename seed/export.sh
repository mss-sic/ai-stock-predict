#!/bin/bash
# ============================================================
# 智策投研 - 种子数据导出脚本
# 导出当前 PostgreSQL 关键数据为部署初始化数据
# 用法: bash seed/export.sh
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
echo "智策投研 — 种子数据导出"
echo "数据库: $DB_HOST/$DB_NAME"
echo "目标:   $SEED_DIR"
echo "============================================"

mkdir -p "$SEED_DIR"

# 导出函数: 使用 COPY 命令 (更快)
export_table() {
    local table="$1"
    local file="$SEED_DIR/${table}.csv"
    echo "→ 导出 $table ..."
    psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" \
        -c "\\COPY $table TO '$file' WITH CSV HEADER" 2>&1 | grep -v "^COPY"
    local rows=$(wc -l < "$file" | tr -d ' ')
    echo "   $rows 行 → ${table}.csv"
}

# ============ PostgreSQL 核心数据 ============
echo ""
echo "【PostgreSQL 数据表】"

# 基础数据
export_table "stocks_basic"

# K线数据 (较大, 压缩)
echo "→ 导出 stocks_daily_k (较大, 请等待) ..."
psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" \
    -c "\\COPY stocks_daily_k TO '$SEED_DIR/stocks_daily_k.csv' WITH CSV HEADER" 2>&1 | grep -v "^COPY"
rows=$(wc -l < "$SEED_DIR/stocks_daily_k.csv" | tr -d ' ')
echo "   $rows 行 → stocks_daily_k.csv"

# 日指标
export_table "stocks_daily_indicator"

# 行情快照
export_table "stock_quotes"

# 信号
export_table "stock_signals"

# 财务
export_table "stock_financials"

# 股东
export_table "stock_shareholders"

# 资讯
export_table "stock_news"

# 研报
export_table "stock_reports"

# 算法榜单
export_table "algorithm_picks"
export_table "algorithm_pick_details"

# 预测
export_table "predictions"

# AI 分析
export_table "ai_analyses"
export_table "ai_stock_scores"
export_table "ai_conversations"

# ============ 压缩大文件 ============
echo ""
echo "【压缩】"
gzip -f "$SEED_DIR/stocks_daily_k.csv"
echo "   stocks_daily_k.csv.gz ($(du -h "$SEED_DIR/stocks_daily_k.csv.gz" | cut -f1))"

# ============ 生成清单 ============
echo ""
echo "【生成清单】"
MANIFEST="$SEED_DIR/manifest.json"
python3 -c "
import json, os, glob
files = []
for f in sorted(glob.glob('$SEED_DIR/*.csv') + glob.glob('$SEED_DIR/*.gz')):
    name = os.path.basename(f)
    size = os.path.getsize(f)
    files.append({'file': name, 'size': size})
total = sum(f['size'] for f in files)
print(json.dumps({'exported_at': '$(date -u +%Y-%m-%dT%H:%M:%SZ)', 'total_size': total, 'files': files}, indent=2, ensure_ascii=False))
" > "$MANIFEST"
cat "$MANIFEST"

echo ""
echo "============================================"
echo "导出完成！数据文件在: $SEED_DIR"
echo "============================================"
