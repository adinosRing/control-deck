"""Current weather for the Kindle dashboard, from Open-Meteo (free, no API key).

The location comes from companion/weather-location.txt: a city name such as
"Bengaluru", or coordinates such as "12.97, 77.59". If the file has no location,
it is detected once from this PC's internet connection (ipwho.is). Add a line
with the word "fahrenheit" to show °F.
"""
import json
import logging
import re
import threading
import time
import urllib.parse
import urllib.request
from pathlib import Path

LOG = logging.getLogger("kindle-dashboard")

REFRESH_SECONDS = 15 * 60
RETRY_SECONDS = 60
TEMPLATE = """# Weather location for the Kindle dashboard.
# Write a city name (for example: Bengaluru) or coordinates (for example: 12.97, 77.59)
# on its own line below. Leave it empty to detect the location from your internet connection.
# Add a line with the word fahrenheit to show temperatures in °F.
"""
COORDINATES = re.compile(r"^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$")


def fetch_json(url, timeout=10):
    request = urllib.request.Request(url, headers={"User-Agent": "KindleControlDeck/1.0"})
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


class Weather:
    def __init__(self, config_path, fetch=fetch_json):
        self.path = Path(config_path)
        self.fetch = fetch
        self._lock = threading.Lock()
        self._snapshot = None
        self._place = None  # (name, latitude, longitude)
        self._place_source = None
        if not self.path.exists():
            self.path.write_text(TEMPLATE, encoding="utf-8")

    def start(self):
        threading.Thread(target=self._run, name="weather", daemon=True).start()
        return self

    def snapshot(self):
        with self._lock:
            return self._snapshot

    def _settings(self):
        location, fahrenheit = "", False
        try:
            lines = self.path.read_text(encoding="utf-8-sig").splitlines()
        except OSError:
            lines = []
        for line in lines:
            text = line.strip()
            if not text or text.startswith("#"):
                continue
            if text.lower() in ("fahrenheit", "f", "°f"):
                fahrenheit = True
            elif not location:
                location = text
        return location, fahrenheit

    def _locate(self, location):
        if self._place and self._place_source == location:
            return self._place
        match = COORDINATES.match(location)
        if match:
            place = ("", float(match.group(1)), float(match.group(2)))
        elif location:
            query = urllib.parse.urlencode({"name": location, "count": 1, "language": "en", "format": "json"})
            results = self.fetch("https://geocoding-api.open-meteo.com/v1/search?" + query).get("results") or []
            if not results:
                raise LookupError(f"Could not find a place called {location!r}.")
            place = (results[0].get("name", location), float(results[0]["latitude"]), float(results[0]["longitude"]))
        else:
            found = self.fetch("https://ipwho.is/")
            if not found.get("success", True) or "latitude" not in found:
                raise LookupError("Could not detect the location; set one in weather-location.txt.")
            place = (found.get("city", ""), float(found["latitude"]), float(found["longitude"]))
        self._place, self._place_source = place, location
        return place

    def update(self):
        """Fetch the weather once. Returns the new snapshot."""
        location, fahrenheit = self._settings()
        try:
            name, latitude, longitude = self._locate(location)
            query = urllib.parse.urlencode({
                "latitude": latitude,
                "longitude": longitude,
                "current": "temperature_2m,weather_code,is_day",
                "daily": "temperature_2m_max,temperature_2m_min",
                "timezone": "auto",
                "forecast_days": 1,
                "temperature_unit": "fahrenheit" if fahrenheit else "celsius",
            })
            data = self.fetch("https://api.open-meteo.com/v1/forecast?" + query)
            current, daily = data["current"], data.get("daily", {})
            snapshot = {
                "temp": current.get("temperature_2m"),
                "code": current.get("weather_code"),
                "is_day": bool(current.get("is_day", 1)),
                "high": (daily.get("temperature_2m_max") or [None])[0],
                "low": (daily.get("temperature_2m_min") or [None])[0],
                "place": name,
            }
        except Exception as exc:  # network down, bad place name, unexpected reply
            LOG.warning("Weather unavailable: %s", exc)
            with self._lock:
                # Keep showing the last good reading if there is one.
                if self._snapshot is None or "error" in self._snapshot:
                    self._snapshot = {"error": str(exc)[:120]}
                return self._snapshot
        with self._lock:
            self._snapshot = snapshot
        return snapshot

    def _run(self):
        while True:
            snapshot = self.update()
            time.sleep(RETRY_SECONDS if "error" in snapshot else REFRESH_SECONDS)
