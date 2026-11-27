"""Album covers of the song playing in the browser, fetched for the Kindle."""

import hashlib
import logging
import threading
import urllib.request
from urllib.parse import urlsplit

LOG = logging.getLogger("kindle-dashboard")

# YouTube and YouTube Music serve covers and thumbnails from these hosts only.
ALLOWED_HOSTS = (".googleusercontent.com", ".ytimg.com", ".ggpht.com")
IMAGE_TYPES = {"image/jpeg", "image/png", "image/webp"}
MAX_BYTES = 1_500_000


def allowed(url):
    try:
        parts = urlsplit(url)
    except ValueError:
        return False
    host = (parts.hostname or "").lower()
    return parts.scheme == "https" and any(host.endswith(suffix) or host == suffix[1:] for suffix in ALLOWED_HOSTS)


class CoverArt:
    """Downloads the current cover in the background; the Kindle fetches it by ID."""

    def __init__(self):
        self.lock = threading.Lock()
        self.url = None
        self.id = None
        self.image = None  # (bytes, content type) of the cover with self.id
        self.loading = None

    def publish(self, video):
        """Swap the browser's cover URL for the ID of a downloaded cover, or "" while it loads."""
        if not isinstance(video, dict):
            return video
        video = dict(video)
        url = video.pop("art", None)
        video["art"] = ""
        if not isinstance(url, str) or not allowed(url):
            return video
        with self.lock:
            if url == self.url and self.image:
                video["art"] = self.id
                return video
            if self.loading != url:
                self.loading = url
                threading.Thread(target=self.download, args=(url,), daemon=True).start()
        return video

    def download(self, url):
        try:
            request = urllib.request.Request(url, headers={
                "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ControlDeck",
                "Accept": "image/jpeg,image/png;q=0.9,image/webp;q=0.8",
            })
            with urllib.request.urlopen(request, timeout=8) as reply:
                kind = reply.headers.get_content_type()
                if kind not in IMAGE_TYPES:
                    raise ValueError(f"unexpected cover type {kind}")
                data = reply.read(MAX_BYTES + 1)
                if len(data) > MAX_BYTES:
                    raise ValueError("cover is too large")
        except Exception as exc:  # network or format problem: show the placeholder
            LOG.info("Could not fetch the album cover: %s", exc)
            with self.lock:
                if self.loading == url:
                    self.loading = None
            return
        with self.lock:
            if self.loading != url:
                return  # the song changed while this one downloaded
            self.url, self.id, self.image = url, hashlib.sha1(url.encode("utf-8")).hexdigest()[:16], (data, kind)
            self.loading = None

    def get(self, cover_id):
        with self.lock:
            return self.image if self.image and cover_id == self.id else None
