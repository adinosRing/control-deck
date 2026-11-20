# Control Deck

I turned a 10th-generation Kindle into a desk controller for my Windows PC. The Kindle draws the dashboard directly on its e-ink screen. Quick Launch opens apps or websites on the PC, and the browser extension sends playback commands to YouTube.

![Control Deck in dark mode](kindle/native/assets/dark.png)

## Build guides

| Device | Starting point |
| --- | --- |
| [Old phone or tablet](docs/phone-tablet.md) | Run a macro deck in the browser of a spare Android or iOS device. |
| [ESP32 touchscreen](docs/esp32-touchscreen.md) | Flash a supported touchscreen board or build your own interface with LVGL. |
| [Kindle](docs/kindle.md) | Build and install this e-ink dashboard, Windows companion, and YouTube extension. |

## Source

- [Kindle app and scriptlet](kindle/) draw the interface, handle taps, and call the PC on the local network.
- [Windows companion](companion/) opens the Quick Launch targets. Its [browser extension](companion/youtube-extension/) reads and controls the selected YouTube player.
- [Dashboard design](design/) contains the HTML, CSS, and JavaScript used to generate the Kindle artwork. Open `index.html` in a browser for a standalone preview. The preview uses sample data.
- [Earlier prototypes](legacy/) include the browser-based Kindle version, standalone native build, and Spotify companion.

The Codex limits and PC performance panels show sample numbers. The shipped Mia panel uses a CSS robot illustration. To customize it, replace the portrait placeholder in the design with your own artwork and regenerate the backdrops. [Backdrop instructions](docs/kindle.md#customize-the-artwork)
