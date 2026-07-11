#!/bin/bash
# 构建 linux/amd64 镜像并推送到阿里云容器镜像
set -euo pipefail

IMAGE="crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest"

echo "============================================"
echo "  构建 & 推送镜像 (linux/amd64)"
echo "  $IMAGE"
echo "============================================"

cd "$(dirname "$0")"

docker buildx build --platform linux/amd64 -t "$IMAGE" --push .

echo ""
echo "推送完成: $IMAGE"
echo ""
echo "服务器端执行:"
echo "  scp deploy.sh root@your-server:/opt/ai-stock-predict/"
echo "  ssh root@your-server 'cd /opt/ai-stock-predict && ./deploy.sh'"
echo ""
echo "或手动逐步执行:"
echo "  docker pull $IMAGE"
echo "  cd /opt/ai-stock-predict/docker"
echo "  docker compose run --rm server migrate"
echo "  docker compose up -d"
