# Kindle Paperwhite 3 and dashboard over a USB cable

These notes cover running Control Deck on a **Kindle Paperwhite 3 (7th gen, 2015)** and connecting it to the PC **over the USB cable** instead of Wi-Fi. They come from getting it working on a real PW3 with Windows 11.

## 1. Know your firmware family

The Paperwhite 3's last firmware is **5.16.2.1.1**. Kindles on 5.16.3 or later use a hard-float ("khf") system, and older ones use soft-float. That decides which hacks fit:

| Package | Works on PW3? |
| --- | --- |
| USBNetLite (`*_install_khf*.bin`) | **No.** MRPI skips it ("not targeting your device") or it fails with `xzdec: not found`. |
| NiLuJe's USBNetwork 0.22.N (`Update_usbnet_0.22.N_install_pw2_and_up.bin`) | **Yes.** |

The Control Deck app itself is a static Go binary, so it runs on both families.

## 2. Install USBNetwork on the Kindle

1. Download `kindle-usbnet-0.22.N-*.tar.xz` from NiLuJe's *Snapshots* thread on MobileRead.
2. Extract it and copy **only** `Update_usbnet_0.22.N_install_pw2_and_up.bin` into the Kindle's `mrpackages` folder.
3. Eject the Kindle, then run **KUAL → Helper → Install MR Packages**. The log at `extensions/MRInstaller/log/mrinstaller.log` should end with `Success! :)`.
4. The default config (`usbnet/etc/config`) has `USE_WIFI="false"`, so SSH is reachable only over the cable. The Kindle's cable address is `192.168.15.244`.
5. Turn it on with **KUAL → USBNetwork → Toggle USBNetwork**, then plug in the cable. The Kindle should no longer go into USB Drive Mode.

## 3. Make Windows see a network adapter

Windows 11 usually installs the Kindle as **USB Serial Device (COMx)**. Its hardware ID is `USB\VID_0525&PID_A4A2`.

1. Get the Microsoft-signed driver: search the [Microsoft Update Catalog](https://www.catalog.update.microsoft.com/Search.aspx?q=USB%20Ethernet%2FRNDIS%20Gadget) for **"USB Ethernet/RNDIS Gadget"** and download the *Acer Incorporated, Windows 7, 8, 8.1 and later* entry (21 KB). Extract the `.cab`.
2. In Device Manager, right-click the COM device and choose **Update driver → Browse → Let me pick → Have Disk**. Pick the extracted `.inf`, then choose **USB Ethernet/RNDIS Gadget**.
   - An unsigned, hand-written `.inf` will **not** work, because Windows 11 refuses it ("The third-party INF does not contain digital signature information").
3. Give the new adapter a fixed address: Settings → Network & internet → Ethernet → IP assignment → Manual → IPv4, IP `192.168.15.201`, mask `255.255.255.0`, no gateway.
4. Check the link with `ping 192.168.15.244`.

## 4. Firewall

Windows treats the cable link as an unidentified (Public) network. Rather than allowing Python on all Public networks, allow only the dashboard port from the cable subnet. Run this in an **admin** PowerShell:

```powershell
# Turn off "block Python" rules Windows may have created from a dismissed popup
Get-NetFirewallRule -Action Block -Enabled True -ErrorAction SilentlyContinue |
  Where-Object { ($_ | Get-NetFirewallApplicationFilter).Program -like "*python*" } |
  Disable-NetFirewallRule
New-NetFirewallRule -DisplayName "Control Deck - Kindle USB" -Direction Inbound -Protocol TCP `
  -LocalPort 8765 -RemoteAddress 192.168.15.0/24 -Action Allow -Profile Any
```

## 5. Pair for the cable

Run `Setup.bat` with the cable connected. It detects `192.168.15.201` and pairs both the cable and Wi-Fi addresses. If your copy of Setup misses it, pair by hand from the project folder:

```
python companion\pair.py --pc-address 192.168.15.201 --pc-address <your Wi-Fi IP>
```

The companion only reads its allowed networks **when it starts**, so restart it after pairing. With USBNetwork on, the Kindle isn't a drive, so copy the pairing files over SSH instead:

```
scp dist\KindleDashboardNative\pc-bridge.json dist\KindleDashboardNative\pc-bridge.pem root@192.168.15.244:/mnt/us/KindleDashboardNative/
```

At the password prompt, press Enter: over the cable, USBNetwork accepts a blank password.

## 6. Troubleshooting checklist

Work through it layer by layer:

1. `ping 192.168.15.244` → the cable link is up.
2. `netstat -ano | findstr :8765` → **the PC companion is actually running and listening.** If this prints nothing, start it with `Start Control Deck.bat` (or `python companion\bridge.py`) and keep the window open.
3. `ssh root@192.168.15.244 "curl -vk --max-time 5 https://192.168.15.201:8765/status"` → the Kindle can reach the companion (refused = not listening, timeout = firewall).
4. `ssh root@192.168.15.244 /mnt/us/KindleDashboardNative/personal-dashboard -bridge-check /mnt/us/KindleDashboardNative` → the pairing matches (`PC connected, …`).

To copy files to the Kindle again, turn USBNetwork off in KUAL first so it goes back to USB Drive Mode.
