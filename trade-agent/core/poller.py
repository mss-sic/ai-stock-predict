"""HTTP polling fallback for signal retrieval."""

import logging
import threading
import time

logger = logging.getLogger("agent.poller")


class SignalPoller:
    """Polls the server REST API for pending_auto signals at a fixed interval.
    Acts as a fallback when WebSocket push is unavailable or disconnected.
    """

    def __init__(self, api_client, interval=30, on_signals=None):
        self.api = api_client
        self.interval = interval
        self.on_signals = on_signals  # callback(signals_list)
        self.thread = None
        self.running = False

    def start(self):
        """Start polling in a background thread."""
        if self.running:
            return
        self.running = True
        self.thread = threading.Thread(target=self._run, daemon=True)
        self.thread.start()
        logger.info(f"Signal poller started (interval={self.interval}s)")

    def stop(self):
        """Stop polling."""
        self.running = False
        logger.info("Signal poller stopped")

    def _run(self):
        while self.running:
            try:
                signals = self.api.get_pending_signals()
                if signals and self.on_signals:
                    for sig in signals:
                        self.on_signals(sig)
            except Exception as e:
                logger.error(f"Poll error: {e}")

            # Sleep in small chunks so we can stop quickly
            for _ in range(self.interval):
                if not self.running:
                    break
                time.sleep(1)
