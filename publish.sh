#!/bin/bash
# 构建 linux/amd64 镜像并推送到阿里云容器镜像
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"

echo "============================================"
echo "  构建 & 推送镜像 (linux/amd64)"
echo "  $IMAGE"
echo "============================================"

cd "$(dirname "$0")"

# 跨平台构建：本地 arm64 Mac → 目标 amd64 服务器
docker buildx build --platform linux/amd64 -t "$IMAGE" --push .

echo "推送完成: $IMAGE"
echo ""
echo "============================================"
echo "  服务器更新命令:"
echo "============================================"
echo ""
echo "docker pull $IMAGE"
echo "cd /opt/ai-stock-predict/docker && docker compose up -d"
echo "docker cp aip-server:/app/web-dist/. /opt/ai-stock-predict/web-pc/dist/"
echo ""
