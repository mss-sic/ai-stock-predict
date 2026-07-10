"""macOS Accessibility (AX) API 辅助工具。

封装 AXUIElement 相关操作，供东方财富 Mac 原生 APP 自动化使用。
原生 Cocoa 应用可通过 AX API 精确读取控件文本与坐标，比截图 OCR 更可靠。
"""

from __future__ import annotations

import logging
import re
import time
from typing import Optional

logger = logging.getLogger("agent.trader.ax")

try:
    from AppKit import NSWorkspace
    from ApplicationServices import (
        AXIsProcessTrusted,
        AXUIElementCreateApplication,
        AXUIElementCopyAttributeValue,
        AXUIElementSetAttributeValue,
        AXUIElementPerformAction,
        kAXWindowsAttribute,
        kAXChildrenAttribute,
        kAXRoleAttribute,
        kAXTitleAttribute,
        kAXValueAttribute,
        kAXPositionAttribute,
        kAXSizeAttribute,
        kAXDescriptionAttribute,
        kAXRoleDescriptionAttribute,
        kAXPressAction,
        kAXFocusedAttribute,
    )
    _AX_AVAILABLE = True
except ImportError as e:  # pyobjc 未安装
    logger.error(f"pyobjc 未安装，AX 功能不可用: {e}")
    _AX_AVAILABLE = False


def ax_available() -> bool:
    """返回 AX 依赖（pyobjc）是否可用。"""
    return _AX_AVAILABLE


def is_trusted() -> bool:
    """返回当前进程是否已获得 macOS 辅助功能授权。"""
    if not _AX_AVAILABLE:
        return False
    return bool(AXIsProcessTrusted())


def find_app_pid(app_name: str) -> Optional[int]:
    """按本地化名称查找正在运行的 APP 进程 PID，未运行返回 None。"""
    if not _AX_AVAILABLE:
        return None
    for app in NSWorkspace.sharedWorkspace().runningApplications():
        if app.localizedName() == app_name:
            return app.processIdentifier()
    return None


def activate_app(app_name: str) -> bool:
    """将目标 APP 激活到前台，返回是否成功找到该 APP。

    使用 ActivateAllWindows | IgnoringOtherApps 组合并先 unhide，确保处于
    隐藏/最小化/失焦等状态时窗口能被 AX 正确读取（仅 IgnoringOtherApps
    在部分状态下会出现 AXWindows 为空的问题）。
    """
    if not _AX_AVAILABLE:
        return False
    # NSApplicationActivateAllWindows=1<<0, IgnoringOtherApps=1<<1
    opts = (1 << 0) | (1 << 1)
    for app in NSWorkspace.sharedWorkspace().runningApplications():
        if app.localizedName() == app_name:
            try:
                app.unhide()
            except Exception:
                pass
            app.activateWithOptions_(opts)
            return True
    return False


def app_element(pid: int):
    """根据 PID 创建 APP 级别的 AXUIElement 根节点。"""
    return AXUIElementCreateApplication(pid)


def attr(element, name):
    """安全读取 AX 元素的指定属性，出错或不存在时返回 None。"""
    try:
        err, val = AXUIElementCopyAttributeValue(element, name, None)
        return val if err == 0 else None
    except Exception:
        return None


def children(element) -> list:
    """返回 AX 元素的直接子节点列表。"""
    return attr(element, kAXChildrenAttribute) or []


def role(element) -> Optional[str]:
    """返回 AX 元素的角色（如 AXButton / AXTextField / AXStaticText）。"""
    return attr(element, kAXRoleAttribute)


def title(element) -> Optional[str]:
    """返回 AX 元素的标题（AXTitle）。"""
    return attr(element, kAXTitleAttribute)


def value(element):
    """返回 AX 元素的值（AXValue）。"""
    return attr(element, kAXValueAttribute)


def position(element) -> Optional[tuple]:
    """返回 AX 元素左上角屏幕坐标 (x, y)，解析失败返回 None。"""
    v = attr(element, kAXPositionAttribute)
    if v is None:
        return None
    m = re.search(r"x:([-\d.]+)\s+y:([-\d.]+)", str(v))
    return (float(m.group(1)), float(m.group(2))) if m else None


def size(element) -> Optional[tuple]:
    """返回 AX 元素尺寸 (w, h)，解析失败返回 None。"""
    v = attr(element, kAXSizeAttribute)
    if v is None:
        return None
    m = re.search(r"w:([-\d.]+)\s+h:([-\d.]+)", str(v))
    return (float(m.group(1)), float(m.group(2))) if m else None


def center(element) -> Optional[tuple]:
    """返回 AX 元素中心点屏幕坐标 (x, y)，用于 pyautogui 点击。"""
    p = position(element)
    s = size(element)
    if p and s:
        return (p[0] + s[0] / 2.0, p[1] + s[1] / 2.0)
    return p


def all_windows(app) -> list:
    """返回 APP 的所有窗口 AX 元素列表（含弹窗/对话框）。"""
    return attr(app, kAXWindowsAttribute) or []


def main_window(app):
    """返回 APP 的主交易窗口（面积最大的窗口），排除小弹窗，无窗口返回 None。

    东方财富的下单确认弹窗是独立窗口且面积很小，且窗口顺序会变化，
    因此以「面积最大」判定主窗口，比固定取 wins[0] 更可靠。
    """
    wins = attr(app, kAXWindowsAttribute)
    if not wins:
        return None

    def area(w):
        s = size(w)
        return (s[0] * s[1]) if s else 0.0

    return max(wins, key=area)


def walk(element, callback, depth: int = 0, max_depth: int = 14):
    """深度优先遍历 AX 树，对每个节点调用 callback(element, depth)。"""
    if depth > max_depth:
        return
    callback(element, depth)
    for c in children(element):
        walk(c, callback, depth + 1, max_depth)


def collect(element, predicate, max_depth: int = 14) -> list:
    """遍历 AX 树，收集所有满足 predicate(element) 的节点。"""
    found = []

    def _cb(el, _d):
        try:
            if predicate(el):
                found.append(el)
        except Exception:
            pass

    walk(element, _cb, 0, max_depth)
    return found


def find_button(element, label: str, max_depth: int = 14):
    """在 AX 树中按标题查找第一个匹配的按钮，未找到返回 None。"""
    matches = collect(
        element,
        lambda el: role(el) == "AXButton" and title(el) == label,
        max_depth,
    )
    return matches[0] if matches else None


def press(element) -> bool:
    """对 AX 元素执行 Press 动作（等价点击），返回是否成功。"""
    try:
        return AXUIElementPerformAction(element, kAXPressAction) == 0
    except Exception:
        return False


def set_value(element, text: str) -> bool:
    """尝试通过 AX 直接设置元素值（部分原生输入框支持），返回是否成功。"""
    try:
        return AXUIElementSetAttributeValue(element, kAXValueAttribute, text) == 0
    except Exception:
        return False


def collect_static_texts(element, max_depth: int = 14) -> list:
    """收集所有非空 AXStaticText，返回 [(value_str, (x, y)), ...]。"""
    result = []

    def _cb(el, _d):
        if role(el) == "AXStaticText":
            v = value(el)
            if v not in (None, ""):
                result.append((str(v), position(el)))

    walk(element, _cb, 0, max_depth)
    return result
