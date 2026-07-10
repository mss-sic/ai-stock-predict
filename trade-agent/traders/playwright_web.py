"""Playwright-based web trading automation (主力 trader).

Operates the brokerage's web trading platform through Playwright.
Supports persistent browser profiles for cookie-based login persistence.
"""

from __future__ import annotations

import logging
import os
import time
from typing import Optional

from .base import AbstractTrader, OrderResult, Position

logger = logging.getLogger("agent.trader.playwright")


class PlaywrightWebTrader(AbstractTrader):
    """Automates a brokerage web trading platform using Playwright."""

    def __init__(self, trading_url=None, profile_dir=None, headless=False):
        self.trading_url = trading_url or os.environ.get(
            "TRADING_URL", "https://jywg.18.cn/"
        )
        self.profile_dir = os.path.expanduser(
            profile_dir or os.environ.get("PLAYWRIGHT_PROFILE", "~/.agent_browser_profile")
        )
        self.headless = headless
        self.browser = None
        self.context = None
        self.page = None

    def connect(self) -> bool:
        """Launch browser and navigate to trading page."""
        try:
            from playwright.sync_api import sync_playwright
            self._pw = sync_playwright().start()

            # Use persistent context for cookie-based login persistence
            os.makedirs(self.profile_dir, exist_ok=True)
            self.context = self._pw.chromium.launch_persistent_context(
                self.profile_dir,
                headless=self.headless,
                viewport={"width": 1280, "height": 900},
                locale="zh-CN",
            )
            self.page = self.context.new_page()

            logger.info(f"Navigating to {self.trading_url}")
            self.page.goto(self.trading_url, timeout=30000)
            time.sleep(2)

            # Check if login is needed
            if self._is_login_page():
                logger.warning("Login required — please log in manually in the browser window")
                logger.warning("Agent will wait up to 120s for manual login...")
                self._wait_for_login(120)

            logger.info("Playwright trader connected")
            return True

        except ImportError:
            logger.error("playwright not installed. Run: pip install playwright && playwright install chromium")
            return False
        except Exception as e:
            logger.error(f"Playwright connect failed: {e}")
            return False

    def disconnect(self):
        """Close browser and clean up."""
        try:
            if self.context:
                self.context.close()
            if self._pw:
                self._pw.stop()
            logger.info("Playwright trader disconnected")
        except Exception as e:
            logger.error(f"Disconnect error: {e}")

    def place_order(self, stock_code, stock_name, action, price, quantity) -> OrderResult:
        """Place an order through the web trading interface.

        This is a skeleton — actual DOM selectors depend on the specific brokerage
        web platform. The implementation below provides a generic flow that should
        be customized for your brokerage.
        """
        if not self.page:
            return OrderResult(success=False, error_msg="Browser not connected")

        try:
            # ── Navigate to trading page if needed ──
            self._ensure_trading_page()

            # ── Fill order form ──
            # NOTE: These selectors are placeholders. Customize for your brokerage.
            time.sleep(1)

            # Enter stock code
            code_input = self.page.locator('input[placeholder*="代码"], input[name="stockCode"]').first
            if code_input:
                code_input.click()
                code_input.fill(stock_code)
                time.sleep(0.5)

            # Select action (buy/sell)
            if action in ("buy", "add"):
                self.page.locator('text=买入, button:has-text("买入")').first.click()
            else:
                self.page.locator('text=卖出, button:has-text("卖出")').first.click()
            time.sleep(0.3)

            # Enter price
            if price > 0:
                price_input = self.page.locator('input[placeholder*="价格"]').first
                if price_input:
                    price_input.fill(str(price))

            # Enter quantity (must be multiple of 100)
            qty_input = self.page.locator('input[placeholder*="数量"]').first
            if qty_input:
                qty_input.fill(str(quantity))

            time.sleep(0.5)

            # Click confirm/submit button
            submit_btn = self.page.locator('button:has-text("确认"), button:has-text("下单")').first
            if submit_btn:
                submit_btn.click()

            time.sleep(1)

            # Handle confirmation dialog if present
            confirm_btn = self.page.locator('button:has-text("确定"), button:has-text("确认提交")').first
            if confirm_btn and confirm_btn.is_visible(timeout=2000):
                confirm_btn.click()
                time.sleep(0.5)

            # Check result
            if self._check_order_success():
                order_id = self._get_order_id()
                logger.info(f"Order placed: {stock_code} {action} {quantity}股 @ {price}")
                return OrderResult(
                    success=True,
                    order_id=order_id,
                    exec_price=price,
                    exec_qty=quantity,
                )
            else:
                error = self._get_error_message()
                logger.warning(f"Order failed: {error}")
                return OrderResult(success=False, error_msg=error)

        except Exception as e:
            logger.error(f"place_order error: {e}")
            return OrderResult(success=False, error_msg=str(e))

    def cancel_order(self, order_id: str) -> bool:
        """Cancel a pending order."""
        if not self.page:
            return False
        try:
            self._ensure_orders_page()
            # Placeholder — customize for your brokerage
            logger.info(f"Cancel order {order_id} (not implemented)")
            return False
        except Exception:
            return False

    def query_orders(self) -> list[dict]:
        """Query all pending/filled orders from the orders page.
        
        This is a stub — DOM selectors depend on the specific brokerage web platform.
        Customize for your brokerage by implementing table scraping.
        """
        if not self.page:
            return []
        try:
            self._ensure_orders_page()
            logger.info("query_orders: stub (customize DOM selectors for your brokerage)")
            return []
        except Exception as e:
            logger.error(f"query_orders error: {e}")
            return []

    def get_balance(self) -> dict | None:
        """Scrape account balance from the trading page.
        Returns dict with: total_assets, available_cash, market_value, total_profit
        """
        if not self.page:
            logger.warning("get_balance: browser not connected")
            return None
        try:
            self._ensure_asset_page()
            time.sleep(1)
            import re

            result = {}
            text = self.page.inner_text('body')
            logger.info(f"Balance page text (first 800 chars): {text[:800]}")

            # Try to parse balance from common patterns
            patterns = {
                'total_assets': r'总资产[：:]\s*[¥￥]?\s*([\d,]+\.?\d*)',
                'available_cash': r'可用[资金]?[：:]\s*[¥￥]?\s*([\d,]+\.?\d*)',
                'market_value': r'(?:持仓)?市值[：:]\s*[¥￥]?\s*([\d,]+\.?\d*)',
                'total_profit': r'(?:累计)?盈亏[：:]\s*[¥￥]?\s*([-\d,]+\.?\d*)',
            }
            for key, pat in patterns.items():
                m = re.search(pat, text)
                if m:
                    result[key] = float(m.group(1).replace(',', ''))

            return result if result else None

        except Exception as e:
            logger.error(f"get_balance error: {e}")
            return None

    def get_positions(self) -> list[dict]:
        """Scrape current positions from the trading page.
        Returns list of dicts: stock_code, stock_name, quantity, cost_price, current_price, pnl, pnl_pct
        """
        if not self.page:
            logger.warning("get_positions: browser not connected")
            return []
        try:
            self._ensure_positions_page()
            time.sleep(1)

            positions = []
            rows = self.page.locator('table tbody tr, .position-item, .holding-row, tr[class]').all()
            for row in rows:
                try:
                    cols = row.locator('td, .col, .cell').all()
                    if len(cols) < 3:
                        continue
                    texts = [c.inner_text().strip() for c in cols]
                    # Filter out header rows
                    if any(t in ('代码', '证券代码', '股票代码') for t in texts):
                        continue
                    pos = {
                        'stock_code': texts[0] if len(texts) > 0 else '',
                        'stock_name': texts[1] if len(texts) > 1 else '',
                        'quantity': int(texts[2].replace(',', '')) if len(texts) > 2 and texts[2].replace(',', '').lstrip('-').isdigit() else 0,
                        'cost_price': float(texts[3].replace(',', '')) if len(texts) > 3 else 0,
                        'current_price': float(texts[4].replace(',', '')) if len(texts) > 4 else 0,
                        'pnl': float(texts[5].replace(',', '')) if len(texts) > 5 else 0,
                        'pnl_pct': float(texts[6].replace('%', '').replace(',', '')) if len(texts) > 6 else 0,
                    }
                    positions.append(pos)
                except (ValueError, IndexError):
                    continue

            if not positions:
                # Fallback: dump table text
                text = self.page.inner_text('body')
                logger.info(f"Positions page text (first 500 chars): {text[:500]}")

            return positions

        except Exception as e:
            logger.error(f"get_positions error: {e}")
            return []

    def query_position(self, stock_code: str) -> Optional[Position]:
        """Query position for a stock (deprecated - use get_positions)."""
        if not self.page:
            return None
        try:
            positions = self.get_positions()
            for p in positions:
                if p['stock_code'] == stock_code:
                    return Position(
                        stock_code=stock_code,
                        quantity=p['quantity'],
                        avg_cost=p['cost_price'],
                        current_price=p['current_price'],
                    )
            return None
        except Exception:
            return None

    # ── Private helpers ──

    def _is_login_page(self):
        try:
            return bool(
                self.page.locator('input[type="password"]').first.is_visible(timeout=2000)
            )
        except Exception:
            return False

    def _wait_for_login(self, timeout_sec):
        deadline = time.time() + timeout_sec
        while time.time() < deadline:
            if not self._is_login_page():
                logger.info("Login detected, proceeding...")
                return
            time.sleep(2)
        raise TimeoutError("Login timed out")

    def _ensure_trading_page(self):
        """Navigate to the trading/order page."""
        # Override with brokerage-specific navigation
        pass

    def _ensure_orders_page(self):
        """Navigate to the orders page."""
        pass

    def _ensure_asset_page(self):
        """Navigate to the asset/balance page."""
        try:
            # Click on 资产 or 资金 menu
            for selector in ['text=资产', 'text=资金', 'text=账户', '.nav-account']:
                el = self.page.locator(selector).first
                if el.is_visible(timeout=1000):
                    el.click()
                    time.sleep(1)
                    return
        except Exception:
            pass

    def _ensure_positions_page(self):
        """Navigate to the positions page."""
        try:
            for selector in ['text=持仓', 'text=我的持仓', '.nav-positions', '.nav-holdings']:
                el = self.page.locator(selector).first
                if el.is_visible(timeout=1000):
                    el.click()
                    time.sleep(1)
                    return
        except Exception:
            pass

    def _check_order_success(self):
        """Check if order was successfully submitted."""
        try:
            # Check for success message
            return bool(
                self.page.locator('text=提交成功, text=委托成功, text=已申报').first.is_visible(timeout=3000)
            )
        except Exception:
            return False

    def _get_order_id(self):
        """Extract order ID from success message."""
        try:
            text = self.page.locator('.order-id, .entrust-no').first.inner_text()
            return text.strip()
        except Exception:
            return f"WEB-{int(time.time())}"

    def _get_error_message(self):
        """Extract error message from page."""
        try:
            return self.page.locator('.error-msg, .alert-danger').first.inner_text()
        except Exception:
            return "下单失败"
