"""Windows companion for the Kindle Personal Dashboard."""
import argparse
import asyncio
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
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / "vendor"))
sys.coinit_flags = 0

import winrt.windows.foundation
import winrt.windows.foundation.collections
import winrt.windows.storage.streams
from winrt.runtime import ApartmentType, init_apartment, uninit_apartment
from winrt.windows.media import MediaPlaybackAutoRepeatMode
from winrt.windows.media.control import GlobalSystemMediaTransportControlsSessionManager as MediaManager
from winrt.windows.media.control import GlobalSystemMediaTransportControlsSessionPlaybackStatus as PlaybackStatus
from pycaw.pycaw import AudioUtilities
from psutil import Error as ProcessError

LOG = logging.getLogger("kindle-dashboard")
MEDIA_LOCK = threading.Lock()


class ActionUnavailable(Exception):
    pass


def launch(name):
    appdata = Path(os.environ["APPDATA"])
    spotify = appdata / "Spotify" / "Spotify.exe"
    code = shutil.which("code.cmd")
    code_app = Path(code).parent.parent / "Code.exe" if code else None
    links = {
        "ChatGPT": "https://chatgpt.com/",
        "Claude": "https://claude.ai/",
        "VS Code": str(code_app) if code_app and code_app.is_file() else code,
        "Browser": "https://www.google.com/",
        "Spotify": str(spotify) if spotify.is_file() else "spotify:",
        "Terminal": shutil.which("wt.exe") or shutil.which("powershell.exe"),
    }
    target = links.get(name)
    if not target:
        raise ActionUnavailable(f"{name} is unavailable on the PC.")
    try:
        os.startfile(target)
    except OSError as exc:
        raise ActionUnavailable(f"Windows could not open {name}.") from exc


def spotify_volume(action=None, value=None):
    levels = []
    for audio in AudioUtilities.GetAllSessions():
        try:
            process = audio.Process
            if process is not None and process.name().lower() == "spotify.exe":
                levels.append(audio.SimpleAudioVolume)
        except ProcessError:
            continue
    if not levels:
        if action:
            raise ActionUnavailable("Play a track in Spotify to enable its volume control.")
        return None, False
    if action == "volume":
        for level in levels:
            level.SetMasterVolume(value / 100, None)
            level.SetMute(0, None)
    elif action == "mute":
        muted = not levels[0].GetMute()
        for level in levels:
            level.SetMute(int(muted), None)
    return round(levels[0].GetMasterVolume() * 100), bool(levels[0].GetMute())


async def spotify_state(action=None, value=None):
    manager = await MediaManager.request_async()
    session = next(
        (item for item in manager.get_sessions() if "spotify" in item.source_app_user_model_id.lower()),
        None,
    )
    if session is None:
        if action:
            raise ActionUnavailable("Open Spotify on the PC and start a track first.")
        return None

    info = session.get_playback_info()
    if action in {"volume", "mute"}:
        spotify_volume(action, value)
    elif action:
        if action == "previous":
            result = await session.try_skip_previous_async()
        elif action == "next":
            result = await session.try_skip_next_async()
        elif action == "toggle":
            result = await session.try_toggle_play_pause_async()
        elif action == "seek":
            timeline = session.get_timeline_properties()
            target = timeline.start_time + (timeline.end_time - timeline.start_time) * (value / 100)
            result = await session.try_change_playback_position_async(round(target.total_seconds() * 10_000_000))
        elif action == "shuffle":
            result = await session.try_change_shuffle_active_async(not bool(info.is_shuffle_active))
        elif action == "repeat":
            mode = MediaPlaybackAutoRepeatMode.NONE if info.auto_repeat_mode not in (None, MediaPlaybackAutoRepeatMode.NONE) else MediaPlaybackAutoRepeatMode.LIST
            result = await session.try_change_auto_repeat_mode_async(mode)
        else:
            raise ValueError("Unknown media action.")
        if not result:
            raise ActionUnavailable(f"Spotify did not accept {action} through Windows media controls.")
        await asyncio.sleep(.20)

    properties = await session.try_get_media_properties_async()
    info = session.get_playback_info()
    timeline = session.get_timeline_properties()
    playing = info.playback_status == PlaybackStatus.PLAYING
    duration = max(0, round((timeline.end_time - timeline.start_time).total_seconds()))
    position = max(0, (timeline.position - timeline.start_time).total_seconds())
    if playing and duration:
        try:
            age = time.time() - timeline.last_updated_time.timestamp()
            if 0 < age < duration:
                position += age
        except (OverflowError, OSError, ValueError):
            pass
    volume, muted = spotify_volume()
    return {
        "title": properties.title[:160],
        "artist": properties.artist[:160],
        "playing": playing,
        "duration": duration,
        "position": min(duration, round(position)) if duration else 0,
        "volume": volume,
        "muted": muted,
        "shuffle": bool(info.is_shuffle_active),
        "repeat": info.auto_repeat_mode not in (None, MediaPlaybackAutoRepeatMode.NONE),
    }


def media(action=None, value=None):
    with MEDIA_LOCK:
        init_apartment(ApartmentType.MULTI_THREADED)
        try:
            return asyncio.run(asyncio.wait_for(spotify_state(action, value), timeout=5))
        finally:
            uninit_apartment()


def status():
    try:
        current = media()
    except Exception:
        LOG.exception("Spotify status lookup failed")
        current = None
    return {"spotify": current}


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
                self.send_json(200, status())
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
            elif self.path == "/spotify" and data.get("action") in {
                "previous", "next", "toggle", "seek", "shuffle", "repeat", "volume", "mute"
            }:
                value = data.get("value")
                if data["action"] in {"seek", "volume"} and (type(value) is not int or not 0 <= value <= 100):
                    raise ValueError("The control value must be between 0 and 100.")
                result = {"spotify": media(data["action"], value), "ok": True}
            else:
                raise ValueError("Unknown dashboard command.")
            self.send_json(200, result)
        except ActionUnavailable as exc:
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
    parser.add_argument("--status", action="store_true")
    args = parser.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(message)s")
    if args.status:
        current = status()
        print(json.dumps({"spotify_session": bool(current["spotify"])}))
        return
    config = json.loads(args.config.read_text(encoding="utf-8"))
    tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    tls.minimum_version = ssl.TLSVersion.TLSv1_2
    tls.load_cert_chain(args.config.parent / "bridge-cert.pem", args.config.parent / "bridge-key.pem")
    with BridgeServer(("0.0.0.0", config["port"]), BridgeHandler) as server:
        server.tls = tls
        server.authorization = ("Bearer " + config["token"]).encode("ascii")
        server.networks = tuple(ipaddress.ip_network(item) for item in config["networks"])
        server.clients = set()
        server.clients_lock = threading.Lock()
        LOG.info("Kindle Dashboard Companion is ready on TCP port %s.", config["port"])
        server.serve_forever()


if __name__ == "__main__":
    main()
