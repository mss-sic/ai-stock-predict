"""Abstract base class for all trader implementations."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional


@dataclass
class OrderResult:
    """Result of an order placement attempt."""
    success: bool
    order_id: str = ""
    exec_price: float = 0.0
    exec_qty: int = 0
    error_msg: str = ""
    status_text: str = ""
    filled_qty: int = 0
    order_detail: Optional[dict] = None


@dataclass
class Position:
    """A single position."""
    stock_code: str
    quantity: int
    avg_cost: float
    current_price: float = 0.0


class AbstractTrader(ABC):
    """Interface for all trade execution implementations."""

    # ── Lifecycle ──

    @abstractmethod
    def connect(self) -> bool:
        """Initialize and connect to the trading interface."""
        ...

    @abstractmethod
    def disconnect(self):
        """Clean up resources."""
        ...

    # ── Order Operations ──

    @abstractmethod
    def place_order(self, stock_code: str, stock_name: str, action: str,
                    price: float, quantity: int) -> OrderResult:
        """Place an order (buy/sell/add/reduce)."""
        ...

    @abstractmethod
    def cancel_order(self, order_id: str) -> bool:
        """Cancel an existing order."""
        ...

    @abstractmethod
    def query_orders(self) -> list[dict]:
        """Query all pending/filled orders (委托列表)."""
        ...

    # ── Account Info ──

    @abstractmethod
    def get_balance(self) -> Optional[dict]:
        """Get account balance.
        Returns dict with keys: total_assets, available_cash, frozen_cash,
        market_value, total_profit, nav.  Or None if unavailable.
        """
        ...

    @abstractmethod
    def get_positions(self) -> list[dict]:
        """Get all current positions.
        Each dict: stock_code, stock_name, quantity, avail_qty, avg_cost, current_price.
        """
        ...

    # ── Single-position Query ──

    @abstractmethod
    def query_position(self, stock_code: str) -> Optional[Position]:
        """Query position for a single stock."""
        ...
