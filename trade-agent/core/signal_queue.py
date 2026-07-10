"""Thread-safe signal queue with deduplication."""

import logging
import threading
from collections import deque

logger = logging.getLogger("agent.queue")


class SignalQueue:
    """A deduplicating, thread-safe queue for incoming trade signals."""

    def __init__(self):
        self._lock = threading.Lock()
        self._seen_ids = set()          # Dedup by signal_id
        self._queue = deque()           # Ordered signals
        self._cond = threading.Condition(self._lock)

    def put(self, signal):
        """Add a signal to the queue (dedup by signalId)."""
        sid = signal.get("signalId") or signal.get("signal_id")
        if not sid:
            logger.warning(f"Signal missing ID, skipping: {signal}")
            return

        with self._lock:
            if sid in self._seen_ids:
                return
            self._seen_ids.add(sid)
            self._queue.append(signal)
            self._cond.notify()

    def get(self, timeout=None):
        """Block until a signal is available, or return None after timeout."""
        with self._cond:
            if not self._queue:
                if not self._cond.wait(timeout):
                    return None
            if self._queue:
                return self._queue.popleft()
            return None

    def size(self):
        """Return current queue size."""
        with self._lock:
            return len(self._queue)

    def clear(self):
        """Clear all pending signals."""
        with self._lock:
            self._queue.clear()
            self._seen_ids.clear()
