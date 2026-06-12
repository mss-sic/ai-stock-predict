# 发布日志

## v1.2.0 (2026-06-13)

### 数据层

- **成交量/成交额单位修复**：修正 Collector 从腾讯 API 获取数据时 volume 以手存储、amount 公式错误的问题。修复后 volume 统一为股，amount = close × volume。含 301 万行历史数据修复脚本 `scripts/collector/fix_volume_unit.py`
- **PE/PB/PS 数据回填**：财报覆盖从 230 只扩展到 530 只（含四大行等大市值股），运行 `backfill_indicator.py` 回填 2024-07 至今 PE/PB/PS/总市值数据（20.9 万条），表 `stocks_daily_indicator` 从 5 天扩展到 ~580 个交易日，覆盖 3,542 只股票
- **文件导入增强**：K 线 CSV 导入同步写入 `stocks_daily_indicator`（PE/PB/PS/市值），导入结果 API 新增 `importedIndicator` 字段
- **预测导入性能**：`/internal/predictions/sync` 从 71.7 万次逐条 `FirstOrCreate` 改为批量 `ON CONFLICT`，查询从 ~1,440,000 降至 ~370，耗时从数分钟→2-5 秒

### 数据采集

- `backfill_financial.py`：改为按市值优先采集，支持 limit 参数，已覆盖 530 只股票
- 4 个采集脚本 volume 单位修正：`batch_collect.py`、`daily_k.py`、`backfill_kline_all.py`、`backfill_one.py`（手×100→股，amount = close×vol）
- 新增 `fix_volume_unit.py`：批量修复 301 万行历史成交量/成交额数据

### 策略回测

- **新增指标**（全部 batch 预加载，回测零 N+1 查询）：
  - MACD_DIF / MACD_DEA（拆分缓存）
  - MA_5 / MA_10 / MA_20 / MA_30 / MA_60
  - RSI_6 / RSI_12 / RSI_24（多周期）
  - PSY_12 / PSY_MA（心理线）
  - 布林线上/中/下轨（`boll_upper` / `boll_middle` / `boll_lower`）
- **PE/PB 回测修复**：预加载中加入 PE/PB/PS/总市值 batch 查询，缺失数据返回 false 而非静默通过
- `IndicatorCache` 新增 `hasIndicatorData` 追踪字段，缺失股票明确跳过条件评估

### 前端

- 成交量显示统一：顶部统计 → 手，表格 → 万手，K 线悬浮 → 万手
- `StockDetailPage.tsx`：移除 amount 错误的 ×1e4，改为原值
- `KLineChart.tsx`：悬浮提示成交量单位修正
- AI 对话卡片样式优化（`AIAnalysisCard.tsx`）
- 设置页增加 Tab 分隔（`SettingsPage.tsx`）

### 概念采集

- `concept_collector.go`：成分股/板块写入从逐条 `FirstOrCreate` 改为 `CreateInBatches(500)` + `OnConflict`

### 新增文件

- `scripts/collector/fix_volume_unit.py` — 成交量历史修复脚本
- `server/internal/model/ai_system_config.go` — AI 系统配置模型
- `CHANGELOG.md` — 本文件

---

## v1.1.0 及之前

- 信号层重构（`BacktestSignal`，T 日收盘生成信号 → T+1 日开盘执行）
- 交易账户/持仓管理
- PK 活动系统（前三名金银铜牌展示、详情页 K 收益曲线）
- AI 对话分析（混合 JSON Widget 流式解析、系统提示词配置）
- 概念板块采集（东方财富 API）
- 文件导入（Excel + CSV）
