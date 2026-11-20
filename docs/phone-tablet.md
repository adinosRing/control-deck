# Build a control deck with an old phone or tablet

A spare Android phone, iPhone, or tablet can run a touch deck through its browser. The phone displays the buttons and sends taps to a host on your computer. Start with the browser you already have and test compatibility before setting up a stand or permanent charger.

## What you need

- A phone or tablet with a working touchscreen and a browser that can load the deck client.
- A PC or Mac that can run Macro Deck's host.
- A local Wi-Fi network both devices can reach, plus the phone's charger and a stand.

## Set up Macro Deck

1. Download and install the host from the upstream releases page for your computer. Start it and open its pairing or web client settings. If a current build is marked beta or pre-release, check the upstream notes for its platform and browser requirements. [Macro Deck releases](https://github.com/Macro-Deck-App/Macro-Deck/releases)
2. Join the same network on both devices. Open the web client address shown by the host in Safari or Chrome on the phone. If the host shows a QR code, scan it. Approve the device in the host if prompted.
3. Create a profile with six large buttons. Start with **Browser**, **Editor**, **Play/Pause**, **Mute**, **Next**, and **Previous**. In the host editor, assign each tile a built-in action or a plugin action supported by your OS.
4. Tap **Browser** on the phone and confirm the browser opens on the computer. Test the media keys with the intended player active. Once those work, add pages for other tasks, such as OBS scenes or Home Assistant controls.
5. For desk use, lock the phone's preferred orientation and set the screen timeout you want. Add a home-screen shortcut if the browser offers it. Installing the Macro Deck web client as a PWA requires HTTPS. [Macro Deck web client documentation](https://github.com/Macro-Deck-App/Macro-Deck#web-based-architecture)

## Troubleshoot

- If the phone cannot load the page, confirm you used the host's LAN address. A URL starting with `localhost` points back to the phone. Check that the devices are on the same network and that the host is allowed through the computer's firewall on that network.
- If a button lights up but the PC does nothing, check its assigned action and the target app. An OS media key may control whichever player the desktop considers active.
- If the old browser cannot render the web client, try its latest available system browser or a compatible native client if the upstream project offers one. Test a single button before transferring your full layout.

## Open-source repositories

| Repository | Use it for |
| --- | --- |
| [Macro Deck](https://github.com/Macro-Deck-App/Macro-Deck) | Browser-based macro pad, profiles, desktop actions, and plugins. |
| [Home Assistant Core](https://github.com/home-assistant/core) and [Frontend](https://github.com/home-assistant/frontend) | A touch dashboard for lights, scenes, and other Home Assistant devices. |

[Back to the build guides](../README.md#build-guides)
