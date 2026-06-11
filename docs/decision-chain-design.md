# 交易策略决策链引擎 — 技术设计文档

> 版本: v1.0 | 日期: 2026-06-11 | 状态: 设计完成

---

## 1. 设计目标

### 1.1 现状问题

当前回测每日只做**一轮固定顺序的线性扫描**：

```
卖 → 减 → 买 → 加 (一轮游)
```

缺陷：

| 问题 | 影响 |
|---|---|
| 卖出后释放的现金不会重新触发买入 | 资金利用率低 |
| 不管仓位轻重永远先卖后买 | 轻仓时应该优先建仓 |
| 减仓/加仓不联动 | 刚卖出的股票不会因为条件满足而买回 |
| 无风险检查入口 | 大盘暴跌时仍在买入 |
| 不可扩展 | 新增决策节点要改主循环 |

### 1.2 目标

把每日执行建模为**有向决策图 + 循环评估**，模拟真实交易者的思维过程：

```
评估仓位 → 决定优先级 → 执行一个动作 → 重新评估 → ... → 无动作可执行
```

---

## 2. 核心架构

### 2.1 决策链主循环

```
每日开始
   │
   ▼
┌─────────────────────────────────────────────┐
│  while (hasActions) && (iter < maxIter):    │
│                                              │
│    1. Assess() — 评估当前仓位状态             │
│    2. BuildQueue() — 按优先级排动作队列       │
│    3. RiskCheck() — 风险钩子拦截             │
│    4. Execute() — 执行队列中最高优先级动作     │
│    5. ApplyChanges() — 合并状态变更          │
│    6. if 本轮无成交: continue (下一优先级)    │
│    7. if 有成交: re-assess (状态已变)        │
│                                              │
└─────────────────────────────────────────────┘
```

### 2.2 状态机

```
     ┌──────────┐
     │  Assess  │ ← 每次循环入口，读取当前 (cash, positions, universe)
     └────┬─────┘
          │
     ┌────▼────┐
     │ Priority │  仓位率>50% → 卖优先
     │  Queue   │  仓位率<30% → 买优先
     └────┬─────┘  仓位率30-50% → 均衡
          │
     ┌────▼─────────────────────────────────┐
     │  Execute Action                       │
     │  inputs:  cash, positions, universe   │
     │  output:  StateChange {               │
     │    CashDelta,                         │
     │    PositionsAdded/Removed/Updated,    │
     │    NewTrades, Logs                    │
     │  }                                    │
     └────┬─────────────────────────────────┘
          │
     ┌────▼────┐   hasChanges  ┌──────────┐
     │ changes?│──────────────▶│  Apply   │──→ Re-Assess
     └────┬────┘               │ Changes  │
          │ noChanges          └──────────┘
     ┌────▼────┐
     │  Skip   │ → 尝试下一优先级
     │ Action  │
     └─────────┘
```

---

## 3. 模块设计

### 3.1 PositionManager — 仓位调度器

```go
type PositionManager struct {
    capital        float64
    maxHoldings    int
    buyPct         float64       // 初始买入仓位%
    addPct         float64       // 加仓仓位%
    reducePct      float64       // 减仓比例%
    stopProfit     float64       // 止盈%
    stopLoss       float64       // 止损% (负数)
    priorityRules  []PriorityRule // 可扩展优先级规则
    riskHooks      []RiskHook     // 可扩展风险钩子
}
```

**核心方法**:

| 方法 | 输入 | 输出 | 说明 |
|---|---|---|---|
| `Assess()` | cash, positions, universe, date, conds | `*DayAssessment` | 评估仓位率/候选标的/止损触发 |
| `BuildQueue()` | `*DayAssessment` | `[]ActionNode` | 按仓位率动态排序动作 |
| `ExecuteStop()` | action, cash, positions | StateChange | 执行止损/止盈 |
| `ExecuteSell()` | action, cash, positions | StateChange | 执行卖出 |
| `ExecuteReduce()` | action, cash, positions | StateChange | 执行减仓 |
| `ExecuteBuy()` | action, cash, positions, universe | StateChange | 执行买入 + 仓位分配 |
| `ExecuteAdd()` | action, cash, positions | StateChange | 执行加仓 |

### 3.2 优先级规则表

| 仓位率 | 类型 | P1 | P2 | P3 | P4 | P5 |
|---|---|---|---|---|---|---|
| >50% | 重仓 | 止损止盈 | 卖出 | 减仓 | 买入 | 加仓 |
| 30-50% | 中性 | 止损止盈 | 卖出 | 买入 | 加仓 | 减仓 |
| <30% | 轻仓 | 止损止盈 | 买入 | 加仓 | 卖出 | 减仓 |

**核心原则**: 止损止盈永远是 P1，仓位越重越优先减，仓位越轻越优先建。

### 3.3 DecisionNode 接口

```go
type ActionType string
const (
    ActionStop   ActionType = "stop"    // 止损止盈
    ActionSell   ActionType = "sell"    // 卖出条件
    ActionReduce ActionType = "reduce"  // 减仓
    ActionBuy    ActionType = "buy"     // 买入
    ActionAdd    ActionType = "add"     // 加仓
)

type ActionNode struct {
    Type     ActionType
    Priority int              // 越小越优先
    Targets  []ActionTarget   // 候选标的
}

type ActionTarget struct {
    Code       string
    Name       string
    CurrentMV  float64       // 当前市值
    CurrentQty int
    Price      float64
    Reason     string
}
```

### 3.4 PositionSizer — 动态仓位分配

**三种模式**:

| 模式 | 计算方式 | 适用场景 |
|---|---|---|
| `fixed_pct` | `剩余现金 × buyPct%` | 当前逻辑，简单直观 |
| `equal_weight` | `剩余现金 / 剩余槽位数` | 等权建仓，分散风险 |
| `kelly` | Kelly公式自适应 | 根据历史胜率/赔率动态调整 |

**Kelly公式**:

```
f* = (p × b - q) / b

p = 胜率 (从历史交易统计)
b = 赔率 = 平均盈利 / 平均亏损
q = 1 - p

实际仓位 = f* × 保守系数(0.25)
         = clamp(5%, 25%)
```

### 3.5 RiskHook — 风险检查（v2）

```go
type RiskHook interface {
    Name() string
    BeforeAction(action ActionNode, a *DayAssessment) string
    // 返回 "" = 通过, 非空 = 跳过原因
}
```

**内置钩子** (v2 阶段):

| 钩子 | 阻断范围 | 逻辑 |
|---|---|---|
| `MarketWideStop` | 买入/加仓 | 大盘指数跌超阈值(如-5%)暂停买入 |
| `ConcentrationLimit` | 买入/加仓 | 单票占总权益超上限(如30%)跳过该票 |
| `DailyTradeLimit` | 卖出/买入/加减仓 | 日内交易次数达上限停止交易 |

**关键原则**: 风险钩子永远不阻断止损/止盈。

---

## 4. 边界条件

| 场景 | 处理 |
|---|---|
| 现金=0 | 跳过买入/加仓，只评估卖出/止损 |
| 持仓=0 | 跳过卖出/减仓，只评估买入 |
| 达到 maxHold | 跳过买入，只评估加仓/卖出/减仓 |
| 价格=0 (停牌) | 跳过该股票，日志记录"停牌" |
| 买入金额 < 100股金额 | A股最小交易单位，跳过 |
| 止损+买入同时触发 | 止损先执行 → 释放现金 → 下一轮再买入 |
| 连续 N 轮无动作 | 退出决策链，记录"决策链收敛" |
| 最大迭代 10 次 | 安全阀，防止无限循环 |

---

## 5. 决策链日志

每天输出结构化决策日志，可审计可复盘：

```
[09:30] position_mgr 评估: 仓位率18% 现金82% 可用槽位2 风险=低
[09:30] position_mgr 决策: 轻仓→买入优先
[09:31] buy_scanner  扫描: 遍历4864只 条件[daily_change>=3, adx>=25]
[09:31] buy_scanner  600519 茅台 ¥1850.50 → ✓ daily_change=3.20 ✓ adx=28.5
[09:31] position_szr 分配: 600519 等权 ¥20500 (12.5%) 买1100股
[09:31] executor     买入: 600519 ¥1850.50×1100=¥20355 现金¥79645
[09:31] position_mgr 重新评估: 仓位率22% 现金78% 可用槽位1
[09:31] buy_scanner  扫描: 遍历4864只 → 无满足条件
[09:31] add_scanner  扫描: 已持仓1只 → 无满足条件
[09:31] system       决策链完成: 持仓1只 现金¥79645 权益¥99995
```

---

## 6. 扩展路线图

| 阶段 | 内容 |
|---|---|
| **v1** (立即) | 决策链主循环 + 仓位管理调度 + 动态优先级 + 结构化日志 |
| **v2** | PositionSizer 等权/Kelly + RiskHook 大盘熔断/集中度限制 |
| **v3** | 自适应参数 (从历史回测学习胜率/赔率) + 多策略PK框架 |
| **v4** | 实盘运行模式 (每日执行策略，查看实盘结果) |

---

## 7. 文件变更清单

| 文件 | 变更 |
|---|---|
| `server/internal/handler/strategy_handler.go` | 新增: `PositionManager`, `PositionSizer`, 重构 `runBacktestAsync` 日循环 |
| `server/internal/handler/decision_chain.go` | 新增: 决策链主循环、状态机、RiskHook 接口 |
| `server/internal/handler/position_sizer.go` | 新增: 三种仓位分配模式实现 |
| `server/internal/model/strategy.go` | 新增字段: `PositionSizing` (fixed/equal/kelly) |

---

## 8. 数据流总览

```
Strategy (策略参数)
   │
   ├─→ buyConds, sellConds, addConds, reduceConds
   ├─→ maxHoldings, buyPct, addPct, reducePct
   ├─→ stopProfit, stopLoss
   └─→ initialCapital, investmentType
        │
        ▼
   PositionManager.Assess()
        │
        ▼
   DayAssessment {
       positionRatio, availableSlots,
       stopTriggers, sellCandidates, buyCandidates...
   }
        │
        ▼
   PositionManager.BuildQueue()
        │
        ▼
   [ActionNode{Stop,P1}, ActionNode{Sell,P2}, ...]
        │
        ▼
   For each ActionNode (按优先级):
        │
        ├─→ RiskHook.BeforeAction()
        │        │
        │        ├─ pass → Execute() → StateChange
        │        └─ block → skip
        │
        └─→ Apply(StateChange) → Re-Assess
                 │
                 └─→ 有变化: 继续循环
                 └─→ 无变化: 试下一优先级
                          │
                          └─→ 全部试完无动作: 退出
```
