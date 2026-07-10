"""东方财富底部标签页（持仓/成交/委托）坐标手动校准工具。

背景：东方财富 Mac 版的「持仓 / 成交 / 委托」标签是自绘控件，AX 树中不可见，
无法自动定位，只能靠屏幕坐标点击。自动扫描点击极易误判（所有点都落到同一
tab）。因此改为「手动点击捕获一次 → 持久化 → 运行时按当前窗口位置还原」：

  1. 引导用户依次点击「持仓」「成交」「委托」三个标签；
  2. 用 Quartz CGEventTap 监听真实鼠标点击，捕获屏幕绝对坐标；
  3. 将坐标换算成「相对交易窗口左上角的像素偏移」后存入 JSON（不做缩放，
     仅随窗口移动自适应）；
  4. 交易执行时读取偏移，加上当前窗口左上角坐标还原为绝对点击点；
  5. 每次切换 tab 后用表头字段校验，若捕获失效（窗口尺寸变化等）则中断并
     提示重新校准。

依赖：pyobjc 的 Quartz（项目已依赖），无需额外安装 pynput。
"""

from __future__ import annotations

import json
import logging
import os
import subprocess
import time
from typing import Optional

from . import ax_helper as ax

logger = logging.getLogger("agent.trader.calibrator")


def _dialog(message: str, title: str = "东财标签校准",
            buttons=("确定",), default: str = "确定",
            timeout: int = 60) -> Optional[str]:
    """弹出 macOS 原生对话框，返回用户点击的按钮文本；超时/失败返回 None。

    校准需要用户按步骤操作，终端输出易被忽略，故用 osascript 弹窗逐步引导。
    Args:
        message: 弹窗正文
        title: 弹窗标题
        buttons: 按钮列表（最多 3 个）
        default: 默认高亮按钮
        timeout: 自动消失秒数
    """
    btn_list = ", ".join(f'"{b}"' for b in buttons)
    script = (
        f'display dialog "{message}" with title "{title}" '
        f'buttons {{{btn_list}}} default button "{default}" '
        f'giving up after {timeout}'
    )
    try:
        out = subprocess.run(
            ["osascript", "-e", script],
            capture_output=True, text=True, timeout=timeout + 5,
        )
        # osascript 输出形如：button returned:确定, gave up:false
        text = out.stdout.strip()
        for part in text.split(", "):
            if part.startswith("button returned:"):
                return part.split(":", 1)[1]
        return None
    except Exception as e:
        logger.warning(f"弹窗失败（回退纯终端提示）: {e}")
        return None


# 校准文件默认路径：trade-agent/tab_calibration.json
_DEFAULT_PATH = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    "tab_calibration.json",
)

# 需要校准的三个标签页（顺序即引导顺序）
_TABS = [
    ("chicang", "持仓"),
    ("chengjiao", "成交"),
    ("weituo", "委托"),
]

# 需要校准的模式标签（顶部，自绘控件，AX 不可见）
_MODE_TABS = [
    ("real", "普通交易"),
    ("simulated", "模拟交易"),
    ("credit", "信用交易"),
]

# 需要校准的买/卖切换标签（下单区顶部，自绘控件，AX 不可见）
_BUY_SELL_TABS = [
    ("buy", "买入"),
    ("sell", "卖出"),
]


def calibration_path() -> str:
    """返回校准文件路径（可用环境变量 TAB_CALIBRATION_PATH 覆盖）。"""
    return os.environ.get("TAB_CALIBRATION_PATH", _DEFAULT_PATH)


def load_calibration(path: Optional[str] = None) -> Optional[dict]:
    """读取已保存的 tab 校准数据，不存在或损坏时返回 None。

    返回结构：
      {
        "win_size": [w, h],            # 校准时窗口尺寸（仅用于告警，不缩放）
        "offsets": {                    # 各 tab 相对窗口左上角的偏移
          "chicang":   [dx, dy],
          "chengjiao": [dx, dy],
          "weituo":    [dx, dy]
        }
      }
    """
    p = path or calibration_path()
    if not os.path.exists(p):
        return None
    try:
        with open(p, "r", encoding="utf-8") as f:
            data = json.load(f)
        if not isinstance(data.get("offsets"), dict):
            return None
        return data
    except Exception as e:
        logger.warning(f"读取 tab 校准文件失败 {p}: {e}")
        return None


def save_calibration(data: dict, path: Optional[str] = None) -> bool:
    """将校准数据写入 JSON 文件，返回是否成功。"""
    p = path or calibration_path()
    try:
        with open(p, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
        logger.info(f"tab 校准已保存: {p}")
        return True
    except Exception as e:
        logger.error(f"保存 tab 校准文件失败 {p}: {e}")
        return False


def _wait_one_click(timeout: float = 30.0) -> Optional[tuple]:
    """阻塞等待用户的一次真实鼠标左键点击，返回其屏幕绝对坐标 (x, y)。

    使用 Quartz CGEventTap 以 ListenOnly 方式监听全局左键按下事件，超时返回 None。
    需要「辅助功能 / 输入监控」授权（与自动化点击同一授权体系）。
    """
    try:
        import Quartz
    except ImportError:
        logger.error("Quartz 不可用，无法监听点击。请安装 pyobjc-framework-Quartz")
        return None

    result = {"pt": None}

    def _cb(proxy, etype, event, refcon):
        if etype == Quartz.kCGEventLeftMouseDown:
            loc = Quartz.CGEventGetLocation(event)
            result["pt"] = (float(loc.x), float(loc.y))
        return event

    mask = Quartz.CGEventMaskBit(Quartz.kCGEventLeftMouseDown)
    tap = Quartz.CGEventTapCreate(
        Quartz.kCGSessionEventTap,
        Quartz.kCGHeadInsertEventTap,
        Quartz.kCGEventTapOptionListenOnly,
        mask, _cb, None,
    )
    if not tap:
        logger.error("无法创建鼠标监听（缺少辅助功能/输入监控授权）")
        return None

    src = Quartz.CFMachPortCreateRunLoopSource(None, tap, 0)
    Quartz.CFRunLoopAddSource(
        Quartz.CFRunLoopGetCurrent(), src, Quartz.kCFRunLoopCommonModes)
    Quartz.CGEventTapEnable(tap, True)

    end = time.time() + timeout
    while time.time() < end and result["pt"] is None:
        Quartz.CFRunLoopRunInMode(Quartz.kCFRunLoopDefaultMode, 0.1, False)

    Quartz.CGEventTapEnable(tap, False)
    return result["pt"]


def _detect_trade_mode_text(win) -> Optional[str]:
    """通过页面文本特征推断当前交易模式（校准时的即时校验用）。

    与 eastmoney_mac._detect_trade_mode 逻辑一致，但仅依赖 AX 文本提取，
    不实例化 trader 对象。用于校准工具中点击模式标签后的即时校验。
    """
    texts = [v for v, _p in ax.collect_static_texts(win)]
    all_text = " ".join(texts)

    credit_kw = ["融资", "融券", "维持担保比例", "担保品"]
    if any(kw in all_text for kw in credit_kw):
        return "credit"

    sim_kw = ["创建新账户", "初始金额"]
    if any(kw in all_text for kw in sim_kw):
        return "simulated"

    real_kw = ["营业部", "股东账号", "上海A股", "深圳A股"]
    if any(kw in all_text for kw in real_kw):
        return "real"

    return None


def run_calibration(app_name: str = "东方财富",
                    detect_view=None,
                    goto_trade=None,
                    path: Optional[str] = None) -> Optional[dict]:
    """交互式校准：引导用户依次点击「持仓/成交/委托」并记录相对窗口偏移。

    Args:
        app_name: 目标 APP 本地化名称。
        detect_view: 可选回调 () -> str|None，点击后返回当前视图标识，用于即时
                     校验用户点对了 tab（'chicang'/'chengjiao'/'weituo'）。
        goto_trade: 可选回调 () -> bool，用于先切到「交易」页再校准。
        path: 校准文件保存路径。

    Returns:
        成功返回校准 dict，失败返回 None。
    """
    if not ax.ax_available():
        print("❌ pyobjc(AX) 不可用，无法校准")
        return None

    pid = ax.find_app_pid(app_name)
    if not pid:
        print(f"❌ 未找到正在运行的「{app_name}」，请先启动并登录")
        return None

    app = ax.app_element(pid)
    ax.activate_app(app_name)
    time.sleep(0.8)

    if goto_trade is not None:
        try:
            goto_trade()
        except Exception as e:
            logger.warning(f"切换交易页失败（忽略，继续校准）: {e}")

    win = ax.main_window(app)
    if win is None:
        print("❌ 无法获取交易窗口")
        return None
    wpos = ax.position(win)
    wsize = ax.size(win)
    if not wpos or not wsize:
        print("❌ 无法读取窗口位置/尺寸")
        return None

    print("=" * 56)
    print("  东方财富「持仓/成交/委托」标签校准")
    print("=" * 56)
    print(f"  交易窗口: 左上角=({wpos[0]:.0f},{wpos[1]:.0f}) "
          f"尺寸=({wsize[0]:.0f}x{wsize[1]:.0f})")
    print("  接下来会依次弹窗引导你点击「持仓/成交/委托」。")
    print("-" * 56)

    _dialog(
        "开始校准东方财富标签坐标。\\n"
        "点『开始』后，请依次点击『持仓 / 成交 / 委托』三个标签。\\n"
        "每步都会有弹窗提示你点哪一个。",
        buttons=("取消", "开始"), default="开始", timeout=120,
    )

    offsets: dict[str, list] = {}
    for key, label in _TABS:
        # 每个标签允许最多 3 次尝试（校验不符时可重来）
        recorded = False
        for attempt in range(3):
            hint = f"请点击东方财富界面上的【{label}】标签" + (
                f"（第{attempt+1}次尝试）" if attempt else "")
            print(f"\n👉 {hint}...（30 秒内）", flush=True)
            # 弹窗提示；点“我要点了”后关闭弹窗再监听点击，避免点到弹窗本身
            _dialog(f"{hint}\\n\\n点『我要点了』后，去点击【{label}】标签。",
                    buttons=("我要点了",), default="我要点了", timeout=60)

            pt = _wait_one_click(timeout=30.0)
            if pt is None:
                r = _dialog(f"未捕获到【{label}】的点击。是否重试？",
                            buttons=("放弃", "重试"), default="重试", timeout=30)
                if r == "重试":
                    continue
                print("❌ 未捕获到点击，校准中断")
                return None

            # 换算相对窗口左上角偏移（不缩放，仅随窗口移动自适应）
            cur = ax.position(win) or wpos  # 用户可能移动过窗口，取最新
            dx = round(pt[0] - cur[0], 1)
            dy = round(pt[1] - cur[1], 1)
            print(f"   已记录 {label}: 屏幕({pt[0]:.0f},{pt[1]:.0f}) → 偏移({dx},{dy})")

            # 即时校验：等界面刷新后确认视图正确
            view = None
            if detect_view is not None:
                time.sleep(0.8)
                try:
                    view = detect_view()
                except Exception:
                    view = None

            if detect_view is None or view == key:
                offsets[key] = [dx, dy]
                recorded = True
                if view == key:
                    print(f"   ✅ 校验通过：当前确为「{label}」视图")
                break
            elif view is None:
                # 无法识别（可能该 tab 无数据），询问是否仍记录
                r = _dialog(
                    f"点击后未能识别到「{label}」视图（可能该标签下暂无数据）。\\n"
                    f"确认你点的是【{label}】吗？",
                    buttons=("重新点", "确认记录"), default="确认记录", timeout=30)
                if r == "确认记录":
                    offsets[key] = [dx, dy]
                    recorded = True
                    print(f"   ⚠️ 未能识别视图，已按点击记录")
                    break
                continue
            else:
                # 校验为其它视图，明显点错，重试
                zh = {"chicang": "持仓", "chengjiao": "成交", "weituo": "委托"}
                r = _dialog(
                    f"校验不符：你点击后进入的是「{zh.get(view, view)}」，"
                    f"但当前应校准【{label}】。\\n请重新点击正确的【{label}】标签。",
                    buttons=("放弃", "重新点"), default="重新点", timeout=30)
                if r == "放弃":
                    print("❌ 用户放弃，校准中断")
                    return None
                continue

        if not recorded:
            _dialog(f"【{label}】多次校验失败，校准中断。",
                    buttons=("确定",), default="确定", timeout=20)
            print(f"❌ 【{label}】校准失败，中断")
            return None

    # ── 模式标签校准（普通交易 / 模拟交易 / 信用交易）──
    print("\n" + "=" * 56)
    print("  接下来校准「普通交易 / 模拟交易」模式标签")
    print("  这些标签在交易页顶部，也是自绘控件。")
    print("-" * 56)

    mode_offsets: dict[str, list] = {}
    r = _dialog(
        "已完成底部标签校准，是否继续校准「普通交易 / 模拟交易」\n模式标签？\n\n"
        "（这些标签在交易页顶部，用于区分实盘/模拟盘）",
        buttons=("跳过", "继续校准"), default="继续校准", timeout=60,
    )
    if r == "继续校准":
        for mkey, mlabel in _MODE_TABS:
            recorded = False
            for attempt in range(3):
                hint = f"请点击东财交易页顶部的【{mlabel}】标签" + (
                    f"（第{attempt+1}次尝试）" if attempt else "")
                print(f"\n👉 {hint}...（30 秒内）", flush=True)
                _dialog(f"{hint}\\n\\n点『我要点了』后，去点击【{mlabel}】标签。",
                        buttons=("我要点了",), default="我要点了", timeout=60)

                pt = _wait_one_click(timeout=30.0)
                if pt is None:
                    r2 = _dialog(f"未捕获到【{mlabel}】的点击。是否重试？",
                                buttons=("放弃", "重试"), default="重试", timeout=30)
                    if r2 == "重试":
                        continue
                    break

                cur = ax.position(win) or wpos
                dx = round(pt[0] - cur[0], 1)
                dy = round(pt[1] - cur[1], 1)
                print(f"   已记录 {mlabel}: 屏幕({pt[0]:.0f},{pt[1]:.0f}) → 偏移({dx},{dy})")

                # 即时校验：等界面刷新后检测模式
                time.sleep(0.8)
                detected = None
                try:
                    detected = _detect_trade_mode_text(win)
                except Exception:
                    detected = None

                if detected == mkey:
                    mode_offsets[mkey] = [dx, dy]
                    recorded = True
                    print(f"   ✅ 校验通过：当前确为「{mlabel}」模式")
                    break
                elif detected is None:
                    r2 = _dialog(
                        f"无法识别当前是否为「{mlabel}」模式。\n"
                        f"确认你点的是【{mlabel}】吗？",
                        buttons=("重新点", "确认记录"), default="确认记录", timeout=30)
                    if r2 == "确认记录":
                        mode_offsets[mkey] = [dx, dy]
                        recorded = True
                        break
                    continue
                else:
                    zh2 = {"real": "普通交易", "simulated": "模拟交易", "credit": "信用交易"}
                    r2 = _dialog(
                        f"校验不符：你点击后检测到「{zh2.get(detected, detected)}」，"
                        f"但当前应校准【{mlabel}】。\n请重新点击正确的【{mlabel}】标签。",
                        buttons=("放弃", "重新点"), default="重新点", timeout=30)
                    if r2 == "放弃":
                        break
                    continue

            if not recorded:
                print(f"   ⚠️ 跳过【{mlabel}】校准，可稍后手动补充")
        if mode_offsets:
            print(f"\n   已校准 {len(mode_offsets)}/{len(_MODE_TABS)} 个模式标签")
    else:
        print("\n   已跳过模式标签校准")

    # ── 买/卖标签校准（下单区顶部，也是自绘控件）──
    print("\n" + "=" * 56)
    print("  接下来校准下单区的「买入 / 卖出」切换标签")
    print("  这些标签位于下单区顶部，也是自绘控件。")
    print("-" * 56)

    buysell_offsets: dict[str, list] = {}
    r = _dialog(
        "已完成模式标签校准，是否继续校准下单区「买入 / 卖出」\n切换标签？\n\n"
        "（这…274 tokens truncated… {len(offsets)} 个标签")
    print(f"  模式标签: {len(mode_offsets)} 个")
    if buysell_offsets:
        print(f"  买/卖标签: {len(buysell_offsets)} 个")

    data = {
        "app_name": app_name,
        "win_size": [round(wsize[0]), round(wsize[1])],
        "offsets": offsets,
        "mode_offsets": mode_offsets,
        "buy_sell_offsets": buysell_offsets,
        "ts": int(time.time()),
    }
    if save_calibration(data, path):
        print("\n✅ 校准完成并已保存。")
        _dialog("✅ 校准完成并已保存！\\n现在可以正常自动交易/读取持仓了。",
                buttons=("好的",), default="好的", timeout=20)
        return data
    return None
