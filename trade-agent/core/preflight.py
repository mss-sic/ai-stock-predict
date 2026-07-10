"""启动预检编排 — 交易代理运行前的环境与账户一致性检查。

将原先散落在 trader.connect() 与主循环中的检查，统一编排为按顺序执行、
每步都有明确用户引导（系统弹窗 + 日志）的预检流程。任一关键步骤失败即阻断
自动交易，避免"环境不对却照常下单"。

预检顺序（对应用户描述的启动流程）：
  1. 依赖与辅助功能授权（pyobjc / 辅助功能 / 输入监控 / pyautogui）
  2. 东方财富 APP 是否运行并可读窗口（未开引导打开并登录）
  3. 「持仓/成交/委托」tab 坐标是否已校准（未校准引导校准）
  4. trader 连接与关键控件自检（下单区/表头可定位）
  5. 当前登录证券账户名 与 后端绑定账户名 是否一致（不一致阻断）

目前仅支持东方财富 Mac 版；账户异常/环境异常按约定仅本地日志 + 系统弹窗，
不依赖后端上报接口。
"""

from __future__ import annotations

import logging
import re
from typing import Optional

from traders import ax_helper as ax
from utils.notify import show_dialog, send_notification

logger = logging.getLogger("agent.preflight")

_TITLE = "智策投研 交易代理 · 启动检查"


def _fail(msg: str, dialog_msg: Optional[str] = None,
          buttons=("我知道了",)) -> bool:
    """统一的失败处理：写错误日志 + 弹阻断式对话框，返回 False。"""
    logger.error(f"[preflight] {msg}")
    show_dialog(dialog_msg or msg, title=_TITLE, buttons=buttons)
    return False


def _check_dependencies() -> bool:
    """步骤 1：校验 pyobjc、辅助功能授权、输入监控、pyautogui。"""
    if not ax.ax_available():
        return _fail(
            "pyobjc 未安装，无法使用 AX 自动化",
            "缺少依赖 pyobjc，无法控制东方财富。\\n\\n"
            "请在终端执行：\\n"
            "pip3 install pyobjc pyautogui\\n\\n"
            "安装后重新启动交易代理。",
        )
    if not ax.is_trusted():
        return _fail(
            "未获得 macOS 辅助功能授权",
            "交易代理未获得【辅助功能】授权，无法读取/操作东方财富。\\n\\n"
            "请打开：系统设置 → 隐私与安全性 →\\n"
            "  ① 辅助功能：添加并勾选运行本代理的程序（终端/IDE）\\n"
            "  ② 输入监控：同样添加并勾选\\n\\n"
            "授权后请【重启该程序】再运行交易代理。",
        )
    try:
        import pyautogui  # noqa: F401
    except ImportError:
        return _fail(
            "pyautogui 未安装",
            "缺少依赖 pyautogui，无法执行点击/输入。\\n\\n"
            "请在终端执行：pip3 install pyautogui\\n然后重新启动。",
        )
    logger.info("[preflight] ① 依赖与授权检查通过")
    return True


def _check_app_running(app_name: str) -> bool:
    """步骤 2：东方财富是否运行且窗口可读，未运行则引导打开并登录。"""
    import time
    pid = ax.find_app_pid(app_name)
    if not pid:
        return _fail(
            f"未找到正在运行的「{app_name}」",
            f"未检测到「{app_name}」正在运行。\\n\\n"
            f"请先打开「{app_name}」并完成登录，\\n"
            f"然后重新启动交易代理。\\n\\n"
            f"（当前仅支持东方财富 Mac 版）",
        )
    app = ax.app_element(pid)
    ax.activate_app(app_name)
    for _ in range(10):
        time.sleep(0.4)
        if ax.main_window(app) is not None:
            logger.info(f"[preflight] ② 「{app_name}」运行正常 (pid={pid})")
            return True
    return _fail(
        f"「{app_name}」无可用窗口",
        f"「{app_name}」已启动但读不到交易窗口。\\n\\n"
        f"请确认已登录且交易窗口已打开，然后重试。",
    )


def _check_calibration(config, app_name: str) -> bool:
    """步骤 3：tab 坐标是否已校准；未校准则弹窗引导立即校准。"""
    from traders import tab_calibrator
    em_cfg = config.get("eastmoney", {})
    path = em_cfg.get("calibration_path")
    data = tab_calibrator.load_calibration(path)
    if data and isinstance(data.get("offsets"), dict) and \
            len({"chicang", "chengjiao", "weituo"} & set(data["offsets"])) == 3:
        logger.info("[preflight] ③ tab 坐标校准已存在")
        return True

    logger.warning("[preflight] ③ 未找到 tab 坐标校准，引导用户校准")
    choice = show_dialog(
        "尚未校准「持仓/成交/委托」标签位置。\\n\\n"
        "这三个标签是东财自绘控件，需手动点一次记录坐标。\\n"
        "点『开始校准』后按弹窗提示依次点击三个标签。",
        title=_TITLE, buttons=("取消", "开始校准"), default="开始校准",
    )
    if choice != "开始校准":
        return _fail(
            "用户取消校准",
            "未完成校准，无法自动交易。\\n"
            "可稍后运行：python3 agent.py --mode calibrate",
        )

    # 借用 trader 做视图校验回调
    from traders.eastmoney_mac import EastMoneyMacTrader
    import pyautogui
    trader = EastMoneyMacTrader(
        app_name=app_name,
        action_delay=em_cfg.get("action_delay", 0.4),
        calibration_path=path,
    )
    trader._pa = pyautogui
    trader._pid = ax.find_app_pid(app_name)
    trader._app = ax.app_element(trader._pid)

    def _goto():
        trader._activate()
        return trader._goto_trade_tab()

    result = tab_calibrator.run_calibration(
        app_name=app_name,
        detect_view=trader._detect_view,
        goto_trade=_goto,
        path=path,
    )
    if not result:
        return _fail("校准未完成", "校准未完成或失败，无法自动交易。")
    logger.info("[preflight] ③ tab 坐标校准完成")
    return True


def _norm_account(s: str) -> str:
    """归一化账户名用于比对：去空格、去括号及其后缀内容。

    东财实盘账户名形如「李江波 (0472)」，后端资金账号形如「李江波」。
    归一化后取「李江波」用于包含/前缀匹配。
    """
    if not s:
        return ""
    # 去掉中英文括号及其后内容（营业部代码等）
    s = re.split(r"[（(]", s, maxsplit=1)[0]
    return s.replace(" ", "").replace("\u3000", "").strip()


def _check_account(trader, api) -> bool:
    """步骤 5：当前东财登录账户 与 后端绑定资金账号 一致性校验。

    东财交易模式(普通/模拟)tab 为自绘控件、AX 读不到文字，改用「证券账户名」
    与后端 accountNumber（资金账号，对应券商侧账户名）做身份比对：
      - 后端 accountNumber 形如「李江波」；东财实盘账户名形如「李江波 (0472)」
      - 归一化去括号后缀后做包含匹配（互为前缀即视为一致）
    """
    local_name = trader.get_account_name()
    if not local_name:
        # 读不到账户名不直接阻断（可能界面布局差异），仅告警放行
        logger.warning("[preflight] ⑤ 未能读取东财账户名，跳过账户一致性校验")
        send_notification("⚠️ 账户校验跳过", "未能读取东财账户名，请留意是否登录正确账户")
        return True

    # 优先用后端资金账号(accountNumber)比对；回退到 account-summary 的 accountName
    account = api.get_account() if api else {}
    remote_no = (account or {}).get("accountNumber", "")
    if not remote_no:
        summary = api.get_account_summary() if api else {}
        remote_no = (summary or {}).get("accountName", "")
    if not remote_no:
        logger.warning("[preflight] ⑤ 后端未返回绑定资金账号，跳过一致性校验")
        return True

    ln, rn = _norm_account(local_name), _norm_account(remote_no)
    # 互为包含即视为一致（李江波 ⊂ 李江波(0472)）
    if ln and rn and (ln in rn or rn in ln):
        logger.info(f"[preflight] ⑤ 账户校验通过：东财={local_name} 后端={remote_no}")
        return True

    return _fail(
        f"账户不一致：东财登录={local_name} 后端资金账号={remote_no}",
        f"⚠️ 登录账户与绑定资金账号不一致，已阻断自动交易！\\n\\n"
        f"东财当前登录：{local_name}\\n"
        f"后端绑定账号：{remote_no}\\n\\n"
        f"请在东方财富登录到正确的交易账户后重启交易代理。",
    )


def _check_login_status(trader) -> bool:
    """步骤 6（仅实盘）：检查交易账户登录状态，未登录时尝试自动登录。

    实盘账户有约 180 分钟在线时长，超时后「登录状态」变为未登录。
    处理策略：
      - 已登录 / 模拟盘（无登录状态标签）→ 放行
      - 未登录且配置了交易密码 → 自动登录（OCR 识别验证码），成功放行
      - 未登录且未配置密码 或 自动登录失败 → 阻断并弹窗引导手动登录
    """
    status = trader.get_login_status() if hasattr(trader, "get_login_status") else None
    if status is None:
        logger.info("[preflight] ⑥ 无登录状态标签（模拟盘/无需登录），放行")
        return True
    if "已登录" in status:
        logger.info(f"[preflight] ⑥ 交易账户已登录：{status}")
        return True

    # 未登录：若配置了交易密码，尝试自动登录
    has_pwd = bool(getattr(trader, "trade_password", ""))
    if has_pwd and hasattr(trader, "login"):
        logger.warning(f"[preflight] ⑥ 交易账户未登录（{status}），尝试自动登录…")
        send_notification("🔐 自动登录中", "检测到交易账户未登录，正在自动登录…")
        try:
            if trader.login():
                logger.info("[preflight] ⑥ 自动登录成功")
                send_notification("✅ 自动登录成功", "交易账户已登录")
                return True
        except Exception as e:
            logger.error(f"[preflight] ⑥ 自动登录异常: {e}", exc_info=True)
        logger.error("[preflight] ⑥ 自动登录失败")

    return _fail(
        f"实盘交易账户未登录：{status}",
        f"⚠️ 实盘交易账户当前【{status}】，无法下单！\\n\\n"
        f"实盘账户在线时长有限（约180分钟），超时需重新登录。\\n"
        + ("自动登录失败（验证码/密码错误或界面异常）。\\n" if has_pwd else "未配置交易密码，无法自动登录。\\n")
        + f"请在东方财富点击「登录证券账户」，输入账号/密码/验证码完成登录，\\n"
        f"然后重启交易代理。",
    )


def _run_preflight_eastmoney(config, api, em_cfg, app_name) -> Optional[object]:
    """东方财富 Mac 版专用预检。"""
    # ② APP 运行
    if not _check_app_running(app_name):
        return None
    # ③ tab 校准
    if not _check_calibration(config, app_name):
        return None
    # ④ trader 连接 + 自检
    import os
    from traders.eastmoney_mac import EastMoneyMacTrader
    trade_mode = api.get_trade_mode() if api else "real"
    # 登录凭据：优先环境变量，其次配置文件（与 factory 保持一致）
    trade_password = os.environ.get("EM_TRADE_PASSWORD", em_cfg.get("trade_password", ""))
    fund_account = os.environ.get("EM_FUND_ACCOUNT", em_cfg.get("fund_account", ""))
    trader = EastMoneyMacTrader(
        app_name=app_name,
        trade_mode=trade_mode,
        confirm_order=em_cfg.get("confirm_order", True),
        action_delay=em_cfg.get("action_delay", 0.4),
        max_retries=em_cfg.get("max_retries", 5),
        calibration_path=em_cfg.get("calibration_path"),
        trade_password=trade_password,
        fund_account=fund_account,
    )
    if not trader.connect():
        _fail("trader 连接/自检失败",
              "连接东方财富或关键控件自检失败。\\n\\n"
              "常见原因：交易页布局变化 / 未登录 / 窗口尺寸异常。\\n"
              "请确认已登录交易页后重试，必要时重新校准。")
        return None
    logger.info("[preflight] ④ trader 连接与控件自检通过")
    # ⑤ 账户名一致性
    if not _check_account(trader, api):
        try:
            trader.disconnect()
        except Exception:
            pass
        return None
    # ⑥ 实盘登录状态（模拟盘自动放行）
    if not _check_login_status(trader):
        try:
            trader.disconnect()
        except Exception:
            pass
        return None
    return trader


def _run_preflight_stub(config, api, mode: str) -> Optional[object]:
    """通用 stub 预检：仅做依赖检查 + trader 创建 + 连接。"""
    from traders.factory import create_trader
    try:
        trader = create_trader(config)
    except ValueError as e:
        _fail(f"trader 创建失败: {e}")
        return None
    if not trader.connect():
        _fail(f"{mode} trader 连接失败",
              f"无法连接 {mode} 交易端。\\n\\n"
              f"请确认客户端已启动并登录，然后重试。")
        return None
    logger.info(f"[preflight] {mode} trader 连接通过")
    return trader


# mode → preflight function mapping
_PREFLIGHT_BY_MODE = {
    "eastmoney_mac": _run_preflight_eastmoney,
    "eastmoney_web": _run_preflight_stub,
    "lobster":       _run_preflight_stub,
}


def run_preflight(config, api=None, trader=None) -> Optional[object]:
    """执行完整启动预检。

    按 config.broker_mode（兼容旧 config.trader）选择对应预检流程：
      eastmoney_mac  → 依赖→APP运行→校准→自检→账户校验
      eastmoney_web  → 依赖→创建→连接
      lobster        → 依赖→创建→连接

    Args:
        config: 已加载的配置字典
        api: APIClient 实例（可选，用于账户校验）
        trader: 可选，外部传入的 trader（为 None 时按 mode 创建）
    Returns:
        通过则返回已 connect() 的 trader 实例；任一步失败返回 None。
    """
    mode = (config.get("broker_mode") or config.get("trader") or "eastmoney_mac")

    # ① 依赖与授权（所有模式通用）
    if not _check_dependencies():
        return None

    # 如果外部传入了 trader（如 calibrate 后复用），直接校验账户
    if trader is not None:
        if not trader.connect():
            return None
        if mode == "eastmoney_mac":
            if not _check_account(trader, api):
                return None
            if not _check_login_status(trader):
                return None
        logger.info(f"[preflight] ✅ {mode} 启动检查通过")
        return trader

    # 按 mode 选择预检流程
    preflight_fn = _PREFLIGHT_BY_MODE.get(mode)
    if preflight_fn is None:
        _fail(f"不支持的 broker_mode: {mode}",
              f"当前仅支持: {list(_PREFLIGHT_BY_MODE)}")
        return None

    if mode == "eastmoney_mac":
        em_cfg = config.get("eastmoney", {})
        app_name = em_cfg.get("app_name", "东方财富")
        trader = preflight_fn(config, api, em_cfg, app_name)
    else:
        trader = preflight_fn(config, api, mode)

    if trader is None:
        return None

    logger.info(f"[preflight] ✅ {mode} 全部启动检查通过")
    send_notification(f"✅ 交易代理就绪 ({mode})",
                      "环境/校准/账户检查全部通过")
    return trader
