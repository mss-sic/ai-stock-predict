## v1.8.0 (2026-06-29)

### 数据补全 — 10 项新数据源 + 采集系统重构

- **龙虎榜** (`dragon_tiger_list` + `dragon_tiger_detail`): 全市场每日上榜 + 买卖席位 TOP5，东财 datacenter 接口
- **融资融券** (`margin_trading`): 全市场融资余额/融券余额，东财 RPT_HSG_DAILYMARKET
- **大宗交易** (`block_trade`): 成交价/折溢价率/买卖方营业部，东财 RPT_BLOCKTRADE_MAIN
- **限售解禁** (`restricted_share_unlock`): 历史+未来90天预告，东财 RPT_FHD_SHARES_RESTRICTED
- **同花顺热点** (`ths_hot_stocks`): 每日强势股+题材归因标签，同花顺 stockhot 接口
- **分红送转** (`dividend_history`): 每股分红/送股/转增历史，东财 RPT_FHD_DIVIDEND
- **一致预期EPS** (`ths_eps_forecast`): 机构一致预期，同花顺 basic.10jqka
- **巨潮公告** (`cninfo_announcements`): 年报/季报/业绩预告，巨潮 cninfo API
- **宏观资讯** (`macro_news`): 央行政策/国际宏观/产业政策，东财 quicknotice

### 采集控制台重构

- Phase 列表从 10 项硬编码扩展到 29 项动态加载（对齐定时任务）
- 移除「行情刷新」冗余按钮和「回填市场风格」独立按钮
- 每个 Phase 增加「采集」+「修复历史」双按钮模式
- 修复历史支持日期范围选择或全部历史数据回填
- 实时数据源（仅支持最新数据）自动隐藏修复按钮、禁用时间范围选择

### 定时任务 & 数据概览

- 10 个新数据源接入定时调度（cron 表达式），支持手动执行和修复模式
- 定时任务执行日志新增 STAT 协议解析（records_new/skip/err 准确统计）
- 数据概览从 10 项扩展为 24 项，新增 14 个统计卡片+明细查询

### Bug 修复

- 修复 `collection_logs` 表缺少 `behavior_stats` 列（v041 迁移自动补齐）
- 修复 SSE `onerror` 闭包捕获过期 `collecting` 状态导致连接 10ms 即断开
- 修复修复 Modal 嵌在 Toast 组件内导致 JSX 解析错误
- 概念数据源确认：东财 slist API 完整覆盖 CPO/低空经济/人形机器人等前沿概念

## v1.7.1 (2026-06-26)

### 概念分析 — AI 调用与提示词管理优化

- **概念分析不再自动调用 AI**: `GetConceptAnalysis` 在 `refresh=false` 且无缓存时直接返回空，仅点击"开始分析"才触发 AI
- **修复 AI 调用记录无法显示用户**: `concept_analysis_handler` 中 `c.GetUint("userID")` → `c.Get("userId")`（大小写匹配 auth 中间件）
- **概念分析 Markdown 渲染修复**: 启用 `remarkGfm` + `rehypeRaw`，表格/列表/引用块美化排版
- **AI 调用记录模块统一**: `AdminPage` / `UserCostPage` 的 MODULE_LABELS 补齐 `concept_analysis`(概念分析) + `ai_agent_review`(AI复盘)，图表顺序从 MODULE_LABELS 自动派生
- **概念分析提示词纳入系统配置**: `concept_analysis` 场景注册到 `ScenePromptVars`，提示词从 `ai_system_configs` 表读取，v036 迁移自动播种，系统设置统一管理

### 数据管理 — 定时任务补全

- **新增 4 个定时任务**: 市场日聚合(16:00) / 市场情绪(17:00) / 市场风格(17:30) / AI评分(每周日)
- `InitializeDefaultTasks` 改为按名称增量检查，重启自动追加新任务
- 重复 quote 任务已处理（旧版"实时行情采集"禁用，保留"实时行情监控"交易时段）

### 采集任务 — 范围选择

- 点击「执行」弹出范围选择 Modal：最新 / 7天 / 30天 / 60天 / 90天 / 全部
- 参数链路：前端 args → handler → service → collector → Python 脚本
- 日志表头「跳过」→「存量」更准确

### 概念板块 — 防覆盖双模式

- **增量模式**（每日 08:00）：board API → UPSERT，检测到近期全量重建自动跳过
- **全量模式**（每周日 06:00）：stock-centric → DELETE+INSERT → `rebuild_concepts.py`
- 全量重建超时阈值从 10 分钟 → 120 分钟（`LONG_RUNNING_PHASES`）

### 数据管道 — 依赖对齐

- 线上修复脚本 `fix_online_pipeline.py`：precompute_aggs → compute_sentiment → BulkCompute
- K线采集自动包含大盘指数(上证/深证/沪深300/中证1000/创业板) + 国债ETF(3只)
- `news_collect.py` SQL 类型转换修复（publish_date varchar → ::date）

## v1.7.0 (2026-06-25)

### 市场复盘 — 独立模块

- 新增 `market_style_daily` 表 + `MarketStyleService` 风格引擎
- 风格时间轴曲线 / 每日深度复盘 / 结构性分析（概念抱团持续性）/ 风格切换预警
- 策略「智能风控」Tab 通俗化：场景卡片替代技术术语

### 交易策略 — 功能迭代

- 新增上榜频率指标（近5日/近20日上榜次数）→ 策略条件可选
- 策略条件新增 indicator 指标：streak_count

## v1.6.2 (2026-06-23)

### 端到端回测 — 系统漏洞修复 7 项

- **MACD 批量预加载** SMA→真实 EMA（Go 端计算，替换 AVG/ROWS BETWEEN）
- **评分归一化** 除以总权重和(totalConfigWeight)替代条件数，修复分数被稀释
- **AdaptiveMinScore 加固** 地板 0.20→0.30，gap 因子 0.3→0.5，过滤噪音信号
- **预加载日期范围** 从 endDate-80d 改为 min(startDate-100d, endDate-80d)，确保指标回溯
- **V2 止损守卫** generateSignalsV2 止损循环增加 Quantity>0 检查，防止0股假止损
- **backfill_macd.py** psycopg2 Decimal→float 类型转换修复
- **EMA 数据回填** 测试组合 10 只股票 3,228 行 EMA12/26/DIF/DEA 已补充

### 回测结果（3轮迭代）
- 收益率：-6.60% → -4.37% → -2.42%（持续改善）
- 夏普：-3.25 → -3.13 → -1.90
- 最大回撤：7.58% → 5.70% → 3.78%

## v1.6.1 (2026-06-23)

### 前端重构 — 组件拆分 + 小白化

- **组件提取** — `TemplateSelector.tsx`(策略模板卡片网格) + `IndicatorPicker.tsx`(分类可搜索指标选择器)，StrategyPage 从 2099→1983 行
- **指标选择器分类** — 10 类折叠分组(技术面-趋势/超买超卖/量价/形态 + 估值 + 基本面 + 资金面 + AI评分 + 预测)，顶部搜索框
- **内联帮助 9 处** — 权重/模糊评分/回溯天数/连续天数/趋势方向/行业相对/条件逻辑/K线周期 均加 Tooltip
- **无效选项灰显** — K线周期「周线/月线」⚠ disabled

### 死代码清理

- 删除 5 个文件共 2029 行，提取共享类型 → `backtest_types.go`(56行)

### 模板选择器

- 新建策略 Modal 3×2 模板卡片，选中后自动写入条件+三模态参数

### Bug 修复

- 修复策略编排页 React hooks 报错（`React.useState` 在 JSX IIFE → 提升到组件顶层）

## v1.6.0 (2026-06-22)

### 策略条件编排引擎重构 — 双模引擎

- **三层架构** — 条件层(LogicGroup AND/OR) + 策略层(PolicyManager) + AI 监督层，替代原有简单 IF 且关系
- **条件层** — 每组条件内 AND 逻辑，多组条件之间 OR 逻辑；UI 按 G1/G2/G3 分组标记，自动递增 LogicGroup
- **策略层** — 根据市场 `composite_score` 自动切换三模式：
  - 🟢 进攻(score≥1.5)：OR 逻辑 + 允许加仓 + 1.2x 仓位乘数
  - 🟡 防御(score≥0.0)：AND 逻辑 + 禁止加仓 + 0.8x 仓位乘数
  - 🔴 空仓(score<0.0)：禁止买入/加仓 + 激进卖出 1.5x
- **AI 监督层** — 接入 DeepSeek/Kimi，审查候选标的并输出可解释决策理由；硬性风控不可被 AI 覆盖

### 新增引擎组件 (8 文件)

- `scoring_engine.go` — 加权评分引擎（Buy/Add）：6 种条件子类型（Basic/Fuzzy/TimeSeries/IndustryRelative/MultiTimeframe），fuzzy sigma 自适应模糊评分
- `decision_tree_engine.go` — 决策树引擎（Sell/Reduce）：支持嵌套条件组 + and/or/not 逻辑
- `context_manager.go` — 市场上下文管理器：读取 MarketSentiment 13 维输出 MarketContext（综合分/风险偏好/行业强势/北向资金）
- `policy_manager.go` — 政策决策管理器：进攻/防御/空仓三模式自动切换
- `risk_manager.go` — 风控管理器：市场熔断/单日亏损熔断/仓位集中度限制
- `backtest_v2.go` — 回测 V2：集成新引擎的完整回测管线，支持 panic recover + ST/停牌/涨停过滤
- `ai_agent.go` — AI 代理：运行时审查候选，动态调整权重(±30%) + 最终确认/否决
- `position_manager_v2.go` — 持仓管理 V2：集成双模引擎

### 新增数据模型

- `condition_template.go` — 复合条件模板（系统预置 MACD 金叉/RSI 超卖/趋势突破/放量滞涨/均线死叉 5 模板）
- `ai_agent_decision.go` — AI 代理决策记录（市场分/偏好乘数/候选数/推理过程/操作清单）
- `strategy.go` — 新增 orchestrat_mode/enable_market_context/enable_ai_agent/industry_filter 等 9 字段
- `strategy_condition.go` — 新增 weight/fuzzy_sigma/lookback_days/industry_relative/timeframe/parent_id 等 10 字段

### P0 优化修复

- **评分区分度修复** — fuzzy sigma 默认值 0.30→1.0，minScore 从硬编码 0.18 → 动态中位数+top1 差值自适应
- **ST/停牌过滤强化** — 从名称检查升级为 stock_profiles 表标记查询 + 涨停过滤(≥9.8%)
- **ContextManager 修复** — 修复 sector_diffusion_score 列缺失(872 次 error)，增加优雅降级 + 5 分钟缓存
- **Nil Pointer Panic** — task_service 增加 nil guard，回测 goroutine 增加 defer/recover

### 后端端点新增

- `GET/PUT /strategies/:id/orchestration` — 策略编排配置
- `GET /strategies/templates` — 系统+用户条件模板列表
- `POST /strategies/templates` — 从条件创建模板
- `GET /strategies/:id/ai-decisions` — AI 代理决策历史

### 前端 UI 新增

- StrategyPage 编排 Tab 新增「政策决策」卡片：模式选择(rule/ai_driven/manual) + 阈值滑块
- 条件卡片显示 G1/G2/G3 分组标记替代 IF/AND
- 策略模板选择器 + 一键应用

### 回测结果 (S72 趋势追踪, 2026-01-01→2026-06-10)

| 版本 | 收益率 | 买入 | 加仓 | 止损 |
|------|--------|------|------|------|
| AND 原始 | +2.14% | 82 | 0 | 31 |
| OR + Policy | -8.82% | 19 | 68 | 4 |

策略分布 54 交易日：空仓 36(68%) / 进攻 10(19%) / 防御 7(13%)


## v1.6.1 (2026-06-23)

### 前端重构 — 组件拆分 + 小白化

- **组件提取** — `TemplateSelector.tsx`(策略模板卡片网格) + `IndicatorPicker.tsx`(分类可搜索指标选择器)，StrategyPage 从 2099→1983 行
- **指标选择器分类** — 10 类折叠分组(技术面-趋势/超买超卖/量价/形态 + 估值 + 基本面 + 资金面 + AI评分 + 预测)，顶部搜索框
- **内联帮助 9 处** — 权重/模糊评分/回溯天数/连续天数/趋势方向/行业相对/条件逻辑/K线周期 均加 Tooltip 解释交易含义
- **无效选项灰显** — K线周期「周线/月线」标记 ⚠ 且 disabled，Tooltip「仅实盘可用」

### 死代码清理

- 删除 `decision_chain.go`、`position_manager_v2.go`、`risk_manager.go`、`position_sizer.go`、`strategy_coverage_test.go` 共 2029 行
- 提取共享类型 → `backtest_types.go` (56行)：`ActionTarget`/`dcPosition`/`dcStockInfo`/`joinActions`

### 模板选择器 — 一键创建策略

- 新建策略 Modal 增加 3×2 模板卡片网格（动量追击/趋势跟随/均值回归/放量突破/价值精选/保守蓝筹）
- 选中模板 → 自动写入 buyConds/sellConds → 自动写入三模态参数(aggressive/defensive/cash)

### Bug 修复

- 修复策略编排页 React hooks 报错（`React.useState` 在 JSX IIFE 内 → 提升到组件顶层）
## v1.6.0 (2026-06-22)

### 策略条件编排引擎重构 — 双模引擎

- **三层架构** — 条件层(LogicGroup AND/OR) + 策略层(PolicyManager) + AI 监督层，替代原有简单 IF 且关系
- **条件层** — 每组条件内 AND 逻辑，多组条件之间 OR 逻辑；UI 按 G1/G2/G3 分组标记，自动递增 LogicGroup
- **策略层** — 根据市场 `composite_score` 自动切换三模式：
  - 🟢 进攻(score≥1.5)：OR 逻辑 + 允许加仓 + 1.2x 仓位乘数
  - 🟡 防御(score≥0.0)：AND 逻辑 + 禁止加仓 + 0.8x 仓位乘数
  - 🔴 空仓(score<0.0)：禁止买入/加仓 + 激进卖出 1.5x
- **AI 监督层** — 接入 DeepSeek/Kimi，审查候选标的并输出可解释决策理由；硬性风控不可被 AI 覆盖

### 新增引擎组件 (8 文件)

- `scoring_engine.go` — 加权评分引擎（Buy/Add）：6 种条件子类型（Basic/Fuzzy/TimeSeries/IndustryRelative/MultiTimeframe），fuzzy sigma 自适应模糊评分
- `decision_tree_engine.go` — 决策树引擎（Sell/Reduce）：支持嵌套条件组 + and/or/not 逻辑
- `context_manager.go` — 市场上下文管理器：读取 MarketSentiment 13 维输出 MarketContext（综合分/风险偏好/行业强势/北向资金）
- `policy_manager.go` — 政策决策管理器：进攻/防御/空仓三模式自动切换
- `risk_manager.go` — 风控管理器：市场熔断/单日亏损熔断/仓位集中度限制
- `backtest_v2.go` — 回测 V2：集成新引擎的完整回测管线，支持 panic recover + ST/停牌/涨停过滤
- `ai_agent.go` — AI 代理：运行时审查候选，动态调整权重(±30%) + 最终确认/否决
- `position_manager_v2.go` — 持仓管理 V2：集成双模引擎

### 新增数据模型

- `condition_template.go` — 复合条件模板（系统预置 MACD 金叉/RSI 超卖/趋势突破/放量滞涨/均线死叉 5 模板）
- `ai_agent_decision.go` — AI 代理决策记录（市场分/偏好乘数/候选数/推理过程/操作清单）
- `strategy.go` — 新增 orchestrat_mode/enable_market_context/enable_ai_agent/industry_filter 等 9 字段
- `strategy_condition.go` — 新增 weight/fuzzy_sigma/lookback_days/industry_relative/timeframe/parent_id 等 10 字段

### P0 优化修复

- **评分区分度修复** — fuzzy sigma 默认值 0.30→1.0，minScore 从硬编码 0.18 → 动态中位数+top1 差值自适应
- **ST/停牌过滤强化** — 从名称检查升级为 stock_profiles 表标记查询 + 涨停过滤(≥9.8%)
- **ContextManager 修复** — 修复 sector_diffusion_score 列缺失(872 次 error)，增加优雅降级 + 5 分钟缓存
- **Nil Pointer Panic** — task_service 增加 nil guard，回测 goroutine 增加 defer/recover

### 后端端点新增

- `GET/PUT /strategies/:id/orchestration` — 策略编排配置
- `GET /strategies/templates` — 系统+用户条件模板列表
- `POST /strategies/templates` — 从条件创建模板
- `GET /strategies/:id/ai-decisions` — AI 代理决策历史

### 前端 UI 新增

- StrategyPage 编排 Tab 新增「政策决策」卡片：模式选择(rule/ai_driven/manual) + 阈值滑块
- 条件卡片显示 G1/G2/G3 分组标记替代 IF/AND
- 策略模板选择器 + 一键应用

### 回测结果 (S72 趋势追踪, 2026-01-01→2026-06-10)

| 版本 | 收益率 | 买入 | 加仓 | 止损 |
|------|--------|------|------|------|
| AND 原始 | +2.14% | 82 | 0 | 31 |
| OR + Policy | -8.82% | 19 | 68 | 4 |

策略分布 54 交易日：空仓 36(68%) / 进攻 10(19%) / 防御 7(13%)



## v1.5.0 (2026-06-21)


### 行情中心卡片增强 (2026-06-21)

- **指数卡片丰富** — 新增今开/最高/最低价展示（从K线DB读取），按板块汇总真实成交额（个股amount求和）
- **板块涨跌统计** — 每个指数卡片下方显示对应市场的涨跌家数 + 比例条
  - 上证 = 沪市主板(60xxxx) + 科创板(688xxx)
  - 深证 = 深市主板(00xxxx) + 创业板(30xxxx)
  - 创业板 = 创业板(30xxxx) 单独统计
- **卡片布局优化** — 三指数等宽卡片 + 两市成交额大卡片 + 涨跌统计 + 涨跌停，层次分明
- **性能优化** — 板块UD查询从关联子查询(8.8s)优化为固定日期等值JOIN(0.34s)
- **指数接口增强** — `/api/v1/indices` 返回high/low/open/amount；`/api/v1/stocks/market-snapshot` 新增shAmount/szAmount/cyAmount/kzAmount/bjAmount及各板块涨跌统计
### 行情中心 — 股票列表页重构

- **市场快照卡片** — 顶部实时指数行情（上证/深证/创业板）+ 涨跌家数统计 + 涨跌停计数 + 两市成交额（较上一日变化），10s 轮询刷新
- **板块分类 Tab** — 全部 / 沪深主板 / 创业板 / 科创板 / 北交所 / ETF国债，Tab 显示各板块股票数量 badge
- **排行切换** — 涨幅榜 / 跌幅榜 / 成交额榜 / 换手率榜 / 异动监控，多维度快速定位
- **增强表格** — 现价 / 涨跌幅红绿着色 / 成交额 / 换手率 / 行业，支持排序和分页
- **异动标记** — 放量（量>20日均量×2）/ 急涨急跌（按板块阈值）/ 高振幅（>10%）自动标注彩色 Tag

### 后端 API 新增

- `GET /api/v1/stocks/market-snapshot` — 全市场快照（涨跌家数/涨停数/成交额/情绪分）
- `GET /api/v1/stocks/ranking` — 涨跌排行（支持 boardType/sortBy/asc/limit）
- `GET /api/v1/stocks/unusual` — 异动监控（放量/急涨跌/高振幅自动识别）
- `GET /api/v1/stocks/board-type-counts` — 各板块股票数量统计
- 股票列表 API 增强：`boardType` / `sortBy` / `sortDir` 参数，JOIN K线获取实时涨跌幅

### 性能优化

- 股票列表查询从关联子查询 O(n²) 优化为固定日期等值 JOIN O(n)
- `prev_k` 从 DISTINCT ON 全表扫描改为主键等值查询

### 数据修复

- v025 migration：ETF/国债 board_type 回填 + stocks_basic 补充债券 ETF（511010/511090/511520）
- 修复 market-snapshot SQL 引用 market_sentiment.limit_up_count（原错误引用 market_daily_agg）

## v1.4.0 (2026-06-19)

### AI 对话

- **get_shareholders 中文标签** — ai_service.go tool switch 补全 `get_shareholders → 股东户数查询`，修复 Agent 工具调用显示英文
- **get_shareholders 图标** — 前端 iconMap 新增 `👥` 图标，修复工具卡片缺图标
- **对话样式优化** — 工具状态卡片渐变背景+柔色边框；消息头像 Sparkles 图标+渐变+发光；用户气泡蓝紫渐变；空状态 Brain 图标居中卡片；进度条多彩流动动画

### AI 策略生成

- **JSON 截断修复** — 新增 `ChatCompletionWithTokens`，策略生成 max_tokens 4096（原 2048），解决 "unexpected end of JSON input"
- **前端 30s 超时** — `aiGenerateStrategy` axios timeout 120s（原全局 30s），匹配后端耗时
- **条件消失修复** — 保存后直接 `fetchStrategyConditions` 重载，不再间接触发；空 `catch` → 具体错误 toast
- **提示词优化** — 限制最多 6 条条件，三级标注指标覆盖度（✅全量/⚠️窄/🚫不可回测），引导优先用 ma_/rsi/kdj/macd

### 数据刷新

- **股东/财务/资讯刷新不更新** — 后端新增 `CollectStockPhaseSSE` SSE 端点（GET `/collector/stock/:code/:phase`），同步执行替代 `go func()` 异步；前端改用 EventSource 实时采集日志，完成后自动 reload
- **研报覆盖仅 2359 只** — report_collect.py 去除 `min(200, ...)` 200 页硬上限，支持全量 42000+ 篇研报拉取

### AI 花费明细

- **统一底层重构** — ai_service.go 抽取 `doChatCompletion` / `doChatCompletionStream` / `doChatCompletionRaw` 三个公共函数，6 个公开方法改为薄壳；每次调用自动解析 `usage` 字段并异步写入 `ai_cost_logs`
- **花费记录** — 新增 MySQL 表 `ai_cost_logs`（用户/模块/模型/Token/缓存命中/费用/耗时/状态）+ `model_prices`（DeepSeek 官方价格种子：V4 Flash/Chat/R1 + Moonshot 三档）
- **管理端页面** — AdminPage 新增「AI 花费明细」Tab：汇总卡片（总费用/今日/本月/调用次数）+ 筛选（日期/用户/模块）+ 明细表格 + 价格管理面板
- **费用公式** — 按 DeepSeek 官方 `(cache_miss×输入价 + cache_hit×缓存价 + completion×输出价) / 1e6`

### Agent 模型配置

- **独立 Agent 模型** — model/handler/service 三层新增 `AgentModelName` / `AgentBaseURL` / `AgentAPIKey`，chat_analysis 场景可选配 Kimi 等独立模型用于工具调用模式
- **前端配置入口** — SettingsPage 系统配置弹窗新增「工具模式专用模型」区域（条件显示，enableTools=true 时展开）

### 系统提示词

- **模板变量迁移** — 全部提示词从 `%s` 位置占位符升级为 `__VAR__` 命名模板变量；新增 `ScenePromptVars` 映射（5 个场景各含可用变量）+ `renderPrompt()` 替换引擎
- **前端变量提示** — SettingsPage 提示词编辑弹窗新增「可用模板变量」标签栏（按场景显示，点击插入变量）；标签文字更新为「模板变量格式：__变量名__」
- **API 端点** — 新增 `GET /api/v1/ai/system-config-vars` 返回所有场景的可用变量定义
- **DB 迁移 v023** — 自动转换 `ai_system_configs` 表中已存在的 `%s` 占位符为对应 `__VAR__`
- **种子数据同步** — v006/v022 迁移的默认提示词全部改为 `__VAR__` 格式

### AI 策略生成（指标参考升级）

- **富指标参考表** — `buildIndicatorReference()` 替代旧压缩格式，输出按分类的结构化 Markdown 表格：字段名/名称/类型/可用操作符/值域/用途/说明（共 80+ 指标，12 个分类）
- **值域提示** — 新增 `buildValueRangeHint()`，每个指标精确值域（如 RSI `0-100（>70超买/<30超卖）`、pe `倍（>0，<20低估）`）
- **条件构建规范** — `indicatorRules` 全面升级：字段映射规则、类型特殊规则（cross/评分/百分号/元单位）、数量与组织规则、完整 JSON 输出示例
- **指标查询函数** — 新增 `GetIndicatorList()` 可复用函数，返回排序后的全部指标元数据
- **DB seed 对齐** — v022 `strategy_gen` seed 精简为与 handler 兼容格式


### 用户 AI 调用分析

- **用户级接口** — 新增 `GET /api/v1/cost/logs` / `summary` / `daily` 三个用户级接口（从 JWT 取 userId，非 admin）
- **汇总卡片** — UserCostPage 顶部 5 张卡片：总花费 / 总 Token / 今日花费 / 今日 Token / 模型调用次数
- **每日柱状图** — ECharts 堆叠柱状图，按模块分色（chat/stock_score/stock_profile/strategy_gen/strategy_opt），支持切换月份
- **调用明细** — 分页表格（时间/模块/模型/Token/费用/耗时/状态）+ 模块筛选下拉
- **用户下拉菜单** — AppTopbar 用户头像改为 Dropdown（个人设置 + AI 调用分析）
- **页面美化** — 统一 lucide-react 图标（DollarSign/Coins/Calendar/Clock/Cpu/TrendingUp/BarChart3/ListFilter），渐变标题栏，卡片 hover 阴影，表格行 hover 高亮，Badge 半透明彩色背景，等宽字体数据
- **图表全月刻度** — 柱状图横坐标填充当月所有日期（即使无数据也显示），空数据时显示灰色空柱
- **AGENTS.md 规范** — 新增第8条「前端 UI 风格规范」：图标/颜色/卡片/表格/字体约束，禁止 emoji 图标和硬编码颜色


## v1.3.3 (2026-06-18)

### 前端
- **股票简介切换刷新** — StockDetailPage 切换股票时 reset profileData 为 null，修复新股票无简介时仍显示旧数据的 bug
- **AI简介导入代码修复** — UploadProfile 剥离 raw_code 的 sh/sz/bj 前缀再取6位代码，修复 "sh600052" 被截断为 "sh6000" 的bug
- **大文件导入 413 修复** — Nginx `client_max_body_size` 提升到 100m，Gin `MaxMultipartMemory` 提升到 100MB
- **AI简介导入记录缺失** — UploadProfile 新增 ImportLog 写入 MySQL，修复导入后历史记录为空

### AI 对话
- **Markdown 表格渲染** — 两个 AI 对话 ReactMarkdown 实例补全 `remarkGfm` 插件 + table/thead/tbody/th/td 组件样式，修复 GFM 表格无法显示
- **持仓按用户过滤** — `get_my_holdings` 工具透传 userId 至 MySQL 查询，`WHERE user_id = ?` 防止多用户数据混淆

### 数据采集
- **list 型 API 响应保护** — `fetch_kline` 增加 `isinstance(data/list, list)` 守卫，防止腾讯接口异常返回导致 `.get()` 崩溃

### 数据采集
- **采集控制台进度条修复** — 修复进度条始终 0/1 问题：前端 phaseProgress 轮询同步不再要求 phaseCurrent 非 undefined
- **采集日志优化** — batch_collect.py 每批次输出关键行为日志："通过腾讯获取 0618 日K线 122 条，新入库 22 只"
- **STAT 行为统计** — 新增 `STAT:kline_fetched=X,kline_new=Y` 行，Go 引擎自动累积到 behaviorStats
- **NULL trade_date 崩溃修复** — stocks_daily_indicator 写入时先检查 MAX(trade_date) 再插入，避免 TimescaleDB NotNullViolation
- **采集控制台刷新重连** — 自动重连时 phaseResults 按 phase 去重，修复刷新后双统计面板
- **实时行情监控** — 新增 quote 阶段，交易时段每 5 分钟自动采集自选+持仓+榜单股票行情
- **realtime_quotes.py** — 新增 `--all` 监控模式，支持无用户上下文的定时任务调用

### 概念板块
- **成分股数据填充** — 新增 `populate_concept_stocks_sina.py`，从新浪 API 填充 175 个概念板块共 8,090 条成分股记录
- **stock_count 修正** — 从新浪 API 恢复准确板块股票数（此前均为 0）
- **概念空数据提示** — ConceptBoardStocks handler 在 stock_concepts 为空时返回 emptyReason
- **乱码修复** — `gn_kdts`（宽带提速）、`gn_wxdh`（卫星导航）名称编码修复
- **populate_concept_stocks.py** — 增加 3 次指数退避重试（东方财富 API 备用）

### 后端
- engine.go 新增 `runQuotePhase` + `runPythonStreamWithArgs` SSE 进度推送
- task_service.go 新增 `{"实时行情监控", "quote", "0 */5 9-15 * * 1-5"}` 定时任务
- board_handler.go 空概念处理 + emptyReason 字段

### 数据模型
- stock_realtime_quote 表新增，存储实时行情快照
- concept_boards.stock_count 从 0 恢复为新浪 API 准确值


## v1.3.2 (2026-06-18)

### AI 简介美化
- 安装 `remark-gfm` 插件，修复 Markdown 表格无法渲染的问题
- SectionCards 组件增强：卡片阴影 + 圆角 + 渐变色标题 + 彩色圆点装饰 + 鼠标悬浮上浮效果
- 表格：带圆角容器包裹，渐变色表头，行间交替底色
- strong/blockquote/code/pre/hr 全面美化，h1-h4 分级样式
- AI 简介外层卡片：紫色阴影 + 圆角，标题栏渐变色背景 + 图标 badge
- AI 更新按钮升级为紫色渐变填充按钮，带悬浮阴影动画和 toast 反馈
- 简介卡片增加 marginTop 50px

### 股票详情页
- **预测曲线修复** — 只展示 `predictDate > 最新K线日期` 的未来预测，已过期预测不再重复绘制
- **预测元数据栏** — K 线图下方显示：预测区间/生成日期/已验证天数（绿色 badge）
- **修复数据功能** — 新增 `repair_kline.py` 脚本，删除历史 K 线+指标 → 重采前复权数据 → 重算 PE/PB/PS
- 后端 API `POST /api/v1/stocks/:code/repair`，前端橙色 Wrench 按钮触发
- 修复包含成交量、成交额、换手率（流通股本优先从实时行情 API 获取）
- **换手率存储统一** — 从百分比（0.68）改为小数（0.0068），前端 `*100` 显示，修复 44 万条历史数据

### 右侧面板
- **历史榜单** — 日期选择改为 `/api/v1/board/dates` 全量榜单日期（不再限制当前股票上榜日期）
- 修复日期格式（`TO_CHAR` → `pick_date::text`）
- **新增自选股 Tab** — 分组筛选下拉 + 最新价/添加后收益率 + 加入/移出按钮
- **新增持仓股 Tab** — 汇总栏（持仓数量 + 总盈亏）+ 每只持仓三行信息（名称/现价/股数/成本/市值/盈亏百分比+金额）

### 后端
- BoardHandler.Dates 修复：日期 SQL 改为 `pick_date::text` 兼容 GORM Scan
- StockHandler.RepairKLine 新增异步修复端点
- Collector.RepairStock 新增

### 前端
- BoardSidebar 重写为三 Tab 布局（历史榜单/自选股/持仓股）
- StockDetailPage 新增 repairLoading 状态 + 修复按钮 + 预测元数据
- api.ts 新增 repairStock / searchStock 接口

## v1.3.0 (2026-06-13)
### AI 对话

## v1.3.1 (2026-06-13)
### 策略PK
- 修复报名按钮在首次报名后消失的问题，支持同一用户多策略报名同一活动
- 后端同步修复：报名校验改为策略级去重（仅禁止同一策略重复报名同一活动）

- **Agent 工具调用** — AI 对话支持自动调用数据分析工具（行情查询/K线分析/技术指标/财务数据/资讯检索）
- 工具调用结果实时流式反馈，多工具并行执行统一展示
- 支持多轮分析（最多 5 轮），AI 可基于工具返回数据迭代深化分析

### AI 对话优化


- 思考进度重新设计为整行横条（状态行 + 工具标签流 + 进度条）
- 工具并行执行时统一展示不再上下堆叠
- 思考过程增加实时耗时计时器
- 前端流式输出 rAF 缓冲渲染，消除卡顿和样式闪烁
- 修复 plan/summary widget 在消息末尾不渲染的问题



### 回测模块

- **最后交易日强制清仓** — 回测最后交易日以收盘价卖出全部持仓，不执行/生成买入信号
- **当日卖出持仓展示** — 实时持仓快照保留当日已卖出股票（半透明 + 已卖出标记 + 盈亏金额）
- **收益分析 Tab** — 回测详情新增按股票汇总收益表格，支持列排序（盈亏/收益率/买卖次数）
- **个股收益详情** — 点击股票查看 K 线图（买入红圈 B / 卖出绿圈 S，含价格和收益率）+ 交易记录表
- **修复** 回测完成时持仓快照显示第一日数据（limit=1 → 取最后一条快照）

### K 线图组件

- **买卖标记重构** — 买入标记在蜡烛下方（红色 B 圈），卖出在蜡烛上方（绿色 S 圈），天然错开不重叠
- **新增 sell 类型标记** — 之前缺失，统一为 board / buy / sell 三种
- **成本线** — 支持 `costLine` 属性，橙色水平虚线和标签
- 标记显示价格和盈亏百分比

### 股票详情

- 若持有该股票，K 线图自动显示持仓成本线

### 持股管理

- 所有数值列支持点击列头排序
- 股票名称可点击跳转详情页

### UI

- 回测 Tab 图标 Emoji → lucide-react（ClipboardList / FileSearch / PieChart / Wallet / TrendingUp / List）
- 去除冗余 emoji 前缀

### 基础设施

- 修复 `/tmp` 目录下二进制被 macOS 清理机制 SIGKILL 的问题
- 新增 `CHANGELOG.md`


## v1.2.0 (2026-06-10)

- 成交量单位修复（股/手统一转换）
- PE/PB 指标回填脚本 + 策略代码修复
- 新增换手率/成交额/量比/振幅等指标采集
- 文件导入大文件超时优化
- 数据库迁移文件规范化


## v1.1.0 (2026-06-05)

- 暗色主题全面适配（CSS 变量替换硬编码颜色）
- AI 分析提示词系统配置化
- PK 活动领奖台优化（前三名金/银/铜卡片）
- 前端混合格式 AI 对话渲染
- 历史对话上下文支持


## v1.0.0

- 初始版本
- 策略回测引擎 + 信号层重构
- 股票数据采集（行情/K线/指标/财务/研报）
- AI 对话分析
- PK 活动系统
- 数据管理（导入/导出）
