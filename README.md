# 智策投研 · AI Stock Predict

基于多数据源采集、算法选股、量化预测、AI 决策的股票预测分析平台。

## 项目架构

```
ai-stock-predict/
├── server/                  # Go API 服务端
│   ├── cmd/server/          # 入口 (Gin 路由)
│   ├── internal/
│   │   ├── collector/       # 数据采集引擎 (Python 脚本调度 + SSE 流)
│   │   ├── config/          # 环境配置
│   │   ├── db/              # PostgreSQL + MySQL 连接 & 自动迁移
│   │   ├── handler/         # HTTP 处理器 (stocks/board/ai/prediction/settings...)
│   │   ├── model/           # 数据模型 (GORM)
│   │   ├── repository/      # 数据访问层
│   │   ├── scheduler/       # 定时任务 (cron)
│   │   └── service/         # 业务逻辑层
│   └── go.mod
├── web-pc/                  # PC 前端 (React 19 + Arco Design)
│   └── src/
│       ├── pages/           # 功能页面
│       ├── services/        # API 调用层 (api.ts)
│       ├── components/      # 通用组件 (KLineChart 等)
│       └── router.tsx       # Hash 路由
├── scripts/collector/       # Python 数据采集脚本
│   ├── full_sync.py         # A 股全量同步 (通达信+新浪)
│   ├── daily_k.py           # 日 K 线采集 (腾讯 API)
│   ├── daily_indicator.py   # PE/PB 指标采集
│   ├── report_collect.py    # 机构研报采集 (东财)
│   ├── shareholder_collect.py # 股东数据采集
│   ├── financial_collect.py  # 财务数据采集
│   ├── news_collect.py      # 资讯数据采集
│   └── quotes_sync.py       # 实时行情同步
├── scripts/predict/         # Python 量化预测脚本
├── seed/                    # 种子数据 (导出/导入)
│   ├── export.py            # 导出当前数据库为 CSV
│   ├── init.py              # 部署时初始化种子数据
│   └── data/                # CSV 数据文件
├── docker/                  # Docker Compose 编排
│   ├── docker-compose.yml   # PostgreSQL (TimescaleDB) + MySQL
│   ├── postgres/init.sql    # PG 初始化脚本
│   └── mysql/init.sql       # MySQL 初始化脚本
└── docs/                    # 文档
```

## 技术栈

| 层级 | 技术 |
|------|------|
| **PC 前端** | React 19, TypeScript, Arco Design, ECharts, Vite |
| **API 服务** | Go 1.25, Gin, GORM, Excelize |
| **股票数据库** | PostgreSQL 16 (TimescaleDB 时序扩展) |
| **业务数据库** | MySQL 8.0 |
| **采集引擎** | Python 3 (mootdx 通达信、腾讯行情 API、东财研报 API) |
| **量化预测** | Python (GRU/LSTM/XGBoost/ARIMA/Transformer/Prophet) |
| **AI 服务** | OpenAI-compatible API (DeepSeek/GPT-4o/通义千问) |
| **容器化** | Docker Compose |

## 功能模块

| 页面 | 路由 | 功能 |
|------|------|------|
| 今日榜单 | `/#/board` | 当日算法精选个股，操作建议 & 风险标签 |
| 历史榜单 | `/#/history` | 按日期回溯历史选股，次日 & 至今涨跌幅 |
| 上榜热力图 | `/#/heatmap` | 20 日矩阵热力图，红涨绿跌着色 |
| 个股详情 | `/#/stock/:code` | K 线图、量化预测叠加、AI 六维评分、财务/股东/研报 |
| 股票列表 | `/#/stocks` | 全量 A 股检索，按名称/代码/行业筛选 |
| 自选股 | `/#/watchlist` | 自选列表 + 收益率跟踪 |
| AI 分析 | `/#/stock/:code` (AI Tab) | AI 对话、六维评分雷达图、操作建议 |
| 交易策略 | `/#/stock/:code` (策略 Tab) | 智能操作建议、压力位/支撑位 |
| 系统设置 | `/#/settings` | AI 模型配置、Key 管理、连通测试 |
| 数据管理 | `/#/data` | Excel 导入、9 类数据采集、实时控制台、采集 & 导入历史 |

## 数据库说明

### PostgreSQL（股票核心数据）

| 表 | 说明 | 数据量 |
|----|------|--------|
| `stocks_basic` | 标的基本信息 (代码/名称/行业) | ~6,000 |
| `stocks_daily_k` | 日 K 线 (开/高/低/收/量) | ~470,000 |
| `stocks_daily_indicator` | PE/PB/PS 等日指标 | ~9,500 |
| `stock_quotes` | 实时行情快照 | ~4,800 |
| `stock_financials` | 财务报表 (营收/利润/ROE) | ~140 |
| `stock_shareholders` | 股东户数 + 十大股东 | ~536 |
| `stock_news` | 个股资讯 | ~180 |
| `stock_reports` | 机构研报 | ~5,200 |
| `stock_signals` | 算法信号值 | ~5,500 |
| `algorithm_picks` | 选股批次 | ~20 |
| `algorithm_pick_details` | 榜单明细 (排名/评分) | ~1,000 |
| `predictions` | 量化模型预测结果 | ~360 |
| `ai_analyses` | AI 分析记录 | ~8 |
| `ai_stock_scores` | AI 六维评分 | ~8 |
| `ai_conversations` | AI 对话历史 | ~6 |

### MySQL（业务数据）

| 表 | 说明 |
|----|------|
| `ai_configs` | AI 模型配置 (provider/key/model) |
| `watchlists` | 自选股 |
| `strategies` | 交易策略 |
| `holdings` | 持仓记录 |
| `risk_alerts` | 风险预警 |
| `collection_logs` | 采集历史记录 |
| `import_logs` | 导入历史记录 |

## 快速开始

### 1. 环境要求

- Go ≥ 1.25
- Node.js ≥ 20
- Python ≥ 3.9 + pip
- Docker & Docker Compose
- macOS / Linux

### 2. 启动数据库

```bash
cd docker
docker compose up -d
# PostgreSQL: localhost:5432 (stock/stock123@stock_predict)
# MySQL:      localhost:3307 (stock/stock123@stock_predict)
```

### 3. 安装 Python 依赖

```bash
cd scripts/collector
pip install -r requirements.txt
```

### 4. 启动 API 服务

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

### 5. 启动前端

```bash
cd web-pc
npm install
npm run dev
# 默认 http://localhost:5173
```

### 6. 初始化数据

#### 方式一：种子数据导入（推荐，一次导入全量预采集数据）

```bash
# 导入预采集的种子数据（含 6000+ 股票、47 万条 K 线、5200 篇研报等）
python3 seed/init.py
```

种子数据文件位于 `seed/data/`，包含 15 张表共 ~50 万行预采集数据。如需重新导出：

```bash
python3 seed/export.py    # 导出当前数据库 → seed/data/
```

#### 方式二：手动采集

打开 `http://localhost:5173/#/data`，在「采集数据」页面选择需要采集的数据类型，支持：
- 股票列表同步、日K线数据、PE/PB指标
- 行业分类、实时行情、股东数据
- 财务数据、资讯数据、研报数据

采集过程实时在控制台展示进度，支持 SSE 流式输出。

#### 方式三：Excel 导入榜单

在数据管理页面上传算法选股 Excel（参考 `MSS20260603.xlsm`）：
- Sheet1: 股票代码 + 信号值
- Sheet2/Sheet3: 上榜日期 + 排名 + 评分

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/stocks` | 股票列表（分页、行业/代码/名称筛选） |
| GET | `/api/v1/stocks/:code` | 个股详情 |
| GET | `/api/v1/stocks/:code/kline` | K 线数据 |
| GET | `/api/v1/stocks/:code/indicator` | 技术指标 |
| GET | `/api/v1/stocks/:code/financials` | 财务数据 |
| GET | `/api/v1/stocks/:code/shareholders` | 股东数据 |
| GET | `/api/v1/stocks/:code/news` | 资讯数据 |
| GET | `/api/v1/stocks/:code/reports` | 研报数据 |
| GET | `/api/v1/indices` | 大盘指数 (上证/深证/创业板) |
| GET | `/api/v1/board/today` | 今日榜单 |
| GET | `/api/v1/board/history?date=2026-06-03` | 历史榜单 |
| GET | `/api/v1/board/dates` | 可用榜单日期 |
| GET | `/api/v1/board/heatmap` | 热力图数据 |
| POST | `/api/v1/import/excel` | 上传 Excel 榜单 |
| GET | `/api/v1/import/history` | 导入历史 |
| POST | `/api/v1/collector/trigger` | 触发采集 (支持 phases 参数) |
| GET | `/api/v1/collector/status` | 采集进度 |
| GET | `/api/v1/collector/stream` | SSE 采集流 |
| GET | `/api/v1/collector/history` | 采集历史 |
| POST | `/api/v1/collector/stock/:code` | 单股采集 |
| GET | `/api/v1/collector/reports/:code` | 单股研报采集 (SSE) |
| POST | `/api/v1/prediction/:code` | 单股量化预测 |
| POST | `/api/v1/prediction/batch` | 批量预测 |
| GET | `/api/v1/prediction/:code` | 预测结果 |
| POST | `/api/v1/ai/analyze` | AI 分析 |
| POST | `/api/v1/ai/analyze/stream` | AI 流式分析 |
| GET | `/api/v1/ai/score/:code` | AI 六维评分 |
| GET | `/api/v1/settings/ai` | AI 配置 |
| PUT | `/api/v1/settings/ai` | 更新 AI 配置 |
| POST | `/api/v1/settings/ai/test` | 测试 AI 连通 |
| POST | `/api/v1/settings/ai/models` | 拉取可用模型列表 |

## 数据采集架构

```
┌─────────────────┐     ┌──────────────┐     ┌────────────┐
│  Python 脚本      │ ──▶ │  PostgreSQL  │ ◀── │  Go API    │
│  (mootdx/腾讯/东财) │     │  (TimescaleDB)│     │  (GORM)    │
└─────────────────┘     └──────────────┘     └────────────┘
       │                                            │
       │ 通达信标准行情                                │ Excel 导入
       │ 腾讯免费 K 线 API                             │ SSE 流式采集
       │ 东财研报/财务/股东 API                         │
       │ 新浪实时指数                                  │
       └────────────────────────────────────────────┘
```

## 部署

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
```

### 部署初始化流程

```bash
# 1. 启动数据库
docker compose -f docker/docker-compose.yml up -d

# 2. 启动服务端 (自动建表)
./server &

# 3. 导入种子数据
python3 seed/init.py

# 4. 配置 AI 模型 (可选)
# 打开 http://localhost:5173/#/settings 填写 API Key

# 5. 导入算法榜单 (可选)
# 打开 http://localhost:5173/#/data 上传 Excel
```

## 数据安全 & 稳定性

- PostgreSQL 使用 **TimescaleDB** 时序扩展，优化 K 线时间序列查询
- 数据采集内置**限流机制**，避免被源站封 IP
- 腾讯 K 线 API 免费不限频，适合批量采集
- 通达信 mootdx 为标准行情协议，稳定可靠
- Docker Compose 支持 `healthcheck`，数据库异常自动重启
- Go 服务内置 **graceful shutdown**，SIGTERM 时安全退出
- SSE 断线自动重连，采集页面刷新后可恢复连接

## 路线图

- [x] 数据采集（9 类：股票列表/K线/PE/行业/行情/股东/财务/资讯/研报）
- [x] Excel 榜单导入 + 信号数据
- [x] 今日榜单 + 历史榜单（次日&至今表现）
- [x] 上榜热力图（20 日红绿着色）
- [x] K 线图 + 技术指标 (MACD/KDJ/RSI/BOLL)
- [x] 量化预测 (6 模型：GRU/LSTM/XGBoost/ARIMA/Transformer/Prophet)
- [x] AI 分析 + 六维评分 + 风险提示
- [x] 机构研报采集 & 展示
- [x] 财务数据 + 股东数据
- [x] 实时大盘指数
- [x] 自选股管理
- [x] 种子数据导出/导入
- [ ] 移动端 taro-h5 适配
- [ ] 行业轮动分析
- [ ] 持股策略与卖出信号
- [ ] 技术回测引擎
