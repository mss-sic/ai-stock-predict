#!/usr/bin/env python3
"""智策投研 — 本地自动交易代理

两种运行模式:
  python3 agent.py --mode daemon   → 7×24 自动化守护进程 (WS + 轮询 + 自动下单)
  python3 agent.py --mode mcp      → MCP stdio 服务器 (供 AI Agent 调用)

Usage:
  python3 agent.py --mode daemon [--config config.yaml]
  python3 agent.py --mode mcp    [--config config.yaml]
"""

import argparse
import logging
import os
import signal
import sys
import time

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(__file__))

from core.auth import load_config, get_server_url, get_token
from core.api_client import APIClient
from core.ws_client import WSClient
from core.poller import SignalPoller
from core.signal_queue import SignalQueue
from core.reporter import Reporter
from core.preflight import run_preflight
from utils.logger import setup_logging
from utils.notify import send_notification
from traders.factory import create_trader, get_trader_capabilities, get_trader_type


# ── Daemon Mode ──

def run_daemon(config):
    """Run the 7×24 automated trading daemon."""
    log = setup_logging(
        level=config.get("log_level", "INFO"),
        log_file=config.get("log_file", "agent.log"),
    )
    log.info("=" * 60)
    log.info("智策投研 Agent Daemon 启动")
    log.info(f"Server: {get_server_url(config)}")
    log.info("=" * 60)

    # Initialize components
    api = APIClient(get_server_url(config), get_token(config))
    queue = SignalQueue()
    reporter = Reporter(api)

    # Verify connection（账户身份完全由 token 识别，无需本地 account_id）
    summary = api.get_account_summary()
    if summary:
        log.info(f"账户连接成功: {summary.get('accountName', 'N/A')} "
                 f"总资产={summary.get('totalAssets', 0):.0f} "
                 f"待执行={summary.get('pendingCount', 0)}")
    else:
        log.warning("无法获取账户信息，请检查 token 和网络连接")

    # ── 从服务端获取 trade_mode（账户类型决定实盘/模拟盘）──
    trade_mode = api.get_trade_mode()
    mode = config.get("broker_mode") or config.get("trader") or "eastmoney_mac"
    log.info(f"trade_mode from server: {trade_mode} (broker_mode={mode})")

    # 注入到 config 中供 preflight/factory 使用
    config["eastmoney"] = config.get("eastmoney", {})
    config["eastmoney"]["trade_mode"] = trade_mode

    # ── 启动预检（授权/APP/校准/连接/账户一致性），不过则阻断退出 ──
    log.info("开始启动预检...")
    trader = run_preflight(config, api=api)
    if trader is None:
        log.error("启动预检未通过，交易代理退出。请按弹窗提示修复后重启。")
        return
    log.info("启动预检通过，交易代理进入就绪状态")
    send_notification("Agent 已启动",
                      f"账户: {summary.get('accountName', 'N/A')}, "
                      f"待执行信号: {summary.get('pendingCount', 0)}")

    # ── Signal handler: when new signal arrives ──
    def on_signal(signal_data):
        """Called when a new signal arrives (from WS or poller)."""
        queue.put(signal_data)

    def on_kicked(reason):
        """Called when server kicks this agent."""
        log.warning(f"被服务器踢出: {reason}")
        send_notification("Agent 被踢出", f"原因: {reason}")

    def on_test_request(request_id):
        """Called when server sends a connectivity test challenge."""
        log.info(f"收到测试请求: {request_id[:8]}...")
        try:
            result = api.respond_test(request_id, success=True, message="agent is alive and connected")
            if result:
                log.info(f"测试响应已发送: {request_id[:8]}")
            else:
                log.warning(f"测试响应发送失败: {request_id[:8]}")
        except Exception as e:
            log.error(f"测试响应异常: {e}")

    # ── Generic command dispatcher (trader-type agnostic) ──
    def _handle_place_order(trader, api, request_id, payload):
        stock_name = payload.get("stockName", "?")
        action = payload.get("orderType", "buy")
        qty = int(payload.get("quantity", 0))
        price = float(payload.get("price", 0))
        result = trader.place_order(
            stock_code=payload.get("stockCode", ""),
            stock_name=stock_name,
            action=action,
            price=price,
            quantity=qty,
        )
        if result.success:
            api.respond_command(request_id, "ok", {
                "orderId": result.order_id,
                "execPrice": result.exec_price,
                "execQty": result.exec_qty,
                "statusText": getattr(result, "status_text", ""),
                "filledQty": getattr(result, "filled_qty", 0),
            })
            status_text = getattr(result, "status_text", "") or "委托中"
            send_notification(
                f"✅ {stock_name} {action} 已委托",
                f"{qty}股 @ {result.exec_price} 委托编号: {result.order_id} 状态: {status_text}")
        else:
            err = result.error_msg or "place_order failed"
            api.respond_command(request_id, "failed", error=err)
            send_notification(f"❌ {stock_name} 下单失败", err)

    def _handle_sync_positions(trader, api, request_id, payload):
        # trader 返回 snake_case，需转换为后端 BrokerPortfolio/BrokerPosition
        # 期望的 camelCase 字段（见 server broker_service.go）
        raw_positions = trader.get_positions() if trader else []
        balance = trader.get_balance() if trader else None
        positions = [{
            "secCode": p.get("stock_code", ""),
            "secName": p.get("stock_name", ""),
            "count": p.get("quantity", 0),
            "availCount": p.get("available", 0),
            "costPrice": p.get("cost_price", 0),
            "price": p.get("current_price", 0),
            "value": p.get("market_value", 0),
            "profit": p.get("pnl", 0),
            "profitPct": p.get("pnl_pct", 0),
        } for p in raw_positions]
        result = {"positions": positions, "posCount": len(positions)}
        if balance:
            result["totalAssets"] = balance.get("total_assets", 0)
            result["availBalance"] = balance.get("available_cash", 0)
            result["totalPosValue"] = balance.get("market_value", 0)
            result["totalProfit"] = balance.get("total_profit", 0)
        api.respond_command(request_id, "ok", result)
        send_notification("✅ 持仓同步完成", f"{len(positions)} 只持仓已上报"
                          + (f"，总资产 {balance.get('total_assets', 0):,.0f}" if balance else ""))

    def _handle_get_balance(trader, api, request_id, payload):
        balance = trader.get_balance() if trader else None
        if balance:
            api.respond_command(request_id, "ok", balance)
            send_notification("✅ 资金查询完成",
                              f"总资产 {balance.get('total_assets', 0):,.0f}，可用 {balance.get('available_cash', 0):,.0f}")
        else:
            api.respond_command(request_id, "failed", error="unable to read balance")
            send_notification("❌ 资金查询失败", "无法读取资金信息")

    def _handle_cancel_order(trader, api, request_id, payload):
        order_id = payload.get("orderId", "")
        ok = trader.cancel_order(order_id) if trader else False
        if ok:
            api.respond_command(request_id, "ok", {})
            send_notification("✅ 撤单成功", f"委托 {order_id} 已撤销")
        else:
            api.respond_command(request_id, "failed", error="cancel_order not supported or failed")
            send_notification("❌ 撤单失败", f"委托 {order_id} 撤销未成功")

    def _handle_query_orders(trader, api, request_id, payload):
        orders = trader.query_orders() if trader else []

        def map_status(text: str) -> int:
            """东财状态文本 → 后端 mxStatusMap int。子串匹配，先长后短防误判。"""
            t = (text or "").strip()
            if not t:
                return 0
            # 顺序敏感：先匹配更具体的复合状态
            rules = [
                (("全部成交", "已成交", "全成"), 4),
                (("部成待撤",), 5),
                (("已报待撤",), 6),
                (("部撤",), 7),
                (("已撤", "撤单"), 8),
                (("废单", "拒单", "作废"), 9),
                (("部分成交", "部成"), 3),
                (("已报", "已申报", "正报", "委托", "待成交"), 2),
                (("未报", "待报"), 1),
                (("已成",), 4),  # 放最后，避免与「已成交」冲突后又误吞
            ]
            for keys, code in rules:
                if any(k in t for k in keys):
                    return code
            return 0

        mapped = []
        for o in orders:
            raw_status = o.get("status") or ""
            code = map_status(raw_status)
            if code == 0:
                log.warning(f"未识别委托状态: '{raw_status}' (合同={o.get('contract_id')})")
            mapped.append({
                "orderId":    o.get("contract_id", ""),
                "stockCode":  o.get("stock_code", ""),
                "stockName":  o.get("stock_name", ""),
                "orderType":  o.get("operation", ""),
                "price":      float(o.get("order_price", 0) or 0),
                "tradePrice": float(o.get("avg_price", 0) or 0),
                "quantity":   int(o.get("order_qty", 0) or 0),
                "filledQty":  int(o.get("filled_qty", 0) or 0),
                "status":     code,
                "createTime": o.get("apply_time", ""),
            })
        api.respond_command(request_id, "ok", mapped)
        # 分类统计：status==4 为已成交，其余为委托中/其它
        filled = sum(1 for m in mapped if m["status"] == 4)
        pending = len(mapped) - filled
        ids = ", ".join(m["orderId"] for m in mapped if m["orderId"]) or "无"
        send_notification(
            f"✅ 查询到 {len(mapped)} 笔订单",
            f"委托中/其它 {pending} 笔，已成交 {filled} 笔｜合同号: {ids}")

    def _handle_get_account_info(trader, api, request_id, payload):
        name = trader.get_account_name() if trader and hasattr(trader, "get_account_name") else ""
        api.respond_command(request_id, "ok", {"accountName": name or "unknown"})
        send_notification("✅ 账户查询完成", f"账户名: {name or 'unknown'}")

    COMMAND_HANDLERS = {
        "sync_positions":  _handle_sync_positions,
        "get_balance":     _handle_get_balance,
        "place_order":     _handle_place_order,
        "cancel_order":    _handle_cancel_order,
        "query_orders":    _handle_query_orders,
        "get_account_info": _handle_get_account_info,
    }

    def on_command(request_id, action, payload):
        """Called when server dispatches a broker command via WS."""
        action_names = {
            "sync_positions": "同步持仓", "get_balance": "查询资金",
            "cancel_order": "撤销委托", "query_orders": "查询委托",
            "get_account_info": "查询账户",
            # place_order is handled in signal flow
        }
        label = action_names.get(action, action)
        log.info(f"收到指令: {action} requestID={request_id[:8]}")
        send_notification(f"📡 收到指令: {label}", f"requestID={request_id[:8]}...")
        try:
            handler = COMMAND_HANDLERS.get(action)
            if handler:
                handler(trader, api, request_id, payload)
            else:
                log.warning(f"未知指令 action={action}，忽略")
                send_notification(f"⚠️ 未知指令: {action}", "该指令不被支持")
                api.respond_command(request_id, "failed", error=f"unknown action: {action}")
        except Exception as e:
            log.error(f"指令处理异常 {action}: {e}", exc_info=True)
            send_notification(f"❌ 指令失败: {label}", str(e))
            api.respond_command(request_id, "failed", error=str(e))

    # ── WS connect handler: send agent_hello ──
    def on_ws_open(ws):
        """Send agent_hello on WebSocket connect to report capabilities.

        格式：{ type: "agent_hello", data: { traderType, capabilities, ... } }
        与服务端 ws/handler.go readPump 的解析结构一致。
        """
        import json
        hello = {
            "type": "agent_hello",
            "data": {
                "traderType": mode,
                "capabilities": get_trader_capabilities(mode),
                "version": "1.0.0",
            },
        }
        try:
            ws.send(json.dumps(hello, ensure_ascii=False))
            log.info(f"agent_hello sent: traderType={mode} caps={get_trader_capabilities(mode)}")
        except Exception as e:
            log.warning(f"agent_hello send failed: {e}")

    # Start WebSocket client (real-time push)
    ws_enabled = config.get("ws_enabled", True)
    ws_client = None
    if ws_enabled:
        ws_client = WSClient(
            server_url=get_server_url(config),
            token=get_token(config),
            on_signal=on_signal,
            on_kicked=on_kicked,
            on_test_request=on_test_request,
            on_command=on_command,
            on_open=on_ws_open,
        )
        ws_client.start()

    # Start poller (fallback)
    poll_interval = config.get("poll_interval", 30)
    poller = SignalPoller(api, interval=poll_interval, on_signals=on_signal)
    poller.start()

    # ── Main execution loop ──
    # trader 已在启动预检中连接并通过账户校验，全程复用该单一实例。
    # 具体 trader 类型由 config.broker_mode 决定（eastmoney_mac / eastmoney_web / lobster）。
    # 注意：place_order 内部已按 max_retries 做「填单/校验失败」重试，
    # 主循环不再叠加重试，避免重复下单。
    mode = get_trader_type(config)
    max_retries = int(config.get(mode if mode.startswith("eastmoney") else mode, {}).get("max_retries", 5))
    log.info(f"主循环启动，等待信号...（下单内部最多重试 {max_retries} 次）")

    running = True

    def shutdown(sig, frame):
        nonlocal running
        log.info("收到退出信号，正在关闭...")
        running = False

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    while running:
        try:
            signal_data = queue.get(timeout=5)
            if signal_data is None:
                continue

            sid = signal_data.get("signalId") or signal_data.get("signal_id")
            stock_code = signal_data.get("stockCode") or signal_data.get("stock_code", "?")
            stock_name = signal_data.get("stockName") or signal_data.get("stock_name", "?")
            action = signal_data.get("actionType") or signal_data.get("action", "?")
            price = float(signal_data.get("price", 0) or 0)
            qty = int(signal_data.get("quantity", 0) or signal_data.get("plannedQty", 0) or 0)

            log.info(f"📥 收到信号 #{sid}: {stock_name}({stock_code}) {action} {qty}股 @ {price}")
            send_notification(
                f"📥 策略信号 #{sid}", f"{stock_name}({stock_code}) {action} {qty}股 @ {price}")

            # 认领信号（CAS：pending_auto → claimed），防重复执行
            claimed = reporter.claim(sid)
            if not claimed:
                log.warning(f"认领失败 #{sid}，跳过")
                send_notification(f"⚠️ 信号认领失败 #{sid}", "可能已被其他代理认领")
                continue

            # 下单执行（内部已含 max_retries 次校验重试，此处只调用一次）
            try:
                result = trader.place_order(
                    stock_code, stock_name, action, price, qty)
            except Exception as e:
                log.error(f"下单异常 #{sid}: {e}", exc_info=True)
                result = None

            if result and result.success:
                # 委托已提交成功（状态=委托中），回传券商委托编号
                reporter.report_submitted(
                    sid, result.order_id, result.exec_price, result.exec_qty)
                status_text = getattr(result, "status_text", "") or "委托中"
                send_notification(
                    f"✅ {stock_name} {action} 已委托",
                    f"{qty}股 @ {result.exec_price} 委托编号: {result.order_id} 状态: {status_text}")
                log.info(f"✅ 委托成功 #{sid}: 委托编号={result.order_id} 状态={status_text}")
            else:
                # 内部重试后仍失败：上报 order_failed + 本地日志 + 通知（后端不动）
                err = (result.error_msg if result else "下单过程异常") or "未知错误"
                reporter.report_failed(sid, f"下单失败: {err}")
                log.error(f"❌ 下单失败 #{sid}（已重试至 {max_retries} 次）: {err}")
                send_notification(
                    f"❌ {stock_name} 下单失败",
                    f"重试 {max_retries} 次仍失败，请手动处理。原因: {err}")

        except Exception as e:
            log.error(f"主循环异常: {e}", exc_info=True)
            time.sleep(5)

    # Cleanup
    log.info("正在停止组件...")
    try:
        trader.disconnect()
    except Exception as e:
        log.warning(f"trader 断开异常: {e}")
    if ws_client:
        ws_client.stop()
    poller.stop()
    log.info("Agent 已退出")


# ── MCP Mode ──

def run_mcp(config=None):
    """Run as an MCP stdio server for AI agent integration."""
    log = logging.getLogger("agent.mcp")
    log.info("MCP server starting (stdio mode)...")

    from mcp.server import AgentMCPServer
    server = AgentMCPServer()
    server.run()


# ── Main ──


# ── Test Broker Mode ──

def run_test_broker(config=None, trader_name="playwright"):
    """Connect to 东方财富 web trading, dump account balance and positions."""
    import json
    import time
    from core.auth import load_config
    if config is None:
        config = load_config()

    print("=" * 60)
    print("🧪 券商连接测试")
    print("=" * 60)

    # trader_name passed from main
    if trader_name == "eastmoney":
        from traders.eastmoney_mac import EastMoneyMacTrader
        from core.api_client import APIClient
        from core.auth import get_server_url, get_token
        # 从服务端获取 trade_mode
        api = APIClient(get_server_url(config), get_token(config))
        trade_mode = api.get_trade_mode()
        print(f"   账户类型: {trade_mode} (来自服务端)")

        em_cfg = config.get("eastmoney", {})
        trader = EastMoneyMacTrader(
            app_name=em_cfg.get("app_name", "东方财富"),
            trade_mode=trade_mode,
            confirm_order=em_cfg.get("confirm_order", True),
            action_delay=em_cfg.get("action_delay", 0.4),
            calibration_path=em_cfg.get("calibration_path"),
        )
        print(f"   方式: 东方财富 Mac 原生 APP ({em_cfg.get('app_name', '东方财富')})")
    elif trader_name == "pyautogui":
        from traders.pyautogui_mac import PyAutoGUIMacTrader
        pa_cfg = config.get("pyautogui", {})
        trader = PyAutoGUIMacTrader(
            app_name=pa_cfg.get("app_name", "东方财富"),
            screenshot_dir=pa_cfg.get("screenshot_dir"),
        )
        print(f"   方式: macOS APP ({pa_cfg.get('app_name', '东方财富')})")
    else:
        from traders.playwright_web import PlaywrightWebTrader
        pw_cfg = config.get("playwright", {})
        trader = PlaywrightWebTrader(
            trading_url=pw_cfg.get("trading_url"),
            profile_dir=pw_cfg.get("profile_dir"),
            headless=pw_cfg.get("headless", False),
        )
        print(f"   方式: 网页 ({pw_cfg.get('trading_url', 'https://jywg.18.cn/')})")

    print("\n⏳ 连接交易端...")
    if not trader.connect():
        print("❌ 交易端连接失败")
        return

    try:
        # ── Balance ──
        print("\n📊 查询账户资金...")
        balance = trader.get_balance()
        if balance:
            print(f"   总资产:     ¥{balance.get('total_assets', 0):,.2f}")
            print(f"   可用资金:   ¥{balance.get('available_cash', 0):,.2f}")
            print(f"   持仓市值:   ¥{balance.get('market_value', 0):,.2f}")
            print(f"   累计盈亏:   ¥{balance.get('total_profit', 0):,.2f}")
        else:
            print("   ⚠️ 未能自动解析资金数据，请查看浏览器窗口")

        # ── Positions ──
        print("\n📋 查询持仓...")
        positions = trader.get_positions()
        if positions:
            print(f"   持仓数量: {len(positions)}")
            print(f"   {'代码':<8} {'名称':<12} {'数量':>8} {'成本':>10} {'现价':>10} {'盈亏':>12}")
            print("   " + "-" * 64)
            for p in positions:
                print(f"   {p.get('stock_code',''):<8} {p.get('stock_name',''):<12} {p.get('quantity',0):>8} {p.get('cost_price',0):>10.3f} {p.get('current_price',0):>10.2f} {p.get('pnl',0):>12.2f}")
        else:
            print("   ⚠️ 未能解析持仓数据")

        # ── Keep browser open ──
        print("\n⏸  按 Enter 键退出...")
        try:
            input()
        except (KeyboardInterrupt, EOFError):
            pass

    except KeyboardInterrupt:
        print("\n👋 用户中断")
    except Exception as e:
        print(f"\n❌ 错误: {e}")
    finally:
        trader.disconnect()
        print("\n✅ 测试完成")


def run_calibrate(config=None):
    """交互式校准东方财富「持仓/成交/委托」标签坐标并持久化。

    东财这三个 tab 是自绘控件、AX 不可见，只能靠屏幕坐标点击。此模式引导用户
    手动依次点击三个标签，用鼠标监听捕获坐标并存为相对窗口偏移，供后续下单/
    读取时按当前窗口位置还原使用。
    """
    from core.auth import load_config
    from traders import tab_calibrator
    from traders.eastmoney_mac import EastMoneyMacTrader

    if config is None:
        config = load_config()
    em_cfg = config.get("eastmoney", {})
    app_name = em_cfg.get("app_name", "东方财富")

    # 借用 trader 的连接/交易页切换/视图识别能力做即时校验
    trader = EastMoneyMacTrader(
        app_name=app_name,
        action_delay=em_cfg.get("action_delay", 0.4),
        calibration_path=em_cfg.get("calibration_path"),
    )
    # 仅需 AX/pyautogui 就绪，不要求已有校准，故直接做前置准备而非 connect()
    import pyautogui
    from traders import ax_helper as ax
    if not ax.ax_available() or not ax.is_trusted():
        print("❌ 未获得辅助功能授权或 pyobjc 不可用，无法校准")
        return
    trader._pa = pyautogui
    trader._pid = ax.find_app_pid(app_name)
    if not trader._pid:
        print(f"❌ 未找到正在运行的「{app_name}」，请先启动并登录")
        return
    trader._app = ax.app_element(trader._pid)

    def _goto():
        trader._activate()
        return trader._goto_trade_tab()

    tab_calibrator.run_calibration(
        app_name=app_name,
        detect_view=trader._detect_view,
        goto_trade=_goto,
        path=em_cfg.get("calibration_path"),
    )


def main():
    parser = argparse.ArgumentParser(description="智策投研 本地自动交易代理")
    parser.add_argument("--mode", choices=["daemon", "mcp", "test-broker", "calibrate"], default="daemon",
                    help="运行模式")
    parser.add_argument("--trader", choices=["eastmoney", "playwright", "pyautogui", "lobster"], default="eastmoney",
                    help="test-broker 使用的 trader")
    parser.add_argument("--config", help="配置文件路径 (默认: config.yaml)")
    args = parser.parse_args()

    if args.mode == "mcp":
        run_mcp()
    elif args.mode == "test-broker":
        config = load_config(args.config)
        run_test_broker(config, getattr(args, "trader", "playwright"))
    elif args.mode == "calibrate":
        config = load_config(args.config)
        run_calibrate(config)
    else:
        config = load_config(args.config)
        run_daemon(config)


if __name__ == "__main__":
    main()
