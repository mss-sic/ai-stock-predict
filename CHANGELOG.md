# Changelog

## v1.3.3 (2026-06-18)

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
