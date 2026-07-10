"""Lobster (龙虾) commercial auto-trading software integration.

Uses Lobster's local HTTP API or CLI to place orders.
Currently a stub — actual integration depends on Lobster's API/SDK.
"""

from __future__ import annotations

import logging
import os
from typing import Optional

from .base import AbstractTrader, OrderResult, Position

logger = logging.getLogger("agent.trader.lobster")


class LobsterTrader(AbstractTrader):
    """Integrates with Lobster auto-trading software."""

    def __init__(self, exe_path: str = "/Applications/Lobster.app",
                 api_url: Optional[str] = None):
        self.exe_path = exe_path
        self.api_url = api_url or "http://127.0.0.1:19999"

    def connect(self) -> bool:
        if os.path.exists(self.exe_path):
            logger.info(f"Lobster found at {self.exe_path}")
            return True
        logger.warning(f"Lobster not found at {self.exe_path}")
        return False

    def disconnect(self):
        logger.info("Lobster trader disconnected")

    def place_order(self, stock_code, stock_name, action, price,
                    quantity) -> OrderResult:
        """Place order via Lobster API. NOT YET IMPLEMENTED — returns failure.

        Once Lobster provides a local HTTP API or CLI, implement here.
        DO NOT return success=True without actually placing an order.
        """
        logger.warning(
            f"Lobster place_order: 龙虾 API 未实现，无法下单 "
            f"{stock_name}({stock_code}) {action} {quantity}股 @ {price}"
        )
        return OrderResult(
            success=False,
            error_msg="龙虾 API 未实现 — 请等待龙虾提供下单接口后配置 api_url",
        )

    def cancel_order(self, order_id: str) -> bool:
        logger.warning("Lobster cancel_order not implemented")
        return False

    def get_balance(self) -> Optional[dict]:
        logger.warning("Lobster get_balance not implemented")
        return None

    def get_positions(self) -> list[dict]:
        logger.warning("Lobster get_positions not implemented")
        return []

    def query_orders(self) -> list[dict]:
        logger.warning("Lobster query_orders not implemented")
        return []

    def query_position(self, stock_code: str) -> Optional[Position]:
        logger.warning("Lobster query_position not implemented")
        return None
