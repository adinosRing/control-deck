# Build a control deck with an ESP32 touchscreen

An integrated ESP32 touchscreen gives you a dedicated controller in a small enclosure. For a first build, choose an exact board revision with a released firmware image. Several panels share similar product names, and a firmware image for the wrong display or touch controller can leave the screen blank or touch misaligned.

## What you need

- A supported integrated touchscreen board, such as a Waveshare **ESP32-S3-Touch-LCD-4** revision listed by Stream32.
- A USB cable that carries data and a computer for flashing and assigning actions.
- The exact product name and revision from the board or its packaging. Check the firmware release for support before buying. [Stream32 board list](https://github.com/FadyFaheem/Stream32#supported-hardware)

## Build a USB deck with Stream32

1. Download the Stream32 desktop app for your computer and check the upstream compatibility notes for your board. [Stream32 releases](https://github.com/FadyFaheem/Stream32/releases)
2. Connect the board to the computer with the data cable. In the desktop app, choose the profile for your **exact** board, then flash the matching firmware. Use the bootloader steps in the board instructions if the flasher cannot see the device. [Stream32 setup and firmware instructions](https://github.com/FadyFaheem/Stream32#getting-started)
3. Keep the board connected and make a page with three buttons. Assign one to open a website, one to send a keyboard shortcut, and one to launch an app. Sync the layout to the device.
4. Tap every key and confirm the expected action runs on the computer. If touches trigger the adjacent button, check the board profile, screen rotation, and any calibration procedure from the firmware project.
5. Add more pages and test after disconnecting and reconnecting the deck. Fit the case only after you have verified touch input and found an accessible USB port.

## Choose a wireless or custom build

- **Bluetooth keyboard shortcuts.** FreeTouchDeck uses BLE for input. Start with a board supported in its README, use the matching firmware or build instructions, then pair it as a keyboard and test a simple shortcut in a blank document. The host still needs to interpret the shortcut you assign. [FreeTouchDeck](https://github.com/DustinWatts/FreeTouchDeck)
- **Custom interface.** Start with Espressif's display demo for your exact board. Confirm native resolution and touch orientation before adding an LVGL button grid. Pick a transport early. Use HID for key combinations, or a small paired companion for app launches and playback status. Keep network calls off the display task so taps remain responsive. [ESP-BSP examples](https://github.com/espressif/esp-bsp#examples) · [LVGL](https://github.com/lvgl/lvgl)

## Open-source repositories

| Repository | Use it for |
| --- | --- |
| [Stream32](https://github.com/FadyFaheem/Stream32) | Firmware, desktop configuration, and program shortcuts for supported touch boards. |
| [FreeTouchDeck](https://github.com/DustinWatts/FreeTouchDeck) | An ESP32 touchscreen macro pad that sends BLE keyboard input. |
| [ESP-IDF](https://github.com/espressif/esp-idf), [ESP-BSP](https://github.com/espressif/esp-bsp), and [LVGL](https://github.com/lvgl/lvgl) | The SDK, board drivers, and UI library for a custom firmware. |

[Back to the build guides](../README.md#build-guides)
