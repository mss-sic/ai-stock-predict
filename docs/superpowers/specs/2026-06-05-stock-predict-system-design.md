# 智策投研 · 股票预测分析系统 — 技术设计

> 日期: 2026-06-05 | 状态: 待审查

## 一、概述

三端系统（PC/H5/服务端），辅助股票投资决策。核心链路：数据采集 → 算法精选榜单 → AI分析/预测 → 策略回测 → 持仓跟踪。

## 二、项目架构

```
ai-stock-predict/
├── server/                    # Go API 服务
│   ├── cmd/server/main.go    # 入口
│   ├── internal/
│   │   ├── collector/        # 数据采集引擎 (a-stock-data 封装)
│   │   ├── handler/          # HTTP handler (Gin)
│   │   ├── service/          # 业务逻辑
│   │   ├── repository/       # 数据访问 (GORM)
│   │   ├── model/            # 数据模型
│   │   └── scheduler/        # 定时任务 (robfig/cron)
│   ├── migrations/           # SQL 迁移
│   └── go.mod
├── web-pc/                   # React 18 + Arco Design
│   └── src/
│       ├── pages/            # 10 个功能页面
│       ├── components/       # K线图、榜单卡片等
│       ├── hooks/
│       └── services/
├── web-mobile/               # Taro 3.x + React
│   └── src/
│       ├── pages/            # 精选页面 (榜单/个股/自选)
│       └── components/
├── docker/
│   └── docker-compose.yml    # pg + mysql + go
├── scripts/                  # Python 采集脚本 (a-stock-data)
└── docs/
```

## 三、数据库设计

### PostgreSQL (行情+时序)

| 表 | 引擎 | 说明 |
|---|---|---|
| `stocks_basic` | 普通表 | 代码、名称、行业、上市日、总股本 |
| `stocks_daily_k` | TimescaleDB hypertable | 日期、开/高/低/收、成交量、成交额、换手率 |
| `stocks_daily_indicator` | TimescaleDB hypertable | PE、PB、PS、总市值、流通市值 |
| `algorithm_picks` | 普通表 | 日期、总评分、上榜股票数 |
| `algorithm_pick_details` | 普通表 | 股票代码、评分、信号标签、风险提示、策略建议 |

### MySQL (业务)

| 表 | 说明 |
|---|---|
| `users` | 用户 |
| `watchlists` | 自选股 |
| `strategies` | 策略定义 (名称/参数JSON) |
| `backtest_results` | 回测结果 (资金曲线/交易明细JSON) |
| `holdings` | 持仓 (成本价/数量/策略关联) |
| `risk_alerts` | 风险预警记录 |
| `import_logs` | Excel导入日志 |

## 四、API 清单 (Go REST)

```
# 行情
GET  /api/v1/stocks                    # 列表(分页/筛选)
GET  /api/v1/stocks/:code              # 个股详情
GET  /api/v1/stocks/:code/kline        # K线 ?from=&to=
GET  /api/v1/stocks/:code/indicator    # 技术指标

# 榜单
GET  /api/v1/board/today               # 今日榜单
GET  /api/v1/board/history?date=       # 历史榜单
GET  /api/v1/board/heatmap             # 上榜热力图
GET  /api/v1/board/heatmap/:code       # 单股连榜

# 预测
GET  /api/v1/forecast/:code            # 走势预测

# AI分析
POST /api/v1/ai/analyze                # AI分析 (对接LLM)

# 策略
POST /api/v1/strategy/backtest         # 运行回测
GET  /api/v1/strategy/backtest/:id     # 回测结果

# 自选/持仓
GET  /api/v1/watchlist
POST /api/v1/watchlist
GET  /api/v1/holdings
POST /api/v1/holdings

# 导入
POST /api/v1/import/excel              # 上传Excel

# 采集管理
POST /api/v1/collector/trigger
GET  /api/v1/collector/status
PUT  /api/v1/collector/schedule
```

## 五、数据采集

| 数据 | 源 | 频率 | 时机 |
|------|-----|------|------|
| 日K线 | 腾讯财经/mootdx | 每日 | 15:30 |
| 基本面指标 | 东财/新浪 | 每日 | 15:30 |
| 基础信息 | mootdx F10 | 每周 | 周六 |
| 研报 | 东财 reportapi | 每日 | 可选 |
| 榜单导入 | Excel上传 | 手动 | PC端 |

**防封策略**：东财接口串行、间隔 ≥1.5s、复用会话、正常UA+Referer。

## 六、前端页面 (PC 10页)

1. **今日榜单** - Arco Table + 排序/筛选/信号标签
2. **历史榜单** - 日期选择 + 榜单对比
3. **上榜热力图** - Canvas 矩阵热力图
4. **个股详情** - ECharts K线 + 估值卡片
5. **走势预测** - 预测路径折线图
6. **AI分析** - 对话式界面
7. **自选股** - 列表管理
8. **策略中心** - 参数配置 + 回测图表 + 交易明细表
9. **持仓跟踪** - 实时刷新卡片 + 盈亏条
10. **风险预警** - 分级列表 + 类型分布

## 七、技术选型

| 层 | 选型 | 理由 |
|----|------|------|
| API | Go + Gin | 高性能、采集与API同进程 |
| ORM | GORM | 双数据库支持 |
| 定时 | robfig/cron | 轻量 cron |
| 图表 | ECharts | K线 + 热力图原生支持 |
| Excel | excelize | Go 原生解析 |
| Taro | 3.x + React | H5/小程序一套 |
| 部署 | Docker Compose | pg+mysql+go 一键 |

## 八、假设与约束

1. MVP 仅一个用户，无鉴权体系（后续加 JWT）
2. 算法团队提供榜单数据（先 Excel 导入，后直接写库）
3. 走势预测/AI分析初版用 Mock，后续对接真实模型
4. 采集脚本暂以 Python (a-stock-data) 运行，Go 通过 exec 或 HTTP 调用
5. 本地开发环境 macOS，Docker 化便于迁移
