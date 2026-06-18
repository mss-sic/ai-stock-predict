#!/bin/bash
# 智策投研 — 线上部署脚本（服务器端执行）
# 拉取最新代码 → 拉取镜像 → 重启服务 → 构建前端 → 部署到生产目录
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9-vpc.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"
DEPLOY_DIR="/opt/ai-stock-predict"
WEB_ROOT="/www/wwwroot/ai-stock-predict"

echo "============================================"
echo "  智策投研 — 线上部署"
echo "============================================"

cd "$DEPLOY_DIR"

echo "[1/5] 拉取最新代码..."
git pull

echo "[2/5] 拉取最新镜像（内网）..."
docker pull "$IMAGE"

echo "[3/5] 重启服务..."
cd docker && docker compose up -d --force-recreate server && cd ..

echo "[4/5] 构建前端..."
cd web-pc && npm install && npm run build && cd ..

echo "[5/5] 部署前端到生产目录..."
mkdir -p "$WEB_ROOT"
rsync -a --delete --exclude=".user.ini" web-pc/dist/ "$WEB_ROOT/"

echo ""
echo "✅ 部署完成"
echo "   Nginx 重载: nginx -s reload  (如需要)"
