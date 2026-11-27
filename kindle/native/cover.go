package main

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	_ "golang.org/x/image/webp"
)

// coverArt is the album cover of the song playing on the PC, decoded on the Kindle.
// Scaled grey copies are cached so a repaint does not resample the picture again.
type coverArt struct {
	ID     string
	source image.Image
	mu     sync.Mutex
	cache  map[image.Point]*image.Gray
}

func newCoverArt(id string, source image.Image) *coverArt {
	return &coverArt{ID: id, source: source, cache: map[image.Point]*image.Gray{}}
}

// scaled crops the cover to the box's shape from the centre and resamples it to size.
func (c *coverArt) scaled(size image.Point) *image.Gray {
	c.mu.Lock()
	defer c.mu.Unlock()
	if img := c.cache[size]; img != nil {
		return img
	}
	src := c.source.Bounds()
	crop := src
	if src.Dx()*size.Y > src.Dy()*size.X {
		w := src.Dy() * size.X / size.Y
		crop.Min.X += (src.Dx() - w) / 2
		crop.Max.X = crop.Min.X + w
	} else {
		h := src.Dx() * size.Y / size.X
		crop.Min.Y += (src.Dy() - h) / 2
		crop.Max.Y = crop.Min.Y + h
	}
	img := image.NewGray(image.Rectangle{Max: size})
	xdraw.CatmullRom.Scale(img, img.Bounds(), c.source, crop, xdraw.Src, nil)
	c.cache[size] = img
	return img
}

// drawCover fills a box given in logical pixels with the cover, at device resolution.
func (c *canvas) drawCover(art *coverArt, x0, y0, x1, y1 int) {
	box := image.Rect(c.px(float64(x0)), c.px(float64(y0)), c.px(float64(x1)), c.px(float64(y1)))
	draw.Draw(c.img, box, art.scaled(box.Size()), image.Point{}, draw.Src)
}

// Layout of the ALBUM ART panel in the 600 × 800 backdrop.
const (
	coverX0, coverY0, coverX1, coverY1 = 405, 133, 575, 303 // picture inside its frame
	coverCentre                        = 490
	coverTextWidth                     = 180
	// The small picture in the YOUTUBE CONTROL panel.
	albumX0, albumY0, albumX1, albumY1 = 35, 135, 133, 226
)

// drawVinyl is shown in the frame when there is no cover to show.
func (c *canvas) drawVinyl(t theme) {
	rectangle(c, coverX0, coverY0, coverX1, coverY1, t.panel)
	cx, cy := float64(coverCentre), float64(coverY0+coverY1)/2
	c.disc(cx, cy, 64, t.frame)
	c.disc(cx, cy, 50, t.panel)
	c.disc(cx, cy, 48, t.frame)
	c.disc(cx, cy, 34, t.panel)
	c.disc(cx, cy, 32, t.frame)
	c.disc(cx, cy, 18, t.panel)
	c.disc(cx, cy, 4, t.frame)
}

// drawCoverPanel fills the ALBUM ART panel from what the PC is playing.
func (a *artwork) drawCoverPanel(c *canvas, t theme, d *dashboard) {
	rectangle(c, 396, 306, 585, 452, t.panel)
	video, art := d.youtube, d.cover
	if !d.pcOnline {
		video, art = nil, nil
	}
	if art != nil && video != nil && art.ID == video.Art {
		c.drawCover(art, coverX0, coverY0, coverX1, coverY1)
		c.drawCover(art, albumX0, albumY0, albumX1, albumY1)
	} else {
		c.drawVinyl(t)
	}
	var lines []string
	artist, source := "", ""
	switch {
	case !d.pcOnline:
		lines, artist = []string{"PC OFFLINE"}, "Connect to the PC"
	case video == nil:
		lines, artist = []string{"Nothing playing"}, "Play a song on"
		source = "YOUTUBE MUSIC"
	default:
		lines = a.wrapTwoLines(a.count, video.Title, coverTextWidth)
		artist = video.Artist
		source = "YOUTUBE"
		if video.Source == "music" {
			source = "YT MUSIC"
		}
		if video.Playing {
			source += " · PLAYING"
		} else {
			source += " · PAUSED"
		}
	}
	baseline := 331
	for _, line := range lines {
		centerText(c, a.count, coverCentre, baseline, line, t.ink)
		baseline += 20
	}
	if artist != "" {
		centerText(c, a.small, coverCentre, baseline+2, a.fitText(a.small, artist, coverTextWidth), t.ink)
	}
	if source != "" {
		centerText(c, a.tiny, coverCentre, 446, source, t.muted)
	}
}

// wrapTwoLines splits text into at most two lines that fit width, ending in "..." if it is cut.
func (a *artwork) wrapTwoLines(face font.Face, text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	first := ""
	rest := words
	for len(rest) > 0 {
		next := strings.TrimSpace(first + " " + rest[0])
		if a.measure(face, next) > width {
			break
		}
		first, rest = next, rest[1:]
	}
	if first == "" {
		// A single word wider than the line: cut it.
		return []string{a.fitText(face, text, width)}
	}
	if len(rest) == 0 {
		return []string{first}
	}
	return []string{first, a.fitText(face, strings.Join(rest, " "), width)}
}
