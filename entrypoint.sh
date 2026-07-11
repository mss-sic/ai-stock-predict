#!/bin/bash
# 智策投研 — Docker 入口脚本
set -e

case "${1:-server}" in
  server)
    echo "[entrypoint] 启动 Web 服务..."
    exec /app/server
    ;;
  migrate)
    shift
    echo "[entrypoint] 执行数据库迁移..."
    exec /app/migrate "$@"
    ;;
  *)
    echo "[entrypoint] 未知命令: $1"
    echo "可用命令: server | migrate [--dry-run] [--force N]"
    exit 1
    ;;
esac
