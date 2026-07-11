#!/bin/bash
# 智策投研 — Docker 入口脚本
# 用法:
#   docker run ... server        # 启动 Web 服务（默认）
#   docker run ... migrate       # 执行数据库迁移
#   docker run ... migrate --dry-run  # 查看待执行迁移

set -e

cd /app

case "${1:-server}" in
  server)
    echo "[entrypoint] 启动 Web 服务..."
    exec ./server
    ;;
  migrate)
    shift
    echo "[entrypoint] 执行数据库迁移..."
    exec ./migrate "$@"
    ;;
  *)
    echo "[entrypoint] 未知命令: $1"
    echo "可用命令: server | migrate [--dry-run] [--force N]"
    exit 1
    ;;
esac
