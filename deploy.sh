#!/bin/bash
# 智策投研 — 线上部署脚本（服务器端执行）
# 拉取最新镜像 → 构建前端 → 部署到生产目录 → 重启服务
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"
DEPLOY_DIR="/opt/ai-stock-predict"
WEB_ROOT="/www/wwwroot/ai-stock-predict"

echo "============================================"
echo "  智策投研 — 线上部署"
echo "============================================"

cd "$DEPLOY_DIR"

echo "[1/4] 拉取最新镜像..."
docker pull "$IMAGE"

echo "[2/4] 重启服务..."
cd docker && docker compose up -d && cd ..

echo "[3/4] 构建前端..."
cd web-pc && npm install && npm run build && cd ..

echo "[4/4] 部署前端到生产目录..."
mkdir -p "$WEB_ROOT"
rsync -a --delete web-pc/dist/ "$WEB_ROOT/"

echo ""
echo "✅ 部署完成"
echo "   Nginx 重载: nginx -s reload  (如需要)"
