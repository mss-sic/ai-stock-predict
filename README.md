# 智策投研 · AI Stock Predict

基于多数据源采集、算法选股、量化预测、AI 决策的股票预测分析平台。

## 项目架构

```
ai-stock-predict/
├── server/                    # Go API 服务 (Gin + GORM)
│   ├── cmd/server/main.go    # 入口
│   └── internal/
│       ├── collector/        # 数据采集引擎
│       ├── config/           # 环境配置
│       ├── db/               # PG + MySQL 连接 & 自动迁移
│       ├── handler/          # HTTP 处理器
│       ├── model/            # 数据模型
│       ├── repository/       # 数据访问层
│       ├── scheduler/        # 定时任务调度
│       └── service/          # 业务逻辑
├── web-pc/                   # PC 前端 (React 19 + Arco Design + Vite)
│   └── src/
│       ├── pages/            # 功能页面
│       ├── services/         # API 调用层
│       └── components/       # 通用组件
├── scripts/
│   ├── collector/            # Python 数据采集脚本
│   └── predict/              # 量化预测脚本
├── docker/                   # Docker Compose 编排
│   ├── docker-compose.yml    # PostgreSQL + MySQL
│   ├── postgres/init.sql     # PG 表结构
│   └── mysql/init.sql        # MySQL 表结构
├── seed/                     # 种子数据工具 (pg_dump / mysqldump)
│   ├── export.sh             # 导出当前数据库 → SQL 文件
│   └── restore.sh            # 导入 SQL 种子数据
└── docs/                     # 设计文档
```

## 技术栈

| 层级 | 技术 |
|------|------|
| **PC 前端** | React 19, TypeScript, Arco Design, ECharts, Vite |
| **API 服务** | Go 1.25, Gin, GORM, Excelize |
| **股票数据库** | PostgreSQL 16 + TimescaleDB 时序扩展 |
| **业务数据库** | MySQL 8.0 |
| **采集引擎** | Python 3 (mootdx 通达信 / 腾讯行情 / 东财研报 API) |
| **量化预测** | Python (GRU / LSTM / XGBoost / ARIMA / Transformer / Prophet) |
| **AI 服务** | OpenAI-compatible API (DeepSeek / GPT-4o / 通义千问) |
| **容器化** | Docker Compose |

## 功能模块

| 页面 | 路由 | 功能 |
|------|------|------|
| 今日榜单 | `/#/board` | 当日算法精选个股，操作建议 & 风险标签 |
| 历史榜单 | `/#/history` | 按日期回溯历史选股，验证涨跌表现 |
| 上榜热力图 | `/#/heatmap` | 20 日矩阵热力图，红涨绿跌着色 |
| 个股详情 | `/#/stock/:code` | K 线图、技术指标、量化预测、AI 六维评分 |
| 股票列表 | `/#/stocks` | 全量 A 股检索，按名称/代码/行业筛选 |
| 自选股 | `/#/watchlist` | 自选列表 + 分组管理 + 收益率跟踪 |
| 交易策略 | `/#/strategy` | 策略条件配置、指标测试、历史回测 |
| 风险管理 | `/#/risk` | 持仓风险预警、定时扫描、告警管理 |
| 数据管理 | `/#/data` | 数据概览、采集控制台、Excel 导入、采集记录 |
| 系统设置 | `/#/settings` | AI 模型配置、Key 管理、连通测试 |

## 数据库

### 双库架构

| 数据库 | 用途 | 存储内容 |
|--------|------|----------|
| **PostgreSQL** | 股票核心数据 | K 线、指标、行情、财务、股东、研报、资讯、信号、预测 |
| **MySQL** | 业务数据 | 用户、自选股、策略、回测、持仓、风险、采集日志 |

### PostgreSQL 主要表

| 表 | 说明 |
|----|------|
| `stocks_basic` | 股票基本信息 (代码/名称/行业/上市日) |
| `stocks_daily_k` | 日 K 线 (开/高/低/收/量) — TimescaleDB hypertable |
| `stocks_daily_indicator` | PE/PB/PS/市值/换手率 — TimescaleDB hypertable |
| `stock_quotes` | 实时行情快照 |
| `stock_financials` | 财务报表 (营收/利润/ROE/EPS) |
| `stock_shareholders` | 股东户数 & 十大股东 |
| `stock_news` | 个股资讯 |
| `stock_reports` | 机构研报 |
| `stock_signals` | 算法信号值 |
| `algorithm_picks` | 选股批次 |
| `algorithm_pick_details` | 榜单明细 (排名/评分) |
| `predictions` | 量化预测结果 |

### MySQL 主要表

| 表 | 说明 |
|----|------|
| `users` | 用户 & 角色 |
| `watchlists` / `watchlist_groups` | 自选股 & 分组 |
| `strategies` / `strategy_conditions` | 策略定义 & 条件 |
| `backtest_tasks` / `backtest_results` | 回测任务 & 结果 |
| `holdings` | 持仓数据 |
| `risk_alerts` | 风险预警 |
| `scheduled_tasks` / `task_logs` | 定时任务 & 执行日志 |
| `collection_logs` / `import_logs` | 采集 & 导入记录 |

---

## 快速开始（本地开发）

### 前置条件

- Go 1.24+
- Node.js 22+
- Python 3.10+
- Docker Desktop

### 1. 启动数据库

```bash
docker compose -f docker/docker-compose.yml up -d
```

### 2. 启动后端

```bash
cd server
go run ./cmd/server/
# 服务运行在 http://localhost:8080
# 首次启动自动建表并创建 admin 用户 (admin / admin123)
```

### 3. 启动前端

```bash
cd web-pc
npm install
npm run dev
# 开发服务器运行在 http://localhost:5173
```

### 4. 初始化种子数据（可选）

如果数据库已有数据，可导出为 SQL 文件供其他环境使用：

```bash
# 导出当前数据
bash seed/export.sh

# 在新环境恢复
bash seed/restore.sh
```

### 5. 配置 AI 模型（可选）

打开 `http://localhost:5173/#/settings`，填写 AI API Key。

---

## 服务器部署

### 环境要求

- Ubuntu 20.04+ / Debian 11+
- Docker + Docker Compose V2
- （可选）宝塔面板管理 Nginx

### 一键部署

```bash
# 克隆项目
git clone https://github.com/your-org/ai-stock-predict.git /opt/ai-stock-predict
cd /opt/ai-stock-predict

# 执行部署
bash deploy.sh
```

脚本自动完成：拉取代码 → 构建前端 → 构建 Docker 镜像 → 启动全部服务 → 部署前端到宝塔目录。

### 手动部署步骤

```bash
# 1. 启动全部服务（PostgreSQL + MySQL + Go 服务端）
cd /opt/ai-stock-predict/docker
docker compose up -d --build

# 2. 构建前端
cd /opt/ai-stock-predict/web-pc
npm install && npm run build

# 3. 恢复种子数据（可选）
cd /opt/ai-stock-predict
bash seed/restore.sh
```

### Docker 服务说明

| 服务 | 端口 | 说明 |
|------|------|------|
| `postgres` | 5432 | PostgreSQL 16 + TimescaleDB，存储股票核心数据 |
| `mysql` | 3307→3306 | MySQL 8.0，存储业务数据 |
| `server` | 8080 | Go API 服务，内置 Python3 采集引擎 |

### 宝塔面板配置

1. 宝塔 → 网站 → 添加站点 → 填写域名
2. 网站目录设置为 `/www/wwwroot/ai-stock-predict`（`web-pc/dist/` 的内容）
3. 配置文件参考 `docker/baota-nginx.conf`
4. 反向代理 `/api/` → `http://127.0.0.1:8080`

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 服务端口 |
| `POSTGRES_DSN` | `host=postgres ...` | PG 连接串（Docker 内用服务名） |
| `MYSQL_DSN` | `stock:stock123@tcp(mysql:3306)/...` | MySQL 连接串 |
| `APP_ROOT` | `/app` | 项目根目录（Docker 内固定） |

### 常用命令

```bash
# 查看服务状态
docker compose -f docker/docker-compose.yml ps

# 查看服务日志
docker logs aip-server -f

# 重启服务
docker compose -f docker/docker-compose.yml restart server

# 停止所有服务
docker compose -f docker/docker-compose.yml down

# 重新构建并启动
docker compose -f docker/docker-compose.yml up -d --build

# 导出/恢复种子数据
bash seed/export.sh    # 导出当前数据库
bash seed/restore.sh   # 恢复到当前数据库
```
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 服务端口 |
| `POSTGRES_DSN` | `host=localhost user=stock password=stock123 dbname=stock_predict port=5432 sslmode=disable TimeZone=Asia/Shanghai` | PG 连接串 |
| `MYSQL_DSN` | `stock:stock123@tcp(127.0.0.1:3307)/stock_predict?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai` | MySQL 连接串 |
| `CRON_EXPR` | `30 15 * * 1-5` | 默认定时采集 cron 表达式 |

---

## 数据采集

### 方式一：采集控制台（推荐）

在数据管理 → 采集控制台，选择需要采集的数据类型，点击"采集"按钮。采集过程实时在控制台展示，支持 SSE 流式输出。

支持的采集类型：

| 类型 | 说明 | 数据源 |
|------|------|--------|
| 股票列表同步 | 全市场股票代码、名称、行业 | 通达信 mootdx |
| 日K线数据 | 每日开/高/低/收、成交量、成交额 | 腾讯免费 API |
| PE/PB指标 | 市盈率、市净率、市值、换手率 | 通达信 mootdx |
| 行业分类 | 申万行业分类 | 通达信 mootdx |
| 实时行情 | 盘中实时行情快照 | 新浪行情 |
| 股东数据 | 股东人数、十大股东、机构持股 | 东财 API |
| 财务数据 | 营收、利润、ROE、EPS、毛利率 | 东财 API |
| 资讯数据 | 个股新闻、公告 | 东财 API |
| 研报数据 | 券商研报、评级、目标价 | 东财 API |

### 方式二：定时任务

在数据管理 → 定时任务，可配置各数据类型的定时采集周期。支持：
- 启用/暂停开关
- 手动触发执行
- 查看执行日志
- 默认任务初始化

### 方式三：Excel 导入榜单

在数据管理 → 导入 Excel，上传算法选股 Excel（参考 `MSS20260603.xlsm`）：
- Sheet1: 股票代码 + 信号值
- Sheet2/Sheet3: 上榜日期 + 排名 + 评分

---

## 种子数据

种子数据用于部署时快速恢复数据库到可用状态。

### 导出（从已有数据库）

```bash
bash seed/export.sh
# 生成 seed/pg_dump.sql 和 seed/mysql_dump.sql
```

### 恢复（到新数据库）

```bash
bash seed/restore.sh
# 自动检测并导入 SQL 文件
# 支持通过 PG_DSN / MYSQL_DSN 环境变量指定连接
```

### 定时导出（crontab）

```bash
# 每天凌晨 3 点自动备份
0 3 * * * cd /opt/ai-stock-predict && bash seed/export.sh
```

---

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/auth/login` | 登录 |
| `POST` | `/api/v1/auth/refresh` | 刷新 Token |
| `GET` | `/api/v1/indices` | 大盘指数 |
| `GET` | `/api/v1/stocks` | 股票列表 |
| `GET` | `/api/v1/stocks/:code` | 个股详情 |
| `GET` | `/api/v1/stocks/:code/kline` | K 线数据 |
| `GET` | `/api/v1/stocks/:code/indicator` | 技术指标 |
| `GET` | `/api/v1/board/today` | 今日榜单 |
| `GET` | `/api/v1/board/history` | 历史榜单 |
| `GET` | `/api/v1/board/heatmap` | 热力图 |
| `POST` | `/api/v1/import/excel` | 上传 Excel |
| `GET` | `/api/v1/import/history` | 导入历史 |
| `POST` | `/api/v1/collector/trigger` | 触发采集 |
| `GET` | `/api/v1/collector/status` | 采集进度 |
| `GET` | `/api/v1/collector/stream` | SSE 采集流 |
| `GET` | `/api/v1/collector/history` | 采集历史 |
| `DELETE` | `/api/v1/collector/history/clear` | 清除采集记录 |
| `POST` | `/api/v1/prediction/:code` | 单股预测 |
| `POST` | `/api/v1/prediction/batch` | 批量预测 |
| `POST` | `/api/v1/ai/analyze` | AI 分析 |
| `GET` | `/api/v1/ai/score/:code` | AI 评分 |
| `GET` | `/api/v1/settings/ai` | AI 配置 |
| `PUT` | `/api/v1/settings/ai` | 更新 AI 配置 |
| `GET` | `/api/v1/strategies` | 策略列表 |
| `POST` | `/api/v1/strategies/:id/backtest` | 执行回测 |
| `GET` | `/api/v1/risks` | 风险预警列表 |
| `POST` | `/api/v1/admin/risks/scan` | 触发风险扫描 |
| `GET` | `/api/v1/admin/scheduled-tasks` | 定时任务列表 |
| `POST` | `/api/v1/admin/scheduled-tasks/:id/run` | 执行定时任务 |
| `GET` | `/api/v1/admin/task-logs` | 任务日志 |

---

## 数据安全 & 稳定性

- PostgreSQL 使用 **TimescaleDB** 时序扩展，优化 K 线时间序列查询
- 数据采集内置**限流机制**，避免被源站封 IP
- 腾讯 K 线 API 免费不限频，适合批量采集
- 通达信 mootdx 为标准行情协议，稳定可靠
- Docker Compose 支持 `healthcheck`，数据库异常自动重启
- Go 服务内置 **graceful shutdown**，SIGTERM 时安全退出
- SSE 断线自动重连，采集页面刷新后可恢复连接

## 开发计划

- [x] 数据采集（9 类：股票/K线/PE/行业/行情/股东/财务/资讯/研报）
- [x] Excel 榜单导入 + 信号数据
- [x] 今日 + 历史榜单（次日&至今表现）
- [x] 上榜热力图（20 日矩阵）
- [x] K 线图 + 技术指标 (MACD/KDJ/RSI/BOLL)
- [x] 量化预测 (6 模型)
- [x] AI 分析 + 六维评分
- [x] 机构研报采集 & 展示
- [x] 财务 + 股东数据
- [x] 实时大盘指数
- [x] 自选股管理 + 分组
- [x] 交易策略 + 条件配置 + 回测引擎
- [x] 风险管理 + 预警扫描
- [x] 定时任务调度 + 日志
- [x] 采集控制台 + 实时 SSE 流
- [ ] 移动端适配
- [ ] 行业轮动分析
- [ ] 自动交易信号
