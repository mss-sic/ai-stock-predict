#!/bin/bash
# 本地构建镜像并推送到 GitHub Container Registry
set -euo pipefail

IMAGE="ghcr.io/mss-sic/ai-stock-predict/server:latest"

echo "============================================"
echo "  构建 & 推送镜像"
echo "  $IMAGE"
echo "============================================"

cd "$(dirname "$0")"

# 构建
docker build -t "$IMAGE" .

# 推送（需要先登录: echo $CR_PAT | docker login ghcr.io -u USER --password-stdin）
docker push "$IMAGE"

echo "推送完成: $IMAGE"
