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
        """回报委托已提交（委托中）。

        委托成功提交到券商但尚未成交（状态=委托中），携带券商委托编号回传。
        服务端 report-result 目前枚举为 executed/order_failed，此处以 executed 上报
        并附带券商委托编号(order_id)，用于后续按委托编号跟踪成交状态。
        """
        self.api.report_result(
            signal_id=signal_id,
            status="executed",
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
