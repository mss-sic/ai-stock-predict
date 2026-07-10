"""Trader factory — maps broker_mode to concrete trader implementations.

The AGENT's broker_mode is a LOCAL concept (independent from the server's
broker_mode).  It decides WHICH desktop client the agent controls:
  eastmoney_mac  → 东方财富 Mac 版 (AX API)
  eastmoney_web  → 东方财富网页版 (Playwright)
  lobster        → 龙虾客户端
"""

from __future__ import annotations

from .base import AbstractTrader

# Agent 端 broker_mode → 实现类名映射
_TRADER_REGISTRY: dict[str, str] = {
    "eastmoney_mac": "EastMoneyMacTrader",
    "eastmoney_web": "PlaywrightWebTrader",
    "lobster":       "LobsterTrader",
}


def create_trader(config: dict) -> AbstractTrader:
    """根据 config.broker_mode 创建对应的 trader 实例。
    
    Args:
        config: 完整配置字典（含 broker_mode 及各 trader 专属配置块）
    
    Returns:
        已实例化但未 connect() 的 trader 对象
    
    Raises:
        ValueError: broker_mode 不在注册表中
    """
    mode = config.get("broker_mode") or config.get("trader") or "eastmoney_mac"

    if mode not in _TRADER_REGISTRY:
        raise ValueError(
            f"不支持的 broker_mode: {mode}，"
            f"可选: {list(_TRADER_REGISTRY)}"
        )

    if mode == "eastmoney_mac":
        import os
        from .eastmoney_mac import EastMoneyMacTrader
        cfg = config.get("eastmoney", {})
        # 登录凭据优先读环境变量，其次配置文件（零硬编码/敏感信息不入库）
        trade_password = os.environ.get("EM_TRADE_PASSWORD", cfg.get("trade_password", ""))
        fund_account = os.environ.get("EM_FUND_ACCOUNT", cfg.get("fund_account", ""))
        return EastMoneyMacTrader(
            app_name=cfg.get("app_name", "东方财富"),
            confirm_order=cfg.get("confirm_order", True),
            action_delay=cfg.get("action_delay", 0.4),
            max_retries=cfg.get("max_retries", 5),
            calibration_path=cfg.get("calibration_path"),
            trade_mode=cfg.get("trade_mode", "simulated"),
            trade_password=trade_password,
            fund_account=fund_account,
        )

    elif mode == "eastmoney_web":
        from .playwright_web import PlaywrightWebTrader
        cfg = config.get("playwright", {})
        return PlaywrightWebTrader(
            trading_url=cfg.get("trading_url"),
            profile_dir=cfg.get("profile_dir"),
            headless=cfg.get("headless", False),
        )

    elif mode == "lobster":
        from .lobster import LobsterTrader
        cfg = config.get("lobster", {})
        return LobsterTrader(
            exe_path=cfg.get("exe_path", "/Applications/Lobster.app"),
            api_url=cfg.get("api_url"),
        )

    raise ValueError(f"未实现的 broker_mode: {mode}")


def get_trader_capabilities(mode: str) -> list[str]:
    """返回某 trader 支持的 capability 列表（用于 agent_hello 上报）。
    
    capability 与服务端下发的 command action 一一对应。
    """
    caps: dict[str, list[str]] = {
        "eastmoney_mac": [
            "place_order", "sync_positions", "get_balance",
            "query_orders", "get_account_info", "cancel_order",
        ],
        "eastmoney_web": [
            "place_order", "sync_positions", "get_balance",
            "query_orders",
        ],
        "lobster": [
            "place_order", "sync_positions", "get_balance",
        ],
    }
    return caps.get(mode, ["place_order", "sync_positions"])


def get_trader_type(config: dict) -> str:
    """返回当前配置的 trader 类型标识（用于 agent_hello.traderType）。"""
    return config.get("broker_mode") or config.get("trader") or "eastmoney_mac"


def list_registered_traders() -> list[str]:
    """返回所有已注册的 trader broker_mode 列表。"""
    return list(_TRADER_REGISTRY.keys())
