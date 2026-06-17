#!/bin/bash
# 智策投研 — 线上部署脚本（服务器端执行）
# 拉取最新镜像 → 重启服务 → 提取前端静态文件
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"
DEPLOY_DIR="/opt/ai-stock-predict"

echo "============================================"
echo "  智策投研 — 线上部署"
echo "============================================"

cd "$DEPLOY_DIR/docker"

echo "[1/3] 拉取最新镜像..."
docker pull "$IMAGE"

echo "[2/3] 重启服务..."
docker compose up -d

sleep 3

echo "[3/3] 提取前端静态文件..."
docker cp aip-server:/app/web-dist/. "$DEPLOY_DIR/web-pc/dist/"

echo ""
echo "✅ 部署完成"
echo "   Nginx 重载: nginx -s reload  (如需要)"
