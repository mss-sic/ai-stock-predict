#!/bin/bash
# 服务器端部署脚本（在服务器上运行）
# 用法: ./deploy.sh
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"
COMPOSE_DIR="/opt/ai-stock-predict/docker"

cd "$COMPOSE_DIR"

echo "═══════════════════════════════════════"
echo "  智策投研 — 服务器部署"
echo "═══════════════════════════════════════"
echo ""

# 1. 拉取最新镜像
echo "▸ 拉取镜像..."
docker pull "$IMAGE"
echo "✓ 镜像已更新"
echo ""

# 2. 执行数据库迁移
echo "▸ 执行数据库迁移..."
if docker compose run --rm server migrate; then
    echo "✓ 迁移完成"
else
    echo ""
    echo "❌ 迁移失败！请根据上方错误修复后重新执行。"
    echo "   可使用 dry-run 查看: docker compose run --rm server migrate --dry-run"
    exit 1
fi
echo ""

# 3. 重启服务
echo "▸ 重启服务..."
docker compose up -d
echo "✓ 服务已重启"
echo ""

# 4. 等待健康检查
echo "▸ 等待服务就绪..."
for i in $(seq 1 15); do
    if curl -sf http://localhost:8080/api/v1/indices > /dev/null 2>&1; then
        echo "✓ 服务就绪"
        break
    fi
    sleep 2
done

echo ""
echo "═══════════════════════════════════════"
echo "  部署完成"
echo "═══════════════════════════════════════"
