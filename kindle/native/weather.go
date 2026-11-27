package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
)

// weatherInfo is the current weather sent by the PC companion (from Open-Meteo).
type weatherInfo struct {
	Temp  *float64 `json:"temp"`
	Code  *int     `json:"code"` // WMO weather code
	IsDay bool     `json:"is_day"`
	High  *float64 `json:"high"`
	Low   *float64 `json:"low"`
	Place string   `json:"place"`
	Error string   `json:"error"`
}

type weatherKind int

const (
	weatherClear weatherKind = iota
	weatherPartly
	weatherCloud
	weatherFog
	weatherDrizzle
	weatherRain
	weatherSnow
	weatherThunder
)

// describeWeather turns a WMO weather code into a short label and an icon.
func describeWeather(code int) (string, weatherKind) {
	switch code {
	case 0:
		return "CLEAR", weatherClear
	case 1:
		return "MOSTLY CLEAR", weatherPartly
	case 2:
		return "PARTLY CLOUDY", weatherPartly
	case 3:
		return "OVERCAST", weatherCloud
	case 45, 48:
		return "FOG", weatherFog
	case 51, 53, 55:
		return "DRIZZLE", weatherDrizzle
	case 56, 57:
		return "FREEZING DRIZZLE", weatherDrizzle
	case 61:
		return "LIGHT RAIN", weatherRain
	case 63:
		return "RAIN", weatherRain
	case 65:
		return "HEAVY RAIN", weatherRain
	case 66, 67:
		return "FREEZING RAIN", weatherRain
	case 71, 73, 75, 77:
		return "SNOW", weatherSnow
	case 80, 81, 82:
		return "SHOWERS", weatherRain
	case 85, 86:
		return "SNOW SHOWERS", weatherSnow
	case 95:
		return "THUNDERSTORM", weatherThunder
	case 96, 99:
		return "STORM + HAIL", weatherThunder
	}
	return "UNKNOWN", weatherCloud
}

// disc fills a circle given in logical coordinates, at device resolution.
func (c *canvas) disc(cx, cy, r float64, ink color.Gray) {
	pcx, pcy, pr := cx*c.scale, cy*c.scale, r*c.scale
	for y := int(math.Floor(pcy - pr)); y <= int(math.Ceil(pcy+pr)); y++ {
		dy := float64(y) + .5 - pcy
		if math.Abs(dy) > pr {
			continue
		}
		half := math.Sqrt(pr*pr - dy*dy)
		c.fillPx(image.Rect(int(math.Round(pcx-half)), y, int(math.Round(pcx+half)), y+1), ink)
	}
}

// cloud draws an outlined cloud whose box starts at (x, y) and is w wide.
func (c *canvas) cloud(x, y, w, stroke float64, ink, bg color.Gray) {
	shape := func(inset float64, color color.Gray) {
		c.disc(x+.3*w, y+.55*w, .2*w-inset, color)
		c.disc(x+.55*w, y+.42*w, .27*w-inset, color)
		c.disc(x+.78*w, y+.6*w, .17*w-inset, color)
		x0, y0 := x+.3*w, y+.55*w
		c.fillPx(image.Rect(c.px(x0), c.px(y0), c.px(x+.78*w), c.px(y+.77*w-inset)), color)
	}
	shape(0, ink)
	shape(stroke, bg)
}

// drawWeatherIcon draws a size × size outline icon with its top-left corner at (x, y).
func (c *canvas) drawWeatherIcon(kind weatherKind, isDay bool, x, y, size float64, ink, bg color.Gray) {
	stroke := 1.6
	sun := func(cx, cy, r float64) {
		if !isDay {
			// Crescent moon.
			c.disc(cx, cy, r*1.25, ink)
			c.disc(cx+r*.75, cy-r*.45, r*1.05, bg)
			return
		}
		c.disc(cx, cy, r, ink)
		c.disc(cx, cy, r-stroke, bg)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			c.line(cx+math.Cos(a)*(r+2.2), cy+math.Sin(a)*(r+2.2), cx+math.Cos(a)*(r+4.6), cy+math.Sin(a)*(r+4.6), 1.4, ink)
		}
	}
	switch kind {
	case weatherClear:
		sun(x+size/2, y+size/2, size*.24)
	case weatherPartly:
		sun(x+size*.36, y+size*.34, size*.17)
		c.cloud(x+size*.2, y+size*.22, size*.82, stroke, ink, bg)
	case weatherFog:
		for i, inset := range []float64{.1, 0, .2} {
			ly := y + size*(.32+float64(i)*.2)
			c.line(x+size*inset, ly, x+size*(.9-inset/2), ly, 1.6, ink)
		}
	default:
		c.cloud(x, y-size*.12, size, stroke, ink, bg)
		below := y + size*.7
		switch kind {
		case weatherRain:
			for i := 0; i < 3; i++ {
				lx := x + size*(.3+float64(i)*.2)
				c.line(lx, below, lx-2.5, below+size*.26, 1.5, ink)
			}
		case weatherDrizzle:
			for i := 0; i < 3; i++ {
				c.disc(x+size*(.3+float64(i)*.2), below+size*.14, 1.3, ink)
			}
		case weatherSnow:
			for i := 0; i < 3; i++ {
				cx, cy := x+size*(.3+float64(i)*.2), below+size*.14+float64(i%2)*2
				c.line(cx-2, cy, cx+2, cy, 1.2, ink)
				c.line(cx, cy-2, cx, cy+2, 1.2, ink)
			}
		case weatherThunder:
			bx, by := x+size*.42, below-1
			c.polygon([][2]float64{{bx + 4, by}, {bx - 1, by + 6}, {bx + 2.5, by + 6}, {bx, by + 11}, {bx + 6.5, by + 4}, {bx + 3, by + 4}, {bx + 5.5, by}}, ink)
		}
	}
}

func degrees(v *float64) string {
	if v == nil {
		return "--"
	}
	return fmt.Sprintf("%d°", int(math.Round(*v)))
}

// drawWeather fills the weather column of the TODAY panel.
func (a *artwork) drawWeather(c *canvas, t theme, w *weatherInfo, online bool) {
	rectangle(c, 263, 362, 375, 450, t.panel)
	const left, width = 271, 102
	switch {
	case !online:
		textAt(c, a.tiny, left, 401, "WEATHER", t.muted)
		textAt(c, a.tiny, left, 417, "PC offline", t.muted)
		return
	case w == nil || w.Temp == nil || w.Code == nil:
		message := "Loading..."
		if w != nil && w.Error != "" {
			message = "Unavailable"
		}
		textAt(c, a.tiny, left, 401, "WEATHER", t.muted)
		textAt(c, a.tiny, left, 417, message, t.muted)
		return
	}
	label, kind := describeWeather(*w.Code)
	c.drawWeatherIcon(kind, w.IsDay, 271, 369, 23, t.ink, t.panel)
	inkText(c, a.weekday, degrees(w.Temp), alignLeft, 301, 389, t.ink, 1)
	textAt(c, a.tiny, left, 409, a.fitText(a.tiny, label, width), t.ink)
	textAt(c, a.tiny, left, 425, a.fitText(a.tiny, "H "+degrees(w.High)+" L "+degrees(w.Low), width), t.muted)
	if w.Place != "" {
		textAt(c, a.tiny, left, 441, a.fitText(a.tiny, strings.ToUpper(w.Place), width), t.muted)
	}
}
