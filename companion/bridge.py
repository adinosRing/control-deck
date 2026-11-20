"""Windows companion for the Kindle Personal Dashboard."""
import argparse
import hmac
import ipaddress
import json
import logging
import os
from pathlib import Path
import shutil
import ssl
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / "vendor"))

from browser_link import BrowserLink, BrowserUnavailable

LOG = logging.getLogger("kindle-dashboard")


class ActionUnavailable(Exception):
    pass


def launch(name):
    code = shutil.which("code.cmd")
    code_app = Path(code).parent.parent / "Code.exe" if code else None
    links = {
        "ChatGPT": "https://chatgpt.com/",
        "Claude": "https://claude.ai/",
        "VS Code": str(code_app) if code_app and code_app.is_file() else code,
        "Browser": "https://www.google.com/",
        "YouTube": "https://www.youtube.com/",
        "Terminal": shutil.which("wt.exe") or shutil.which("powershell.exe"),
    }
    target = links.get(name)
    if not target:
        raise ActionUnavailable(f"{name} is unavailable on the PC.")
    try:
        os.startfile(target)
    except OSError as exc:
        raise ActionUnavailable(f"Windows could not open {name}.") from exc


class BridgeHandler(BaseHTTPRequestHandler):
    def setup(self):
        self.request.settimeout(8)
        super().setup()

    def log_message(self, *_):
        pass

    def send_json(self, code, data):
        body = json.dumps(data, allow_nan=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def authorized(self):
        client = ipaddress.ip_address(self.client_address[0])
        header = self.headers.get("Authorization", "")
        if not any(client in network for network in self.server.networks) or not hmac.compare_digest(
            header.encode("utf-8"), self.server.authorization
        ):
            self.send_json(403, {"error": "PC pairing failed."})
            return False
        with self.server.clients_lock:
            if client not in self.server.clients:
                self.server.clients.add(client)
                LOG.info("Paired dashboard connected from %s.", client)
        return True

    def do_GET(self):
        if self.authorized():
            if self.path == "/status":
                link = self.server.browser
                try:
                    video = link.query("status") if link.ready() else None
                except BrowserUnavailable:
                    video = None
                self.send_json(200, {"youtube": video, "browser_ready": link.ready()})
            else:
                self.send_json(404, {"error": "Unknown dashboard request."})

    def do_POST(self):
        if not self.authorized():
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 512:
                raise ValueError("Invalid request length.")
            data = json.loads(self.rfile.read(length))
            if not isinstance(data, dict):
                raise ValueError("Invalid command.")
            if self.path == "/launch" and isinstance(data.get("name"), str):
                launch(data["name"])
                result = {"ok": True}
            elif self.path == "/youtube" and data.get("action") in {
                "previous", "next", "toggle", "seek", "captions", "repeat", "volume", "mute"
            }:
                value = data.get("value")
                if data["action"] in {"seek", "volume"} and (type(value) is not int or not 0 <= value <= 100):
                    raise ValueError("The control value must be between 0 and 100.")
                result = {"youtube": self.server.browser.query(data["action"], value), "browser_ready": True, "ok": True}
            else:
                raise ValueError("Unknown dashboard command.")
            self.send_json(200, result)
        except (ActionUnavailable, BrowserUnavailable) as exc:
            self.send_json(409, {"error": str(exc)})
        except (ValueError, TypeError):
            self.send_json(400, {"error": "Invalid dashboard command."})
        except Exception:
            LOG.exception("Dashboard action failed")
            self.send_json(503, {"error": "The PC could not complete that action."})


class BridgeServer(ThreadingHTTPServer):
    daemon_threads = True
    request_queue_size = 12

    def process_request_thread(self, request, address):
        # TLS handshakes happen in worker threads and respect a short timeout.
        try:
            request.settimeout(5)
            with self.tls.wrap_socket(request, server_side=True) as wrapped:
                super().process_request_thread(wrapped, address)
        except (ssl.SSLError, TimeoutError, OSError):
            self.shutdown_request(request)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=Path, default=HERE / "bridge-config.json")
    args = parser.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(message)s")
    config = json.loads(args.config.read_text(encoding="utf-8"))
    browser = BrowserLink(config["browser_token"])
    tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    tls.minimum_version = ssl.TLSVersion.TLSv1_2
    tls.load_cert_chain(args.config.parent / "bridge-cert.pem", args.config.parent / "bridge-key.pem")
    with BridgeServer(("0.0.0.0", config["port"]), BridgeHandler) as server:
        server.tls = tls
        server.browser = browser
        browser_server = browser.listen()
        server.authorization = ("Bearer " + config["token"]).encode("ascii")
        server.networks = tuple(ipaddress.ip_network(item) for item in config["networks"])
        server.clients = set()
        server.clients_lock = threading.Lock()
        LOG.info("Kindle Dashboard Companion is ready on TCP port %s.", config["port"])
        try:
            server.serve_forever()
        finally:
            browser_server.shutdown()


if __name__ == "__main__":
    main()
