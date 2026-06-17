#!/bin/bash
# 智策投研 — 线上部署脚本（服务器端执行）
# 拉取最新镜像 → 构建前端 → 重启服务
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"
DEPLOY_DIR="/opt/ai-stock-predict"

echo "============================================"
echo "  智策投研 — 线上部署"
echo "============================================"

cd "$DEPLOY_DIR"

echo "[1/3] 构建前端..."
cd web-pc && npm install && npm run build && cd ..

echo "[2/3] 拉取最新镜像..."
docker pull "$IMAGE"

echo "[3/3] 重启服务..."
cd docker && docker compose up -d

echo ""
echo "✅ 部署完成"
echo "   Nginx 重载: nginx -s reload  (如需要)"
