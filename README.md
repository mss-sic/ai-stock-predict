# 智策投研 · AI Stock Predict

基于多数据源采集、算法选股、AI 决策的股票预测分析平台。

## 项目架构

```
ai-stock-predict/
├── server/                  # Go API 服务端
│   ├── cmd/server/          # 入口 (Gin 路由)
│   ├── internal/
│   │   ├── collector/       # 数据采集 (通达信/腾讯 A 股行情 + Excel 解析)
│   │   ├── config/          # 环境配置
│   │   ├── db/              # PostgreSQL + MySQL 连接 & 迁移
│   │   ├── handler/         # HTTP 处理器
│   │   ├── model/           # 数据模型 (GORM)
│   │   ├── repository/      # 数据访问层
│   │   ├── scheduler/       # 定时任务 (cron)
│   │   └── service/         # 业务逻辑层
│   └── go.mod
├── web-pc/                  # PC 前端 (React 19 + Arco Design)
│   └── src/
│       ├── pages/           # 11 个功能页面
│       ├── services/        # API 调用层
│       └── router.tsx       # Hash 路由
├── scripts/collector/       # Python 数据采集脚本
│   ├── stock_basic.py       # A 股标的基本信息采集
│   ├── daily_k.py           # 日 K 线采集
│   └── batch_collect.py     # 批量采集调度
├── docker/                  # Docker Compose 编排
│   ├── docker-compose.yml   # PostgreSQL + MySQL
│   ├── postgres/init.sql    # PG 初始化脚本
│   └── mysql/init.sql       # MySQL 初始化脚本
└── docs/                    # 文档
```

## 技术栈

| 层级 | 技术 |
|------|------|
| **PC 前端** | React 19, TypeScript 6, Arco Design 2, ECharts 6, Vite 8 |
| **API 服务** | Go 1.25, Gin, GORM, Excelize |
| **股票数据库** | PostgreSQL 16 (TimescaleDB 时序扩展) |
| **业务数据库** | MySQL 8.0 |
| **采集引擎** | Python (mootdx 通达信、腾讯行情 API) |
| **容器化** | Docker Compose |

## 功能模块

| 页面 | 路由 | 功能 |
|------|------|------|
| 今日榜单 | `/#/board` | 当日算法精选 50 只个股 |
| 历史榜单 | `/#/history` | 按日期回溯历史选股 |
| **上榜热力图** | `/#/heatmap` | 20 日日历/矩阵热力图，红涨绿跌着色 |
| 个股详情 | `/#/stock/:code` | K 线图、技术指标 (MACD/KDJ/RSI) |
| 交易策略 | `/#/strategy` | 止盈止损策略、回测 |
| 持仓跟踪 | `/#/holdings` | 模拟持仓管理 |
| 风险预警 | `/#/risk` | 风险信号监控 |
| AI 分析 | `/#/ai/:code` | 智能分析对话 |
| 行情预测 | `/#/forecast/:code` | 走势预测 |
| 自选股 | `/#/watchlist` | 自选列表 |
| **数据管理** | `/#/data` | Excel 导入、采集触发、导入历史 |

## 数据库说明

### PostgreSQL（股票核心数据）

| 表 | 说明 | 关键字段 |
|----|------|---------|
| `stocks_basic` | 标的基本信息 | code, name, industry, concept_tags |
| `stocks_daily_k` | 日 K 线 | code, trade_date, open, close, high, low, volume |
| `stocks_daily_indicator` | 每日指标 | pe, pb, ps, total_market_cap |
| `algorithm_picks` | 选股批次 | pick_date, total_stocks |
| `algorithm_pick_details` | 选股明细 | pick_date, stock_code, rank, score |
| `stock_signals` | 算法信号值 | code, signal_value |
| `backtest_results` | 回测结果 | strategy_id, metrics (JSON) |

### MySQL（业务数据）

| 表 | 说明 |
|----|------|
| `users` | 用户 |
| `watchlists` | 自选股 |
| `strategies` | 交易策略 |
| `holdings` | 持仓记录 |
| `risk_alerts` | 风险预警 |
| `import_logs` | 导入历史 |

## 快速开始

### 1. 环境要求

- Go ≥ 1.25
- Node.js ≥ 20
- Python ≥ 3.9
- Docker & Docker Compose
- macOS / Linux

### 2. 启动数据库

```bash
cd docker
docker compose up -d
# PostgreSQL: localhost:5432 (stock/stock123@stock_predict)
# MySQL:      localhost:3307 (stock/stock123@stock_predict)
```

### 3. 启动 API 服务

```bash
cd server
go build -o server ./cmd/server/
./server
# 默认监听 :8080

# 环境变量（可选）
# PORT=8080
# POSTGRES_DSN="host=localhost user=stock password=stock123 dbname=stock_predict port=5432 sslmode=disable TimeZone=Asia/Shanghai"
# MYSQL_DSN="stock:stock123@tcp(127.0.0.1:3307)/stock_predict?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
# CRON_EXPR="30 15 * * 1-5"
```

### 4. 启动前端

```bash
cd web-pc
npm install
npm run dev
# 默认 http://localhost:5173
```

### 5. 初始化数据

#### 方式一：自动采集（推荐）

```bash
# 安装 Python 依赖
cd scripts/collector
pip install -r requirements.txt

# 采集标的基本信息（约 5000+ 只 A 股）
python stock_basic.py

# 采集日 K 线数据
python daily_k.py
```

#### 方式二：Excel 导入

打开 `http://localhost:5173/#/data`，上传算法选股 Excel 文件（参考 `MSS20260603.xlsm` 格式）：
- **Sheet1**：`[股票代码, 信号值]`（5510 行信号数据）
- **Sheet2**：日期标题行 + 每日 50 只上榜股票（代码+名称）

### 6. 触发采集任务

```bash
# 手动触发
curl -X POST http://localhost:8080/api/v1/collector/trigger

# 查看状态
curl http://localhost:8080/api/v1/collector/status
```

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/stocks` | 股票列表（分页、行业筛选） |
| GET | `/api/v1/stocks/:code` | 个股详情 |
| GET | `/api/v1/stocks/:code/kline` | K 线数据 |
| GET | `/api/v1/stocks/:code/indicator` | 技术指标 |
| GET | `/api/v1/stocks/:code/signal` | 算法信号值 |
| GET | `/api/v1/board/today` | 今日榜单 |
| GET | `/api/v1/board/history?date=2026-06-03` | 历史榜单 |
| GET | `/api/v1/board/heatmap` | 热力图数据（基础） |
| GET | `/api/v1/board/heatmap-enriched` | 热力图数据（含名称和涨跌幅） |
| POST | `/api/v1/import/excel` | 上传 Excel |
| GET | `/api/v1/import/history` | 导入历史 |
| POST | `/api/v1/collector/trigger` | 触发采集 |
| GET | `/api/v1/collector/status` | 采集状态 |
| PUT | `/api/v1/collector/schedule` | 更新定时 |
| GET | `/api/v1/forecast/:code` | 行情预测 |
| POST | `/api/v1/ai/analyze` | AI 分析 |

## 数据采集架构

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│  Python 脚本  │ ──▶ │  PostgreSQL  │ ◀── │  Go API    │
│  (mootdx/腾讯) │     │  (时序数据)   │     │  (GORM)    │
└─────────────┘     └──────────────┘     └────────────┘
       │                                       │
       │ 通达信标准行情                          │ Excel 导入
       │ 腾讯免费 K 线 API                       │ (算法选股+信号)
       │                                       │
       └───────────────────────────────────────┘
```

- **通达信 (mootdx)**：标的基本信息、财务数据、行业分类
- **腾讯行情**：日 K 线数据（免费、不限频）
- **定时采集**：每个交易日 15:30 自动执行
- **手工导入**：算法团队产出的 Excel 榜单

## 数据安全 & 稳定性

- PostgreSQL 使用 **TimescaleDB** 时序扩展，优化时间序列查询
- 数据采集内置**限流机制**，避免被源站封 IP
- 腾讯 K 线 API 不限频且无需鉴权，适合大规模批量采集
- 通达信 mootdx 为标准行情协议，稳定可靠
- Docker Compose 支持 `healthcheck`，数据库异常自动重启
- Go 服务内置 **graceful shutdown**，SIGTERM 时安全退出
- API 服务与数据库分离部署，各司其职

## 部署到生产

### 构建前端

```bash
cd web-pc
npm run build        # 输出到 dist/
# 部署 dist/ 到 Nginx / CDN
```

### 构建后端

```bash
cd server
CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server/
# 二进制 ~15MB，可直接部署
```

### macOS LaunchAgent（开发环境）

```bash
# 将编译好的 server 二进制放入 /tmp/stock-server
# launchctl 配置自动重启
launchctl load ~/Library/LaunchAgents/com.stock.server.plist
```

## 路线图

- [x] 数据采集引擎（通达信 + 腾讯 K 线）
- [x] Excel 榜单导入
- [x] 上榜热力图（红涨绿跌着色）
- [x] K 线图 + 技术指标
- [x] 数据管理（导入历史、采集调度）
- [ ] 移动端 taro-h5 适配
- [ ] 行业轮动分析
- [ ] 持股策略与卖出信号
- [ ] 技术回测引擎
- [ ] 研报/资讯集成
- [ ] AI 投资建议增强
