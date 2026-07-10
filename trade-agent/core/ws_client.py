"""WebSocket client for real-time signal push and command dispatch from server."""

import json
import logging
import threading
import time

import websocket

logger = logging.getLogger("agent.ws")


class WSClient:
    """WebSocket client that connects to the server's signal push endpoint."""

    def __init__(self, server_url, token, on_signal, on_kicked,
                 on_test_request=None, on_command=None, on_open=None):
        ws_url = server_url.replace("https://", "wss://").replace("http://", "ws://")
        self.url = f"{ws_url}/api/v1/ws/signals?token={token}"
        self.token = token
        self.on_signal = on_signal          # callback(signal_data)
        self.on_kicked = on_kicked          # callback(reason)
        self.on_test_request = on_test_request  # callback(request_id)
        self.on_command = on_command        # callback(request_id, action, payload)
        self.on_open = on_open            # callback(ws) — called when WS connects
        self.ws = None
        self.thread = None
        self.running = False
        self.reconnect_delay = 5

    def start(self):
        if self.running:
            return
        self.running = True
        self.thread = threading.Thread(target=self._run, daemon=True)
        self.thread.start()
        logger.info("WebSocket client started")

    def stop(self):
        self.running = False
        if self.ws:
            try:
                self.ws.close()
            except Exception:
                pass
        logger.info("WebSocket client stopped")

    def send(self, data: dict):
        """Send a JSON message through the WebSocket."""
        if self.ws:
            try:
                import json
                self.ws.send(json.dumps(data, ensure_ascii=False))
            except Exception as e:
                logger.error(f"WS send error: {e}")

    def _run(self):
        while self.running:
            try:
                self.ws = websocket.WebSocketApp(
                    self.url,
                    on_message=self._on_message,
                    on_error=self._on_error,
                    on_close=self._on_close,
                    on_open=self._on_open,
                )
                self.ws.run_forever(ping_interval=30, ping_timeout=10)
            except Exception as e:
                logger.error(f"WebSocket error: {e}")

            if not self.running:
                break

            delay = min(self.reconnect_delay, 60)
            logger.info(f"Reconnecting in {delay}s...")
            time.sleep(delay)
            self.reconnect_delay = min(self.reconnect_delay * 2, 60)

    def _on_open(self, ws):
        self.reconnect_delay = 5
        logger.info("WebSocket connected")
        if self.on_open:
            try:
                self.on_open(ws)
            except Exception as e:
                logger.error(f"on_open callback error: {e}")

    def _on_message(self, ws, message):
        try:
            data = json.loads(message)
            msg_type = data.get("type", "")

            if msg_type == "new_signal":
                signal = data.get("data", {})
                logger.info(f"Received signal: {signal.get('stockCode')} {signal.get('actionType')}")
                if self.on_signal:
                    self.on_signal(signal)

            elif msg_type == "kicked":
                reason = data.get("data", {}).get("reason", "unknown")
                logger.warning(f"Kicked by server: {reason}")
                if self.on_kicked:
                    self.on_kicked(reason)

            elif msg_type == "command":
                # Server-dispatched broker command (sync_positions / place_order / etc.)
                cmd = data.get("data", {})
                request_id = cmd.get("requestId", "")
                action = cmd.get("action", "")
                payload = cmd.get("payload")
                logger.info(f"Received command: {action} requestID={request_id[:8]}")
                if self.on_command:
                    self.on_command(request_id, action, payload)
                else:
                    logger.warning(f"No on_command handler, dropping {action} {request_id[:8]}")

            elif msg_type == "test_request":
                request_id = data.get("data", {}).get("requestId", "")
                logger.info(f"Received test request: {request_id[:8]}")
                if self.on_test_request:
                    self.on_test_request(request_id)

            elif msg_type == "heartbeat":
                pass

        except json.JSONDecodeError:
            logger.warning(f"Invalid WS message: {message[:100]}")

    def _on_error(self, ws, error):
        logger.error(f"WebSocket error: {error}")

    def _on_close(self, ws, close_status_code, close_msg):
        logger.info(f"WebSocket closed: {close_status_code} {close_msg}")
