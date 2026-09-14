#!/bin/bash
# 智策投研 — 本地服务端启动脚本
#
# 流程：编译最新代码 → 停止占用端口的旧进程 → 启动新服务 → 校验端口
# 用法：./start.sh
#
# 注意：Go 代码中多处使用 scripts/collector 等相对路径，
#      因此必须以项目根目录作为工作目录启动。

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
PORT="${PORT:-8080}"
LOG_OUT="/tmp/stock-server-out.log"
LOG_ERR="/tmp/stock-server-err.log"
PID_FILE="/tmp/stock-server.pid"

# load_env 加载 server/.env 中的本地配置（文件不存在则沿用代码内置默认值）
load_env() {
  local env_file="$ROOT/server/.env"
  [ -f "$env_file" ] || return 0
  set -a
  # shellcheck disable=SC1090
  source "$env_file"
  set +a
  echo "[start] 已加载 $env_file"
}

# build_server 编译最新 Go 代码，产出 server/server-bin
build_server() {
  echo "[start] 编译最新代码..."
  cd "$ROOT/server"
  go build -o bin/server ./cmd/server/
  cp bin/server server-bin
  cd "$ROOT"
  echo "[start] 编译完成 → server/server-bin"
}

# stop_old 终止占用 $PORT 的旧进程，最多等待 10 秒，超时则强制终止
stop_old() {
  local pids
  pids="$(lsof -ti :"$PORT" 2>/dev/null || true)"
  if [ -z "$pids" ]; then
    echo "[start] 端口 $PORT 空闲，无需停止旧进程"
    return 0
  fi
  echo "[start] 停止旧进程: $(echo "$pids" | tr '\n' ' ')"
  kill $pids 2>/dev/null || true
  for _ in $(seq 1 20); do
    lsof -ti :"$PORT" >/dev/null 2>&1 || return 0
    sleep 0.5
  done
  echo "[start] 旧进程未响应，强制终止"
  kill -9 $(lsof -ti :"$PORT") 2>/dev/null || true
  sleep 1
}

# start_server 后台启动新服务并记录 PID
start_server() {
  echo "[start] 启动新服务 (端口 $PORT)..."
  nohup "$ROOT/server/server-bin" >"$LOG_OUT" 2>"$LOG_ERR" &
  echo "$!" >"$PID_FILE"
}

# verify 校验端口是否已进入监听状态
verify() {
  sleep 2
  if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "[start] 启动成功 PID=$(cat "$PID_FILE") 端口=$PORT"
    echo "[start] 日志: $LOG_OUT | $LOG_ERR"
    return 0
  fi
  echo "[start] 启动失败，请查看日志: $LOG_ERR" >&2
  return 1
}

load_env
build_server
stop_old
start_server
verify
