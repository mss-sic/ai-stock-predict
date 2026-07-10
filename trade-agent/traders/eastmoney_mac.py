"""东方财富 Mac 原生 APP 自动交易执行器（主力 trader）。

东方财富 Mac 版为原生 Cocoa 应用，本执行器通过 macOS Accessibility (AX) API
精确读取交易界面控件并定位坐标，配合 pyautogui 触发真实鼠标/键盘交互完成：
  - 读取账户资金（总资产/可用资金/持仓盈亏等）
  - 读取持仓列表
  - 模拟盘/实盘下单（买入/卖出）

相比截图 OCR + 盲按快捷键，AX 方案可精确定位元素、稳定读值，显著更可靠。

前置条件：
  1. 已安装 pyobjc（pyobjc-framework-Quartz / ApplicationServices）与 pyautogui
  2. 运行本 Agent 的进程已在「系统设置 → 隐私与安全性 → 辅助功能」中授权
  3. 东方财富 APP 已启动并登录（模拟盘或实盘账户）
"""

from __future__ import annotations

import logging
import re
import time
from typing import Optional

from . import ax_helper as ax
from .base import AbstractTrader, OrderResult, Position

logger = logging.getLogger("agent.trader.eastmoney")

# ── 资金面板字段映射（AX StaticText 中文标签 → 内部字段名）──
_BALANCE_FIELDS = {
    "总资产": "total_assets",
    "可用资金": "available_cash",
    "总市值": "market_value",
    "持仓盈亏": "total_profit",
    "资金余额": "cash_balance",
    "冻结资金": "frozen",
}

# ── 资金面板布局常量 ──
# 坐标相对化后不再依赖硬编码 x 区间；仅保留同行 y 容差用于「标签-数值」配对。
_ROW_Y_TOLERANCE = 8  # 同一行标签与数值的 y 容差（像素）


class EastMoneyMacTrader(AbstractTrader):
    """东方财富 Mac 原生 APP 自动交易执行器（基于 AX API）。"""

    def __init__(self, app_name: str = "东方财富", confirm_order: bool = True,
                 action_delay: float = 0.4, max_retries: int = 5,
                 calibration_path: Optional[str] = None,
             trade_mode: str = "simulated",
             trade_password: str = "", fund_account: str = ""):
        """初始化执行器。

        Args:
            app_name: 目标 APP 本地化名称（默认「东方财富」）
            confirm_order: 下单后是否自动点击二次确认弹窗
            action_delay: 每步操作之间的等待秒数（防止 UI 未刷新）
            max_retries: 下单前信息校验不通过时的最大重试次数
            calibration_path: 「持仓/成交/委托」tab 坐标校准文件路径（None 用默认）

            trade_mode: 交易模式 "real"(普通交易) / "simulated"(模拟交易) / "credit"(信用交易)
            trade_password: 实盘交易密码，用于登录超时后自动重新登录（为空则不自动登录）
            fund_account: 资金账号，登录时主动填入；为空则沿用东财登录框预填值
        注意：持仓/成交/委托 tab 是东方财富自绘控件，AX 不可见，只能坐标点击。
        这些坐标由用户手动校准一次后持久化（见 tab_calibrator），保存的是相对
        交易窗口左上角的像素偏移；运行时按当前窗口位置还原为绝对坐标，随窗口
        移动自适应。未校准或校验失效时会明确报错并提示重新校准，绝不盲点击。
        """
        self.app_name = app_name
        self.confirm_order = confirm_order
        self.action_delay = action_delay
        self.max_retries = max_retries
        self.calibration_path = calibration_path
        self.trade_mode = trade_mode
        self.trade_password = trade_password
        self.fund_account = fund_account
        self._detected_mode: Optional[str] = None  # 实际检测到的模式
        # tab 相对窗口左上角的偏移 {'chicang':(dx,dy), 'chengjiao':..., 'weituo':...}
        self._tab_offsets: dict[str, tuple] = {}
        self._pid: Optional[int] = None
        self._app = None
        self._pa = None  # pyautogui 模块引用
        self._ocr = None  # ddddocr 实例（懒加载，用于验证码识别）

    # ── 连接 / 断开 ──

    def connect(self) -> bool:
        """检查 AX 依赖、辅助功能授权，并定位东方财富 APP 进程。"""
        if not ax.ax_available():
            logger.error("pyobjc 未安装，无法使用 AX 自动化。"
                         "请运行: pip install pyobjc-framework-Quartz pyobjc-framework-ApplicationServices")
            return False

        if not ax.is_trusted():
            logger.error("当前进程未获得 macOS 辅助功能授权。"
                         "请在「系统设置 → 隐私与安全性 → 辅助功能」中勾选运行本 Agent 的终端/程序。")
            return False

        try:
            import pyautogui
            pyautogui.FAILSAFE = False  # 禁用 FAILSAFE，避免鼠标误触角落导致整个Agent崩溃
            pyautogui.PAUSE = 0.2
            self._pa = pyautogui
        except ImportError:
            logger.error("pyautogui 未安装，无法执行点击/输入。请运行: pip install pyautogui")
            return False

        self._pid = ax.find_app_pid(self.app_name)
        if not self._pid:
            logger.error(f"未找到正在运行的「{self.app_name}」APP，请先启动并登录。")
            return False

        self._app = ax.app_element(self._pid)
        # 先激活到前台再检查窗口：部分情况下未激活时 AX 读不到窗口，故重试等待
        ax.activate_app(self.app_name)
        win_ready = False
        for _ in range(10):
            time.sleep(0.4)
            if ax.main_window(self._app) is not None:
                win_ready = True
                break
        if not win_ready:
            logger.error(f"「{self.app_name}」无可用窗口。")
            return False

        # 关键控件可定位性自检：不通过则阻断，避免因窗口尺寸差异盲操作
        ok, reason = self._self_check()
        if not ok:
            logger.error(f"启动自检失败：{reason}")
            return False

        # 加载「持仓/成交/委托」tab 坐标校准（相对窗口偏移，随窗口位置还原）
        if not self._load_tab_calibration():
            logger.error(
                "未找到 tab 坐标校准，且无法自动定位（东财 tab 为自绘控件）。"
                "请先运行校准：python3 agent.py --mode calibrate")
            return False

        logger.info(f"东方财富 trader 已连接 (pid={self._pid})")
        return True

    def _load_tab_calibration(self) -> bool:
        """加载持久化的 tab 相对偏移，成功返回 True。

        校准数据由 tab_calibrator.run_calibration 生成，存的是每个 tab 相对
        交易窗口左上角的像素偏移。运行时无需缩放，仅在点击时叠加当前窗口左上角
        坐标即可还原绝对点击点，从而随窗口移动自适应。
        """
        from . import tab_calibrator
        data = tab_calibrator.load_calibration(self.calibration_path)
        if not data:
            return False
        offsets = data.get("offsets") or {}
        loaded = {}
        for key in ("chicang", "chengjiao", "weituo"):
            v = offsets.get(key)
            if isinstance(v, (list, tuple)) and len(v) == 2:
                loaded[key] = (float(v[0]), float(v[1]))
        if len(loaded) < 3:
            logger.warning(f"tab 校准数据不完整: {list(loaded.keys())}")
            return False
        self._tab_offsets = loaded
        logger.info(f"已加载 tab 校准偏移: {loaded}")
        return True

    def _self_check(self):
        """连接后自检关键控件是否可定位，返回 (ok, reason)。

        校验项：能进入交易页、交易模式正确、下单区可定位。
        任一不满足则判定当前界面不适配，阻断后续盲操作。
        """
        if not self._activate() or not self._goto_trade_tab():
            return False, "无法切换到交易页（未登录或界面异常）"
        win = self._window()
        if win is None:
            return False, "无法获取主窗口"

        # 交易模式校验（模拟/实盘/信用）
        if not self._ensure_trade_mode():
            target_name = self.TRADE_MODE_NAMES.get(self.trade_mode, self.trade_mode)
            return False, f"无法切换到「{target_name}」模式，请手动切换后重试"

        if self._find_field_by_label(win, "证券代码") is None:
            return False, "未定位到「证券代码」输入框（下单区布局不匹配）"
        # 买入提交按钮（先确保处于买入模式）
        self._select_mode(win, True)
        submit_btn = self._find_submit_button_retry("买入", attempts=2)
        if submit_btn is None:
            # 实盘可能默认在卖出 tab，买入/卖出 tab 也是自绘控件
            submit_btn = self._find_submit_button_retry("卖出", attempts=2)
        if submit_btn is None:
            return False, "未定位到「买入」或「卖出」提交按钮（下单区布局不匹配）"
        return True, ""


    # ── 交易模式检测与切换 ──

    # 东财 Mac 版交易页左侧竖排 3 个模式标签（AX 可见按钮）
    TRADE_MODE_LABELS = {"普通交易": "real", "模拟交易": "simulated", "信用交易": "credit"}
    TRADE_MODE_NAMES = {"real": "普通交易", "simulated": "模拟交易", "credit": "信用交易"}

    def _detect_trade_mode(self) -> Optional[str]:
        """检测当前东财 APP 处于哪个交易模式。

        东财交易页顶部的模式标签（普通交易/模拟交易/信用交易）是自绘控件，
        AX 不可见。改为通过页面内容特征 + 资金格式多级推断：

        第 1 级 — 内容关键词（页面 AXStaticText）：
          信用：融资/融券/维持担保比例/担保品（优先级最高）
          模拟：创建新账户/初始金额/交易设置/模拟交易
          实盘：营业部/股东账号/上海A股/深圳A股/银证转账/客户号

        第 2 级 — 左侧 AXButton 标签名称（作为辅助）：
          左侧面板有「普通交易」「模拟交易」等 AXButton（模式切换后残留）

        第 3 级 — 资金面板布局特征：
          模拟盘：总资产 = 初始金额制（无冻结资金等真实券商字段）
          实盘：  有 资金余额/冻结资金/可用资金 三级分离

        Returns:
            "real" / "simulated" / "credit" 或 None（完全无法检测）
        """
        win = self._window()
        if win is None:
            return None

        texts = [v for v, _p in ax.collect_static_texts(win)]
        all_text = " ".join(texts)

        # ── 第 1 级：内容关键词检测 ──

        # 信用交易（优先级最高：通常同时存在普通交易功能）
        credit_kw = ["融资", "融券", "维持担保比例", "担保品", "信用账户"]
        if any(kw in all_text for kw in credit_kw):
            logger.info("检测到信用交易模式（融资融券特征）")
            return "credit"

        # 模拟盘特征
        sim_kw = ["创建新账户", "初始金额", "交易设置", "模拟交易",
                  "初始总资产", "退出登录"]
        sim_hits = [kw for kw in sim_kw if kw in all_text]
        if sim_hits:
            logger.info(f"检测到模拟交易模式（特征: {sim_hits}）")
            return "simulated"

        # 实盘特征（券商账户特有信息）
        real_kw = ["营业部", "股东账号", "上海A股", "深圳A股",
                   "银证转账", "客户号", "资金账号", "交易记录"]
        real_hits = [kw for kw in real_kw if kw in all_text]
        if real_hits:
            logger.info(f"检测到普通交易模式（特征: {real_hits}）")
            return "real"

        # ── 第 2 级：左侧 AXButton 名称辅助 ──
        # 左侧面板的模式标签虽为自绘控件，但部分按钮文本可能残留于 AX 树
        btn_texts = set()
        for b in ax.collect(win, lambda el: ax.role(el) == "AXButton"):
            t = ax.title(b)
            if t:
                btn_texts.add(t.strip())

        if "模拟交易" in btn_texts and "普通交易" not in btn_texts:
            logger.info("检测到模拟交易模式（左侧按钮特征）")
            return "simulated"
        if "普通交易" in btn_texts and "模拟交易" not in btn_texts:
            logger.info("检测到普通交易模式（左侧按钮特征）")
            return "real"

        # ── 第 3 级：资金面板布局特征 ──
        # 模拟盘：总资产 + 可用资金 简单两段（无冻结/余额分离）
        # 实盘：资金余额、可用资金、冻结资金 三段分离
        has_frozen = "冻结资金" in all_text or "冻结" in all_text
        has_cash_balance = "资金余额" in all_text
        has_avail = "可用资金" in all_text

        if has_frozen or has_cash_balance:
            logger.info("检测到普通交易模式（资金面板三级分离特征）")
            return "real"
        if has_avail:
            # 仅有可用资金无冻结资金 → 模拟盘可能性大
            logger.info("检测到模拟交易模式（资金面板简洁特征）")
            return "simulated"

        # ── 完全无法检测 ──
        # 返回 None 让调用方决定策略（走坐标切换 or 手动干预），
        # 不盲猜，避免实盘误判为模拟盘导致错误下单。
        logger.warning("无法通过页面文本/按钮/资金特征检测交易模式")
        return None

    def _switch_trade_mode(self, target: str) -> bool:
        """切换到指定的交易模式（real/simulated/credit）。

        模式标签是自绘控件（AX 不可见），需要校准坐标后点击。
        坐标存于 tab_calibration.json 的 mode_offsets 字段，
        与持仓/成交/委托标签使用相同的校准体系。

        校准方法：
          在 tab_calibration.json 中增加 mode_offsets:
            {"real": [dx, dy], "simulated": [dx, dy], "credit": [dx, dy]}
          坐标含义：相对于交易窗口左上角的像素偏移

        Args:
            target: "real" / "simulated" / "credit"
        Returns:
            切换成功返回 True
        """
        label = self.TRADE_MODE_NAMES.get(target)
        if not label:
            logger.error(f"未知交易模式: {target}")
            return False

        # 先检测当前模式，已经是目标则无需切换
        current = self._detect_trade_mode()
        if current == target:
            logger.info(f"已处于 {label} 模式，无需切换")
            self._detected_mode = target
            return True

        # 尝试从校准数据读取模式标签坐标
        from . import tab_calibrator
        data = tab_calibrator.load_calibration(self.calibration_path)
        mode_offsets = (data or {}).get("mode_offsets", {})
        offset = mode_offsets.get(target)

        if offset and len(offset) == 2:
            # 使用校准坐标点击
            win = self._window()
            if win is None:
                return False
            wpos = ax.position(win)
            if wpos:
                x, y = wpos[0] + offset[0], wpos[1] + offset[1]
                logger.info(f"切换交易模式: → {label} @ ({x:.0f},{y:.0f})")
                self._pa.click(x, y)
                time.sleep(self.action_delay + 1.0)
                detected = self._detect_trade_mode()
                if detected == target:
                    logger.info(f"交易模式切换成功: {label}")
                    self._detected_mode = target
                    return True
                logger.warning(f"校准坐标点击后模式仍未切换 detected={detected}")
        else:
            logger.warning(
                f"未找到「{label}」模式标签的校准坐标。"
                f"请在 tab_calibration.json 中手动添加 mode_offsets.{target}，"
                f"或在东财 APP 中手动点击切换到 {label} 模式后重试。"
            )

        return False

    def _ensure_trade_mode(self) -> bool:
        """确保东财处于预期的交易模式，不符则自动切换。

        Returns:
            True 如果当前模式正确或切换成功
        """
        detected = self._detect_trade_mode()
        self._detected_mode = detected

        if detected == self.trade_mode:
            logger.info(f"交易模式正确: {self.TRADE_MODE_NAMES.get(detected, detected)}")
            return True

        if detected is None:
            logger.warning(f"无法检测当前模式，尝试切换到 {self.trade_mode}")
        else:
            logger.info(f"当前模式={detected}，需要切换到 {self.trade_mode}")

        return self._switch_trade_mode(self.trade_mode)

    def disconnect(self):
        """释放引用（AX 无需显式关闭）。"""
        self._app = None
        self._pid = None
        logger.info("东方财富 trader 已断开")

    # ── 内部辅助 ──

    def _activate(self) -> bool:
        """将东方财富 APP 激活到前台，并轮询等待主窗口可读。

        实测：activateWithOptions 是异步的，激活后窗口需短暂时间才能被 AX
        读取。固定短延时在 WS 命令线程调用时常常窗口尚未就绪导致
        _window() 返回 None。这里改为轮询等待（最多 ~3s）。
        """
        ok = ax.activate_app(self.app_name)
        if not ok:
            logger.warning("_activate: 未找到东方财富 APP")
            return False
        # 轮询等待主窗口真正可读
        for _ in range(15):
            time.sleep(0.2)
            if ax.main_window(self._app) is not None:
                return True
        logger.warning("_activate: 激活后主窗口仍不可读（超时）")
        return False

    def _window(self):
        """返回当前主窗口 AX 元素。"""
        return ax.main_window(self._app)

    def _goto_trade_tab(self) -> bool:
        """切换到「交易」标签页。"""
        win = self._window()
        if win is None:
            return False
        btn = ax.find_button(win, "交易")
        if btn is None:
            logger.warning("未找到「交易」标签按钮")
            return False
        ax.press(btn)
        time.sleep(self.action_delay + 0.6)
        return True

    def _click_element(self, element) -> bool:
        """通过 pyautogui 点击 AX 元素中心，返回是否成功。"""
        c = ax.center(element)
        if not c:
            return False
        self._pa.click(c[0], c[1])
        time.sleep(0.2)
        return True

    def _fill_field(self, element, text: str) -> bool:
        """向输入框写入文本：点击聚焦 → 光标移到行尾 → 单向退格清空 → 逐字输入。

        关键：不使用 ⌘A 全选。pyautogui 在 macOS 上发送 ⌘A 时，command 修饰键
        与 a 键是两个独立事件，存在竞态——修饰键偶发未及时生效时，a 会作为普通
        字符落入输入框，中文输入法再将其转成候选汉字（锕/呵），导致证券代码污染。

        清空策略：点击后光标可能落在文本中间，若直接退格会从中间删起、删不干净
        （如价格 136.03 变成错值）。故先用「右方向键」把光标推到行尾，再单向
        「退格」逐个左删至空。方向键/退格键均不产生字符，也不受中文输入法影响。
        """
        c = ax.center(element)
        if not c:
            return False
        self._pa.click(c[0], c[1])
        time.sleep(0.2)
        # 读当前值长度，计算需要的按键次数（留足余量）
        try:
            cur = str(ax.value(element) or "")
        except Exception:
            cur = ""
        n = max(len(cur), 12) + 6
        # ① 先把光标移到行尾（右方向键，不产生字符）
        for _ in range(n):
            self._pa.press("right")
        # ② 从行尾单向退格清空
        for _ in range(n):
            self._pa.press("backspace")
        time.sleep(0.1)
        # 逐字输入（纯数字/小数点，不受中文输入法影响）
        self._pa.typewrite(str(text), interval=0.08)
        time.sleep(0.2)
        return True

    def _find_field_by_label(self, win, label: str):
        """定位某中文标签同一行右侧的输入框（AXTextField）。

        东方财富下单区为「标签(左) + 输入框(右)」同行布局，
        通过标签的 y 坐标匹配同行、x 更大的 AXTextField。
        """
        # 找到标签 StaticText 的位置
        label_pos = None
        for v, p in ax.collect_static_texts(win):
            if p and v.strip() == label:
                label_pos = p
                break
        if not label_pos:
            return None

        ly = label_pos[1]
        fields = ax.collect(
            win, lambda el: ax.role(el) == "AXTextField"
        )
        best, best_dy = None, 999
        for f in fields:
            fp = ax.position(f)
            if not fp:
                continue
            dy = abs(fp[1] - ly)
            # 同一行（y 接近）且在标签右侧
            if dy <= 14 and fp[0] >= label_pos[0] and dy < best_dy:
                best, best_dy = f, dy
        return best

    # ── 只读：账户身份 ──

    # 账户名右侧最近文本的最大 x 距离（像素）。实盘下单区账户名紧邻标签
    # （dx≈55），而资金区标题「证券账户」右侧最近文本可能是很远的行情区
    # 文本（dx≈390，如"停"牌标记），用此上限排除误匹配。
    _ACCOUNT_NAME_MAX_DX = 200

    def get_account_name(self) -> Optional[str]:
        """读取交易页当前登录的证券账户名称，用于与后端绑定账户校验。

        东财交易页存在多个「证券账户」标签：
          - 资金区标题「证券账户」：右侧无紧邻账户名（最近文本可能是很远的
            行情区文字，如"停"牌标记，会造成误读）
          - 下单区「证券账户」：右侧紧邻真实账户名，实盘形如「李江波 (0472)」，
            模拟盘形如「智投测试ljb」

        策略：遍历所有「证券账户」标签，取其同行右侧、x 距离在阈值内的最近
        文本作为候选账户名；在所有候选里优先选择"看起来像账户名"的（含中文
        或字母、非纯数字/单字状态标记）。

        Returns:
            账户名字符串（保留原始形态，如「李江波 (0472)」）；无法解析返回 None。
        """
        if not self._activate() or not self._goto_trade_tab():
            return None
        win = self._window()
        if win is None:
            return None

        texts = [(v.strip(), p) for v, p in ax.collect_static_texts(win) if p]
        labels = [p for v, p in texts if v == "证券账户"]
        if not labels:
            logger.warning("未找到「证券账户」标签，无法读取账户名")
            return None

        candidates = []  # (dx, value)
        for lx, ly in labels:
            for v, p in texts:
                if not v or v == "证券账户":
                    continue
                if abs(p[1] - ly) > _ROW_Y_TOLERANCE:
                    continue
                if p[0] <= lx:
                    continue
                dx = p[0] - lx
                if dx > self._ACCOUNT_NAME_MAX_DX:
                    continue  # 距离过远，排除行情区等误匹配
                candidates.append((dx, v))

        if not candidates:
            logger.warning("未能解析证券账户名（无符合距离的右侧文本）")
            return None

        # 优先选"像账户名"的候选：含中文姓名或字母，且长度≥2、非纯数字
        def _looks_like_name(s: str) -> bool:
            if len(s) < 2:
                return False
            if s.replace(".", "").replace("-", "").isdigit():
                return False
            return bool(re.search(r"[\u4e00-\u9fa5A-Za-z]", s))

        named = sorted([c for c in candidates if _looks_like_name(c[1])])
        chosen = (named or sorted(candidates))[0][1]
        logger.info(f"当前证券账户: {chosen}")
        return chosen

    def get_login_status(self) -> Optional[str]:
        """读取实盘账户「登录状态」（已登录 / 未登录）。

        实盘账户有最长约 180 分钟在线时长，超时后需重新登录交易账号才能操作。
        账户区存在「登录状态」标签，同行右侧为状态值「已登录」或「未登录」。
        模拟盘通常无此标签。

        Returns:
            "已登录" / "未登录" 等状态字符串；无「登录状态」标签（如模拟盘）
            或无法读取时返回 None。
        """
        win = self._window()
        if win is None:
            if not self._activate() or not self._goto_trade_tab():
                return None
            win = self._window()
            if win is None:
                return None

        texts = [(v.strip(), p) for v, p in ax.collect_static_texts(win) if p]
        label_pos = None
        for v, p in texts:
            if v == "登录状态":
                label_pos = p
                break
        if not label_pos:
            return None  # 无此标签（模拟盘）

        lx, ly = label_pos
        best, best_dx = None, 1e9
        for v, p in texts:
            if not v or v == "登录状态":
                continue
            if abs(p[1] - ly) > _ROW_Y_TOLERANCE:
                continue
            if p[0] <= lx:
                continue
            dx = p[0] - lx
            if dx > self._ACCOUNT_NAME_MAX_DX:
                continue
            if dx < best_dx:
                best_dx, best = dx, v
        if best:
            logger.info(f"实盘登录状态: {best}")
        return best

    def is_logged_in(self) -> bool:
        """判断实盘账户是否处于可交易的已登录状态。

        实盘登录状态有三种：「已登录」（可交易）、「未登录」（需登录）、
        「已锁定」（超时锁定，需重新登录/解锁）。仅「已登录」视为可交易。

        Returns:
            True  —— 已登录，或模拟盘（无登录状态标签，视为可交易）
            False —— 明确读到「未登录」「已锁定」等非可交易状态
        """
        status = self.get_login_status()
        if status is None:
            return True  # 模拟盘无此概念，放行
        return "已登录" in status

    # ── 自动登录（实盘超时后重新登录）──

    # 登录框识别：宽约 400、高 > 300 的浮层窗口
    _LOGIN_WIN_W_RANGE = (360, 460)
    _LOGIN_WIN_MIN_H = 300
    # 验证码图片相对「验证码输入框右边界」的截图锚点（像素，实测标定）
    # 图片起点在输入框内偏右处(-83)，终点到「点击换图」按钮左端前(-4)
    _CAPTCHA_LEFT_FROM_FIELD_RIGHT = -83
    _CAPTCHA_RIGHT_FROM_CHANGE_BTN = -4
    _CAPTCHA_Y_PAD = 4  # 上下各扩展像素

    def _find_login_window(self):
        """查找当前是否存在券商登录浮层窗口，返回窗口 AX 元素或 None。"""
        if self._app is None:
            return None
        wlo, whi = self._LOGIN_WIN_W_RANGE
        for w in ax.all_windows(self._app):
            s = ax.size(w)
            if s and wlo < s[0] < whi and s[1] > self._LOGIN_WIN_MIN_H:
                return w
        return None

    def _open_login_window(self):
        """点击「登录证券账户」或「解锁证券账户」打开登录/解锁浮层。

        实盘超时后有两种入口：
          - 「登录证券账户」：完全登出，需账号+密码+验证码
          - 「解锁证券账户」：会话锁定（已锁定状态），通常需密码+验证码
        两者浮层结构类似，统一用 _locate_login_fields 定位输入框。

        Returns:
            登录/解锁浮层窗口 AX 元素，或 None（未找到按钮/浮层）
        """
        login = self._find_login_window()
        if login is not None:
            return login
        win = self._window()
        if win is None:
            return None
        btn = ax.find_button(win, "登录证券账户") or ax.find_button(win, "解锁证券账户")
        if btn is None:
            logger.warning("未找到「登录/解锁证券账户」按钮，可能已登录或界面异常")
            return None
        logger.info(f"点击「{ax.title(btn)}」打开登录/解锁浮层")
        ax.press(btn)
        time.sleep(self.action_delay + 1.5)
        return self._find_login_window()

    def _locate_login_fields(self, login):
        """定位登录浮层中「账号/密码/验证码输入框」及「点击换图」按钮。

        登录浮层可能同时含券商登录组与注册组，以「已预填资金账号(长数字)」
        的输入框锚定券商登录组，取其下方 100px 内的密码框、验证码框。

        Returns:
            dict{acct, pwd, captcha, change_btn_pos} 或 None（结构不匹配）
        """
        fields = []
        for f in ax.collect(login, lambda el: ax.role(el) == "AXTextField"):
            p = ax.position(f)
            s = ax.size(f)
            v = ax.value(f)
            if p:
                fields.append({"pos": p, "size": s, "val": v, "el": f})
        # 账号框：值为纯数字且长度≥8（东财会预填资金账号）
        acct = next(
            (f for f in fields
             if (f["val"] or "").strip().isdigit() and len((f["val"] or "").strip()) >= 8),
            None,
        )
        if not acct:
            # 无预填时退化为取最上方输入框作为账号框
            fields_by_y = sorted([f for f in fields if f["pos"]], key=lambda x: x["pos"][1])
            if not fields_by_y:
                return None
            acct = fields_by_y[0]
        ay = acct["pos"][1]
        group = sorted(
            [f for f in fields if 0 <= f["pos"][1] - ay < 100],
            key=lambda f: f["pos"][1],
        )
        # group[0]=账号 group[1]=密码 group[2]=验证码
        if len(group) < 3:
            logger.warning(f"登录框输入框数量不足(={len(group)})，无法自动登录")
            return None
        pwd, captcha = group[1], group[2]
        # 同行「点击换图」按钮定位验证码图右界
        change_pos = None
        for bb in ax.collect(login, lambda el: ax.role(el) == "AXButton"):
            if (ax.title(bb) or "").strip() == "点击换图":
                p = ax.position(bb)
                if p and abs(p[1] - captcha["pos"][1]) < 12:
                    change_pos = p
                    break
        return {
            "acct": acct, "pwd": pwd, "captcha": captcha,
            "change_btn_pos": change_pos,
        }

    def _recognize_captcha(self, captcha_field, change_btn_pos) -> Optional[str]:
        """截取验证码图片并用 ddddocr 识别，返回识别文本或 None。

        以验证码输入框右边界为左锚、「点击换图」按钮左端为右锚定位图片区域，
        截图后放大 4 倍提升识别率。仅接受 4 位纯数字结果。
        """
        try:
            import ddddocr
            import io
        except ImportError:
            logger.error("ddddocr 未安装，无法自动识别验证码。请运行: pip install ddddocr")
            return None
        if not change_btn_pos:
            logger.warning("未定位到「点击换图」按钮，无法确定验证码图片区域")
            return None
        cf = captcha_field["pos"]
        cs = captcha_field["size"]
        field_right = cf[0] + cs[0]
        x1 = int(field_right + self._CAPTCHA_LEFT_FROM_FIELD_RIGHT)
        x2 = int(change_btn_pos[0] + self._CAPTCHA_RIGHT_FROM_CHANGE_BTN)
        y1 = int(cf[1] - self._CAPTCHA_Y_PAD)
        h = int(cs[1] + self._CAPTCHA_Y_PAD * 2)
        w = max(x2 - x1, 40)
        try:
            shot = self._pa.screenshot(region=(x1, y1, w, h))
            big = shot.resize((w * 4, h * 4))
            buf = io.BytesIO()
            big.save(buf, format="PNG")
            if not hasattr(self, "_ocr") or self._ocr is None:
                self._ocr = ddddocr.DdddOcr(show_ad=False)
            code = self._ocr.classification(buf.getvalue())
            code = (code or "").strip()
            logger.info(f"验证码识别: {code!r}")
            return code
        except Exception as e:
            logger.error(f"验证码截图/识别失败: {e}")
            return None

    def login(self, max_attempts: int = 5) -> bool:
        """实盘交易账户自动登录（超时掉线后重新登录）。

        流程：打开登录浮层 → 填资金账号(可选)/密码 → 截图OCR识别验证码 →
        填验证码 → 点登录 → 校验是否已登录；验证码错误则「点击换图」重试。

        Args:
            max_attempts: 验证码识别+登录的最大尝试次数

        Returns:
            True 登录成功；False 失败（凭据缺失/结构不匹配/多次验证码失败）
        """
        if not self.trade_password:
            logger.error("未配置交易密码(trade_password)，无法自动登录")
            return False
        if self._pa is None:
            logger.error("pyautogui 未初始化，请先 connect()")
            return False
        if not self._activate() or not self._goto_trade_tab():
            logger.error("无法进入交易页，无法登录")
            return False
        # 已登录则无需重复登录
        if self.is_logged_in() and self._find_login_window() is None:
            logger.info("账户已登录，无需重新登录")
            return True

        login = self._open_login_window()
        if login is None:
            logger.error("无法打开登录浮层")
            return False

        for attempt in range(1, max_attempts + 1):
            logger.info(f"[登录] 第 {attempt}/{max_attempts} 次尝试")
            login = self._find_login_window()
            if login is None:
                # 可能已登录成功导致浮层关闭
                if self.is_logged_in():
                    logger.info("登录成功（浮层已关闭）")
                    return True
                login = self._open_login_window()
                if login is None:
                    logger.error("登录浮层丢失且无法重开")
                    return False

            loc = self._locate_login_fields(login)
            if not loc:
                logger.error("登录框结构不匹配，无法自动登录")
                return False

            # 填资金账号（若配置且与预填不同）
            if self.fund_account:
                cur = (ax.value(loc["acct"]["el"]) or "").strip()
                if cur != self.fund_account.strip():
                    self._fill_field(loc["acct"]["el"], self.fund_account)

            # 填密码
            self._fill_field(loc["pwd"]["el"], self.trade_password)
            time.sleep(0.2)

            # 识别并填验证码
            code = self._recognize_captcha(loc["captcha"], loc["change_btn_pos"])
            if not code or not (code.isdigit() and len(code) == 4):
                logger.warning(f"验证码识别无效({code!r})，点击换图重试")
                self._click_change_captcha(loc["change_btn_pos"])
                time.sleep(0.8)
                continue
            self._fill_field(loc["captcha"]["el"], code)
            time.sleep(0.2)

            # 点击登录按钮
            if not self._click_login_button(login):
                logger.error("未找到登录按钮")
                return False
            time.sleep(2.0)

            # 校验登录结果
            if self._find_login_window() is None and self.is_logged_in():
                logger.info("✅ 自动登录成功")
                return True
            # 仍未登录：验证码/密码错误，换图重试
            logger.warning("登录未成功（验证码或密码错误），换图重试")
            login2 = self._find_login_window()
            if login2 is not None:
                loc2 = self._locate_login_fields(login2)
                if loc2:
                    self._click_change_captcha(loc2["change_btn_pos"])
                    time.sleep(0.8)

        logger.error(f"自动登录失败（已尝试 {max_attempts} 次）")
        return False

    # 解锁浮层：小尺寸窗口（实测 361×216），仅含 1 个密码输入框 + 确定按钮
    _UNLOCK_WIN_W_RANGE = (300, 460)
    _UNLOCK_WIN_H_RANGE = (150, 300)

    def _find_unlock_window(self):
        """查找「解锁证券账户」浮层（小窗口，仅需密码），返回窗口或 None。"""
        if self._app is None:
            return None
        wlo, whi = self._UNLOCK_WIN_W_RANGE
        hlo, hhi = self._UNLOCK_WIN_H_RANGE
        for w in ax.all_windows(self._app):
            s = ax.size(w)
            if not s:
                continue
            if wlo < s[0] < whi and hlo < s[1] < hhi:
                # 校验含「解锁」提示文本，避免误匹配其他小窗
                txts = " ".join(v for v, _ in ax.collect_static_texts(w))
                if "解锁" in txts or "已锁定" in txts:
                    return w
        return None

    def unlock(self) -> bool:
        """实盘账户「已锁定」状态解锁（仅需交易密码，无验证码）。

        与 login() 不同：会话锁定时东财显示「解锁证券账户」按钮，点击后弹出
        小浮层（约361×216），仅含 1 个密码输入框 + 「确定」按钮，无需账号/验证码。

        Returns:
            True 解锁成功；False 失败（无密码/结构不匹配/密码错误）
        """
        if not self.trade_password:
            logger.error("未配置交易密码(trade_password)，无法自动解锁")
            return False
        if self._pa is None:
            logger.error("pyautogui 未初始化，请先 connect()")
            return False
        if not self._activate() or not self._goto_trade_tab():
            logger.error("无法进入交易页，无法解锁")
            return False

        win = self._window()
        if win is None:
            return False
        # 若解锁浮层已打开则直接复用，否则点击「解锁证券账户」按钮打开
        unlock_win = self._find_unlock_window()
        if unlock_win is None:
            btn = ax.find_button(win, "解锁证券账户")
            if btn is None:
                logger.warning("未找到「解锁证券账户」按钮，可能未锁定或界面异常")
                return False
            ax.press(btn)
            time.sleep(self.action_delay + 1.0)
            unlock_win = self._find_unlock_window()
        if unlock_win is None:
            logger.error("未找到解锁浮层")
            return False

        # 定位唯一的密码输入框
        pwd_fields = ax.collect(unlock_win, lambda el: ax.role(el) == "AXTextField")
        if not pwd_fields:
            logger.error("解锁浮层未找到密码输入框")
            return False
        self._fill_field(pwd_fields[0], self.trade_password)
        time.sleep(0.3)

        # 点击「确定」
        ok_btn = None
        for bb in ax.collect(unlock_win, lambda el: ax.role(el) == "AXButton"):
            if (ax.title(bb) or "").strip() == "确定":
                ok_btn = bb
                break
        if ok_btn is None:
            logger.error("解锁浮层未找到「确定」按钮")
            return False
        ax.press(ok_btn) or self._click_element(ok_btn)
        time.sleep(2.0)

        # 校验解锁结果
        if self.is_logged_in():
            logger.info("✅ 账户解锁成功")
            return True
        logger.error("解锁失败（密码错误或界面异常）")
        return False

    def _click_change_captcha(self, change_btn_pos) -> None:
        """点击「点击换图」刷新验证码。"""
        if change_btn_pos:
            try:
                self._pa.click(change_btn_pos[0], change_btn_pos[1])
            except Exception as e:
                logger.warning(f"点击换图失败: {e}")

    def _click_login_button(self, login) -> bool:
        """点击登录浮层中的「登录」按钮（标题可能含空格如「登   录」）。"""
        for bb in ax.collect(login, lambda el: ax.role(el) == "AXButton"):
            t = (ax.title(bb) or "").replace(" ", "").strip()
            if t == "登录":
                return ax.press(bb) or self._click_element(bb)
        return False

    # ── 只读：资金 ──

    def get_balance(self) -> Optional[dict]:
        """读取账户资金概况。

        Returns:
            dict: total_assets/available_cash/market_value/total_profit/
                  cash_balance/frozen，解析失败返回 None
        """
        if not self._activate() or not self._goto_trade_tab():
            return None
        win = self._window()
        if win is None:
            return None

        texts = ax.collect_static_texts(win)
        # 收集所有资金标签位置，以及全部文本（用于就近取值）
        # 坐标相对化：不再依赖硬编码 x 区间，而是「以标签为锚，取同一行、
        # 标签右侧最近的数值」，自适应窗口移动/缩放。
        label_items = []   # (x, y, field)
        all_items = []     # (x, y, raw)
        for v, p in texts:
            if not p:
                continue
            s = v.strip()
            all_items.append((p[0], p[1], s))
            if s in _BALANCE_FIELDS:
                label_items.append((p[0], p[1], _BALANCE_FIELDS[s]))

        def _parse_num(s: str):
            s2 = s.replace(",", "").replace("元", "").strip()
            try:
                return float(s2)
            except ValueError:
                return None

        result = {}
        for lx, ly, field in label_items:
            # 同一行(y 接近) 且在标签右侧(x 更大) 的最近数值
            best, best_dx = None, 1e9
            for x, y, raw in all_items:
                if abs(y - ly) > _ROW_Y_TOLERANCE:
                    continue
                if x <= lx:
                    continue
                num = _parse_num(raw)
                if num is None:
                    continue
                dx = x - lx
                if dx < best_dx:
                    best_dx, best = dx, num
            if best is not None:
                result[field] = best

        if not result:
            logger.warning("未能解析资金数据，请确认已切换到交易页并登录账户")
            return None
        logger.info(f"资金: 总资产={result.get('total_assets')} "
                    f"可用={result.get('available_cash')}")
        return result

    # ── 只读：持仓 ──

    def get_positions(self) -> list[dict]:
        """读取当前持仓列表。

        持仓表列布局（AX 探测得到的表头 x 坐标）：
        证券代码/证券名称/持仓数量/可用数量/摊薄成本价/浮动盈亏/浮动盈亏比/市价/市值
        """
        if not self._activate() or not self._goto_trade_tab():
            return []
        # 确保底部表格切到「持仓」视图（可能上次停在委托/成交 tab）
        if self._detect_view() != "chicang":
            if not self._click_chicang_tab():
                logger.error("无法切到「持仓」视图，放弃读取持仓")
                return []
        win = self._window()
        if win is None:
            return []
        col_map = {
            "证券代码": "stock_code", "证券名称": "stock_name",
            "持仓数量": "quantity", "可用数量": "available",
            # 模拟盘列头
            "摊薄成本价": "cost_price", "浮动盈亏": "pnl",
            "浮动盈亏比": "pnl_pct", "市价": "current_price", "市值": "market_value",
            # 实盘列头（名称不同）
            "成本价": "cost_price", "最新价": "current_price",
            "盈亏": "pnl", "盈亏比例%": "pnl_pct", "最新市值": "market_value",
        }
        headers = []  # (x, field)
        header_y = None
        for v, p in ax.collect_static_texts(win):
            pass  # 表头是 AXButton(排序按钮)，改用按钮收集

        header_btns = ax.collect(
            win, lambda el: ax.role(el) == "AXButton" and (ax.title(el) or "") in col_map
        )
        for b in header_btns:
            bp = ax.position(b)
            if bp:
                headers.append((bp[0], col_map[ax.title(b)]))
                header_y = bp[1] if header_y is None else max(header_y, bp[1])

        if not headers or header_y is None:
            logger.warning("未找到持仓表头，可能无持仓或界面未加载")
            return []

        headers.sort(key=lambda h: h[0])

        # 数据区右边界：最后一列表头 x + 半个列宽，排除表格右侧的行情区/
        # 自选股面板文本（如 x=1233 的股票名）串入持仓行导致列值错位。
        if len(headers) >= 2:
            col_w = (headers[-1][0] - headers[0][0]) / (len(headers) - 1)
        else:
            col_w = 80.0
        right_bound = headers[-1][0] + col_w * 0.6

        # 收集数据区（y > 表头，且在表格 x 范围内）的所有 StaticText，按行(y)聚类
        cells = [(v, p) for v, p in ax.collect_static_texts(win)
                 if p and p[1] > header_y + 5
                 and headers[0][0] - 30 <= p[0] <= right_bound]
        rows: dict[int, list] = {}
        for v, p in cells:
            row_key = None
            for ry in rows:
                if abs(ry - p[1]) <= 8:
                    row_key = ry
                    break
            if row_key is None:
                row_key = round(p[1])
                rows[row_key] = []
            rows[row_key].append((p[0], v.strip()))

        positions = []
        for ry in sorted(rows):
            row_cells = rows[ry]
            record = {}
            for cx, cval in row_cells:
                # 匹配最近的列
                field = min(headers, key=lambda h: abs(h[0] - cx))[1]
                record[field] = cval
            code = record.get("stock_code", "")
            if not (code.isdigit() and len(code) == 6):
                continue  # 过滤非持仓行
            positions.append({
                "stock_code": code,
                "stock_name": record.get("stock_name", ""),
                "quantity": _to_int(record.get("quantity")),
                "available": _to_int(record.get("available")),
                "cost_price": _to_float(record.get("cost_price")),
                "current_price": _to_float(record.get("current_price")),
                "market_value": _to_float(record.get("market_value")),
                "pnl": _to_float(record.get("pnl")),
                "pnl_pct": _to_float(record.get("pnl_pct", "").replace("%", "")),
            })

        logger.info(f"持仓数量: {len(positions)}")
        return positions

    def query_position(self, stock_code: str) -> Optional[Position]:
        """查询指定股票的持仓。"""
        for p in self.get_positions():
            if p["stock_code"] == stock_code:
                return Position(
                    stock_code=stock_code,
                    quantity=p["quantity"],
                    avg_cost=p["cost_price"],
                    current_price=p["current_price"],
                )
        return None

    # ── 下单 ──

    def place_order(self, stock_code, stock_name, action, price, quantity) -> OrderResult:
        """在东方财富交易页下单（买入/卖出），含完整异常流程与委托核对。

        流程：
          1. 下单前清理残留弹窗
          2. 填单 + 二次确认弹窗信息校验（代码/价格/数量），不符则中止并重试（≤max_retries）
          3. 校验通过点「是」提交，解析成功弹窗「委托提交成功，委托编号：XXX」
          4. 切换「成交」→「委托」标签刷新，按合同编号（回退代码/价格/数量）匹配委托明细
          5. 返回携带委托编号、状态、成交数量、完整明细的 OrderResult

        Args:
            stock_code: 6 位证券代码
            stock_name: 证券名称（仅用于日志）
            action: buy/add 买入，sell/reduce 卖出
            price: 委托价格（>0 为限价）
            quantity: 委托数量（100 的整数倍）

        Returns:
            OrderResult: 下单结果（order_id 为委托编号/合同编号）
        """
        if not self._pa:
            return OrderResult(success=False, error_msg="pyautogui 未初始化，请先 connect()")
        if not self._activate() or not self._goto_trade_tab():
            return OrderResult(success=False, error_msg="无法切换到交易页")

        # 价格规整：东财下单框只接受 2 位小数（A股最小价位 0.01），
        # 策略信号价可能带多余小数（如 136.0303）。此处统一四舍五入到分，
        # 确保「下单填值」与「确认弹窗校验」使用同一精度，避免校验永远不等。
        if price and price > 0:
            price = round(float(price), 2)

        # 下单前检查实盘登录状态（实盘约180分钟在线，超时需重新登录/解锁）
        # 模拟盘无「登录状态」标签，is_logged_in 返回 True 放行。
        # 检测到锁定/未登录时尝试自动恢复一次，失败则拒绝下单（不盲目下单）。
        if not self.is_logged_in():
            status = self.get_login_status()
            recovered = False
            if self.trade_password:
                if status and "锁定" in status:
                    logger.warning(f"下单前检测到账户已锁定（{status}），尝试解锁…")
                    recovered = self.unlock()
                else:
                    logger.warning(f"下单前检测到账户未登录（{status}），尝试登录…")
                    recovered = self.login()
            if not recovered:
                logger.error(f"实盘账户不可交易（状态={status}），拒绝下单")
                return OrderResult(
                    success=False,
                    error_msg=f"实盘账户不可交易（{status}），请先在东财登录/解锁交易账户")

        is_buy = action in ("buy", "add")
        submit_label = "买入" if is_buy else "卖出"

        try:
            # 下单前先清理残留弹窗，避免上一次未关闭的弹窗干扰
            self._dismiss_all_dialogs()

            last_err = ""
            for attempt in range(1, self.max_retries + 1):
                logger.info(f"下单尝试 {attempt}/{self.max_retries}: "
                            f"{stock_name}({stock_code}) {action} {quantity}股 @ {price}")

                # ① 填写下单表单（模式/代码/数量/价格）
                ok, fill_err = self._fill_order_form(is_buy, stock_code, price, quantity)
                if not ok:
                    last_err = fill_err
                    logger.warning(f"填单失败: {fill_err}，重试")
                    self._dismiss_all_dialogs()
                    continue

                # ② 点击提交按钮
                submit_btn = self._find_submit_button_retry(submit_label, attempts=6)
                if not submit_btn:
                    last_err = f"未找到「{submit_label}」提交按钮"
                    continue
                self._click_element(submit_btn)
                time.sleep(self.action_delay + 0.5)

                # ③ 等待二次确认弹窗（标题为买入/卖出，含 是/否）
                dialog, detail = self._wait_confirm_dialog(attempts=8)
                if not dialog:
                    # 未出现确认框：可能是「请检查输入参数是否正确」等参数错误提示
                    _w, _t, text = self._wait_result_dialog(attempts=3)
                    if text:
                        last_err = f"提交被拒（参数错误）: {text}"
                        logger.warning(last_err)
                    else:
                        last_err = "未弹出下单确认框（请检查代码/价格/数量是否有效）"
                    self._dismiss_all_dialogs()
                    continue

                # ④ 校验确认弹窗信息（代码 + 价格 + 数量）
                valid, verr = self._validate_confirm(detail, stock_code, price, quantity)
                if not valid:
                    logger.warning(f"确认信息校验失败: {verr}，点「否」中止并重试")
                    self._click_dialog_button(dialog, "否")
                    last_err = verr
                    self._dismiss_all_dialogs()
                    continue

                # ⑤ 信息正确，点「是」提交
                if not self.confirm_order:
                    logger.info(f"confirm_order=False，弹窗已弹出但未确认。明细: {detail}")
                    self._click_dialog_button(dialog, "否")
                    return OrderResult(success=False, error_msg="confirm_order=False，未自动确认")
                if not self._click_dialog_button(dialog, "是"):
                    last_err = "点击确认「是」失败"
                    continue
                time.sleep(self.action_delay + 0.6)

                # ⑥ 检查结果弹窗：成功「委托提交成功，委托编号：X」或错误「委托数量错误」
                _w, _t, text = self._wait_result_dialog(attempts=10)
                if text and any(k in text for k in ("错误", "失败", "不正确", "请检查")):
                    logger.warning(f"提交后错误弹窗: {text}")
                    self._dismiss_all_dialogs()
                    # 参数/数量错误重试无意义，直接返回失败
                    return OrderResult(success=False, error_msg=f"提交被拒: {text}")

                weituo_id = ""
                if text:
                    m = re.search(r"委托编号[:：]\s*(\d+)", text)
                    if m:
                        weituo_id = m.group(1)
                    logger.info(f"下单成功弹窗: {text}")
                else:
                    logger.warning("未捕获到成功弹窗文本，继续尝试从委托列表核对")

                # 关闭成功弹窗回到主界面
                self._dismiss_all_dialogs()

                # ⑦ 切换 成交→委托 标签刷新，匹配委托明细
                detail_row = self._query_order_after_submit(
                    weituo_id, stock_code, price, quantity)

                order_id = (weituo_id
                            or (detail_row.get("contract_id") if detail_row else "")
                            or f"EM-{stock_code}-{int(time.time())}")
                status_text = (detail_row.get("status") if detail_row else "") or "委托中"
                filled = _to_int(detail_row.get("filled_qty")) if detail_row else 0

                logger.info(f"✅ 委托成功: {stock_name}({stock_code}) "
                            f"委托编号={order_id} 状态={status_text} 已成交={filled}")
                return OrderResult(
                    success=True,
                    order_id=order_id,
                    exec_price=float(price or 0),
                    exec_qty=int(quantity),
                    status_text=status_text,
                    filled_qty=filled,
                    order_detail=detail_row,
                )

            return OrderResult(
                success=False,
                error_msg=f"下单失败（已重试{self.max_retries}次）: {last_err}")

        except Exception as e:
            logger.error(f"下单异常: {e}", exc_info=True)
            return OrderResult(success=False, error_msg=str(e))

    def _fill_order_form(self, is_buy: bool, stock_code: str, price, quantity):
        """填写下单表单（模式/代码/数量/价格），返回 (ok, err_msg)。

        价格必须在证券代码之后填写，避免被系统异步带出的现价覆盖，并回读校验。
        """
        win = self._window()
        self._select_mode(win, is_buy)
        win = self._window()
        time.sleep(self.action_delay)

        # 证券代码（回车后带出名称/现价）
        code_field = self._find_field_by_label(win, "证券代码")
        if not code_field:
            return False, "未找到证券代码输入框"
        self._fill_field(code_field, stock_code)
        self._pa.press("return")
        time.sleep(self.action_delay + 0.6)
        win = self._window()

        # 数量
        qty_label = "买入数量" if is_buy else "卖出数量"
        qty_field = self._find_field_by_label(win, qty_label)
        if not qty_field:
            return False, "未找到数量输入框"
        self._fill_field(qty_field, str(int(quantity)))
        time.sleep(self.action_delay)

        # 价格（最后填 + 回读校验）
        if price and price > 0:
            price_label = "买入价格" if is_buy else "卖出价格"
            target_price = f"{price:g}"
            win = self._window()
            price_field = self._find_field_by_label(win, price_label)
            if not price_field:
                return False, "未找到价格输入框"
            self._fill_field(price_field, target_price)
            time.sleep(self.action_delay)
            actual = str(ax.value(self._find_field_by_label(self._window(), price_label)) or "").strip()
            if actual and abs(_to_float(actual) - price) > 1e-6:
                logger.warning(f"价格被覆盖({actual})，重填 {target_price}")
                self._fill_field(self._find_field_by_label(self._window(), price_label), target_price)
                time.sleep(self.action_delay)
        return True, ""

    def _validate_confirm(self, detail: str, stock_code: str, price, quantity):
        """校验二次确认弹窗明细与下单目标一致（代码/价格/数量），返回 (ok, err)。"""
        detail = detail or ""
        if stock_code not in detail:
            return False, f"确认弹窗代码不符。明细: {detail}"
        if price and price > 0:
            m = re.search(r"委托价格[:：]\s*([\d.]+)", detail)
            if m and abs(float(m.group(1)) - price) > 1e-6:
                return False, f"确认弹窗价格({m.group(1)})≠目标({price})。明细: {detail}"
        mq = re.search(r"委托数量[:：]\s*(\d+)", detail)
        if mq and int(mq.group(1)) != int(quantity):
            return False, f"确认弹窗数量({mq.group(1)})≠目标({quantity})。明细: {detail}"
        return True, ""

    def _query_order_after_submit(self, contract_id: str, stock_code: str, price, quantity):
        """提交成功后切换「成交」→「委托」标签刷新，并匹配目标委托行。

        用户实测：东方财富需先点「成交」再点「委托」标签，委托列表才会刷新。
        """
        self._activate()
        self._goto_trade_tab()
        for _ in range(5):
            if not self._refresh_weituo_tab():
                logger.error("无法切到「委托」视图，放弃委托明细核对")
                return None
            rows = self._read_weituo_table()
            row = self._match_weituo_row(rows, contract_id, stock_code, price, quantity)
            if row:
                logger.info(f"委托明细匹配成功: {row}")
                return row
            time.sleep(0.5)
        logger.warning("未在委托列表中匹配到目标委托")
        return None

    def _tab_xy(self, key: str) -> Optional[tuple]:
        """按校准偏移 + 当前窗口左上角还原某 tab 的绝对点击坐标。

        Args:
            key: 'chicang' / 'chengjiao' / 'weituo'
        Returns:
            (x, y) 绝对屏幕坐标；无校准或无窗口返回 None。
        """
        off = self._tab_offsets.get(key)
        if not off:
            return None
        win = self._window()
        if win is None:
            return None
        wpos = ax.position(win)
        if not wpos:
            return None
        return (wpos[0] + off[0], wpos[1] + off[1])

    def _switch_tab(self, key: str, verify: bool = True,
                    attempts: int = 3) -> bool:
        """点击指定 tab 并（可选）用表头字段校验是否切换成功。

        校准坐标失效（窗口尺寸变化等）时校验会失败，此时返回 False，
        由上层中断并提示重新校准，绝不静默使用错误坐标。

        Args:
            key: 'chicang' / 'chengjiao' / 'weituo'
            verify: 是否点击后用 _detect_view 校验
            attempts: 点击+校验的重试次数
        Returns:
            切换（并校验）成功返回 True。
        """
        for i in range(attempts):
            xy = self._tab_xy(key)
            if xy is None:
                logger.error(f"无法还原 {key} tab 坐标（未校准或窗口不可用）")
                return False
            self._pa.click(*xy)
            time.sleep(self.action_delay + 0.4)
            if not verify:
                return True
            if self._detect_view() == key:
                return True
            logger.warning(f"切换 {key} tab 校验未通过（第{i+1}次），坐标={xy}")
        logger.error(
            f"切换到「{key}」失败：校准坐标可能已失效（窗口尺寸变化？）。"
            f"请重新校准：python3 agent.py --mode calibrate")
        return False

    def _refresh_weituo_tab(self) -> bool:
        """点击「成交」再点「委托」标签以强制刷新委托列表，返回是否成功。

        用户实测：东方财富需先点「成交」再点「委托」，委托列表才会刷新。
        任一步校验失败即返回 False（提示重新校准）。
        """
        if not self._switch_tab("chengjiao"):
            return False
        if not self._switch_tab("weituo"):
            return False
        return True

    def _refresh_chengjiao_tab(self) -> bool:
        """点击「委托」再点「成交」标签以强制刷新成交列表，返回是否成功。

        与委托刷新对称：东财列表不会主动刷新，需切走再切回来才拿到最新数据。
        """
        if not self._switch_tab("weituo"):
            return False
        if not self._switch_tab("chengjiao"):
            return False
        return True

    def _click_chicang_tab(self) -> bool:
        """点击并切换到「持仓」标签页，返回是否成功（含校验）。"""
        return self._switch_tab("chicang")

    # 各视图的特征列头字段（用于识别当前底部表格处于哪个 tab）
    _HDR_CHICANG = {"持仓数量", "可用数量",
                    "摊薄成本价", "浮动盈亏", "浮动盈亏比", "市价", "市值",
                    "成本价", "最新价", "盈亏", "盈亏比例%", "最新市值"}
    _HDR_CHENGJIAO = {"成交时间", "成交编号", "成交价格"}
    _HDR_WEITUO = {"申报时间", "状态说明", "委托价格", "委托数量"}

    def _detect_header_fields(self) -> set:
        """返回当前底部表格实际出现的列头字段集合（用于识别当前处于哪个标签页）。"""
        win = self._window()
        if win is None:
            return set()
        names = (self._HDR_CHICANG | self._HDR_CHENGJIAO | self._HDR_WEITUO |
                 {"证券代码", "证券名称", "操作", "合同编号", "成交数量",
                  "成交均价", "成交金额"})
        found = set()
        for b in ax.collect(win, lambda el: ax.role(el) == "AXButton"):
            t = ax.title(b)
            if t and t.strip() in names:
                found.add(t.strip())
        return found

    def _detect_view(self) -> Optional[str]:
        """识别当前底部表格所处视图：'chicang' / 'chengjiao' / 'weituo' / None。"""
        f = self._detect_header_fields()
        # 持仓表独有「持仓数量/摊薄成本价」；成交表独有「成交时间/成交编号」
        if f & self._HDR_CHICANG:
            return "chicang"
        if f & self._HDR_CHENGJIAO:
            return "chengjiao"
        if "合同编号" in f or (f & self._HDR_WEITUO):
            return "weituo"
        return None

    def _read_weituo_table(self) -> list:
        """读取当前「委托」表格全部数据行。

        列布局(AX 探测)：申报时间/证券代码/证券名称/操作/状态说明/
        委托价格/委托数量/成交数量/合同编号/成交均价/成交金额

        坐标相对化：表头 y、数据区左右边界均由「实际找到的列头按钮」动态推导，
        不再依赖固定像素范围，以适配不同窗口尺寸/分辨率。
        返回 [dict, ...]，键为内部字段名。
        """
        win = self._window()
        if win is None:
            return []
        col_map = {
            "申报时间": "apply_time", "证券代码": "stock_code", "证券名称": "stock_name",
            "操作": "operation", "状态说明": "status", "委托价格": "order_price",
            "委托数量": "order_qty", "成交数量": "filled_qty", "合同编号": "contract_id",
            "成交均价": "avg_price", "成交金额": "amount",
        }
        # 动态定位列头按钮：委托表列头是 AXButton，标题命中 col_map。
        # 通过统计各标题按钮的 y 值，取出现列头最多的那一行作为表头 y。
        header_candidates = []  # (x, y, field)
        for b in ax.collect(win, lambda el: ax.role(el) == "AXButton"):
            p = ax.position(b)
            t = ax.title(b)
            if p and t and t.strip() in col_map:
                header_candidates.append((p[0], p[1], col_map[t.strip()]))
        if not header_candidates:
            return []

        # 按 y 聚类，选列头数量最多的行为表头行
        y_groups: dict[int, list] = {}
        for x, y, field in header_candidates:
            key = None
            for gy in y_groups:
                if abs(gy - y) <= 6:
                    key = gy
                    break
            if key is None:
                key = round(y)
                y_groups[key] = []
            y_groups[key].append((x, field))
        header_y = max(y_groups, key=lambda gy: len(y_groups[gy]))
        cols = sorted(y_groups[header_y])  # [(x, field), ...]
        if not cols:
            return []

        # 数据区左右边界由列头 x 动态推导（留出边距容纳单元格文本宽度）
        x_min = cols[0][0] - 30
        x_max = cols[-1][0] + 200
        cells = [(p[0], p[1], v) for v, p in ax.collect_static_texts(win)
                 if p and p[1] > header_y + 5 and x_min <= p[0] <= x_max]
        rows: dict[int, list] = {}
        for x, y, v in cells:
            key = None
            for ry in rows:
                if abs(ry - y) <= 8:
                    key = ry
                    break
            if key is None:
                key = round(y)
                rows[key] = []
            rows[key].append((x, v))

        result = []
        for ry in sorted(rows):
            rec = {}
            for cx, cv in rows[ry]:
                field = min(cols, key=lambda c: abs(c[0] - cx))[1]
                rec[field] = cv.strip()
            code = rec.get("stock_code", "")
            if code.isdigit() and len(code) == 6:
                result.append(rec)
        return result

    def _match_weituo_row(self, rows: list, contract_id: str, stock_code: str,
                          price, quantity):
        """在委托行中匹配目标委托：优先按合同编号，回退到 代码+价格+数量。"""
        if contract_id:
            for r in rows:
                if r.get("contract_id") == contract_id:
                    return r
        # 回退匹配（无合同编号时）：代码 + 价格 + 数量
        for r in rows:
            if r.get("stock_code") != stock_code:
                continue
            if price and price > 0 and abs(_to_float(r.get("order_price")) - price) > 1e-3:
                continue
            if _to_int(r.get("order_qty")) != int(quantity):
                continue
            return r
        return None

    def _read_chengjiao_table(self) -> list:
        """读取当前「成交」表格全部数据行（假定已切到成交 tab）。

        列布局（AX 探测）：成交时间/证券代码/证券名称/操作/成交价格/
        成交数量/成交金额/合同编号/成交编号

        返回每行 dict，字段均为 snake_case 原始字段名。
        注意：本方法不负责切 tab / 刷新，由调用方（query_orders）统一编排。
        """
        win = self._window()
        if win is None:
            return []
        col_map = {
            "成交时间": "deal_time", "证券代码": "stock_code", "证券名称": "stock_name",
            "操作": "operation", "成交价格": "deal_price", "成交数量": "deal_qty",
            "成交金额": "amount", "合同编号": "contract_id", "成交编号": "deal_id",
        }
        header_candidates = []  # (x, y, field)
        for b in ax.collect(win, lambda el: ax.role(el) == "AXButton"):
            p = ax.position(b)
            t = ax.title(b)
            if p and t and t.strip() in col_map:
                header_candidates.append((p[0], p[1], col_map[t.strip()]))
        if not header_candidates:
            return []

        # 按 y 聚类，选列头数量最多的行为表头行
        y_groups: dict[int, list] = {}
        for x, y, field in header_candidates:
            key = None
            for gy in y_groups:
                if abs(gy - y) <= 6:
                    key = gy
                    break
            if key is None:
                key = round(y)
                y_groups[key] = []
            y_groups[key].append((x, field))
        header_y = max(y_groups, key=lambda gy: len(y_groups[gy]))
        cols = sorted(y_groups[header_y])  # [(x, field), ...]
        if not cols:
            return []

        x_min = cols[0][0] - 30
        x_max = cols[-1][0] + 200
        cells = [(p[0], p[1], v) for v, p in ax.collect_static_texts(win)
                 if p and p[1] > header_y + 5 and x_min <= p[0] <= x_max]
        rows: dict[int, list] = {}
        for x, y, v in cells:
            key = None
            for ry in rows:
                if abs(ry - y) <= 8:
                    key = ry
                    break
            if key is None:
                key = round(y)
                rows[key] = []
            rows[key].append((x, v))

        result = []
        for ry in sorted(rows):
            rec = {}
            for cx, cv in rows[ry]:
                field = min(cols, key=lambda c: abs(c[0] - cx))[1]
                rec[field] = cv.strip()
            code = rec.get("stock_code", "")
            if code.isdigit() and len(code) == 6:
                result.append(rec)
        return result

    # ── 弹窗管理 ──

    def _list_dialogs(self) -> list:
        """列出当前所有独立弹窗窗口（面积明显小于主窗口者）。

        返回 [(window, title, text), ...]，text 为窗口内所有静态文本拼接。
        """
        wins = ax.all_windows(self._app)
        if not wins or len(wins) < 2:
            return []

        def _area(w):
            s = ax.size(w)
            return (s[0] * s[1]) if s else 0.0

        main = max(wins, key=_area)
        main_area = _area(main)
        dialogs = []
        for w in wins:
            if w is main:
                continue
            # 仅将明显小于主窗口的窗口视为弹窗
            if main_area and _area(w) >= main_area * 0.6:
                continue
            texts = [v for v, _p in ax.collect_static_texts(w)]
            dialogs.append((w, ax.title(w) or "", " ".join(texts)))
        return dialogs

    def _dismiss_all_dialogs(self, max_rounds: int = 8):
        """关闭所有残留弹窗。安全起见只点「否/取消/确定」或回车，绝不点「是」。

        避免在清理阶段误点「是」而提交订单。
        """
        for _ in range(max_rounds):
            dialogs = self._list_dialogs()
            if not dialogs:
                return
            w, _title, _text = dialogs[0]
            clicked = False
            for label in ("否", "取消", "确定"):
                btn = ax.find_button(w, label)
                if btn:
                    self._click_element(btn)
                    clicked = True
                    break
            if not clicked:
                # 无可点按钮则回车关闭
                self._pa.press("return")
            time.sleep(0.4)

    def _wait_result_dialog(self, attempts: int = 10):
        """等待下单结果弹窗（成功「委托提交成功」或错误「XX错误/请检查」）。

        返回 (window, title, text)，未出现返回 (None, None, None)。
        """
        keywords = ("成功", "委托编号", "错误", "失败", "不正确", "请检查", "参数")
        for _ in range(attempts):
            for w, title, text in self._list_dialogs():
                if any(k in text for k in keywords) or any(k in title for k in keywords):
                    return w, title, text
            time.sleep(0.3)
        return None, None, None

    def _select_mode(self, win, is_buy: bool):
        """点击买入/卖出模式切换标签。

        通过定位下单区提交按钮（买入/卖出）间接确定当前模式所在区域；
        若已存在对应模式的提交按钮则无需切换。

        实盘中买/卖切换标签为自绘控件（AX 不可见），通过校准坐标切换。
        校准坐标存于 tab_calibration.json 的 buy_sell_offsets 字段。
        """
        target = "买入" if is_buy else "卖出"
        target_key = "buy" if is_buy else "sell"

        # 若已能找到目标提交按钮，说明已处于目标模式
        if self._find_submit_button(win, target):
            return

        # 尝试 AX 按钮点击（模拟盘可见）
        tab = ax.find_button(win, target)
        if tab:
            ax.press(tab)
            time.sleep(self.action_delay)
            # 验证切换结果
            if self._find_submit_button_retry(target, attempts=3):
                return

        # 实盘买/卖标签为自绘控件，使用校准坐标点击
        reverse = "卖出" if is_buy else "买入"
        reverse_key = "sell" if is_buy else "buy"
        if self._find_submit_button(win, reverse):
            # 当前在反向模式，尝试用校准坐标切换到目标模式
            from . import tab_calibrator
            data = tab_calibrator.load_calibration(self.calibration_path)
            buysell = (data or {}).get("buy_sell_offsets", {})
            offset = buysell.get(target_key)
            if offset and len(offset) == 2:
                wpos = ax.position(win)
                if wpos:
                    x, y = wpos[0] + offset[0], wpos[1] + offset[1]
                    logger.info(f"切换买/卖模式: → {target} @ ({x:.0f},{y:.0f})")
                    self._pa.click(x, y)
                    time.sleep(self.action_delay + 0.5)
                    if self._find_submit_button_retry(target, attempts=3):
                        logger.info(f"买/卖模式切换成功: {target}")
                        return
                    logger.warning(f"校准坐标点击后未能切换到 {target}")
            else:
                logger.warning(
                    f"「{target}」标签为自绘控件且无校准坐标。"
                    f"请在 tab_calibration.json buy_sell_offsets 中添加 {target_key} 偏移，"
                    f"或运行 python3 agent.py --mode calibrate 校准。"
                )
        else:
            logger.warning(f"未找到「{target}」或「{reverse}」提交按钮，下单区可能不可用")

    def _find_submit_button(self, win, label: str):
        """定位下单区「买入/卖出」提交按钮，排除顶部导航栏的同名按钮。

        坐标相对化：以主窗口 origin+size 换算相对比例，
        提交按钮位于窗口左侧下单区（相对 x < 0.5）且垂直靠下（相对 y > 0.3），
        以适配不同窗口尺寸/分辨率，而非写死绝对像素。
        """
        wpos = ax.position(win)
        wsize = ax.size(win)
        candidates = ax.collect(
            win, lambda el: ax.role(el) == "AXButton" and ax.title(el) == label
        )
        for b in candidates:
            p = ax.position(b)
            if not p:
                continue
            if wpos and wsize and wsize[0] > 0 and wsize[1] > 0:
                rx = (p[0] - wpos[0]) / wsize[0]
                ry = (p[1] - wpos[1]) / wsize[1]
                # 下单区提交按钮：左半部、垂直中下部
                if 0.1 < rx < 0.5 and ry > 0.3:
                    return b
            else:
                # 无法取窗口几何时的兜底（旧逻辑）
                if 200 < p[0] < 470 and p[1] > 380:
                    return b
        return None

    def _find_submit_button_retry(self, label: str, attempts: int = 6):
        """多次重新获取窗口并查找提交按钮，规避填单后 UI 短暂重渲染导致的查找失败。"""
        for i in range(attempts):
            btn = self._find_submit_button(self._window(), label)
            if btn:
                return btn
            time.sleep(0.3)
        return None

    def _find_confirm_dialog(self):
        """在所有窗口中查找下单确认弹窗（标题为「买入」/「卖出」的小窗口）。

        东方财富确认弹窗是独立窗口，内含说明文本与「是 / 否」按钮。
        返回 (dialog_window, detail_text) 或 (None, None)。
        """
        for w in ax.all_windows(self._app):
            wt = ax.title(w)
            s = ax.size(w)
            # 弹窗特征：标题为买入/卖出，且尺寸远小于主窗口
            if wt in ("买入", "卖出") and s and s[0] < 500 and s[1] < 500:
                detail = ""
                for v, _p in ax.collect_static_texts(w):
                    if "证券代码" in v or "委托" in v:
                        detail = v.replace("\n", " ")
                        break
                return w, detail
        return None, None

    def _wait_confirm_dialog(self, attempts: int = 8):
        """重试等待下单确认弹窗出现，返回 (dialog_window, detail_text)。"""
        for _ in range(attempts):
            dialog, detail = self._find_confirm_dialog()
            if dialog:
                return dialog, detail
            time.sleep(0.3)
        return None, None

    def _find_cancel_dialog(self):
        """查找撤单确认弹窗。

        撤单弹窗为独立小窗口，内含文本如「是否确定撤销[688331]的[买入]委托?」，
        按钮为「是 / 否」。标题不固定（可能是「提示」「撤单」等），故不依赖标题，
        改为遍历所有小尺寸窗口，匹配含「撤销」或「撤单」字样的说明文本。

        返回 (dialog_window, detail_text) 或 (None, None)。
        """
        for w in ax.all_windows(self._app):
            s = ax.size(w)
            if not s or s[0] >= 500 or s[1] >= 500:
                continue
            for v, _p in ax.collect_static_texts(w):
                if "撤销" in v or "撤单" in v:
                    return w, v.replace("\n", " ")
        return None, None

    def _wait_cancel_dialog(self, attempts: int = 8):
        """重试等待撤单确认弹窗出现，返回 (dialog_window, detail_text)。"""
        for _ in range(attempts):
            dialog, detail = self._find_cancel_dialog()
            if dialog:
                return dialog, detail
            time.sleep(0.3)
        return None, None

    def _double_click_at(self, x: float, y: float):
        """在屏幕坐标 (x, y) 双击。

        pyautogui 的 click(clicks=2) 在 macOS 上发送的是两次独立单击
        （clickState 均为 1），东财不识别为双击。这里用 Quartz 构造原生
        双击事件（第二次 down/up 的 clickState=2），确保被识别为双击。
        失败时回退到 pyautogui 连点。
        """
        try:
            import Quartz
            src = (Quartz.kCGEventLeftMouseDown, Quartz.kCGEventLeftMouseUp)
            pt = Quartz.CGPointMake(x, y)

            def _post(evt_type, click_state):
                ev = Quartz.CGEventCreateMouseEvent(
                    None, evt_type, pt, Quartz.kCGMouseButtonLeft)
                Quartz.CGEventSetIntegerValueField(
                    ev, Quartz.kCGMouseEventClickState, click_state)
                Quartz.CGEventPost(Quartz.kCGHIDEventTap, ev)

            # 第一次单击
            _post(src[0], 1)
            _post(src[1], 1)
            time.sleep(0.05)
            # 第二次单击，clickState=2 → 系统识别为双击
            _post(src[0], 2)
            _post(src[1], 2)
            time.sleep(0.2)
            return
        except Exception as e:
            logger.warning(f"_double_click_at: Quartz 双击失败，回退 pyautogui: {e}")
        self._pa.click(x, y, clicks=2, interval=0.1)
        time.sleep(0.2)

    def _click_dialog_button(self, dialog, label: str) -> bool:
        """点击确认弹窗中的指定按钮（「是」/「否」/「确定」），返回是否成功。"""
        btn = ax.find_button(dialog, label)
        if label == "是" and btn is None:
            btn = ax.find_button(dialog, "确定")
        if btn is None:
            logger.warning(f"确认弹窗未找到「{label}」按钮")
            return False
        ok = self._click_element(btn)
        if ok:
            logger.info(f"已点击弹窗「{label}」")
        return ok

    def cancel_order(self, order_id: str) -> bool:
        """撤单：切换到委托 tab，找到指定 contract_id 的订单，点击撤单按钮。

        东方财富委托列表中，状态为「已申报」「部分成交」的订单行内有「撤单」按钮。
        该按钮为 AXButton（非 AXStaticText），通过 y 坐标匹配定位目标行。

        Args:
            order_id: 合同编号（contract_id，如「271231」），来自下单后查询结果。

        Returns:
            撤单请求已提交返回 True；未找到订单/按钮/无法操作返回 False。
        """
        if not order_id:
            logger.warning("cancel_order: order_id 为空")
            return False

        # 1. 切换到委托 tab
        if not self._activate() or not self._goto_trade_tab():
            return False
        if not self._switch_tab("weituo", verify=False):
            logger.warning("cancel_order: 无法切换到委托 tab")
            return False
        self._refresh_weituo_tab()
        time.sleep(self.action_delay)

        # 2. 读取委托列表，定位目标行
        win = self._window()
        if win is None:
            return False
        rows = self._read_weituo_table()
        target = None
        for r in rows:
            if r.get("contract_id") == order_id:
                target = r
                break
        if target is None:
            logger.warning(f"cancel_order: 未找到合同编号 {order_id} 的委托")
            return False

        status = target.get("status", "")
        if status in ("已成交", "已撤单", "已拒绝", "废单"):
            logger.warning(f"cancel_order: 订单 {order_id} 状态={status}，无法撤单")
            return False

        logger.info(f"cancel_order: 找到订单 {order_id} status={status}，定位数据行并双击撤单...")

        # 3. 定位目标行的屏幕坐标（双击该行会弹出撤单确认弹窗）
        stock_code = target.get("stock_code", "")
        # 先找表头 y（AXButton 列头的最小 y），用于排除表头误匹配
        header_y = None
        col_names = {"申报时间", "证券代码", "证券名称", "操作", "状态说明",
                     "委托价格", "委托数量", "成交数量", "合同编号", "成交均价", "成交金额"}
        for b in ax.collect(win, lambda el: ax.role(el) == "AXButton"):
            p = ax.position(b)
            t = ax.title(b)
            if p and t and t.strip() in col_names:
                if header_y is None or p[1] < header_y:
                    header_y = p[1]

        # 在所有 StaticText 中找 order_id 对应的行 y 坐标
        texts = ax.collect_static_texts(win)
        target_y = None
        for v, p in texts:
            if p and str(v).strip() == order_id:
                if header_y is None or p[1] > (header_y or 0) + 5:
                    target_y = p[1]
                    break
        if target_y is None:
            # 兜底：匹配 stock_code（6 位）所在行
            for v, p in texts:
                if p and v and stock_code in str(v) and len(str(v).strip()) == 6:
                    if header_y is None or p[1] > (header_y or 0) + 5:
                        target_y = p[1]
                        break
        if target_y is None:
            logger.warning(f"cancel_order: 无法定位订单 {order_id} 的数据行坐标")
            return False

        # 取该行「证券代码」单元格（左侧核心列）的 x 作为双击点。
        # 合同编号在最右列，东财对最右列双击不弹撤单窗；证券代码/名称列更可靠。
        click_x, click_y = None, target_y
        for v, p in texts:
            if p and abs(p[1] - target_y) < 8 and str(v).strip() == stock_code:
                click_x = p[0]
                break
        if click_x is None:
            # 兜底：用该行最左侧的 StaticText
            row_cells = [(p[0], p[1]) for v, p in texts if p and abs(p[1] - target_y) < 8]
            if row_cells:
                click_x = min(c[0] for c in row_cells)
        if click_x is None:
            logger.warning(f"cancel_order: 无法定位订单 {order_id} 行的可双击单元格")
            return False
        target_xy = (click_x, click_y)

        # 4. 双击该行证券代码单元格 → 弹出撤单确认弹窗
        logger.info(f"cancel_order: 双击数据行证券代码单元格 @ {target_xy}")
        self._double_click_at(target_xy[0], target_xy[1])
        time.sleep(self.action_delay + 0.5)

        # 5. 处理撤单确认弹窗（文本如「是否确定撤销[688331]的[买入]委托?」）
        dialog, detail = self._wait_cancel_dialog(attempts=10)
        if not dialog:
            # 诊断：dump 所有窗口标题+尺寸，定位弹窗真实形态
            try:
                wins = ax.all_windows(self._app)
                logger.warning(f"cancel_order: 双击后未检测到撤单弹窗，当前 {len(wins)} 个窗口：")
                for w in wins:
                    logger.warning(f"  窗口 title='{ax.title(w)}' size={ax.size(w)}")
            except Exception as e:
                logger.warning(f"cancel_order: dump 窗口失败: {e}")
            return False
        logger.info(f"cancel_order: 检测到撤单确认弹窗: {detail}")
        ok = self._click_dialog_button(dialog, "是")
        if not ok:
            logger.warning("cancel_order: 点击「是」失败")
            return False
        time.sleep(self.action_delay)

        # 6. 刷新验证
        self._refresh_weituo_tab()
        time.sleep(self.action_delay)
        updated_rows = self._read_weituo_table()
        for r in updated_rows:
            if r.get("contract_id") == order_id:
                new_status = r.get("status", "")
                if new_status in ("已撤单", "已拒绝"):
                    logger.info(f"cancel_order: 订单 {order_id} 已成功撤单 (status={new_status})")
                    return True
                else:
                    logger.warning(f"cancel_order: 订单 {order_id} 状态未变: {status} → {new_status}")
                    return False

        # 订单已从列表消失（可能成交了）
        logger.info(f"cancel_order: 订单 {order_id} 已从委托列表消失")
        return True

    def query_orders(self) -> list[dict]:
        """查询全部委托+成交记录。

        复用下单成功后验证过的刷新逻辑：东财列表不会主动刷新，需「切走→切回」
        才能拿到最新状态。
        - 委托表：_refresh_weituo_tab（点成交→点委托）
        - 成交表：_refresh_chengjiao_tab（点委托→点成交）

        字段与委托表统一：contract_id / stock_code / stock_name /
        operation / status / order_price / order_qty / filled_qty / avg_price / amount
        """
        result = []
        # 激活窗口并切到交易页（tab 是自绘控件，坐标点击需窗口在前台）
        if not self._activate() or not self._goto_trade_tab():
            logger.warning("query_orders: 无法激活东财交易页")
            return []

        # 1) 委托表：先刷新（点成交→点委托）再读
        if self._refresh_weituo_tab():
            rows = self._read_weituo_table()
            for r in rows:
                logger.info(f"[委托行] 合同={r.get('contract_id')} 代码={r.get('stock_code')} "
                            f"状态='{r.get('status')}' 委托量={r.get('order_qty')} "
                            f"成交量={r.get('filled_qty')} 价格={r.get('order_price')}")
            result.extend(rows)
        else:
            logger.warning("query_orders: 委托 tab 刷新失败（可能需重新校准）")

        # 2) 成交表：先刷新（点委托→点成交）再读
        if self._refresh_chengjiao_tab():
            cj_rows = self._read_chengjiao_table()
            for r in cj_rows:
                logger.info(f"[成交行] 合同={r.get('contract_id')} 代码={r.get('stock_code')} "
                            f"成交量={r.get('deal_qty')} 成交价={r.get('deal_price')}")
                # 映射成交字段到委托格式（成交=全部成交）
                result.append({
                    "contract_id": r.get("contract_id", ""),
                    "stock_code":   r.get("stock_code", ""),
                    "stock_name":   r.get("stock_name", ""),
                    "operation":    r.get("operation", ""),
                    "status":       "全部成交",
                    "order_price":  r.get("deal_price", 0),
                    "order_qty":    r.get("deal_qty", 0),
                    "filled_qty":   r.get("deal_qty", 0),
                    "avg_price":    r.get("deal_price", 0),
                    "amount":       r.get("amount", 0),
                    "apply_time":   r.get("deal_time", ""),
                })
        else:
            logger.warning("query_orders: 成交 tab 刷新失败（可能需重新校准）")
        return result


def _to_int(s) -> int:
    """将字符串安全转换为整数，失败返回 0。"""
    try:
        return int(str(s).replace(",", "").strip())
    except (ValueError, AttributeError):
        return 0


def _to_float(s) -> float:
    """将字符串安全转换为浮点数，失败返回 0.0。"""
    try:
        return float(str(s).replace(",", "").replace("%", "").strip())
    except (ValueError, AttributeError):
        return 0.0
