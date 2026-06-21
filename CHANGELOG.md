
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
