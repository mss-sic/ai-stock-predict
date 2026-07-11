#!/bin/bash
# 服务器端部署脚本
set -euo pipefail

# 与 docker-compose.yml 中 server.image 保持一致
IMAGE="crpi-t3tis8f2l2fb8jc9-vpc.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"
COMPOSE_DIR="/opt/ai-stock-predict/docker"
WEB_ROOT="/www/wwwroot/ai-stock-predict"

cd "$COMPOSE_DIR"

echo "═══════════════════════════════════════"
echo "  智策投研 — 服务器部署"
echo "═══════════════════════════════════════"
echo ""

# 1. 拉取最新 server 镜像
echo "▸ 拉取 server 镜像..."
docker pull "$IMAGE"
echo "✓ 镜像已更新"
echo ""

# 2. 验证镜像包含 migrate 二进制
echo "▸ 验证 migrate..."
if ! docker run --rm --entrypoint /bin/ls "$IMAGE" /app/migrate > /dev/null 2>&1; then
    echo "❌ 镜像中未找到 /app/migrate，请先运行 publish.sh 构建新镜像"
    exit 1
fi
echo "✓ migrate 二进制存在"
echo ""

# 3. 执行数据库迁移
echo "▸ 执行数据库迁移..."
if docker compose run --rm --no-deps server migrate; then
    echo "✓ 迁移完成"
else
    echo ""
    echo "❌ 迁移失败！请根据上方错误修复。"
    echo "   dry-run: docker compose run --rm --no-deps server migrate --dry-run"
    echo "   强制修复: docker compose run --rm --no-deps server migrate --force <版本号>"
    exit 1
fi
echo ""

# 4. 重启服务
echo "▸ 重启服务..."
docker compose up -d server
echo "✓ 服务已重启"
echo ""

# 5. 等待健康检查
echo "▸ 等待服务就绪..."
for i in $(seq 1 15); do
    if curl -sf http://localhost:8080/api/v1/indices > /dev/null 2>&1; then
        echo "✓ 服务就绪"
        break
    fi
    sleep 2
done

echo "▸ 构建前端..."
cd web-pc && npm install && npm run build && cd ..

echo "▸ 部署前端到生产目录..."
mkdir -p "$WEB_ROOT"
rsync -a --delete --exclude=".user.ini" web-pc/dist/ "$WEB_ROOT/"

echo ""
echo "═══════════════════════════════════════"
echo "  部署完成"
echo "═══════════════════════════════════════"
