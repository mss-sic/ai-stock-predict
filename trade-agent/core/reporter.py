"""Execution result reporter — claims signals and reports results back to server."""

import logging

logger = logging.getLogger("agent.reporter")


class Reporter:
    """Handles signal claiming and result reporting to the server API."""

    def __init__(self, api_client):
        self.api = api_client

    def claim(self, signal_id):
        """Claim a signal for execution. Returns claimed signal data or None."""
        result = self.api.claim_signal(signal_id)
        if result and result.get("claimed"):
            logger.info(f"Signal {signal_id} claimed: {result.get('stockCode')} {result.get('action')}")
            return result
        logger.warning(f"Failed to claim signal {signal_id}")
        return None

    def report_executed(self, signal_id, order_id, exec_price, exec_qty):
        """Report successful execution."""
        self.api.report_result(
            signal_id=signal_id,
            status="executed",
            order_id=order_id,
            exec_price=exec_price,
            exec_qty=exec_qty,
        )
        logger.info(f"Signal {signal_id} reported: executed @ {exec_price} x {exec_qty}")

    def report_submitted(self, signal_id, order_id, exec_price, exec_qty):
        """回报委托已提交（委托中/待成交）。

        委托成功提交到券商但尚未成交，信号应流转为 pending_order（委托中），
        而非 executed（已成交）。后续由 OrderSync 服务根据券商委托编号轮询
        成交状态，成交后再 FinalizeSignalExecution 更新持仓/资金。
        """
        self.api.report_result(
            signal_id=signal_id,
            status="submitted",
            order_id=order_id,
            exec_price=exec_price,
            exec_qty=exec_qty,
        )
        logger.info(f"Signal {signal_id} reported: submitted (委托中) 委托编号={order_id}")

    def report_failed(self, signal_id, error_msg):
        """Report failed execution."""
        self.api.report_result(
            signal_id=signal_id,
            status="order_failed",
            error_msg=error_msg,
        )
        logger.info(f"Signal {signal_id} reported: failed — {error_msg}")
