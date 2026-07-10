"""macOS native app automation via AppleScript + screencapture + OCR.

Uses native macOS tools (screencapture, osascript) for window management
and screenshot capture, with pytesseract OCR for data extraction.
No pyautogui dependency required for basic operations.
"""

from __future__ import annotations

import logging
import os
import re
import subprocess
import tempfile
import time
from typing import Optional

from .base import AbstractTrader, OrderResult, Position

logger = logging.getLogger("agent.trader.pyautogui")


class PyAutoGUIMacTrader(AbstractTrader):
    """Automates a native macOS trading app using AppleScript + OCR."""

    def __init__(self, app_name="东方财富", screenshot_dir=None):
        self.app_name = app_name
        self.screenshot_dir = os.path.expanduser(
            screenshot_dir or os.path.join(os.getcwd(), "screenshots")
        )
        os.makedirs(self.screenshot_dir, exist_ok=True)

    def connect(self) -> bool:
        try:
            # Try importing pyautogui for keyboard operations
            import pyautogui
            pyautogui.FAILSAFE = False
            pyautogui.PAUSE = 0.3
            self._has_pyautogui = True
        except ImportError:
            self._has_pyautogui = False
            logger.info("pyautogui not installed — keyboard automation disabled, OCR only")
        return self._ensure_app_front()

    def disconnect(self):
        pass

    def _ensure_app_front(self):
        try:
            subprocess.run([
                "osascript", "-e",
                f'tell application "{self.app_name}" to activate'
            ], check=True, timeout=5)
            time.sleep(1)
            return True
        except Exception as e:
            logger.error(f"activate failed: {e}")
            return False

    def _capture_screenshot(self, region=None) -> str:
        """Capture screenshot using native screencapture. Returns file path."""
        path = os.path.join(self.screenshot_dir, f"shot_{int(time.time())}.png")
        cmd = ["screencapture", "-x"]
        if region:
            x, y, w, h = region
            cmd.extend(["-R", f"{x},{y},{w},{h}"])
        cmd.append(path)
        subprocess.run(cmd, check=True, timeout=10)
        logger.info(f"screenshot: {path}")
        return path

    def _ocr_text(self, screenshot_path=None, region=None):
        """Run OCR on a screenshot. Returns extracted text."""
        try:
            from PIL import Image
            import pytesseract

            if not screenshot_path:
                screenshot_path = self._capture_screenshot(region)
            img = Image.open(screenshot_path)
            text = pytesseract.image_to_string(img, lang='chi_sim+eng')
            return text
        except ImportError as e:
            logger.error(f"OCR missing: {e}")
            return ""

    def get_balance(self) -> dict | None:
        if not self._ensure_app_front():
            return None
        text = self._ocr_text()
        if not text:
            return None

        result = {}
        pats = {
            'total_assets': r'总资产[：:]\s*[¥￥]?\s*([\d,]+\.?\d*)',
            'available_cash': r'可用[资金]?[：:]\s*[¥￥]?\s*([\d,]+\.?\d*)',
            'market_value': r'(?:持仓)?市值[：:]\s*[¥￥]?\s*([\d,]+\.?\d*)',
            'total_profit': r'(?:累计)?(?:总)?盈亏[：:]\s*[¥￥]?\s*([\-\d,]+\.?\d*)',
        }
        for k, p in pats.items():
            m = re.search(p, text)
            if m:
                result[k] = float(m.group(1).replace(',', ''))

        if not result:
            logger.info(f"Balance OCR text (first 1000):\n{text[:1000]}")
        return result if result else None

    def get_positions(self) -> list[dict]:
        if not self._ensure_app_front():
            return []
        text = self._ocr_text()
        if not text:
            return []

        positions = []
        # Match 6-digit stock code + Chinese name + numeric fields
        pat = r'(\d{6})\s+([\u4e00-\u9fa5\*]{2,8})\s+.*?(\d+)\s+.*?([\d,]+\.?[\d]+)\s+.*?([\d,]+\.?[\d]+)\s+.*?([\-\d,]+\.?[\d]+)'
        for m in re.finditer(pat, text):
            try:
                positions.append({
                    'stock_code': m.group(1),
                    'stock_name': m.group(2).replace('*', ''),
                    'quantity': int(m.group(3).replace(',', '')),
                    'cost_price': float(m.group(4).replace(',', '')),
                    'current_price': float(m.group(5).replace(',', '')),
                    'pnl': float(m.group(6).replace(',', '')),
                })
            except (ValueError, IndexError):
                continue

        if not positions:
            logger.info(f"Positions OCR text (first 2000):\n{text[:2000]}")
        return positions

    def query_position(self, stock_code: str) -> Optional[Position]:
        for p in self.get_positions():
            if p['stock_code'] == stock_code:
                return Position(stock_code=stock_code, quantity=p['quantity'],
                                avg_cost=p['cost_price'], current_price=p['current_price'])
        return None

    def place_order(self, stock_code, stock_name, action, price, quantity) -> OrderResult:
        """⚠️ 已停用：盲操作快捷键无法确认下单结果，禁止用于真实下单。

        此前实现用同花顺 F1/F2/F3 快捷键盲按后直接返回 success=True 并伪造委托号，
        对东方财富无效且会导致"错误回传下单成功"。真实下单请使用 EastMoneyMacTrader。
        """
        logger.error(
            f"pyautogui trader 已停用（无法确认下单结果），拒绝下单 "
            f"{stock_name}({stock_code}) {action} {quantity}@{price}"
        )
        return OrderResult(
            success=False,
            error_msg="pyautogui trader 已停用：盲操作无法确认下单结果，请使用 eastmoney",
        )

    def cancel_order(self, order_id: str) -> bool:
        return False


    def query_orders(self) -> list[dict]:
        """查询全部委托列表（暂未实现）。"""
        logger.warning("pyautogui query_orders 暂未实现")
        return []
