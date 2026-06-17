# AGENTS.md — 智策投研项目开发规范

## 1. 本地服务端启动规则

**固定启动流程，禁止使用其他方式启动服务端，防止端口冲突：**

```bash
# 步骤 1: 编译 Go 服务端
cd server && go build -o bin/server ./cmd/server/ && cp bin/server server-bin

# 步骤 2: 停止旧进程 + 重启
kill $(lsof -ti :8080) 2>/dev/null; sleep 1
launchctl start com.stock.server

# 步骤 3: 验证
sleep 2 && lsof -ti :8080
```

- **永远不要**直接运行 `go run` 或 `./server` 启动服务端
- **永远不要**使用其他端口（8080 是唯一固定端口）
- 前端使用 Vite 开发服务器 `npm run dev`（端口 5173），HMR 自动热更新，修改前端代码无需重启
- 修改 Python 采集脚本无需重启服务端

## 2. 线上版本同步规则

修改数据库表结构时必须：
- 在 `server/internal/db/migrations_data.go` 中新增迁移版本
- 同时生成独立的线上修复 SQL 脚本，记录在 `docs/sql-fixes/` 目录下
- 文件名格式: `YYYY-MM-DD_fix_description.sql`
- 迁移版本号递增，描述清晰

示例：
```go
// v017: 新增 stock_profiles 表
{
    Version:     17,
    Description: "新增 stock_profiles 表 (AI 简介 + 六维评分)",
    Up: func() error {
        return db.PG.AutoMigrate(&model.StockProfile{})
    },
},
```

## 3. 功能完成确认 & 变更日志

每次功能对话结束时：
- **主动确认**：询问用户「功能是否已完成，是否需要归档？」
- 按天在 `CHANGELOG.md` 中补充更新条目（`## vX.Y.Z (YYYY-MM-DD)` 格式）
- 条目按功能模块分组（AI 简介、股票详情、数据修复、右侧面板等）
- 用户确认后 `git commit` 提交代码
- **不要**在用户未确认的情况下自动 commit

## 4. 发布上线规则

当用户要求发布上线时：
- **前提检查**：确保所有代码已 `git commit`
- 运行 `./publish.sh` 构建 linux/amd64 镜像并推送到阿里云 Registry
- 镜像地址: `crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest`
- 推送完成后给出服务器端更新命令：

```bash
docker pull crpi-t3tis8f2l2fb8jc9.cn-hangzhou.personal.cr.aliyuncs.com/lijiangbo/ai-stock-predict:latest
cd /opt/ai-stock-predict/docker && docker compose up -d
```

---

## 项目架构速查

| 层级 | 目录 | 说明 |
|------|------|------|
| 前端 | `web-pc/` | React + Vite + Arco Design，端口 5173 |
| 后端 | `server/` | Go + Gin + GORM，端口 8080 |
| 采集 | `scripts/collector/` | Python 脚本，腾讯 API + mootdx |
| 数据库 | PostgreSQL | `stock_predict` 库，`stocks_basic` / `stocks_daily_k` / `predictions` 等 |
| 部署 | `docker/` | Docker Compose 编排 |

## 常用命令

```bash
# 前端构建
cd web-pc && npm run build

# 前端开发
cd web-pc && npm run dev

# 后端编译
cd server && go build -o bin/server ./cmd/server/

# 运行 Python 采集脚本
cd scripts/collector && python3 batch_collect.py

# 修复单只股票数据
cd scripts/collector && python3 repair_kline.py 600519

# 构建并推送镜像
./publish.sh
```
