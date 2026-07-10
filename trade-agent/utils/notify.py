"""macOS system notifications."""

import logging
import subprocess

logger = logging.getLogger("agent.notify")


def send_notification(title, message, sound="default"):
    """Send a macOS system notification via osascript."""
    try:
        script = f'''
        display notification "{message}" with title "{title}" sound name "{sound}"
        '''
        subprocess.run(["osascript", "-e", script], timeout=5)
        logger.info(f"Notification sent: {title}")
    except Exception as e:
        logger.warning(f"Notification failed: {e}")


def show_dialog(message, title="智策投研 交易代理",
                buttons=("确定",), default=None, timeout=120):
    """弹出可交互的阻断式对话框，返回用户点击的按钮文本；失败/超时返回 None。

    与 send_notification（右上角一闪而过的通知）不同，此对话框会阻塞等待用户
    响应，适合启动预检失败时的强提醒与引导（如缺授权、账户不符、需校准）。

    Args:
        message: 弹窗正文（可用 \\n 换行）
        title: 弹窗标题
        buttons: 按钮文本元组（最多 3 个）
        default: 默认高亮按钮（None 时取最后一个）
        timeout: 自动关闭秒数
    Returns:
        用户点击的按钮文本；无法弹窗或超时未点返回 None。
    """
    default = default or buttons[-1]
    btn_list = ", ".join(f'"{b}"' for b in buttons)
    # 转义正文中的双引号，避免 AppleScript 语法破坏
    safe_msg = message.replace('"', '\\"')
    script = (
        f'display dialog "{safe_msg}" with title "{title}" '
        f'buttons {{{btn_list}}} default button "{default}" '
        f'giving up after {timeout}'
    )
    try:
        out = subprocess.run(
            ["osascript", "-e", script],
            capture_output=True, text=True, timeout=timeout + 5,
        )
        text = out.stdout.strip()
        for part in text.split(", "):
            if part.startswith("button returned:"):
                return part.split(":", 1)[1]
        return None
    except Exception as e:
        logger.warning(f"对话框弹出失败: {e}")
        return None
