# 智策投研 — Agent 交易接口文档 v2.0

> 面向本地 Agent 自动交易的完整 REST + WebSocket API 参考  
> 更新日期：2026-07-10｜适配架构重构后的 strategy_runs 资金模型

---

## 目录

1. [认证机制](#1-认证机制)
2. [信号生命周期](#2-信号生命周期)
3. [REST API 接口](#3-rest-api-接口)
   - [3.1 获取待执行信号](#31-获取待执行信号)
   - [3.2 认领信号](#32-认领信号)
   - [3.3 上报执行结果](#33-上报执行结果)
   - [3.4 获取信号详情](#34-获取信号详情)
   - [3.5 获取账户信息](#35-获取账户信息)
   - [3.6 获取账户概览](#36-获取账户概览)
   - [3.7 同步持仓到服务端](#37-同步持仓到服务端)
   - [3.8 同步委托/订单到服务端](#38-同步委托订单到服务端)
   - [3.9 指令回传](#39-指令回传)
   - [3.10 连通性测试](#310-连通性测试)
   - [3.11 检查 Agent 状态](#311-检查-agent-状态)
4. [WebSocket 协议](#4-websocket-协议)
5. [交易执行流程](#5-交易执行流程)
6. [架构重构后已知问题与修复建议](#6-架构重构后已知问题与修复建议)

---

## 1. 认证机制

所有 Agent API 均通过 **Agent Token** 认证。Token 在服务端账户管理页生成：

```
方式 A（推荐）：HTTP Header
  X-Agent-Token: tk_xxxxxxxxxxxxxxxx

方式 B（兼容）：Query Parameter
  /api/v1/live/pending-auto-signals?token=tk_xxxxxxxxxxxxxxxx
```

Token 与服务端的 `trading_accounts.agent_token` 字段匹配，认证通过后自动确定 Agent 所绑定的资金账户。

**Token 生命周期：**
- 生成：`POST /api/v1/live/accounts/:id/generate-agent-token`
- 吊销：`DELETE /api/v1/live/accounts/:id/agent-token`
- Token 吊销后，已连接的 Agent 会被 WS 服务端踢下线

---

## 2. 信号生命周期

```
评分引擎出候选 → trade_exec 生成信号 → 设置 pending_auto
                        │
                        ▼
         Agent 通过 WS push / HTTP poll 感知
                        │
                        ▼
              Agent claim 信号 (pending_auto → claimed)
                        │
                        ▼
              Agent 实际执行下单 (券商 API/UI 自动化)
                        │
              ┌─────────┴──────────┐
              ▼                    ▼
         成功 executed         失败 order_failed
              │                    │
              ▼                    ▼
     记录 LiveTrade          重置为 pending_auto
     更新 run 资金/持仓         Agent 可重试
```

**状态枚举：**

| 状态 | 含义 | 谁触发 |
|------|------|--------|
| `pending_auto` | 待 Agent 自动执行 | 服务端 trade_exec |
| `claimed` | 已被 Agent 认领 | Agent claim API |
| `executed` | 执行成功 | Agent report_result |
| `order_failed` | 下单失败，已重置 | Agent report_result |

---

## 3. REST API 接口

### 3.1 获取待执行信号

> 获取当前账户下所有活跃策略运行中、执行日期为今天的待执行信号。

```
GET /api/v1/live/pending-auto-signals?token=tk_xxxx
```

**请求参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `token` | string | 是 | Agent Token（或 Header `X-Agent-Token`） |

**响应结构：**
```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "signalId": 1001,
      "runId": 3,
      "stockCode": "600519",
      "stockName": "贵州茅台",
      "actionType": "buy",
      "orderPrice": 1650.00,
      "plannedQty": 100,
      "plannedAmount": 165000.00,
      "execDate": "2026-07-10",
      "status": "pending_auto",
      "reason": "AI评分0.82，突破20日均线",
      "createdAt": "2026-07-10 09:15:00"
    }
  ]
}
```

**响应字段说明：**
| 字段 | 类型 | 说明 |
|------|------|------|
| `signalId` | uint | 信号唯一 ID |
| `runId` | uint | 所属策略运行 ID |
| `stockCode` | string | 股票代码 |
| `stockName` | string | 股票名称 |
| `actionType` | string | buy / add / sell / reduce / stop |
| `orderPrice` | float64 | 建议下单价格 |
| `plannedQty` | int | 计划数量（股） |
| `plannedAmount` | float64 | 计划金额（元） |
| `execDate` | string | 执行日期 YYYY-MM-DD |
| `status` | string | 当前状态 |
| `reason` | string | 信号生成原因 |
| `createdAt` | string | 创建时间 |


---

### 3.2 认领信号

> 将信号状态从 `pending_auto` 变更为 `claimed`，防止被重复处理。

```
POST /api/v1/live/signals/{signalId}/claim?token=tk_xxxx
```

**路径参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| `signalId` | uint | 信号 ID |

**成功响应 (200):**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "claimed": true,
    "signalId": 1001,
    "stockCode": "600519",
    "stockName": "贵州茅台",
    "action": "buy",
    "price": 1650.00,
    "quantity": 100,
    "amount": 165000.00
  }
}
```

**失败响应 (409):**
```json
{
  "error": "signal status is not pending_auto",
  "currentStatus": "claimed"
}
```

> 使用 `WHERE status = 'pending_auto'` 条件更新，天然防并发。RowsAffected=0 表示已被其他 Agent 认领。

---

### 3.3 上报执行结果

> Agent 完成下单后，上报执行结果。服务端更新信号状态、记录实盘交易、更新策略资金。

```
POST /api/v1/live/signals/{signalId}/report-result?token=tk_xxxx
```

**请求体：**
```json
{
  "status": "executed",
  "orderId": "ORD20260710001",
  "errorMsg": "",
  "execPrice": 1649.50,
  "execQty": 100
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `status` | string | 是 | `"executed"` 或 `"order_failed"` |
| `orderId` | string | 否 | 券商返回的委托单号 |
| `errorMsg` | string | 否 | 失败时的错误描述 |
| `execPrice` | float64 | 否 | 实际成交价格 |
| `execQty` | int | 否 | 实际成交数量 |

**成功响应:**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "reported": true,
    "signalId": 1001,
    "status": "executed"
  }
}
```

**失败响应 (409):**
```json
{
  "error": "signal is not in claimed state",
  "currentStatus": "pending_auto"
}
```

**order_failed 行为：**
- 信号状态重置为 `pending_auto`，Agent 可重新认领执行
- 不会写入 LiveTrade 记录

**executed 行为（当前实现）：**
- 信号状态更新为 `executed`
- 写入 `live_trades` 表记录
- ⚠️ **缺失**: 当前未调用 `executeSignal` 更新 `strategy_runs.available_cash` 和 `strategy_runs.position_value`

---

### 3.4 获取信号详情

> 获取单个信号的完整信息，包含关联的策略运行名称。

```
GET /api/v1/live/signals/{signalId}/detail?token=tk_xxxx
```

**响应结构:**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 1001,
    "runId": 3,
    "runName": "龙虾多因子策略",
    "stockCode": "600519",
    "stockName": "贵州茅台",
    "actionType": "buy",
    "orderPrice": 1650.00,
    "plannedQty": 100,
    "plannedAmount": 165000.00,
    "execDate": "2026-07-10",
    "status": "claimed",
    "reason": "AI评分0.82，突破20日均线",
    "brokerOrderId": "",
    "execPrice": 0,
    "execQty": 0,
    "suggestedPremium": 0,
    "orderPriceLimit": 0,
    "createdAt": "2026-07-10T09:15:00+08:00",
    "updatedAt": "2026-07-10T09:16:00+08:00"
  }
}
```

---

### 3.5 获取账户信息

> 获取 Agent 所绑定的完整账户信息。

```
GET /api/v1/live/agent/account?token=tk_xxxx
```

**响应结构:**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 5,
    "name": "东方财富模拟-龙虾交易",
    "broker": "东方财富",
    "accountType": "simulated",
    "accountNumber": "智投测试ljb",
    "status": "active",
    "brokerMode": "lobster",
    "initialCapital": 1000000.00,
    "availableCash": 475096.60,
    "frozenCash": 2360.04,
    "totalAssets": 475096.60,
    "totalMarketValue": 0,
    "totalProfit": -524903.40,
    "nav": 0.4751
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint | 账户 ID |
| `name` | string | 账户名称 |
| `broker` | string | 券商名称 |
| `accountType` | string | `real` / `simulated` |
| `accountNumber` | string | 账号编号 |
| `status` | string | `active` / `archived` |
| `brokerMode` | string | `manual` / `mx_moni` / `lobster` |
| `initialCapital` | float64 | 初始资金 |
| `availableCash` | float64 | 可用现金（⚠️ 这是账户级别，包含所有策略已分配资金） |
| `frozenCash` | float64 | 冻结资金 |
| `totalAssets` | float64 | 总资产 |
| `totalMarketValue` | float64 | 持仓总市值 |
| `totalProfit` | float64 | 累计盈亏 |
| `nav` | float64 | 净值 |

> ⚠️ **重要**: `availableCash` 是账户级别的现金，**不等于** Agent 可用的策略资金。
> 策略资金见 [3.7 获取策略运行信息](#37-获取策略运行信息)。

---

### 3.6 获取账户概览

> 返回账户概览 + 持仓列表 + 今日待执行信号数。

```
GET /api/v1/live/agent/account-summary?token=tk_xxxx
```

**响应结构:**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accountId": 5,
    "accountName": "东方财富模拟-龙虾交易",
    "totalAssets": 475096.60,
    "availBalance": 475096.60,
    "totalProfit": -524903.40,
    "pendingCount": 3,
    "positionCount": 2,
    "positions": [
      {
        "stockCode": "000001",
        "stockName": "平安银行",
        "quantity": 500,
        "avgCost": 12.50,
        "currentPrice": 12.80,
        "pnl": 150.00,
        "pnlPct": 2.40
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `accountId` | uint | 账户 ID |
| `accountName` | string | 账户名称 |
| `totalAssets` | float64 | 总资产 |
| `availBalance` | float64 | 可用余额（账户级） |
| `totalProfit` | float64 | 累计盈亏 |
| `pendingCount` | int64 | 今日待执行信号数 |
| `positionCount` | int | 持仓数量 |
| `positions` | array | 持仓列表 |

**positions 元素：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `stockCode` | string | 股票代码 |
| `stockName` | string | 股票名称 |
| `quantity` | int | 持有数量 |
| `avgCost` | float64 | 持仓均价 |
| `currentPrice` | float64 | 当前价格 |
| `pnl` | float64 | 浮动盈亏 |
| `pnlPct` | float64 | 盈亏百分比 |

> ⚠️ **待修复**: 当前 positions 使用 `user_id` 过滤，应改为按账户的 strategy_runs 过滤 `live_positions`。详见 [Issue #1](#61-issue-1-getaccountsummary-返回全部用户持仓而非账户持仓)。

---

### 3.7 同步持仓到服务端

> Agent 从券商获取当前持仓后，推送到服务端更新 `live_positions` 和 `holdings`。

```
POST /api/v1/live/agent/positions/sync?token=tk_xxxx
```

**请求体：**
```json
{
  "positions": [
    {
      "stockCode": "600519",
      "stockName": "贵州茅台",
      "quantity": 100,
      "avgCost": 1640.00,
      "currentPrice": 1650.00,
      "marketValue": 165000.00,
      "unrealizedPnl": 1000.00
    }
  ],
  "balance": {
    "availableCash": 475096.60,
    "frozenCash": 2360.04,
    "totalAssets": 640096.60,
    "totalMarketValue": 165000.00,
    "totalProfit": -524903.40,
    "nav": 0.6401
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `positions` | array | 是 | 持仓列表 |
| `positions[].stockCode` | string | 是 | 股票代码 |
| `positions[].stockName` | string | 否 | 股票名称 |
| `positions[].quantity` | int | 是 | 持有数量 |
| `positions[].avgCost` | float64 | 否 | 持仓均价 |
| `positions[].currentPrice` | float64 | 否 | 当前价格 |
| `positions[].marketValue` | float64 | 否 | 市值 |
| `positions[].unrealizedPnl` | float64 | 否 | 浮动盈亏 |
| `balance` | object | 否 | 账户余额（不传则只更新持仓） |
| `balance.availableCash` | float64 | 否 | 可用资金 |
| `balance.frozenCash` | float64 | 否 | 冻结资金 |
| `balance.totalAssets` | float64 | 否 | 总资产 |
| `balance.totalMarketValue` | float64 | 否 | 持仓市值 |
| `balance.totalProfit` | float64 | 否 | 累计盈亏 |
| `balance.nav` | float64 | 否 | 净值 |

**成功响应:**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "syncedPositions": 1,
    "syncedHoldings": 1
  }
}
```

**处理逻辑：**
1. 更新 `trading_accounts` 余额字段（如果提供了 balance）
2. Upsert `live_positions`（按 run_id + stock_code）
3. Upsert `holdings`（按 account_id + stock_code）

---

### 3.8 同步委托/订单到服务端

> Agent 从券商获取当日委托单后，推送到服务端。

```
POST /api/v1/live/agent/orders/sync?token=tk_xxxx
```

**请求体：**
```json
{
  "orders": [
    {
      "orderId": "ORD20260710001",
      "stockCode": "600519",
      "stockName": "贵州茅台",
      "actionType": "buy",
      "orderPrice": 1650.00,
      "orderQty": 100,
      "filledQty": 100,
      "filledPrice": 1649.50,
      "orderStatus": "filled",
      "orderTime": "2026-07-10 09:30:15"
    }
  ]
}
```

---

### 3.9 指令回传

> Agent 响应服务端通过 WebSocket 下发的指令（如测试命令）。

```
POST /api/v1/live/agent/commands/{requestId}/response?token=tk_xxxx
```

**请求体：**
```json
{
  "requestId": "cmd_a1b2c3d4",
  "status": "ok",
  "result": {
    "balance": 475096.60,
    "positions": 2
  },
  "error": ""
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `requestId` | string | 指令请求 ID（与 WS 下发的 requestId 一致） |
| `status` | string | `"ok"` 或 `"failed"` |
| `result` | object | 指令执行结果（可选） |
| `error` | string | 失败原因（可选） |

---

### 3.10 连通性测试

> 服务端主动测试 Agent 是否在线。

```
POST /api/v1/live/test-agent?token=tk_xxxx
```

**流程：**
服务端 → WebSocket 发送 `{"type": "test_request", "data": {"requestId": "uuid"}}`  
Agent → REST 回传 `POST /api/v1/live/agent-test-response`  
服务端 → 收到响应判定连通

**Agent 端应监听 WS 的 `test_request` 类型消息，并通过 API Client 的 `respond_test()` 回传。**

---

### 3.11 检查 Agent 状态

```
GET /api/v1/live/agent-status?token=tk_xxxx
```

**响应:**
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "connected": true,
    "accountId": 5,
    "message": "agent status retrieved"
  }
}
```

---

## 4. WebSocket 协议

### 连接

```
ws://{server}/api/v1/ws/signals?token=tk_xxxx
```

### 服务端 → Agent (Push)

#### 4.1 新信号通知
```json
{
  "type": "new_signal",
  "accountId": 5,
  "data": {
    "signalId": 1001,
    "stockCode": "600519",
    "actionType": "buy",
    "orderPrice": 1650.00,
    "plannedQty": 100
  }
}
```

#### 4.2 指令下发
```json
{
  "type": "command",
  "accountId": 5,
  "data": {
    "type": "command",
    "requestId": "cmd_a1b2c3d4",
    "accountId": 5,
    "action": "sync_positions",
    "payload": {}
  }
}
```

**支持的 action：**

| action | 说明 |
|--------|------|
| `sync_positions` | 要求从券商拉取最新持仓 |
| `get_balance` | 要求查询账户余额 |
| `place_order` | 要求下单 |
| `cancel_order` | 要求撤单 |
| `query_orders` | 要求查询当日委托 |

#### 4.3 连通性测试
```json
{
  "type": "test_request",
  "accountId": 5,
  "data": {
    "requestId": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2026-07-10T09:30:00+08:00"
  }
}
```

#### 4.4 被踢下线
```json
{
  "type": "kicked",
  "accountId": 5,
  "data": {
    "reason": "new_connection"
  }
}
```

`reason` 可能值：`"new_connection"` (新连接挤掉旧连接) / `"token_revoked"` (Token 被吊销)

#### 4.5 心跳
服务端每 54 秒发送一次 WebSocket Ping 帧，Agent 只需回复 Pong。无需处理业务层心跳消息。

### Agent → 服务端 (WS)

#### 4.6 连接握手 (agent_hello)
```json
{
  "type": "agent_hello",
  "data": {
    "traderType": "lobster",
    "capabilities": ["place_order", "sync_positions", "get_balance", "query_orders", "cancel_order"]
  }
}
```

#### 4.7 心跳
```json
{
  "type": "heartbeat"
}
```

---

## 5. 交易执行流程

### 5.1 完整时序图

```
服务端                    Agent (Python)               券商客户端
──────                    ──────────────               ──────────

1. trade_exec 生成信号
   status=pending_auto
   ─────────────────────► WS push "new_signal"
   (或 HTTP poll 轮询)     │
                            │
2.                          ├─ claim 信号
                            │  POST /signals/:id/claim
   ◄────────────────────────┤
   status=claimed           │
                            │
3.                          ├─ 获取信号详情
                            │  GET /signals/:id/detail
                            │
4.                          ├─ 实际下单               ──► 券商API
                            │                            │
6.                          │                       ◄── 成交回报
                            │
6.                          ├─ 上报执行结果
                            │  POST /signals/:id/report-result
   ◄────────────────────────┤  {status:"executed", execPrice, execQty}
                            │
7. 更新信号 status=executed  │
   更新 run.AvailableCash    │
   更新 run.PositionValue    │
   更新 live_positions       │
   同步 holdings             │
   记录 LiveTrade            │
```

### 5.2 Agent 执行伪代码

```python
def process_signal(signal):
    # 1. 认领（防止重复执行）
    claimed = api.claim_signal(signal['signalId'])
    if not claimed:
        return  # 已被其他 Agent 认领

    # 2. 执行下单（不做预算校验，服务端负责资金管理）
    trader = get_trader()
    result = trader.place_order(
        stock_code=signal['stockCode'],
        action=signal['actionType'],
        price=signal['orderPrice'],
        quantity=signal['plannedQty']
    )

    # 3. 上报结果（服务端据此更新信号状态 + 策略资金 + 持仓）
    if result.success:
        api.report_result(signal['signalId'], status='executed',
                          order_id=result.order_id,
                          exec_price=result.price,
                          exec_qty=result.quantity)
    else:
        api.report_result(signal['signalId'], status='order_failed',
                          error_msg=result.error)
```

---

## 6. 架构重构后已知问题与修复建议

> **Agent 定位确认**: Agent 是账户级哑执行器，不关心策略运行细节。
> 只负责：查询券商数据 + 执行下单 + 上报结果。不做预算校验、不区分运行。
> 服务端负责信号状态流转、资金更新、用户通知。

> 以下问题已排除了之前基于错误假设的分析（如缺少运行详情 API、多运行信号归属等——这些 Agent 不需要关心）。

---

### 6.1 Issue #P0: ReportResult 不更新策略运行资金

**位置:** `agent_handler.go:417` ReportResult

**问题:** Agent 上报 `executed` 后，服务端只写入 `LiveTrade` 记录 + 更新信号状态，
但 **未更新策略运行的可用现金和持仓市值**。

这意味着：
- `strategy_runs.available_cash` 不会减少（买入）或增加（卖出）
- `strategy_runs.position_value` 不会更新
- `live_positions` 不会增/减持仓记录
- 后续信号的预算校验会基于过时数据

**根本原因:** `ReportResult` 只做了记账（LiveTrade），没有走 `executeSignal` 的资金/持仓变更流程。

**修复方案:** 在 `ReportResult` 的 `status == "executed"` 分支中，调用
`liveTradingService.FinalizeSignalExecution(sig.RunID, sig.ID, body.ExecPrice, body.ExecQty)`
完成完整的信号执行流程（资金更新 + 持仓更新 + 交易记录）。✅ **已修复**

---

### 6.2 Issue #P1: GetAccountSummary 返回全部用户持仓

**位置:** `agent_handler.go:542`

```go
// 当前代码（错误 — 返回该用户的所有账户持仓）
db.MySQL.Where("user_id = ?", account.UserID).
    Order("stock_code ASC").Find(&positions)
```

**修复:**
```go
var runIDs []uint
db.MySQL.Model(&model.StrategyRun{}).
    Where("account_id = ? AND status IN ?", account.ID, []string{"active", "paused"}).
    Pluck("id", &runIDs)

db.MySQL.Where("strategy_run_id IN ?", runIDs).
    Order("stock_code ASC").Find(&positions)
```

---

### 6.3 Issue #P2: 缺少 Agent 端撤单 API

**需求:** Agent 下单后可能因网络异常、价格变动等原因需要撤单重试。
当前没有面向 Agent 的撤单回调（现有多条撤单接口都需要用户认证）。

**建议新增:**
```
POST /api/v1/live/agent/signals/:id/cancel?token=tk_xxxx
```
行为：将信号状态从 `claimed` 重置为 `pending_auto`，Agent 可重新认领。

---

### 6.4 Issue #P2: Go Model status 字段 size 与 DB 不一致

**位置:** `model/backtest_signal.go:43`

```go
Status string `gorm:"size:10;default:pending"`
```

v73 迁移已将列扩为 `VARCHAR(30)`，但 Model 标注仍是 `size:10`。

**修复:** `gorm:"size:10"` → `gorm:"size:30"`  ✅ **已修复**

---

### 6.5 已识别设计遗漏

#### 遗漏 #1: 信号超时回退机制

**问题:** Agent 掉线后，`claimed` 状态的信号永久卡住，不会自动回退。

**影响:** 若 Agent claim 信号后崩溃/掉线，该信号无人认领也无法被服务端手动执行。

**建议:** 在 `trade_exec` 的每日运行流程中增加超时检查：
```sql
UPDATE backtest_signals SET status = 'pending_auto', skip_reason = 'claim_timeout'
WHERE status = 'claimed' AND updated_at < NOW() - INTERVAL 30 MINUTE
```

#### 遗漏 #2: Agent 重连后无「我的 claimed 信号」查询

**问题:** Agent 断线重连后，不知道之前认领了哪些信号。虽然 `GetPendingAutoSignals` 返回 `claimed` 状态的信号，但 Agent 需要重新 claim（会 409），或需知道哪些已在处理中。

**建议选项:**
- **方案 A**: `GetPendingAutoSignals` 区分返回 `pending_auto` 和 `claimed` 两个子列表
- **方案 B**: 新增 `GET /agent/my-claimed-signals` 返回当前 Agent 认领的信号（利用 `updated_at` + `skip_reason = 'agent claimed'` 判断）

#### 遗漏 #3: order_failed 时 exec_price/exec_qty 未被清理

**问题:** Agent 上报 `order_failed` 时可能附带部分信息（如 `execQty=0`），这些值写入了 `exec_price=0 / exec_qty=0`，残留到下次重试成功时可能混淆。

**建议:** `order_failed` 分支中清空 `exec_price=0, exec_qty=0, broker_order_id=''`。

#### 遗漏 #4: ReportResult 的双重 DB 写入

**问题:** `executed` 路径中信号被写两次：
1. `updates` 块设置 `status='executed'`, `exec_price`, `exec_qty`
2. `FinalizeSignalExecution` → `executeSignal` 再次写入相同字段

**影响:** 无正确性问题，但浪费一次 DB I/O。

**优化方向:** 对 `executed` 路径跳过最初的 `updates` 块，仅让 `FinalizeSignalExecution` 处理所有写入。

---

### 6.6 关于未来扩展

基于「Agent 可获取账户策略数据做本地 AI 分析」的需求方向，后续可考虑：
- 新增 `GET /api/v1/live/agent/runs` — 返回账户下所有活跃策略运行及其盈亏
- 新增 `GET /api/v1/live/agent/signals/history` — 返回历史信号列表
- MCP 工具扩展：增加账户分析、策略表现对比等只读工具

---

## 附录 A: 常见错误码

| HTTP 状态 | 含义 | 常见原因 |
|-----------|------|----------|
| 401 | 认证失败 | Token 无效或已过期 |
| 403 | 权限不足 | 信号不属于此账户 / 账户未配置 agent 模式 |
| 404 | 资源不存在 | signalId 不存在 |
| 409 | 状态冲突 | 信号已被认领 / 状态不是 pending_auto |
| 410 | 指令过期 | command requestId 已超时 |

## 附录 B: Agent 端 API Client 映射

| Python API Client 方法 | HTTP 调用 |
|------------------------|-----------|
| `get_pending_signals()` | `GET /api/v1/live/pending-auto-signals` |
| `claim_signal(id)` | `POST /api/v1/live/signals/:id/claim` |
| `report_result(id, ...)` | `POST /api/v1/live/signals/:id/report-result` |
| `get_signal_detail(id)` | `GET /api/v1/live/signals/:id/detail` |
| `get_account()` | `GET /api/v1/live/agent/account` |
| `get_account_summary()` | `GET /api/v1/live/agent/account-summary` |
| `respond_command(rid, ...)` | `POST /api/v1/live/agent/commands/:requestId/response` |
| `sync_positions(...)` | `POST /api/v1/live/agent/positions/sync` |
| `sync_orders(...)` | `POST /api/v1/live/agent/orders/sync` |
| `respond_test(rid, ...)` | `POST /api/v1/live/agent-test-response` |
| `get_trade_mode()` | `GET /api/v1/live/agent/account` → 解析 `accountType` |
