"""MCP Server — exposes agent capabilities via Model Context Protocol (stdio).

Allows AI agents (Claude Desktop, Codex, etc.) to:
- Query pending trade signals
- Execute signals manually
- Check account status
- Report execution results
"""

import json
import logging
import sys

from core.auth import load_config, get_server_url, get_token
from core.api_client import APIClient

logger = logging.getLogger("agent.mcp")


# ── Tool Definitions ──

TOOLS = [
    {
        "name": "get_pending_signals",
        "description": "获取当前账户待执行的自动交易信号列表（pending_auto 状态）",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "required": [],
        },
    },
    {
        "name": "get_signal_detail",
        "description": "获取指定信号的完整详情，包括止损止盈、AI置信度等",
        "inputSchema": {
            "type": "object",
            "properties": {
                "signal_id": {
                    "type": "integer",
                    "description": "信号ID",
                },
            },
            "required": ["signal_id"],
        },
    },
    {
        "name": "claim_signal",
        "description": "认领一个信号用于执行，防止其他 agent 重复执行",
        "inputSchema": {
            "type": "object",
            "properties": {
                "signal_id": {
                    "type": "integer",
                    "description": "要认领的信号ID",
                },
            },
            "required": ["signal_id"],
        },
    },
    {
        "name": "execute_signal",
        "description": "执行一个已认领的交易信号（通过 Playwright 自动化下单）",
        "inputSchema": {
            "type": "object",
            "properties": {
                "signal_id": {
                    "type": "integer",
                    "description": "要执行的信号ID",
                },
            },
            "required": ["signal_id"],
        },
    },
    {
        "name": "report_result",
        "description": "手动上报交易执行结果到服务端",
        "inputSchema": {
            "type": "object",
            "properties": {
                "signal_id": {
                    "type": "integer",
                    "description": "信号ID",
                },
                "status": {
                    "type": "string",
                    "description": "执行状态：executed 或 order_failed",
                    "enum": ["executed", "order_failed"],
                },
                "order_id": {
                    "type": "string",
                    "description": "券商订单号",
                },
                "error_msg": {
                    "type": "string",
                    "description": "错误信息（status=order_failed 时必填）",
                },
                "exec_price": {
                    "type": "number",
                    "description": "实际成交价格",
                },
                "exec_qty": {
                    "type": "integer",
                    "description": "实际成交数量",
                },
            },
            "required": ["signal_id", "status"],
        },
    },
    {
        "name": "get_account_summary",
        "description": "获取账户资金概况、持仓列表和待执行信号数量（服务端DB数据）",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "required": [],
        },
    },
    {
        "name": "get_broker_balance",
        "description": "连接东方财富网页交易端，实时查询账户资金（总资产、可用资金、持仓市值、累计盈亏）",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "required": [],
        },
    },
    {
        "name": "get_broker_positions",
        "description": "连接东方财富网页交易端，实时查询当前持仓列表（代码、名称、数量、成本、现价、盈亏）",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "required": [],
        },
    },
]


class AgentMCPServer:
    """MCP stdio server wrapping the agent API client."""

    def __init__(self):
        config = load_config()
        self.api = APIClient(get_server_url(config), get_token(config))
        self.config = config

    def _create_trader(self):
        """按 config 中的 trader 设置创建执行器实例（默认东方财富 Mac APP）。"""
        name = self.config.get("trader", "eastmoney")
        if name == "eastmoney":
            from traders.eastmoney_mac import EastMoneyMacTrader
            em_cfg = self.config.get("eastmoney", {})
            return EastMoneyMacTrader(
                app_name=em_cfg.get("app_name", "东方财富"),
                confirm_order=em_cfg.get("confirm_order", True),
                action_delay=em_cfg.get("action_delay", 0.4),
            )
        from traders.playwright_web import PlaywrightWebTrader
        pw_cfg = self.config.get("playwright", {})
        return PlaywrightWebTrader(
            trading_url=pw_cfg.get("trading_url"),
            profile_dir=pw_cfg.get("profile_dir"),
            headless=pw_cfg.get("headless", False),
        )

    def handle_request(self, request):
        """Handle a single JSON-RPC request."""
        method = request.get("method", "")
        req_id = request.get("id")

        if method == "initialize":
            return self._response(req_id, {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {
                    "name": "stock-trading-agent",
                    "version": "1.0.0",
                },
            })

        elif method == "tools/list":
            return self._response(req_id, {"tools": TOOLS})

        elif method == "tools/call":
            params = request.get("params", {})
            tool_name = params.get("name", "")
            tool_args = params.get("arguments", {})
            result = self._call_tool(tool_name, tool_args)
            return self._response(req_id, {
                "content": [{"type": "text", "text": json.dumps(result, ensure_ascii=False, indent=2)}],
            })

        elif method == "notifications/initialized":
            return None  # No response for notifications

        else:
            return self._error(req_id, -32601, f"Method not found: {method}")

    def _call_tool(self, name, args):
        """Route tool calls to API client methods."""
        try:
            if name == "get_pending_signals":
                return self.api.get_pending_signals()

            elif name == "get_signal_detail":
                return self.api.get_signal_detail(args["signal_id"])

            elif name == "claim_signal":
                return self.api.claim_signal(args["signal_id"])

            elif name == "execute_signal":
                return self._execute_signal(args["signal_id"])

            elif name == "report_result":
                return self.api.report_result(
                    signal_id=args["signal_id"],
                    status=args["status"],
                    order_id=args.get("order_id", ""),
                    error_msg=args.get("error_msg", ""),
                    exec_price=args.get("exec_price", 0),
                    exec_qty=args.get("exec_qty", 0),
                )

            elif name == "get_account_summary":
                return self.api.get_account_summary()

            elif name == "get_broker_balance":
                return self._query_broker_balance()

            elif name == "get_broker_positions":
                return self._query_broker_positions()

            else:
                return {"error": f"Unknown tool: {name}"}

        except Exception as e:
            logger.error(f"Tool {name} error: {e}")
            return {"error": str(e)}

    def _execute_signal(self, signal_id):
        """Claim and execute a signal."""
        # First get signal detail
        detail = self.api.get_signal_detail(signal_id)
        if not detail:
            return {"success": False, "error": "Signal not found"}

        # Claim it
        claimed = self.api.claim_signal(signal_id)
        if not claimed or not claimed.get("claimed"):
            return {"success": False, "error": "Failed to claim signal"}

        # Execute via trader
        try:
            trader = self._create_trader()

            if not trader.connect():
                return {"success": False, "error": "Failed to connect to trader"}

            result = trader.place_order(
                stock_code=detail.get("stockCode", ""),
                stock_name=detail.get("stockName", ""),
                action=detail.get("actionType", "buy"),
                price=float(detail.get("orderPrice", 0) or 0),
                quantity=int(detail.get("plannedQty", 0) or 0),
            )

            trader.disconnect()

            if result.success:
                self.api.report_result(
                    signal_id=signal_id,
                    status="executed",
                    order_id=result.order_id,
                    exec_price=result.exec_price,
                    exec_qty=result.exec_qty,
                )
                return {
                    "success": True,
                    "order_id": result.order_id,
                    "exec_price": result.exec_price,
                    "exec_qty": result.exec_qty,
                    "status_text": getattr(result, "status_text", ""),
                    "filled_qty": getattr(result, "filled_qty", 0),
                }
            else:
                self.api.report_result(
                    signal_id=signal_id,
                    status="order_failed",
                    error_msg=result.error_msg,
                )
                return {"success": False, "error": result.error_msg}

        except Exception as e:
            self.api.report_result(
                signal_id=signal_id,
                status="order_failed",
                error_msg=str(e),
            )
            return {"success": False, "error": str(e)}

    def _response(self, req_id, result):
        return json.dumps({"jsonrpc": "2.0", "id": req_id, "result": result})

    def _error(self, req_id, code, message):
        return json.dumps({
            "jsonrpc": "2.0",
            "id": req_id,
            "error": {"code": code, "message": message},
        })


    def _query_broker_balance(self):
        """Connect to broker and get real-time balance."""
        try:
            trader = self._create_trader()
            if not trader.connect():
                return {"error": "交易端连接失败（检查 APP 是否运行/已授权辅助功能）"}
            balance = trader.get_balance()
            trader.disconnect()
            return balance if balance else {"error": "未能解析资金数据"}
        except Exception as e:
            return {"error": str(e)}

    def _query_broker_positions(self):
        """Connect to broker and get real-time positions."""
        try:
            trader = self._create_trader()
            if not trader.connect():
                return {"error": "交易端连接失败（检查 APP 是否运行/已授权辅助功能）"}
            positions = trader.get_positions()
            trader.disconnect()
            return positions if positions else {"error": "未能解析持仓数据"}
        except Exception as e:
            return {"error": str(e)}

    def run(self):
        """Run the MCP server on stdio."""
        logger.info("MCP server starting (stdio mode)...")
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
            try:
                request = json.loads(line)
                response = self.handle_request(request)
                if response:
                    sys.stdout.write(response + "\n")
                    sys.stdout.flush()
            except json.JSONDecodeError:
                logger.warning(f"Invalid JSON: {line[:100]}")
            except Exception as e:
                logger.error(f"MCP error: {e}")
