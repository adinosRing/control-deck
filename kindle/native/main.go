package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const width, height = 600, 800

//go:embed assets/dark.png assets/light.png assets/plex-mono.ttf assets/pixelify.ttf
var assets embed.FS

type theme struct {
	name                                  string
	paper, panel, ink, frame, line, track color.Gray
	shade, muted                          color.Gray
}

var dark = theme{"dark", color.Gray{25}, color.Gray{32}, color.Gray{237}, color.Gray{209}, color.Gray{140}, color.Gray{102}, color.Gray{69}, color.Gray{195}}
var light = theme{"light", color.Gray{242}, color.Gray{249}, color.Gray{24}, color.Gray{41}, color.Gray{117}, color.Gray{173}, color.Gray{216}, color.Gray{59}}

var tracks = []struct {
	title, artist string
	length        int
}{
	{"Midnight Drive", "Nexora", 236},
	{"Afterglow", "Signal Coast", 204},
	{"Static Bloom", "Vesper Lane", 261},
}
var shortcuts = []string{"ChatGPT", "Claude", "VS Code", "Browser", "YouTube", "Terminal"}

type dashboard struct {
	theme                               theme
	track, elapsed, volume, lastVolume  int
	playing, shuffle, repeat            bool
	dialogTitle, dialogText             string
	confirmExit                         bool
	updated                             time.Time
	connectedUI, pcOnline, browserReady bool
	youtube                             *youtubeVideo
	stats                               *pcStats
	todo                                *todoList
	weather                             *weatherInfo
	kindleBattery                       int // -1 when unknown
	kindleCharging                      bool
	pending                             *bridgeAction
}

func newDashboard(pref string, now time.Time) *dashboard {
	model := &dashboard{theme: dark, elapsed: 102, volume: 80, lastVolume: 80, playing: true, updated: now, kindleBattery: -1}
	if pref == light.name {
		model.theme = light
	}
	return model
}

func (d *dashboard) step(direction int) {
	if d.shuffle {
		direction = 1 + rand.IntN(len(tracks)-1)
	}
	d.track = (d.track + direction + len(tracks)) % len(tracks)
	d.elapsed = 0
}

func (d *dashboard) advance(now time.Time) {
	seconds := int(now.Sub(d.updated) / time.Second)
	d.updated = now
	if !d.playing || seconds <= 0 {
		return
	}
	// Bound catch-up after the Kindle has slept.
	d.elapsed += min(seconds, 600)
	for d.elapsed >= tracks[d.track].length {
		length := tracks[d.track].length
		if d.repeat {
			d.elapsed %= length
		} else {
			remaining := d.elapsed - length
			d.step(1)
			d.elapsed = remaining
		}
	}
}

func (d *dashboard) show(title, message string) {
	d.dialogTitle, d.dialogText = title, message
	d.confirmExit = false
}

func (d *dashboard) tap(point image.Point, now time.Time) (exit, flash bool) {
	if !d.connectedUI {
		d.advance(now)
	}
	x, y := point.X, point.Y
	if d.dialogTitle != "" {
		if point.In(image.Rect(345, 433, 513, 488)) {
			d.dialogTitle = ""
		} else if d.confirmExit && point.In(image.Rect(90, 433, 255, 488)) {
			return true, false
		}
		return false, false
	}
	if y >= 39 && y < 84 {
		switch {
		case x >= 403 && x < 446:
			if d.theme.name == light.name {
				d.theme = dark
			} else {
				d.theme = light
			}
			return false, true
		case x >= 446 && x < 490:
			if d.connectedUI {
				d.show("CONTROL DECK", "Quick Launch, YouTube, the PC monitor, weather and the to-do list come from the PC companion. Tap a task to tick it off in todo.txt.")
			} else {
				d.show("CONTROL DECK", "Offline dashboard for Kindle. Music, PC stats, tasks and shortcuts use sample data.")
			}
		case x >= 490:
			d.show("EXIT DASHBOARD?", "Return to the Kindle Library.")
			d.confirmExit = true
		}
	}
	if d.connectedUI {
		if row, ok := todoRowAt(point); ok {
			if d.todo != nil && d.pcOnline && row < len(d.todo.Items) {
				item := d.todo.Items[row]
				d.pending = &bridgeAction{Action: "todo-toggle", Value: item.Line, Text: item.Text}
			}
			return false, false
		}
		d.pending = connectedTap(point)
		return false, false
	}
	if y >= 205 && y < 240 && x >= 153 && x <= 365 {
		d.elapsed = min((x-154)*tracks[d.track].length/210, tracks[d.track].length-1)
		d.elapsed = max(0, d.elapsed)
	}
	if y >= 245 && y < 289 && x >= 23 && x < 372 {
		switch {
		case x < 89:
			d.shuffle = !d.shuffle
		case x < 155:
			d.step(-1)
		case x < 230:
			d.playing = !d.playing
		case x < 300:
			d.step(1)
		default:
			d.repeat = !d.repeat
		}
	}
	if y >= 289 && y < 321 {
		if x >= 29 && x < 74 {
			if d.volume > 0 {
				d.lastVolume, d.volume = d.volume, 0
			} else {
				d.volume = d.lastVolume
			}
		}
		if x >= 78 && x <= 314 {
			d.volume = (x - 78) * 100 / 236
			if d.volume > 0 {
				d.lastVolume = d.volume
			}
		}
	}
	if y >= 708 && y < 792 && x >= 15 && x < 584 {
		index := (x - 15) / 97
		name := shortcuts[index]
		d.show(strings.ToUpper(name), name+" shortcut selected. This action is simulated in the offline dashboard.")
	}
	return false, false
}

// The backdrops are rendered at twice the design size and scaled down to the
// screen, so the artwork stays sharp on high-resolution Kindles.
const backdropScale = 2

type artwork struct {
	scale                                  float64
	sources, backdrops                     map[string]image.Image
	title, label, body, small, tiny        font.Face
	day, weekday, month, count, date, week font.Face
	weekdaySmall                           font.Face
}

// openArtwork loads the fonts and artwork for a screen scale (1 = 600 × 800).
func openArtwork(scale float64) (*artwork, error) {
	if scale <= 0 {
		scale = 1
	}
	art := &artwork{scale: scale, sources: make(map[string]image.Image), backdrops: make(map[string]image.Image)}
	for _, name := range []string{dark.name, light.name} {
		data, err := assets.ReadFile("assets/" + name + ".png")
		if err != nil {
			return nil, err
		}
		source, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if source.Bounds() != image.Rect(0, 0, width*backdropScale, height*backdropScale) {
			return nil, fmt.Errorf("invalid %s backdrop dimensions %v", name, source.Bounds().Size())
		}
		art.sources[name] = source
	}
	for _, item := range []struct {
		file string
		size float64
		dest *font.Face
	}{
		{"pixelify.ttf", 25, &art.title}, {"pixelify.ttf", 16, &art.label},
		{"plex-mono.ttf", 16, &art.body}, {"plex-mono.ttf", 14, &art.small}, {"plex-mono.ttf", 12, &art.tiny},
		{"pixelify.ttf", 40, &art.day}, {"pixelify.ttf", 27, &art.weekday}, {"pixelify.ttf", 23, &art.weekdaySmall},
		{"pixelify.ttf", 15, &art.month}, {"pixelify.ttf", 17, &art.count},
		{"plex-mono.ttf", 14, &art.date}, {"plex-mono.ttf", 13, &art.week},
	} {
		data, err := assets.ReadFile("assets/" + item.file)
		if err != nil {
			return nil, err
		}
		parsed, err := opentype.Parse(data)
		if err != nil {
			return nil, err
		}
		*item.dest, err = opentype.NewFace(parsed, &opentype.FaceOptions{Size: item.size * scale, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
	}
	return art, nil
}

// backdrop returns the theme artwork at the canvas size, scaling it down from 2× once and caching it.
func (a *artwork) backdrop(theme string) image.Image {
	if cached, ok := a.backdrops[theme]; ok {
		return cached
	}
	source := a.sources[theme]
	w, h := scaledSize(a.scale)
	var scaled image.Image = source
	if source.Bounds().Size() != image.Pt(w, h) {
		target := image.NewGray(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(target, target.Bounds(), source, source.Bounds(), xdraw.Src, nil)
		scaled = target
	}
	a.backdrops[theme] = scaled
	return scaled
}

func timestamp(seconds int) string {
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func connectedTap(point image.Point) *bridgeAction {
	x, y := point.X, point.Y
	switch {
	case point.In(image.Rect(15, 708, 584, 792)):
		return &bridgeAction{Name: shortcuts[(x-15)/97]}
	case point.In(image.Rect(153, 205, 365, 240)):
		return &bridgeAction{Action: "seek", Value: max(0, min(100, (x-154)*100/210))}
	case point.In(image.Rect(23, 245, 372, 289)):
		for i, right := range []int{89, 155, 230, 300, 372} {
			if x < right {
				return &bridgeAction{Action: []string{"captions", "previous", "toggle", "next", "repeat"}[i]}
			}
		}
	case point.In(image.Rect(29, 289, 74, 321)):
		return &bridgeAction{Action: "mute"}
	case y >= 289 && y < 321 && x >= 78 && x <= 314:
		return &bridgeAction{Action: "volume", Value: (x - 78) * 100 / 236}
	}
	return nil
}

func (a *artwork) render(d *dashboard, now time.Time) *image.Gray {
	return a.draw(d, now).img
}

func (a *artwork) draw(d *dashboard, now time.Time) *canvas {
	t := d.theme
	img := newCanvas(a.scale)
	draw.Draw(img.img, img.img.Bounds(), a.backdrop(t.name), image.Point{}, draw.Src)
	rectangle(img, 12, 5, 266, 31, t.paper)
	textAt(img, a.small, 17, 22, now.Format("3:04 PM"), t.ink)
	textAt(img, a.small, 109, 22, strings.ToUpper(now.Format("Mon, Jan 2")), t.ink)
	if d.kindleBattery >= 0 {
		a.drawKindleBattery(img, t, d.kindleBattery, d.kindleCharging)
	}

	song := tracks[d.track]
	position, volume := d.elapsed, d.volume
	playing, shuffle, repeat := d.playing, d.shuffle, d.repeat
	rectangle(img, 152, 125, 368, 243, t.panel)
	status := "NOW PLAYING"
	if !d.playing {
		status = "PAUSED"
	}
	if d.connectedUI {
		status, song.title, song.artist = "PC OFFLINE", "Connect to the PC", "Use Wi-Fi or PC hotspot"
		song.length, position, volume = 0, 0, -1
		playing, shuffle, repeat = false, false, false
		if d.pcOnline {
			status, song.title, song.artist = "YOUTUBE READY", "Open YouTube", "Play a video on the PC"
			if !d.browserReady {
				status, song.title, song.artist = "BROWSER SETUP", "Enable extension", "Set up YouTube on PC"
			}
		}
		if track := d.youtube; track != nil && d.pcOnline {
			status = "YOUTUBE PAUSED"
			if track.Playing {
				status = "YOUTUBE PLAYING"
			}
			if track.Title != "" {
				song.title, song.artist = track.Title, track.Artist
			}
			song.length, position = track.Duration, track.Position
			playing, shuffle, repeat = track.Playing, track.Captions, track.Repeat
			if track.Volume != nil {
				volume = *track.Volume
			}
			if track.Muted {
				volume = 0
			}
		}
	}
	rectangle(img, 154, 130, 163+a.measure(a.label, status), 148, t.frame)
	textAt(img, a.label, 158, 144, status, t.paper)
	textAt(img, a.title, 154, 172, a.fitText(a.title, song.title, 210), t.ink)
	textAt(img, a.body, 154, 192, a.fitText(a.body, song.artist, 210), t.ink)
	rectangle(img, 154, 217, 364, 219, t.frame)
	length, elapsed := "--:--", "--:--"
	if song.length > 0 {
		progress := 154 + min(song.length, max(0, position))*210/song.length
		rectangle(img, 154, 215, progress, 221, t.frame)
		rectangle(img, progress-4, 212, progress+5, 223, t.ink)
		elapsed, length = timestamp(position), timestamp(song.length)
	}
	textAt(img, a.small, 154, 241, elapsed, t.ink)
	textAt(img, a.small, 364-a.measure(a.small, length), 241, length, t.ink)

	if !playing {
		rectangle(img, 180, 256, 211, 278, t.panel)
		img.triangleRight(189, 206, 266.5, 9.5, t.ink)
	}
	if shuffle {
		outline(img, image.Rect(35, 250, 80, 283), t.frame)
	}
	if repeat {
		outline(img, image.Rect(307, 250, 352, 283), t.frame)
	}
	rectangle(img, 79, 299, 312, 309, t.track)
	for index := 0; index < (volume*18+50)/100; index++ {
		x := 80 + index*231/18
		rectangle(img, x, 300, 79+(index+1)*231/18, 308, t.frame)
	}
	rectangle(img, 322, 293, 366, 316, t.panel)
	level := "--"
	if volume >= 0 {
		level = fmt.Sprintf("%d%%", volume)
	}
	textAt(img, a.small, 358-a.measure(a.small, level), 309, level, t.ink)

	a.drawToday(img, t, now)
	if d.connectedUI {
		stats := d.stats
		if !d.pcOnline {
			stats = nil
		}
		a.drawStats(img, t, stats)
		a.drawTodo(img, t, d.todo, d.pcOnline)
		a.drawWeather(img, t, d.weather, d.pcOnline)
	}

	if d.dialogTitle != "" {
		rectangle(img, 71, 307, 529, 496, t.panel)
		outline(img, image.Rect(71, 307, 529, 496), t.frame)
		outline(img, image.Rect(75, 311, 525, 492), t.frame)
		textAt(img, a.title, 92, 343, d.dialogTitle, t.ink)
		y, line := 369, ""
		for _, word := range strings.Fields(d.dialogText) {
			next := strings.TrimSpace(line + " " + word)
			if a.measure(a.body, next) > 410 {
				textAt(img, a.body, 92, y, line, t.ink)
				y, line = y+20, word
			} else {
				line = next
			}
		}
		textAt(img, a.body, 92, y, line, t.ink)
		outline(img, image.Rect(345, 433, 513, 484), t.frame)
		textAt(img, a.label, 398, 464, "CLOSE", t.ink)
		if d.confirmExit {
			outline(img, image.Rect(90, 433, 255, 484), t.frame)
			textAt(img, a.label, 149, 464, "EXIT", t.ink)
		}
	}
	return img
}

func writePNG(path string, frame image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	err = encoder.Encode(file, frame)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func main() {
	pref := flag.String("preferences", "", "path where the display theme is saved")
	preview := flag.String("preview", "", "render a PNG and exit")
	previewTheme := flag.String("theme", "dark", "theme for a PNG preview")
	bridgeCheck := flag.String("bridge-check", "", "test pairing from the given directory")
	previewStats := flag.String("preview-stats", "", "JSON file of PC stats to show in a PNG preview")
	previewTodo := flag.String("preview-todo", "", "JSON file of a to-do list to show in a PNG preview")
	previewWeather := flag.String("preview-weather", "", "JSON file of weather to show in a PNG preview")
	previewBattery := flag.Int("battery", -1, "Kindle battery level to show in a PNG preview")
	previewCharging := flag.Bool("charging", false, "show the Kindle as charging in a PNG preview")
	previewScreen := flag.String("screen", "600x800", "screen size for a PNG preview, e.g. 1072x1448 for a Paperwhite 3")
	flag.Parse()
	if *bridgeCheck != "" {
		bridge, err := readBridge(*bridgeCheck)
		if err != nil {
			log.Fatal(err)
		}
		reply, err := bridge.poll(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		weather := "none"
		if reply.Weather != nil {
			weather = "ok"
			if reply.Weather.Error != "" {
				weather = "error: " + reply.Weather.Error
			}
		}
		fmt.Printf("PC connected, browser extension: %t, YouTube video: %t, PC stats: %t, to-do: %t, weather: %s\n",
			reply.BrowserReady, reply.YouTube != nil, reply.Stats != nil, reply.Todo != nil, weather)
		return
	}
	now := time.Now()
	if *preview != "" {
		var screenW, screenH int
		if _, err := fmt.Sscanf(*previewScreen, "%dx%d", &screenW, &screenH); err != nil {
			log.Fatalf("invalid -screen %q: %v", *previewScreen, err)
		}
		view := newViewport(screenW, screenH)
		art, err := openArtwork(view.scale)
		if err != nil {
			log.Fatal(err)
		}
		model := newDashboard(*previewTheme, now)
		model.kindleBattery, model.kindleCharging = *previewBattery, *previewCharging
		if *previewStats != "" {
			raw, err := os.ReadFile(*previewStats)
			if err != nil {
				log.Fatal(err)
			}
			model.connectedUI, model.pcOnline, model.browserReady = true, true, true
			if err := json.Unmarshal(raw, &model.stats); err != nil {
				log.Fatal(err)
			}
		}
		if *previewWeather != "" {
			raw, err := os.ReadFile(*previewWeather)
			if err != nil {
				log.Fatal(err)
			}
			model.connectedUI, model.pcOnline = true, true
			if err := json.Unmarshal(raw, &model.weather); err != nil {
				log.Fatal(err)
			}
		}
		if *previewTodo != "" {
			raw, err := os.ReadFile(*previewTodo)
			if err != nil {
				log.Fatal(err)
			}
			model.connectedUI, model.pcOnline = true, true
			if err := json.Unmarshal(raw, &model.todo); err != nil {
				log.Fatal(err)
			}
		}
		if err := writePNG(*preview, view.fit(art.render(model, now), model.theme.paper)); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *pref == "" {
		binary, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		*pref = filepath.Join(filepath.Dir(binary), "theme.txt")
	}
	saved, err := os.ReadFile(*pref)
	if err != nil && !os.IsNotExist(err) {
		log.Print(err)
	}
	if err := runDevice(newDashboard(strings.TrimSpace(string(saved)), now), *pref); err != nil {
		log.Fatal(err)
	}
}
