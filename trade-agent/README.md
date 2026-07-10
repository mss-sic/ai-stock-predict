# 智策投研 — 本地自动交易代理 (Trade Agent)

独立部署的本地交易代理，与[智策投研](https://github.com/ai-stock-predict)服务端配合，
实现 **远程信号 → 本地自动化下单 → 结果回传** 的完整闭环。

## 架构

```
服务端 (Go)                              本地 Agent (Python)
─────────                               ────────────────────

┌──────────────────────┐                ┌────────────────────┐
│   ExecChannel 路由    │                │  Trader 工厂        │
│                      │                │                    │
│  manual  → 手动确认   │    WebSocket   │  eastmoney_mac     │
│  api     → 妙想API   │◄──────────────►│  eastmoney_web     │
│  agent   → WS转发    │   (实时指令)     │  lobster           │
│                      │                │  (future...)       │
└──────────────────────┘                └────────────────────┘
         │                                       │
         │  REST API (回传结果)                    │
         │◄──────────────────────────────────────│
         │                                       │
    ┌─────▼──────┐                    ┌──────────▼─────────┐
    │ PostgreSQL │                    │  东方财富 Mac 版     │
    │   MySQL    │                    │  (AX API 自动化)     │
    └────────────┘                    └────────────────────┘
```

**核心设计**：服务端按 `ExecChannel` 路由（manual / api / agent），Agent 端按 `broker_mode` 选择具体客户端，两端解耦。

### 执行通道分类

| 通道 | 服务端 broker_mode 值 | 说明 |
|------|----------------------|------|
| `manual` | `manual` | 纯手动，生成待确认信号 |
| `api` | `mx_moni`（及未来 API 券商） | 服务端直连券商 API |
| `agent` | `lobster`（及未来 Agent 券商） | WebSocket 转发给本地 Agent |

### Agent 端 trader 类型

| broker_mode | 客户端 | 技术方案 |
|-------------|--------|----------|
| `eastmoney_mac` | 东方财富 Mac 版 | macOS AX API + pyautogui |
| `eastmoney_web` | 东方财富网页版 | Playwright（Chrome 自动化） |
| `lobster` | 龙虾.app | 待龙虾提供 API/SDK |

---

## 功能

- 🔄 **全自动守护进程** — WebSocket 实时接收指令 + HTTP 轮询兜底
- 🤖 **MCP 协议接口** — 供 Claude Desktop / Codex 等 AI Agent 直接调用
- 📡 **双向指令通道** — 服务端下发同步持仓、下单、撤单、查询委托等指令
- 🔒 **Agent Token 认证** — 绑定单一资金账户，信号完全隔离
- 🎯 **Trader 工厂** — 按 `broker_mode` 自动选择执行客户端，新增券商只需注册即可
- 🧪 **启动预检** — 依赖授权 / APP 运行 / 坐标校准 / 账户一致性，逐项检查

---

## 快速开始

### 1. 安装

```bash
cd trade-agent
pip install -r requirements.txt
playwright install chromium
```

### 2. 配置

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，填入:
#   server_url: 智策投研服务端地址
#   agent_token: 从服务端账户管理页生成的 Agent Token（账户身份由 token 识别）
#   broker_mode:  本地执行客户端类型 (eastmoney_mac / eastmoney_web / lobster)
```

### 3. 校准（仅 eastmoney_mac 需要）

东方财富 Mac 版的「持仓/成交/委托」底部标签和「普通交易/模拟交易」顶部标签是自绘控件，AX 不可见，需要手动校准一次坐标：

```bash
python3 agent.py --mode calibrate
```

按弹窗提示依次点击标签，坐标会保存到 `tab_calibration.json`，之后随窗口位置自动适配。

### 4. 运行

```bash
# 守护进程模式（7×24 自动交易）
python3 agent.py --mode daemon

# MCP 接口模式（供 AI Agent 调用）
python3 agent.py --mode mcp

# 测试模式（验证连接和读取）
python3 agent.py --mode test-broker
```

---

## 启动预检流程 (eastmoney_mac)

```
① 依赖与授权检查（pyobjc / 辅助功能 / 输入监控 / pyautogui）
② 东方财富 APP 是否运行且窗口可读
③ 持仓/成交/委托 tab 坐标 + 模式标签坐标是否已校准
④ trader 连接与关键控件自检（下单区/表头定位）
⑤ 东财登录账户名 与 服务端绑定账户名 是否一致
```

任一失败即阻断，弹窗提示修复方式。

---

## WebSocket 协议

### 服务端 → Agent (Push)

```
type: "command"
data: {
  "requestId": "cmd_a1b2c3d4",
  "action": "place_order",
  "payload": { "stockCode": "600519", "price": 1650, "quantity": 100, ... }
}
```

支持的 action：`place_order` / `sync_positions` / `get_balance` / `cancel_order` / `query_orders`

### Agent → 服务端 (REST 回传)

```
POST /api/v1/agent/commands/:requestId/response?token=xxx
{ "requestId": "cmd_a1b2c3d4", "status": "ok", "result": {...} }
```

### Agent → 服务端 (WS agent_hello, 连接时)

```json
{
  "type": "agent_hello",
  "data": {
    "traderType": "eastmoney_mac",
    "capabilities": ["place_order", "sync_positions", "get_balance", "query_orders", "cancel_order"]
  }
}
```

服务端接收后存储 traderType 和 capabilities，供前端展示 Agent 能力。

---

## REST API 接口

### 信号操作

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/live/pending-auto-signals?token=xxx` | 获取待执行信号列表 |
| `POST` | `/api/v1/live/signals/:id/claim?token=xxx` | 认领信号 |
| `POST` | `/api/v1/live/signals/:id/report-result?token=xxx` | 回传执行结果 |
| `GET` | `/api/v1/live/signals/:id/detail?token=xxx` | 获取信号详情 |

### 账户信息

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/agent/account?token=xxx` | 获取账户完整信息 |
| `GET` | `/api/v1/agent/account-summary?token=xxx` | 账户概况 + 持仓列表 |

### 指令回传

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/agent/commands/:requestId/response?token=xxx` | 指令执行结果回传 |
| `POST` | `/api/v1/agent/positions/sync?token=xxx` | 同步持仓 |
| `POST` | `/api/v1/agent/orders/sync?token=xxx` | 同步委托/订单 |

### 连接管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/agent/test?token=xxx` | 测试连通性 |

---

## MCP 工具列表

| 工具 | 说明 |
|------|------|
| `get_account_info` | 获取账户完整信息 |
| `get_pending_signals` | 查询待执行信号 |
| `get_signal_detail` | 信号详情 |
| `claim_signal` | 认领信号防重复 |
| `execute_signal` | 执行交易信号 |
| `report_result` | 上报执行结果 |
| `get_account_summary` | 账户概况 + 持仓 |
| `sync_positions` | 同步持仓到服务端 |
| `sync_orders` | 同步委托到服务端 |

---

## 目录结构

```
trade-agent/
├── agent.py              # 主入口 (--mode daemon|mcp|test-broker|calibrate)
├── config.example.yaml   # 配置模板
├── config.yaml           # 你的配置 (gitignore)
├── requirements.txt      # Python 依赖
├── tab_calibration.json  # 坐标校准数据 (自动生成)
├── core/                 # 核心模块
│   ├── auth.py           # 配置加载
│   ├── api_client.py     # REST API 封装
│   ├── ws_client.py      # WebSocket 客户端
│   ├── poller.py         # HTTP 轮询
│   ├── signal_queue.py   # 信号去重队列
│   ├── reporter.py       # 结果上报
│   └── preflight.py      # 启动预检编排
├── traders/              # 交易执行器
│   ├── base.py           # AbstractTrader 抽象接口
│   ├── factory.py        # Trader 工厂 (按 broker_mode 创建)
│   ├── eastmoney_mac.py  # 东方财富 Mac 版 (AX API, 主力)
│   ├── playwright_web.py # 东方财富网页版 (Playwright)
│   ├── lobster.py        # 龙虾客户端 (待 API)
│   ├── pyautogui_mac.py  # OCR 盲操作 (已停用下单)
│   ├── ax_helper.py      # macOS AX API 封装
│   └── tab_calibrator.py # 坐标校准工具
├── mcp/                  # MCP 协议
│   └── server.py         # MCP stdio 服务
└── utils/                # 工具
    ├── notify.py         # macOS 通知
    └── logger.py         # 日志
```

---

## 新增券商流程

### API 直连类（如未来同花顺 OpenAPI）

1. 服务端 `brokerChannelMap` 加 `"ths_api": ChannelAPI`
2. 服务端实现 `THSAPIBroker`（`Broker` 接口）
3. Agent 端无改动

### Agent 代理类（如未来同花顺 Mac 版）

1. Agent 端新建 `traders/tonghuashun_mac.py` 实现 `AbstractTrader`
2. `factory.py` `_TRADER_REGISTRY` 注册
3. `preflight.py` 加预检步骤
4. `config.yaml` 加配置块
5. **服务端无改动**（仍走 Agent 通道）

---

## 许可

与智策投研主项目一致。
