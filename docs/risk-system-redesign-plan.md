# 风控预警体系全面重建计划（最终版）

> 创建日期：2026-07-08
> 状态：Phase 1 实施中

---

## 一、数据模型（v0xx Migration）

### 1.1 `risk_alerts` 表变更

```sql
-- 新增字段
ALTER TABLE risk_alerts ADD COLUMN user_id INT DEFAULT 0;
ALTER TABLE risk_alerts ADD COLUMN strategy_id INT DEFAULT 0;
ALTER TABLE risk_alerts ADD COLUMN rule_key VARCHAR(50) DEFAULT '';
ALTER TABLE risk_alerts ADD COLUMN dimension VARCHAR(20) DEFAULT 'stock';
ALTER TABLE risk_alerts ADD COLUMN severity_score INT DEFAULT 0;
ALTER TABLE risk_alerts ADD COLUMN evidence JSON;
ALTER TABLE risk_alerts ADD COLUMN status VARCHAR(15) DEFAULT 'active';
ALTER TABLE risk_alerts ADD COLUMN acknowledged_at DATETIME;
ALTER TABLE risk_alerts ADD COLUMN resolved_at DATETIME;
ALTER TABLE risk_alerts ADD COLUMN resolution_note VARCHAR(200);
ALTER TABLE risk_alerts ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;

-- 数据迁移：ignored → status
UPDATE risk_alerts SET status = 'ignored' WHERE ignored = true AND status = 'active';

-- 新增索引
CREATE UNIQUE INDEX idx_alert_dedup ON risk_alerts(user_id, stock_code, rule_key, hit_date);
CREATE INDEX idx_alert_user_status ON risk_alerts(user_id, status);
CREATE INDEX idx_alert_rule_date ON risk_alerts(rule_key, hit_date);
```

**关键设计决策**：
- `user_id=0` → 市场级告警（全局可见），`stock_code='__MARKET__'` → 无个股关联
- `ignored` 字段保留不删（避免旧代码报错），但新逻辑全部走 `status`
- API 返回时映射 `status == 'ignored'` → `ignored: true`（前端向前兼容）

### 1.2 新建 `risk_rules`（MySQL）

| 字段 | 类型 | 说明 |
|------|------|------|
| `rule_key` | VARCHAR(50) PK | 唯一标识，如 `ma_bearish_alignment` |
| `name` | VARCHAR(100) | 中文名 |
| `dimension` | VARCHAR(20) | market/stock/portfolio/liquidity/event/operational/behavior |
| `default_level` | VARCHAR(10) | high/medium/low |
| `enabled` | BOOL | 默认 true |
| `thresholds` | JSON | 可配置参数 |
| `description` | TEXT | 规则说明 |
| `weight` | DECIMAL(4,3) | 评分权重，默认 0.1 |

### 1.3 新建 `risk_snapshots`（MySQL）

| 字段 | 类型 | 说明 |
|------|------|------|
| `snapshot_date` | DATE PK | |
| `market_risk_level` | VARCHAR(10) | low/medium/high/critical |
| `market_risk_score` | INT | 0-100 |
| `total_alerts_high` | INT | |
| `total_alerts_medium` | INT | |
| `total_alerts_low` | INT | |
| `dimension_breakdown` | JSON | `{"stock":12, "market":3, "portfolio":2}` |

---

## 二、34 条规则清单

### 市场风险（4条）
| # | rule_key | 级别 | 数据源 | 说明 |
|---|----------|------|--------|------|
| M1 | `fear_greed_overheat` | high | `market_sentiment` | 恐贪>80 连续3日 |
| M2 | `market_breadth_decay` | medium | `market_style_daily.ma20_above` | MA20以上占比<30% |
| M3 | `northbound_outflow_streak` | medium | `northbound_daily_view` | 连续5日净流出 |
| M4 | `volatility_spike` | medium | `stocks_daily_k` 聚合 | 全市场振幅中位数>历史90分位 |

### 个股风险（17条）
| S1 | `heavy_volume_drop` | high | K线+`volume_ratio` | 单日跌>5%且量比>2.0 |
| S2 | `shrinking_rebound` | medium | K线 | 反弹3日量能递减 |
| S3 | `ma_bearish_alignment` | medium | K线 | MA5<MA10<MA20<MA60 |
| S4 | `rsi_overbought` | medium | K线(Go计算) | RSI(14)>80 |
| S5 | `macd_divergence` | high | K线(Go计算) | 顶背离峰值检测 |
| S6 | `bollinger_squeeze` | low | K线(Go计算) | 带宽<历史20分位 |
| S7 | `turnover_abnormal` | medium | 日频指标 | 换手>20%或<0.1% |
| S8 | `major_outflow_streak` | medium | 资金流向 | 主力连续5日净流出 |
| S9 | `margin_collapse` | high | 融资融券 | 融资余额5日降幅>10% |
| S10 | `block_discount` | medium | 大宗交易 | 折价>8% |
| S11 | `dragon_institution_sell` | high | 龙虎榜 | 机构净卖出>买入2倍 |
| S12 | `st_delist_risk` | high | 基础信息 | ST/面值<1.5元 |
| S13 | `ai_score_crash` | medium | AI评分 | 3日降>2分 |
| S14 | `sharp_decline` | high | K线 | 5日跌>8%+量确认 |
| S15 | `ma20_breakdown` | medium | K线 | 下穿MA20+2%缓冲带 |
| S16 | `pe_extreme` | high | 日频指标 | PE>200+多维判断 |
| S17 | `profit_decline` | high | 财务 | 净利同比<-50% |

### 组合风险（4条）
| P1 | `industry_concentration` | medium | 持仓 | 单行业>40% |
| P2 | `correlation_high` | medium | 持仓(Go计算) | 相关系数均值>0.7 |
| P3 | `var_breach` | high | 持仓(Go计算) | 95%VaR>5% |
| P4 | `position_overlimit` | high | 持仓 | 总仓位>策略上限 |

### 流动性风险（3条）
| L1 | `volume_too_low` | medium | K线 | 日均成交<2000万 |
| L2 | `limit_down_locked` | high | K线 | 跌停未开板 |
| L3 | `turnover_decay` | low | 日频指标 | 30日换手衰减<0.5% |

### 信用/事件风险（3条）
| E1 | `major_reduction` | high | 公告标题 | 关键词减持/转让 |
| E2 | `litigation_violation` | high | 公告标题 | 诉讼/违规/处罚 |
| E3 | `dividend_ex_near` | low | 分红 | 5日内除权 |

### 操作/行为风险（3条）
| B1 | `overtrading` | low | 交易记录 | 日交易>5笔 |
| B2 | `stop_loss_missed` | high | 持仓 | 亏损超止损未卖 |
| B3 | `live_backtest_divergence` | medium | 快照vs回测 | 偏离>15% |

---

## 三、实施顺序与测试标准

### Phase 1：数据底座（迁移+框架骨架）
- v0xx迁移 + 34条seed + RiskEngine框架 + 旧ignored→新status
- 测试：编译通过、迁移无数据丢失、UNIQUE索引正常、旧API兼容

### Phase 2：市场+个股核心规则（12条SQL）
- M1-M4, S1/S3/S7/S8/S9/S10/S12/S14
- 测试：单元测试、去重验证、阈值热更新、启用/禁用

### Phase 3：复杂计算规则（8条Go+SQL）
- S2/S4/S5/S6, P1-P4, L1-L3
- 测试：RSI/MACD/布林带与pandas对比、数据不足降级、退市auto-resolve

### Phase 4：事件+行为规则（6条）+ 新旧并行
- E1-E3, B1-B3 + 3天并行验证
- 测试：关键词召回率≥70%、并行输出差异日志

### Phase 5：调度集成
- 移除独立cron、AfterClose/PreMarket新增stage、盘中轻量熔断
- 测试：盘后自动生成snapshot、非交易日跳过、上游失败降级

### Phase 6：API层
- 新增8个端点 + 保留3个旧端点兼容
- 测试：200全部、旧RiskPage不报错、参数校验

### Phase 7：通知推送
- Webhook分发+频率限制+重试+夜间静默
- 测试：全通道推送、50条合并摘要、失败重试、夜间延迟

### Phase 8：前端
- RiskDashboard + RiskRules + 通知设置
- 测试：加载无报错、规则编辑保存、build通过

### Phase 9：回测验证
- 每条规则独立回测 + 有效性排名
- 测试：收益差/回撤差/胜率差、误报率标记

---

## 四、冲突兼容清单

| 保留（不改） | 策略 |
|-------------|------|
| `GET /api/v1/risks` | 改为 `WHERE status='active'`，返回格式不变 |
| `PUT /api/v1/risks/:id/ignore` | 改为设 `status='ignored'` |
| `POST /api/v1/admin/risks/scan` | 调用新引擎 |
| 前端 RiskPage.tsx | 零改动 |
| 前端 api.ts 3个函数 | 保留，新增独立函数 |
| `ignored` 字段 | 保留但废弃，API 映射兼容 |
| 旧 pipeline handler | 函数签名不变，内部重定向 |

---

## 五、通知渠道设计

```
钉钉机器人: Markdown + @所有人(high级)
飞书机器人: Interactive Card
企业微信:   Markdown

频率限制: 每通道5min≤3条, critical例外
夜间静默: 22:00-07:00, critical例外
失败重试: 10min间隔, 最多3次
```
