"""Windows companion for the Kindle Personal Dashboard."""
import argparse
import hmac
import ipaddress
import json
import logging
import os
import re
from pathlib import Path
import shutil
import ssl
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / "vendor"))

from browser_link import BrowserLink, BrowserUnavailable
from pc_stats import PCStats
from cover_art import CoverArt
from weather import Weather

LOG = logging.getLogger("kindle-dashboard")


class ActionUnavailable(Exception):
    pass


CYBERPUNK_STEAM_APP = "1091500"


def steam_libraries():
    """Steam library folders, from the registry and libraryfolders.vdf."""
    roots = []
    try:
        import winreg
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Valve\Steam") as key:
            roots.append(Path(winreg.QueryValueEx(key, "SteamPath")[0]))
    except (ImportError, OSError):
        pass
    roots.append(Path(r"C:\Program Files (x86)\Steam"))
    libraries = []
    for root in roots:
        libraries.append(root)
        vdf = root / "steamapps" / "libraryfolders.vdf"
        try:
            text = vdf.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for match in re.finditer(r'"path"\s+"([^"]+)"', text):
            libraries.append(Path(match.group(1).replace("\\\\", "\\")))
    unique = []
    for library in libraries:
        if library not in unique:
            unique.append(library)
    return unique


RED_GAME_FOLDERS = ("Cyberpunk 2077", "The Witcher 3", "The Witcher 3 Wild Hunt")


def find_red_launcher():
    """Find REDprelauncher.exe in Steam, GOG or Epic game folders."""
    candidates = [
        library / "steamapps" / "common" / folder / "REDprelauncher.exe"
        for library in steam_libraries()
        for folder in RED_GAME_FOLDERS
    ]
    for letter in "CDEFGH":
        for parent in ("GOG Games", r"Program Files (x86)\GOG Galaxy\Games", r"Program Files\Epic Games", "Epic Games", "Games"):
            root = Path(f"{letter}:\\") / parent
            for pattern in ("Cyberpunk*/REDprelauncher.exe", "*Witcher 3*/REDprelauncher.exe"):
                try:
                    candidates.extend(root.glob(pattern))
                except OSError:
                    pass
    return next((path for path in candidates if path.is_file()), None)


def start_menu_shortcut(*names):
    """A Start Menu shortcut (.lnk) whose name matches, from the user's or all users' Start Menu."""
    wanted = {name.lower() for name in names}
    roots = [Path(os.environ.get(var, ""), "Microsoft", "Windows", "Start Menu", "Programs") for var in ("APPDATA", "PROGRAMDATA")]
    for root in roots:
        if not root.is_dir():
            continue
        for shortcut in root.rglob("*.lnk"):
            if shortcut.stem.lower() in wanted:
                return shortcut
    return None


def app_path(exe):
    """The full path Windows registers for an app such as brave.exe ("App Paths")."""
    try:
        import winreg
    except ImportError:
        return None
    for hive in (winreg.HKEY_CURRENT_USER, winreg.HKEY_LOCAL_MACHINE):
        try:
            with winreg.OpenKey(hive, rf"Software\Microsoft\Windows\CurrentVersion\App Paths\{exe}") as key:
                path = Path(winreg.QueryValueEx(key, "")[0].strip('"'))
                if path.is_file():
                    return path
        except OSError:
            continue
    return None


def find_browser():
    """Brave, Chrome or Edge, in that order."""
    local = Path(os.environ.get("LOCALAPPDATA", ""))
    programs = [Path(os.environ.get(var, "")) for var in ("PROGRAMFILES", "PROGRAMFILES(X86)")]
    for exe, folder in (("brave.exe", ("BraveSoftware", "Brave-Browser", "Application")),
                        ("chrome.exe", ("Google", "Chrome", "Application")),
                        ("msedge.exe", ("Microsoft", "Edge", "Application"))):
        found = app_path(exe)
        if found:
            return found
        for base in [local, *programs]:
            candidate = base.joinpath(*folder, exe)
            if candidate.is_file():
                return candidate
    return None


def open_first(targets):
    """Open the first target that Windows accepts; stale shortcuts are skipped."""
    for target in targets:
        if not target:
            continue
        try:
            os.startfile(str(target))
            return True
        except OSError as exc:
            LOG.info("Could not open %s: %s", target, exc)
    return False


CLAUDE_PACKAGE = "Claude_pzs8sxrjxfjjc"  # the Claude desktop app's package family on Windows


def package_app_id(name):
    """The launch ID (family!app) of an installed Windows app package, e.g. the Claude desktop app."""
    script = (
        "$p = Get-AppxPackage -Name $env:CD_APP | Select-Object -First 1; "
        "if ($p) { $id = (Get-AppxPackageManifest $p).Package.Applications.Application | "
        "Select-Object -First 1 -ExpandProperty Id; \"$($p.PackageFamilyName)!$id\" }"
    )
    try:
        result = subprocess.run(
            ["powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script],
            capture_output=True, text=True, timeout=10, env={**os.environ, "CD_APP": name},
            creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
        )
    except (OSError, subprocess.SubprocessError):
        return None
    app_id = result.stdout.strip()
    return app_id if "!" in app_id and not app_id.endswith("!") else None


def claude_targets():
    # Never the Start menu shortcut: an old install can leave one pointing at a deleted claude.exe.
    app_id = package_app_id("Claude")
    yield f"shell:AppsFolder\\{app_id}" if app_id else None
    local = Path(os.environ.get("LOCALAPPDATA", ""))
    if (local / "Packages" / CLAUDE_PACKAGE).is_dir():
        yield f"shell:AppsFolder\\{CLAUDE_PACKAGE}!Claude"
    for candidate in (local / "AnthropicClaude" / "claude.exe", local / "Programs" / "Claude" / "Claude.exe"):
        yield candidate if candidate.is_file() else None
    yield "claude://"
    yield "https://claude.ai/"


def find_vscode():
    code = shutil.which("code.cmd")
    if code:
        app = Path(code).parent.parent / "Code.exe"
        if app.is_file():
            return app
    return start_menu_shortcut("Visual Studio Code")


def open_web_app(name, url):
    """An installed web app if there is one, else the site in its own app window (no tabs or address bar)."""
    shortcut = start_menu_shortcut(name)
    if shortcut:
        os.startfile(str(shortcut))
        return
    browser = find_browser()
    if browser:
        subprocess.Popen([str(browser), f"--app={url}"], close_fds=True,
                         creationflags=getattr(subprocess, "DETACHED_PROCESS", 0))
        return
    os.startfile(url)


def launch(name):
    try:
        if name == "Cyberpunk":
            launcher = find_red_launcher()
            if launcher:
                # Run it from the game folder, as the RED launcher expects.
                os.startfile(str(launcher), cwd=str(launcher.parent))
            else:
                os.startfile(f"steam://rungameid/{CYBERPUNK_STEAM_APP}")
        elif name == "Claude":
            if not open_first(claude_targets()):
                raise ActionUnavailable("Windows could not open Claude.")
        elif name == "VS Code":
            app = find_vscode()
            if not app:
                raise ActionUnavailable("VS Code is not installed on the PC.")
            os.startfile(str(app))
        elif name == "Browser":
            browser = find_browser()
            os.startfile(str(browser) if browser else "https://www.google.com/")
        elif name == "YouTube":
            open_web_app("YouTube", "https://www.youtube.com/")
        elif name == "YouTube Music":
            open_web_app("YouTube Music", "https://music.youtube.com/")
        elif name == "ChatGPT":
            open_web_app("ChatGPT", "https://chatgpt.com/")
        elif name == "Terminal":
            terminal = shutil.which("wt.exe") or shutil.which("powershell.exe")
            if not terminal:
                raise ActionUnavailable("No terminal found on the PC.")
            os.startfile(terminal)
        else:
            raise ActionUnavailable(f"{name} is unavailable on the PC.")
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
                self.send_json(200, {"youtube": self.server.covers.publish(video), "browser_ready": link.ready(),
                                     "stats": self.server.stats.snapshot(),
                                     "weather": self.server.weather.snapshot()})
            elif self.path.startswith("/art?id="):
                cover = self.server.covers.get(self.path[len("/art?id="):])
                if not cover:
                    self.send_json(404, {"error": "That album cover is no longer playing."})
                    return
                data, kind = cover
                self.send_response(200)
                self.send_header("Content-Type", kind)
                self.send_header("Cache-Control", "no-store")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)
            else:
                self.send_json(404, {"error": "Unknown dashboard request."})

    def do_POST(self):
        if not self.authorized():
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 1024:
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
                video = self.server.browser.query(data["action"], value)
                result = {"youtube": self.server.covers.publish(video), "browser_ready": True, "ok": True}
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
        server.stats = PCStats().start()
        server.covers = CoverArt()
        server.weather = Weather(HERE / "weather-location.txt").start()
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
