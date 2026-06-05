# 智策投研 MVP — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建数据采集管道 + PC端榜单展示 MVP，打通 自动采集→存储→API→前端 全链路。

**Architecture:** Go API 服务 (Gin+GORM) 连接 PostgreSQL(TimescaleDB) 存行情数据 + MySQL 存业务数据，Python 脚本调用 a-stock-data 采集，React+Arco Design PC 端展示。

**Tech Stack:** Go 1.22+, Gin, GORM, PostgreSQL 16+TimescaleDB, MySQL 8, React 18, Arco Design, ECharts, Docker Compose

---

## Phase 1: 基础设施搭建

### Task 1: Go 项目脚手架

**Files:**
- Create: `server/go.mod`
- Create: `server/cmd/server/main.go`
- Create: `server/internal/config/config.go`

- [ ] **Step 1: 初始化 Go module**

```bash
cd server && go mod init github.com/ai-stock-predict/server
```

- [ ] **Step 2: 创建入口 main.go**

```go
package main

import (
    "log"
    "github.com/ai-stock-predict/server/internal/config"
)

func main() {
    cfg := config.Load()
    log.Printf("server starting on :%s", cfg.Port)
}
```

- [ ] **Step 3: 创建配置模块 config.go**

加载环境变量：DB DSN、端口、采集计划 cron 表达式。默认值内嵌。

- [ ] **Step 4: 验证编译通过**

```bash
cd server && go build ./cmd/server/
```

### Task 2: Docker Compose 编排

**Files:**
- Create: `docker/docker-compose.yml`
- Create: `docker/postgres/init.sql`
- Create: `docker/mysql/init.sql`

- [ ] **Step 1: 编写 docker-compose.yml**

三个服务：postgres (TimescaleDB 镜像)、mysql:8、go-server (Dockerfile 后续补)。
pg 端口 5432，mysql 端口 3306，go 端口 8080。

- [ ] **Step 2: PostgreSQL 初始化脚本**

创建 `stocks_basic`, `stocks_daily_k` (hypertable), `stocks_daily_indicator` (hypertable), `algorithm_picks`, `algorithm_pick_details` 表。

- [ ] **Step 3: MySQL 初始化脚本**

创建 `users`, `watchlists`, `strategies`, `backtest_results`, `holdings`, `risk_alerts`, `import_logs` 表。

- [ ] **Step 4: 验证容器启动**

```bash
cd docker && docker compose up -d postgres mysql
docker compose ps
```

### Task 3: Go 数据库连接 + 迁移

**Files:**
- Create: `server/internal/db/postgres.go`
- Create: `server/internal/db/mysql.go`
- Create: `server/internal/db/migrate.go`
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: PostgreSQL 连接封装**

用 GORM + TimescaleDB driver，连接池配置 20 连接，自动重连。

- [ ] **Step 2: MySQL 连接封装**

用 GORM + MySQL driver，连接池配置 10 连接。

- [ ] **Step 3: AutoMigrate 集成**

在 main.go 启动时执行 GORM AutoMigrate，创建所有模型表。

- [ ] **Step 4: 验证**

```bash
go run ./cmd/server/ → 无报错，数据库表自动创建
```

## Phase 2: 数据模型

### Task 4: PostgreSQL 数据模型

**Files:**
- Create: `server/internal/model/stock_basic.go`
- Create: `server/internal/model/stock_daily_k.go`
- Create: `server/internal/model/stock_daily_indicator.go`
- Create: `server/internal/model/algorithm_pick.go`

- [ ] **Step 1: StockBasic 模型**

字段：Code(string, pk), Name, Industry, ConceptTags(JSON), ListedDate, TotalShares, UpdatedAt

- [ ] **Step 2: StockDailyK 模型 (hypertable)**

字段：Code, TradeDate(partition key), Open, High, Low, Close, Volume, Amount, TurnoverRate
联合主键 (Code, TradeDate)

- [ ] **Step 3: StockDailyIndicator 模型 (hypertable)**

字段：Code, TradeDate, PE, PB, PS, TotalMarketCap, CirculatingMarketCap

- [ ] **Step 4: AlgorithmPick + AlgorithmPickDetail 模型**

Pick: Date(unique), TotalStocks, GeneratedAt
Detail: PickDate, StockCode, Rank, Score, SignalTags(JSON), RiskLevel, Suggestion(enum: buy/hold/sell)

- [ ] **Step 5: 编译验证**

### Task 5: MySQL 数据模型

**Files:**
- Create: `server/internal/model/user.go`
- Create: `server/internal/model/watchlist.go`
- Create: `server/internal/model/strategy.go`
- Create: `server/internal/model/backtest_result.go`
- Create: `server/internal/model/holding.go`
- Create: `server/internal/model/risk_alert.go`
- Create: `server/internal/model/import_log.go`

- [ ] **Step 1: User 模型**

ID(uint, pk), Username, PasswordHash, CreatedAt

- [ ] **Step 2: Watchlist 模型**

ID, UserID, StockCode, AddedAt。唯一索引 (UserID, StockCode)

- [ ] **Step 3: Strategy 模型**

ID, UserID, Name, Params(JSON) — 策略参数如止盈%、止损%、最大持有日等

- [ ] **Step 4: BacktestResult 模型**

ID, UserID, StrategyID, StockCode, StartDate, EndDate, TotalReturn, SharpeRatio, MaxDrawdown, WinRate, TradeCount, Trades(JSON), EquityCurve(JSON)

- [ ] **Step 5: Holding 模型**

ID, UserID, StockCode, CostPrice, Quantity, StrategyID, CreatedAt

- [ ] **Step 6: RiskAlert 模型**

ID, StockCode, Level(enum), Type, Description, HitDate, Ignored(bool)

- [ ] **Step 7: ImportLog 模型**

ID, FileName, RowsImported, Status, ErrorMsg, ImportedAt

- [ ] **Step 8: 编译验证**

## Phase 3: 数据采集

### Task 6: Python 采集脚本

**Files:**
- Create: `scripts/collector/daily_k.py`
- Create: `scripts/collector/stock_basic.py`
- Create: `scripts/collector/requirements.txt`

- [ ] **Step 1: 日K线采集脚本**

调用 a-stock-data SKILL 中腾讯财经/mootdx 端点。
输入：股票代码列表，日期间隔。输出：JSON 到 stdout。
内置限流（东财接口间隔 ≥1.5s）、重试（3次）、日志。

- [ ] **Step 2: 股票基础信息采集脚本**

调用 mootdx 财务/F10 + 东财获取 PE/PB/市值。
输出 JSON 到 stdout。

- [ ] **Step 3: requirements.txt**

mootdx, requests, pandas, stockstats

- [ ] **Step 4: 手动测试脚本**

```bash
cd scripts && python collector/daily_k.py --code 600519 --days 30
# 验证返回 JSON 正确
```

### Task 7: Go 采集引擎

**Files:**
- Create: `server/internal/collector/engine.go`
- Create: `server/internal/collector/python.go`
- Create: `server/internal/collector/task.go`

- [ ] **Step 1: Python 调用封装 (python.go)**

通过 `os/exec` 调用 Python 脚本，解析 stdout JSON。设置 120s 超时。

- [ ] **Step 2: 采集任务定义 (task.go)**

定义 `DailyKTask` — 获取所有已入库股票代码，按交易日拉取日K。
定义 `StockBasicTask` — 更新股票基础信息。
每个 task 返回采集统计（成功/失败数）。

- [ ] **Step 3: 采集引擎 (engine.go)**

串联 task 执行：先更新基础信息 → 再拉日K。
错误处理：单只股票失败不阻塞整体。
记录采集日志到 import_logs 表。

- [ ] **Step 4: 编译验证**

### Task 8: Excel 导入器

**Files:**
- Create: `server/internal/collector/excel.go`

- [ ] **Step 1: Excel 解析**

用 `excelize` 库解析 .xlsx/.xlsm。
按 Sheet2 格式：行=股票序列，列=日期对（日期/代码/名称交替）。
解析出 {date: [{code, name}]} 结构。

- [ ] **Step 2: 导入到 algorithm_picks 表**

每日期一行 algorithm_picks，每股票一行 algorithm_pick_details。
幂等：同日期已存在则跳过或覆盖（可配置）。

- [ ] **Step 3: 编译验证**

### Task 9: 定时调度器

**Files:**
- Create: `server/internal/scheduler/scheduler.go`
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: Scheduler 实现**

用 robfig/cron 库。
默认 cron：`30 15 * * 1-5`（交易日 15:30）。
启动时检查今天是否交易日（简单判断：周一到周五 + 非中国假日）。

- [ ] **Step 2: 集成到 main.go**

启动 scheduler，注册采集任务。
优雅关闭：SIGTERM/SIGINT 时停止 scheduler。

- [ ] **Step 3: 编译验证**

## Phase 4: API 层

### Task 10: 股票行情 API

**Files:**
- Create: `server/internal/repository/stock_repo.go`
- Create: `server/internal/service/stock_service.go`
- Create: `server/internal/handler/stock_handler.go`
- Modify: `server/cmd/server/main.go` (注册路由)

- [ ] **Step 1: StockRepository**

方法：`List(filter)`, `GetByCode(code)`, `GetKLine(code, from, to)`, `GetIndicator(code, date)`, `UpsertBasic`, `UpsertDailyK`

- [ ] **Step 2: StockService**

封装业务逻辑：K线数据补全（缺失日期返回空）、指标计算（从 raw 数据算 MA5/MA10/MA20 等）

- [ ] **Step 3: StockHandler**

4 个 endpoint：
- `GET /api/v1/stocks` — 分页列表，支持 ?industry=&keyword=
- `GET /api/v1/stocks/:code` — 个股详情
- `GET /api/v1/stocks/:code/kline` — K线 ?from=&to=
- `GET /api/v1/stocks/:code/indicator` — 技术指标

- [ ] **Step 4: 路由注册 + 测试**

```bash
curl http://localhost:8080/api/v1/stocks
```

### Task 11: 榜单 API

**Files:**
- Create: `server/internal/repository/board_repo.go`
- Create: `server/internal/service/board_service.go`
- Create: `server/internal/handler/board_handler.go`

- [ ] **Step 1: BoardRepository**

方法：`GetTodayBoard()`, `GetBoardByDate(date)`, `GetHeatmapData(from, to)`, `GetStockHeatmap(code)`, `UpsertBoard(date, picks)`

- [ ] **Step 2: BoardService**

计算热力图矩阵：每只股票 × 每个交易日的上榜/评分/涨跌幅

- [ ] **Step 3: BoardHandler**

4 个 endpoint：
- `GET /api/v1/board/today`
- `GET /api/v1/board/history?date=`
- `GET /api/v1/board/heatmap` — ?from=&to=
- `GET /api/v1/board/heatmap/:code`

- [ ] **Step 4: 路由注册 + 测试**

### Task 12: Excel 导入 + 采集管理 API

**Files:**
- Create: `server/internal/handler/import_handler.go`
- Create: `server/internal/handler/collector_handler.go`

- [ ] **Step 1: ImportHandler**

`POST /api/v1/import/excel` — multipart 文件上传，调用 excel 解析器，写入 algorithm_picks 表，返回导入统计。

- [ ] **Step 2: CollectorHandler**

`POST /api/v1/collector/trigger` — 手动触发采集
`GET /api/v1/collector/status` — 返回最近采集日志
`PUT /api/v1/collector/schedule` — 更新 cron 表达式

- [ ] **Step 3: 路由注册 + 编译验证**

### Task 13: 预测 + AI 分析 API（Mock）

**Files:**
- Create: `server/internal/handler/forecast_handler.go`
- Create: `server/internal/handler/ai_handler.go`

- [ ] **Step 1: ForecastHandler (Mock)**

`GET /api/v1/forecast/:code?horizon=5`
返回模拟预测路径：N 个数据点 + 上/下置信区间。
用确定性随机种子（基于 code+date），保证同一天同一股票返回一致。

- [ ] **Step 2: AIHandler (Mock)**

`POST /api/v1/ai/analyze` — body: {code, question}
返回模拟 AI 回复（预设模板 + 基于股票数据的变量填充）。
后续对接真实 LLM API。

- [ ] **Step 3: 编译验证**

## Phase 5: PC 前端

### Task 14: React + Arco Design 项目脚手架

**Files:**
- Create: `web-pc/` (Vite + React 项目)
- Create: `web-pc/src/App.tsx`
- Create: `web-pc/src/services/api.ts`
- Create: `web-pc/src/router.tsx`

- [ ] **Step 1: 初始化项目**

```bash
npm create vite@latest web-pc -- --template react-ts
cd web-pc && npm install @arco-design/web-react @arco-design/icon echarts echarts-for-react react-router-dom axios dayjs
```

- [ ] **Step 2: API 服务层 (api.ts)**

封装 axios 实例，baseURL 指向 `http://localhost:8080/api/v1`。
统一错误处理和响应拦截。

- [ ] **Step 3: 路由配置**

10 个页面路由：/, /board/history, /board/heatmap, /stock/:code, /forecast/:code, /ai/:code, /watchlist, /strategy, /holdings, /risk

- [ ] **Step 4: App.tsx 布局框架**

Arco Layout: Sider(侧边导航) + Header + Content。
导航菜单对应 10 个页面。

- [ ] **Step 5: 验证**

```bash
cd web-pc && npm run dev
# 浏览器打开 localhost:5173，看到布局框架
```

### Task 15: 今日榜单页面

**Files:**
- Create: `web-pc/src/pages/BoardPage.tsx`
- Create: `web-pc/src/components/StockTag.tsx`
- Create: `web-pc/src/components/SignalBadge.tsx`

- [ ] **Step 1: BoardPage 主体**

Arco Table：列 = 排名、代码、名称、概念标签、评分、涨跌幅、信号、操作
支持排序（按评分、涨跌幅）、筛选（行业、信号类型）

- [ ] **Step 2: StockTag + SignalBadge 组件**

StockTag：概念/行业标签，Arco Tag 多色。
SignalBadge：买入(绿)/持有(蓝)/卖出(红)，图标 + 文字。

- [ ] **Step 3: 对接 API**

`GET /api/v1/board/today` → 渲染表格。
每行操作列：查看详情(跳转个股页)、添加自选。

- [ ] **Step 4: 验证**

### Task 16: 个股详情页面

**Files:**
- Create: `web-pc/src/pages/StockDetailPage.tsx`
- Create: `web-pc/src/components/KLineChart.tsx`

- [ ] **Step 1: KLineChart 组件**

用 ECharts candlestick 类型。
支持缩放、拖拽、MA 均线叠加（MA5/10/20/60）。

- [ ] **Step 2: StockDetailPage 布局**

上半部分：基本信息卡片（名称、代码、行业、市值、PE/PB）+ 涨跌幅。
中部：KLineChart（默认 90 日）。
下半部分：标签页切换（资金流向、技术指标、基本面）。

- [ ] **Step 3: 对接 API**

`GET /api/v1/stocks/:code` + `GET /api/v1/stocks/:code/kline` + `GET /api/v1/stocks/:code/indicator`

### Task 17: 历史榜单 + 热力图页面

**Files:**
- Create: `web-pc/src/pages/HistoryBoardPage.tsx`
- Create: `web-pc/src/pages/HeatmapPage.tsx`

- [ ] **Step 1: HistoryBoardPage**

日期选择器（Arco DatePicker），选择历史日期后展示该日榜单。
小型统计卡片：当日上榜股票平均涨跌幅、连榜股票数等。

- [ ] **Step 2: HeatmapPage**

用 ECharts heatmap 类型。
X 轴：交易日，Y 轴：股票代码。
单元格颜色：涨跌幅（红涨绿跌），hover 显示详情。
视图切换：按评分着色 / 按涨跌幅着色。

- [ ] **Step 3: 对接 API**

`GET /api/v1/board/history?date=` + `GET /api/v1/board/heatmap`

### Task 18: 走势预测 + AI 分析页面

**Files:**
- Create: `web-pc/src/pages/ForecastPage.tsx`
- Create: `web-pc/src/pages/AIAnalysisPage.tsx`

- [ ] **Step 1: ForecastPage**

预测周期选择（5/10/20/30 日按钮组）。
ECharts line chart：预测路径主线 + 置信区间填充带（areaStyle）。

- [ ] **Step 2: AIAnalysisPage**

对话式界面：输入框 + 消息列表。
AI 消息使用 Arco Typography + 代码块渲染。
Mock 回复模式（后续对接真实 LLM）。

- [ ] **Step 3: 对接 API**

`GET /api/v1/forecast/:code` + `POST /api/v1/ai/analyze`

### Task 19: 自选股 + 策略回测 + 持仓 + 风险页面

**Files:**
- Create: `web-pc/src/pages/WatchlistPage.tsx`
- Create: `web-pc/src/pages/StrategyPage.tsx`
- Create: `web-pc/src/pages/HoldingsPage.tsx`
- Create: `web-pc/src/pages/RiskPage.tsx`

- [ ] **Step 1: WatchlistPage**

自选股列表 + 搜索添加（Arco InputSearch + AutoComplete）。
每行：代码、名称、最新价、涨跌幅、操作（删除）。

- [ ] **Step 2: StrategyPage**

左侧：策略参数表单（止盈%、止损%、最大持有日等）+ 预设策略选择。
右侧：回测结果区域（收益率、夏普比率、最大回撤）+ 资金曲线图 + 交易明细表。
点击"运行回测"调用 API。

- [ ] **Step 3: HoldingsPage**

持仓卡片网格：每张卡片显示代码、名称、现价、成本价、浮动盈亏、止盈/止损距离。
信号灯：绿色(正常)、黄色(接近止盈/止损)、红色(已触及)。
实时 tick 模拟（2s 刷新）。

- [ ] **Step 4: RiskPage**

分级预警列表：Arco Table 按等级分组（高/中/低）。
筛选：等级、类型。
操作：忽略单条、查看详情。

### Task 20: Excel 导入 + 采集管理入口

**Files:**
- Create: `web-pc/src/pages/DataManagementPage.tsx`

- [ ] **Step 1: DataManagementPage**

标签页切换：
- 数据导入：文件上传（Arco Upload）+ 拖拽区域，上传后显示导入统计。
- 采集状态：当前采集计划、最近采集日志列表、手动触发按钮。

对接 API：`POST /api/v1/import/excel` + collector endpoints。

## Phase 6: 集成验证

### Task 21: 端到端集成测试

- [ ] **Step 1: 启动全栈**

```bash
cd docker && docker compose up -d
cd server && go run ./cmd/server/
cd web-pc && npm run dev
```

- [ ] **Step 2: 导入测试数据**

用 MSS20260603.xlsm 通过 API 导入。
验证榜单 API 返回正确数据。

- [ ] **Step 3: PC 端页面验收**

逐页检查：榜单展示、个股详情 K线、热力图渲染、预测图表。

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: MVP 全栈实现"
```

---

## 附录：后续迭代（不在 MVP 范围）

- H5/小程序 (Taro) 开发
- 真实 LLM 对接（AI 分析）
- 真实预测模型对接
- 用户鉴权 (JWT)
- 实盘交易接口对接
- 研报/新闻模块
- 技术回测完整功能
- CI/CD 流水线
