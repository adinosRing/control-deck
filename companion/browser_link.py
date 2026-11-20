"""Private loopback link between Control Deck and the YouTube extension."""

import hmac
import json
import logging
import re
from threading import Event, Lock, Thread
from uuid import uuid4

from websockets.exceptions import ConnectionClosed
from websockets.sync.server import serve

LOG = logging.getLogger("kindle-dashboard")
LOOPBACK_PORT = 8766


class BrowserUnavailable(Exception):
    pass


class BrowserLink:
    def __init__(self, token):
        self.token = token.encode("ascii")
        self.lock = Lock()
        self.peer = None
        self.pending = {}

    def listen(self):
        server = serve(
            self.handle, "127.0.0.1", LOOPBACK_PORT,
            origins=[re.compile(r"chrome-extension://[a-p]{32}")],
            max_size=8192, open_timeout=4,
        )
        Thread(target=server.serve_forever, daemon=True).start()
        return server

    def handle(self, socket):
        try:
            hello = json.loads(socket.recv(timeout=4))
            token = hello.get("token", "").encode("utf-8")
            if not hmac.compare_digest(token, self.token):
                socket.close(1008, "Invalid desktop pairing")
                return
        except (ConnectionClosed, TimeoutError, ValueError, AttributeError):
            return

        with self.lock:
            self.peer = socket
        LOG.info("YouTube browser extension connected.")
        try:
            for message in socket:
                try:
                    response = json.loads(message)
                    if not isinstance(response, dict) or not isinstance(response.get("id"), str):
                        continue
                    with self.lock:
                        request = self.pending.get(response["id"])
                        if request and request[0] is socket:
                            request[2].append(response)
                            request[1].set()
                except ValueError:
                    continue
        except ConnectionClosed:
            pass
        finally:
            with self.lock:
                if self.peer is socket:
                    self.peer = None
                for peer, done, replies in self.pending.values():
                    if peer is socket:
                        done.set()

    def ready(self):
        with self.lock:
            return self.peer is not None

    def query(self, action, value=None):
        token = uuid4().hex
        done = Event()
        replies = []
        with self.lock:
            peer = self.peer
            if peer is None:
                raise BrowserUnavailable("Enable the Control Deck YouTube extension in your PC browser.")
            self.pending[token] = (peer, done, replies)
        try:
            peer.send(json.dumps({"id": token, "action": action, "value": value}))
            if not done.wait(3.5) or not replies:
                raise BrowserUnavailable("The YouTube tab did not respond. Reload the video tab.")
            response = replies[0]
            if response.get("error"):
                raise BrowserUnavailable(str(response["error"])[:180])
            return response.get("video")
        except ConnectionClosed as exc:
            raise BrowserUnavailable("The browser extension disconnected.") from exc
        finally:
            with self.lock:
                self.pending.pop(token, None)
