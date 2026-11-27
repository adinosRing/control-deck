"""Render the dashboard's 600 × 800 monochrome-themed artwork for Kindle at 2× (1200 × 1600).

The Kindle app scales it down to the screen, so it stays sharp on high-resolution
Kindles such as the Paperwhite 3.
"""

import argparse
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--browser", help="Optional path to an installed Chromium browser")
    parser.add_argument("--web", type=Path, default=ROOT / "design")
    parser.add_argument("--output", type=Path, default=ROOT / "kindle" / "native" / "assets")
    parser.add_argument("--scale", type=int, default=2, help="Pixel density; the app expects 2")
    args = parser.parse_args()

    args.output.mkdir(parents=True, exist_ok=True)
    handler = partial(SimpleHTTPRequestHandler, directory=str(args.web.resolve()))
    with ThreadingHTTPServer(("127.0.0.1", 0), handler) as server:
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with sync_playwright() as playwright:
                browser = playwright.chromium.launch(
                    executable_path=args.browser,
                    args=["--disable-lcd-text", "--disable-background-networking"],
                )
                page = browser.new_page(viewport={"width": 600, "height": 800}, device_scale_factor=args.scale, timezone_id="UTC")
                page.clock.set_fixed_time("2026-01-26T10:24:00Z")
                page.goto(f"http://127.0.0.1:{server.server_port}/")
                page.locator("main.dashboard").wait_for()
                page.evaluate("document.fonts.ready")
                for theme in ("dark", "light"):
                    if page.locator("html").get_attribute("data-theme") != theme:
                        page.locator("#theme").click()
                    page.locator(f'html[data-theme="{theme}"]').wait_for(state="attached")
                    page.screenshot(path=args.output / f"{theme}.png", animations="disabled")
                browser.close()
        finally:
            server.shutdown()
            thread.join()


if __name__ == "__main__":
    main()
