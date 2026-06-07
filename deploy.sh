#!/bin/bash
# 智策投研 — 一键部署脚本
# 适用于 Ubuntu + Docker + 宝塔面板 环境
set -euo pipefail

APP_ROOT="/opt/ai-stock-predict"
BRANCH="${1:-main}"

echo "============================================"
echo "  智策投研 · 一键部署"
echo "  时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "============================================"
echo ""

# ═══ 1. 检查环境 ═══
echo "【1/6】检查环境..."

if ! command -v docker &>/dev/null; then
  echo "❌ 未安装 Docker，请先安装: curl -fsSL https://get.docker.com | bash"
  exit 1
fi

if ! docker compose version &>/dev/null 2>&1; then
  echo "❌ 需要 Docker Compose V2"
  exit 1
fi

if ! command -v git &>/dev/null; then
  echo "安装 git..."
  apt-get update -qq && apt-get install -y -qq git
fi

echo "  ✓ Docker $(docker --version | cut -d' ' -f3 | cut -d',' -f1)"
echo "  ✓ Docker Compose $(docker compose version --short 2>/dev/null || echo 'v2')"

# ═══ 2. 拉取代码 ═══
echo ""
echo "【2/6】拉取代码 (分支: $BRANCH)..."

if [ -d "$APP_ROOT/.git" ]; then
  cd "$APP_ROOT"
  git fetch origin
  git reset --hard "origin/$BRANCH"
  echo "  ✓ 已更新到最新版本"
else
  mkdir -p "$APP_ROOT"
  git clone --branch "$BRANCH" --depth 1 https://github.com/your-org/ai-stock-predict.git "$APP_ROOT" 2>/dev/null || {
    echo "  ⚠️  Git clone 失败，使用当前目录"
    mkdir -p "$APP_ROOT"
  }
  echo "  ✓ 代码已克隆"
fi

cd "$APP_ROOT"

# ═══ 3. 构建前端 ═══
echo ""
echo "【3/6】构建前端..."

if command -v node &>/dev/null && command -v npm &>/dev/null; then
  cd "$APP_ROOT/web-pc"
  npm install --silent 2>/dev/null || echo "  ⚠️  npm install 有警告"
  npm run build 2>/dev/null || echo "  ⚠️  npm build 有警告"
  echo "  ✓ 前端构建完成 → web-pc/dist/"
  cd "$APP_ROOT"
else
  echo "  ⚠️  未安装 Node.js，跳过前端构建"
  echo "  请确保 web-pc/dist/ 目录已存在"
fi

# ═══ 4. 构建 & 启动 Docker 服务 ═══
echo ""
echo "【4/6】启动 Docker 服务..."

cd "$APP_ROOT/docker"
docker compose build --no-cache --pull && docker compose up -d 2>&1 | tail -3
echo "  ✓ Docker 服务已启动"

# 等待服务就绪
echo "  等待服务就绪..."
sleep 5
for i in $(seq 1 30); do
  if curl -s http://127.0.0.1:8080/api/v1/indices > /dev/null 2>&1; then
    echo "  ✓ 服务已就绪"
    break
  fi
  sleep 2
done

# ═══ 5. 恢复种子数据（可选） ═══
echo ""
echo "【5/6】检查种子数据..."

if [ -f "$APP_ROOT/seed/data/pg_dump_"*.sql ] 2>/dev/null; then
  echo "  发现种子数据文件"
  read -r -p "  是否恢复种子数据? [y/N]: " REPLY
  if [ "${REPLY,,}" = "y" ]; then
    # 更新 restore.sh 中的容器名
    cd "$APP_ROOT"
    PG_CONTAINER="docker-postgres-1" \
    MYSQL_CONTAINER="docker-mysql-1" \
    bash seed/restore.sh
  fi
else
  echo "  ⊘ 无种子数据文件，跳过（服务端会自动建空表）"
fi

# ═══ 6. 部署前端 ═══
echo ""
echo "【6/6】部署前端静态文件..."

WEBROOT="/www/wwwroot/ai-stock-predict"
if [ -d "/www/wwwroot" ]; then
  mkdir -p "$WEBROOT"
  if [ -d "$APP_ROOT/web-pc/dist" ]; then
    cp -r "$APP_ROOT/web-pc/dist/"* "$WEBROOT/"
    echo "  ✓ 前端已部署到 $WEBROOT"
  fi

  # 宝塔 Nginx 配置提示
  if [ -d "/www/server/nginx" ]; then
    echo ""
    echo "  ┌──────────────────────────────────────────┐"
    echo "  │  宝塔面板 Nginx 配置                       │"
    echo "  │  1. 网站 → 添加站点 → 填写域名              │"
    echo "  │  2. 设置 → 配置文件 → 参考:                │"
    echo "  │     docker/baota-nginx.conf              │"
    echo "  │  3. 网站目录: $WEBROOT                   │"
    echo "  │  4. 反向代理: http://127.0.0.1:8080       │"
    echo "  └──────────────────────────────────────────┘"
  fi
else
  echo "  ⊘ 未检测到宝塔面板，跳过前端部署"
  echo "  请手动将 web-pc/dist/ 部署到 Nginx 静态目录"
fi

echo ""
echo "============================================"
echo "  部署完成！"
echo "  API:    http://127.0.0.1:8080"
echo "  默认账号: admin / admin123"
echo "============================================"
echo ""
echo "查看日志: docker logs aip-server -f"
echo "查看状态: docker compose -f $APP_ROOT/docker/docker-compose.yml ps"
