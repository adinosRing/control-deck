# Build Control Deck on a Kindle

This is the build used for a **10th-generation Kindle Basic (KT4), firmware 5.16.9**. The app lays out a 600 × 800 interface and draws it natively at the screen's real resolution (for example 1072 × 1448 on a Paperwhite 3) using FBInk and reads touchscreen input from Linux. The Kindle and Windows companion communicate over the local network. YouTube runs in the computer's browser.

## What you need

- An already-jailbroken Kindle with scriptlet support and the FBInk binary for its model. Use the KindleModding documentation for the exact model and firmware. Other models may need changes to resolution, input, or build settings. [Kindle scriptlets](https://kindlemodding.org/kindle-dev/scriptlets.html) · [FBInk](https://github.com/NiLuJe/FBInk)
- A USB data cable and a Windows PC on the same Wi-Fi network as the Kindle, or a PC hotspot.
- Go 1.27 or newer, Python 3.10 or newer, OpenSSL, and a Chromium-family browser that supports Manifest V3 extension service workers with WebSockets. The project was used with Brave. [Chromium WebSocket support](https://developer.chrome.com/docs/extensions/how-to/web-platform/websockets)

## 1. Build and pair

From the repository root in PowerShell, run the tests, then cross-compile the Kindle binary.

```powershell
New-Item -ItemType Directory -Force dist/KindleDashboardNative | Out-Null
Push-Location kindle/native
go test ./...
$env:GOOS = 'linux'
$env:GOARCH = 'arm'
$env:GOARM = '7'
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags '-s -w' -o ../../dist/KindleDashboardNative/personal-dashboard .
Pop-Location
```

Find the PC's IPv4 address on the network the Kindle will join, for example with `ipconfig`. Substitute it in the next command. If OpenSSL is installed but missing from PATH, pass its executable path with `--openssl`. You can pass `--pc-address` more than once for a PC that has multiple LAN addresses. By default the companion accepts the corresponding /24 subnets. Set `--allow-network` if your subnet differs.

```powershell
python -m pip install -r companion/requirements.txt
python companion/pair.py --pc-address 192.168.1.10
```

The pairing command generates a browser token, a separate Kindle token, and a TLS certificate for this PC. Keep the generated files local. Transfer only the two pairing files in `dist/KindleDashboardNative` to the Kindle alongside its compiled binary.

## 2. Start the PC and browser components

1. Run `python companion/bridge.py` from the repository root and leave it running. The Kindle uses HTTPS on TCP port **8765**, and the browser extension connects through Windows loopback on **8766**. Allow the Kindle's local subnet through the firewall if required. Keep both ports off public internet.
2. In Brave, open `brave://extensions`. In Chrome, open `chrome://extensions`. Enable **Developer mode**, select **Load unpacked**, and choose the `companion/youtube-extension` folder after pairing has generated `pairing.js`. [Chromium extension loading instructions](https://developer.chrome.com/docs/extensions/get-started/tutorial/hello-world#load_an_unpacked_extension)
3. The companion also samples the PC every 2.5 seconds for the **PC Performance Monitor**: CPU, RAM, battery and network speed through `psutil`, and GPU and VRAM through `nvidia-smi` when an NVIDIA GPU is installed. Values the PC cannot report show `--`. A desktop without a battery shows **AC POWER**. The Kindle polls every 5 seconds.
4. The **TO-DO** panel reads `companion/todo.txt`, which the companion creates on first run. Write one task per line. A line starting with `x ` is done, and lines starting with `#` are ignored. Tapping a task on the Kindle adds or removes the `x ` and saves the file. Open tasks are listed first, up to eight. To keep the list somewhere else, start the companion with `python companion/bridge.py --todo "C:\Users\you\Documents\todo.txt"`.
5. The **TODAY** panel shows the weather. The companion fetches it from [Open-Meteo](https://open-meteo.com/) every 15 minutes (free, no API key). On first run it creates `companion/weather-location.txt`: write a city name (`Bengaluru`) or coordinates (`12.97, 77.59`) on its own line, and add a line `fahrenheit` for °F. If you leave it empty, the location is detected from the PC's internet connection via ipwho.is.
6. Open or reload a YouTube video in that browser profile and press Play once. The extension chooses an active YouTube player, preferring a playing tab. Captions and next-video controls depend on the video or playlist.

## 3. Install and test on the Kindle

1. Connect the Kindle by USB. Copy the entire `dist/KindleDashboardNative` folder to the root of Kindle storage. Copy [Personal Dashboard Offline.sh](../kindle/documents/Personal%20Dashboard%20Offline.sh) into `documents`.
2. Eject the drive and unplug the cable. Join the Kindle to the same local network as the PC, then open **Personal Dashboard Offline** from the Kindle Library.
3. Tap **YouTube** in Quick Launch. Open a video on the PC and confirm the title appears on the Kindle. Test play/pause and volume, then try captions or loop. **Exit** closes the Kindle app.

The launcher expects this layout.

```text
Kindle USB root/
├── KindleDashboardNative/
│   ├── personal-dashboard
│   ├── pc-bridge.json
│   └── pc-bridge.pem
└── documents/
    └── Personal Dashboard Offline.sh
```

## Troubleshoot

- **App absent from the Kindle Library.** Confirm the file ends in `.sh`, is inside `documents`, and the installed Kindle hotfix includes scriptlet support. [Scriptlet setup](https://kindlemodding.org/kindle-dev/scriptlets.html)
- **PC Offline.** Check the companion output, local network, firewall, and the address used in pairing. If the IP changed, rerun the pairing command and recopy the generated files. Check the Kindle's date and time if a TLS connection fails.
- **Browser Setup.** Load or reload the unpacked extension in the same profile as the YouTube tab. Reload the video, then retry **Play**.
- **Performance panel shows `--`.** Make sure `pip install -r companion/requirements.txt` installed `psutil`, and that the Kindle is not showing **PC Offline**. GPU and VRAM need `nvidia-smi` on the PATH, so they stay `--` on AMD or Intel graphics.
- **Preview the panel without a Kindle.** Run `go run . -preview preview.png -preview-stats stats.json` from `kindle/native`, where `stats.json` holds a sample reply from the companion.
- **Blank screen or touch issues.** Reconnect the Kindle over USB and read `KindleDashboardNative/dashboard.log`. Confirm the firmware has FBInk and the screen/touch hardware fits the KT4 profile.

## Customize the artwork

Edit [the design](../design/) and render both backgrounds before rebuilding. The HTML preview paints a static layout, so changes to button positions also need corresponding updates to the native touch regions in [main.go](../kindle/native/main.go).

```powershell
python -m pip install playwright
python -m playwright install chromium
python tools/render_backdrops.py
```

The backgrounds are rendered at 2× (1200 × 1600), and the app scales them down to the screen it runs on. To check a layout without a Kindle, render a preview at any screen size from `kindle/native`:

```powershell
go run . -preview preview.png -screen 1072x1448 -preview-stats stats.json -preview-todo todo.json
```

## Open-source repositories

| Repository | Use it for |
| --- | --- |
| [FBInk](https://github.com/NiLuJe/FBInk) | Drawing text and images directly on an e-ink framebuffer. |
| [KindleModding sh_integration](https://github.com/KindleModding/sh_integration) | Making a shell script appear as a launchable item in the Kindle Library. |
| [KindleModding Universal Hotfix](https://github.com/KindleModding/Hotfix) | Model-specific post-jailbreak tools, including scriptlet support. |
| [KOReader](https://github.com/koreader/koreader) | Reference code for e-reader touch input and display handling. |

[Back to the build guides](../README.md#build-guides)
