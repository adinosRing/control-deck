"""Live PC performance sampler for the Kindle dashboard.

CPU, RAM, battery and network come from psutil. GPU and VRAM come from
nvidia-smi when an NVIDIA GPU is present; otherwise they are reported as null
and the Kindle shows "--".
"""
import logging
import shutil
import subprocess
import sys
import threading
import time
from collections import deque

import psutil

LOG = logging.getLogger("kindle-dashboard")

SAMPLE_SECONDS = 2.5
HISTORY_POINTS = 24  # about one minute of history in each sparkline
GB = 1024 ** 3


def _percent(value):
    return None if value is None else max(0, min(100, round(value)))


class PCStats:
    def __init__(self):
        self._lock = threading.Lock()
        self._snapshot = None
        self._history = {name: deque(maxlen=HISTORY_POINTS) for name in ("cpu", "gpu", "ram", "vram")}
        self._nvidia_smi = shutil.which("nvidia-smi")
        self._last_net = None

    def start(self):
        psutil.cpu_percent(interval=None)  # The first reading is always 0, so prime it.
        thread = threading.Thread(target=self._run, name="pc-stats", daemon=True)
        thread.start()
        return self

    def snapshot(self):
        with self._lock:
            return self._snapshot

    def _run(self):
        while True:
            try:
                sample = self._sample()
                with self._lock:
                    for name in self._history:
                        if sample[name] is not None:
                            self._history[name].append(sample[name])
                    sample["history"] = {name: list(points) for name, points in self._history.items()}
                    self._snapshot = sample
            except Exception:
                LOG.exception("Could not read PC stats")
            time.sleep(SAMPLE_SECONDS)

    def _sample(self):
        memory = psutil.virtual_memory()
        sample = {
            "cpu": _percent(psutil.cpu_percent(interval=None)),
            "ram": _percent(memory.percent),
            "ram_used": round((memory.total - memory.available) / GB, 1),
            "ram_total": round(memory.total / GB, 1),
            "gpu": None,
            "vram": None,
            "vram_used": None,
            "vram_total": None,
        }
        sample.update(self._battery())
        sample.update(self._network())
        sample.update(self._gpu())
        return sample

    @staticmethod
    def _battery():
        try:
            battery = psutil.sensors_battery()
        except Exception:
            battery = None
        if battery is None:
            # A desktop PC: no battery, always on mains power.
            return {"battery": None, "plugged": True}
        return {"battery": _percent(battery.percent), "plugged": bool(battery.power_plugged)}

    def _network(self):
        counters = psutil.net_io_counters()
        now = time.monotonic()
        result = {"down": None, "up": None}
        if self._last_net:
            then, received, sent = self._last_net
            elapsed = now - then
            if elapsed > 0:
                result = {
                    "down": round(max(0, counters.bytes_recv - received) / elapsed, 1),
                    "up": round(max(0, counters.bytes_sent - sent) / elapsed, 1),
                }
        self._last_net = (now, counters.bytes_recv, counters.bytes_sent)
        return result

    def _gpu(self):
        if not self._nvidia_smi:
            return {}
        flags = subprocess.CREATE_NO_WINDOW if sys.platform == "win32" else 0
        try:
            output = subprocess.run(
                [self._nvidia_smi, "--query-gpu=utilization.gpu,memory.used,memory.total",
                 "--format=csv,noheader,nounits"],
                capture_output=True, text=True, timeout=3, creationflags=flags, check=True,
            ).stdout
            usage, used, total = (float(part) for part in output.splitlines()[0].split(","))
        except (OSError, subprocess.SubprocessError, ValueError, IndexError):
            return {}
        return {
            "gpu": _percent(usage),
            "vram": _percent(used * 100 / total) if total else None,
            "vram_used": round(used / 1024, 1),
            "vram_total": round(total / 1024, 1),
        }
