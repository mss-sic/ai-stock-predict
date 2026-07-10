"""API client — wraps all REST calls to the server."""

import json
import logging
import requests

logger = logging.getLogger("agent.api")


class APIClient:
    """HTTP client for the agent server API."""

    def __init__(self, server_url, agent_token):
        self.base = server_url.rstrip("/")
        self.token = agent_token
        self.session = requests.Session()
        self.session.headers.update({
            "X-Agent-Token": self.token,
            "Content-Type": "application/json",
        })
        self.session.timeout = 30

    def _url(self, path):
        return f"{self.base}{path}"

    # ── Signal Operations ──

    def get_pending_signals(self):
        """Fetch pending_auto signals for this agent's account."""
        try:
            resp = self.session.get(
                self._url("/api/v1/live/pending-auto-signals"),
                params={"token": self.token},
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get("code") == 0:
                return data.get("data", [])
            logger.error(f"get_pending_signals error: {data}")
            return []
        except Exception as e:
            logger.error(f"get_pending_signals failed: {e}")
            return []

    def claim_signal(self, signal_id):
        """Claim a signal for execution (pending_auto → claimed)."""
        try:
            resp = self.session.post(
                self._url(f"/api/v1/live/signals/{signal_id}/claim"),
                params={"token": self.token},
            )
            if resp.status_code == 200:
                data = resp.json()
                return data.get("data", {})
            logger.warning(f"claim_signal {signal_id}: {resp.status_code} {resp.text}")
            return None
        except Exception as e:
            logger.error(f"claim_signal {signal_id} failed: {e}")
            return None

    def report_result(self, signal_id, status, order_id="", error_msg="",
                      exec_price=0.0, exec_qty=0):
        """Report execution result back to server."""
        payload = {
            "status": status,
            "orderId": order_id,
            "errorMsg": error_msg,
            "execPrice": exec_price,
            "execQty": exec_qty,
        }
        try:
            resp = self.session.post(
                self._url(f"/api/v1/live/signals/{signal_id}/report-result"),
                params={"token": self.token},
                json=payload,
            )
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"report_result {signal_id} failed: {e}")
            return None

    def get_signal_detail(self, signal_id):
        """Get full details for a signal."""
        try:
            resp = self.session.get(
                self._url(f"/api/v1/live/signals/{signal_id}/detail"),
                params={"token": self.token},
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get("code") == 0:
                return data.get("data", {})
            return {}
        except Exception as e:
            logger.error(f"get_signal_detail {signal_id} failed: {e}")
            return {}

    # ── Account Info ──

    def get_account(self):
        """Get full account information (name, type, broker, capital, etc.)."""
        try:
            resp = self.session.get(
                self._url("/api/v1/live/agent/account"),
                params={"token": self.token},
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get("code") == 0:
                return data.get("data", {})
            return {}
        except Exception as e:
            logger.error(f"get_account failed: {e}")
            return {}

    def get_account_summary(self):
        """Get account balance and positions."""
        try:
            resp = self.session.get(
                self._url("/api/v1/live/agent/account-summary"),
                params={"token": self.token},
            )
            resp.raise_for_status()
            data = resp.json()
            if data.get("code") == 0:
                return data.get("data", {})
            return {}
        except Exception as e:
            logger.error(f"get_account_summary failed: {e}")
            return {}

    # ── Command Response ──

    def get_trade_mode(self) -> str:
        """从服务端获取账户类型，映射为 trade_mode。

        accountType "real" → "real"
        accountType "simulated" → "simulated"
        默认为 "simulated"（安全兜底）
        """
        account = self.get_account()
        at = account.get("accountType", "")
        if at == "real":
            return "real"
        # simulated 或其他均为模拟盘
        logger.info(f"accountType={at} → trade_mode=simulated")
        return "simulated"

    def respond_command(self, request_id, status, result=None, error=""):
        """Respond to a server-dispatched command."""
        payload = {
            "requestId": request_id,
            "status": status,
            "error": error,
        }
        if result is not None:
            payload["result"] = result
        try:
            resp = self.session.post(
                self._url(f"/api/v1/live/agent/commands/{request_id}/response"),
                params={"token": self.token},
                json=payload,
            )
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"respond_command {request_id} failed: {e}")
            return None

    # ── Position & Order Sync ──

    def sync_positions(self, positions, balance=None):
        """Sync local positions and optional balance to server."""
        payload = {"positions": positions}
        if balance is not None:
            payload["balance"] = balance
        try:
            resp = self.session.post(
                self._url("/api/v1/live/agent/positions/sync"),
                params={"token": self.token},
                json=payload,
            )
            resp.raise_for_status()
            data = resp.json()
            return data.get("data", {})
        except Exception as e:
            logger.error(f"sync_positions failed: {e}")
            return {}

    def sync_orders(self, orders):
        """Sync local order/entrust list to server."""
        payload = {"orders": orders}
        try:
            resp = self.session.post(
                self._url("/api/v1/live/agent/orders/sync"),
                params={"token": self.token},
                json=payload,
            )
            resp.raise_for_status()
            data = resp.json()
            return data.get("data", {})
        except Exception as e:
            logger.error(f"sync_orders failed: {e}")
            return {}

    # ── Connectivity ──

    def respond_test(self, request_id, success=True, message="agent is alive"):
        """Respond to a server test challenge."""
        payload = {
            "requestId": request_id,
            "success": success,
            "message": message,
        }
        try:
            resp = self.session.post(
                self._url("/api/v1/live/agent-test-response"),
                json=payload,
            )
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            logger.error(f"respond_test failed: {e}")
            return None
