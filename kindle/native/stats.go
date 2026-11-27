package main

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
)

// pcStats is the live performance data sent by the Windows companion.
// Pointer fields are nil when the PC cannot report that value (for example,
// no NVIDIA GPU or no battery).
type pcStats struct {
	CPU       *int     `json:"cpu"`
	GPU       *int     `json:"gpu"`
	RAM       *int     `json:"ram"`
	VRAM      *int     `json:"vram"`
	RAMUsed   *float64 `json:"ram_used"`
	RAMTotal  *float64 `json:"ram_total"`
	VRAMUsed  *float64 `json:"vram_used"`
	VRAMTotal *float64 `json:"vram_total"`
	Battery   *int     `json:"battery"`
	Plugged   *bool    `json:"plugged"`
	Down      *float64 `json:"down"` // bytes per second
	Up        *float64 `json:"up"`   // bytes per second
	History   struct {
		CPU  []int `json:"cpu"`
		GPU  []int `json:"gpu"`
		RAM  []int `json:"ram"`
		VRAM []int `json:"vram"`
	} `json:"history"`
}

// Pixel layout of the PC PERFORMANCE MONITOR panel in the 600 × 800 backdrop.
type metricColumn struct {
	left, right    int     // area between the dotted column dividers
	cx, cy         float64 // gauge centre
	sparkL, sparkR int     // sparkline box outline (inclusive)
}

var metricColumns = [4]metricColumn{
	{16, 124, 72.5, 564.5, 27, 117},
	{126, 231, 178.5, 564.5, 133, 224},
	{233, 337, 285.5, 564.5, 240, 330},
	{339, 443, 391.5, 564.5, 346, 436},
}

const (
	gaugeInner, gaugeOuter = 29.0, 37.0
	sparkTop, sparkBottom  = 618, 653 // sparkline box outline (inclusive)
)

func clampPercent(value int) int { return max(0, min(100, value)) }

func gray(a, b color.Gray, mix float64) color.Gray {
	return color.Gray{uint8(math.Round(float64(a.Y)*(1-mix) + float64(b.Y)*mix))}
}

// drawGauge paints the ring at device resolution with 4 × 4 supersampling so the edges stay smooth on e-ink.
func drawGauge(c *canvas, t theme, cx, cy float64, value *int) {
	sweep := -1.0
	if value != nil {
		sweep = float64(clampPercent(*value)) / 100 * 2 * math.Pi
	}
	s := c.scale
	pcx, pcy := cx*s, cy*s
	inner, outer := gaugeInner*s, gaugeOuter*s
	for y := int(pcy - outer - 1); y <= int(pcy+outer+1); y++ {
		for x := int(pcx - outer - 1); x <= int(pcx+outer+1); x++ {
			var filled, track int
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					dx := float64(x) + (float64(sx)+.5)/4 - pcx
					dy := float64(y) + (float64(sy)+.5)/4 - pcy
					d := math.Hypot(dx, dy)
					if d < inner || d >= outer {
						continue
					}
					// Angle measured clockwise from 12 o'clock.
					angle := math.Atan2(dx, -dy)
					if angle < 0 {
						angle += 2 * math.Pi
					}
					if angle <= sweep {
						filled++
					} else {
						track++
					}
				}
			}
			if filled+track == 0 {
				continue
			}
			ink := gray(t.panel, t.frame, float64(filled)/16)
			ink = gray(ink, t.track, float64(track)/16)
			c.img.SetGray(x, y, ink)
		}
	}
}

// drawSpark fills the area under the history line, adds the dotted texture and strokes the line,
// all at device resolution.
func drawSpark(c *canvas, t theme, left, right int, points []int) {
	rectangle(c, left+1, sparkTop+1, right, sparkBottom, t.panel)
	if len(points) < 2 {
		return
	}
	x0, x1 := c.px(float64(left+1)), c.px(float64(right))-1
	y0, y1 := c.px(float64(sparkTop+1)), c.px(float64(sparkBottom))-1
	u := c.unit()
	w, h := float64(x1-x0), float64(y1-y0)
	lineY := func(x int) float64 {
		pos := float64(x-x0) / w * float64(len(points)-1)
		i := min(int(pos), len(points)-2)
		frac := pos - float64(i)
		v := float64(clampPercent(points[i]))*(1-frac) + float64(clampPercent(points[i+1]))*frac
		return float64(y0) + h - float64(u) - v/100*(h-2*float64(u))
	}
	prev := lineY(x0)
	for x := x0; x <= x1; x++ {
		ly := lineY(x)
		for y := int(math.Ceil(ly)); y <= y1; y++ {
			ink := t.shade
			// Dotted texture in logical-pixel-sized dots.
			gx, gy := (x-x0)/u, (y-y0)/u
			if gy%3 == 2 && (gx+(gy/3%2)*2)%4 == 0 {
				ink = t.track
			}
			c.img.SetGray(x, y, ink)
		}
		top, bottom := math.Min(prev, ly), math.Max(prev, ly)
		for y := int(math.Floor(top)); y <= int(math.Ceil(bottom))+u-1 && y <= y1; y++ {
			if y >= y0 {
				c.img.SetGray(x, y, t.frame)
			}
		}
		prev = ly
	}
}

func centerText(c *canvas, face font.Face, cx float64, baseline int, value string, ink color.Gray) {
	c.textPx(face, c.px(cx)-font.MeasureString(face, value).Ceil()/2, c.px(float64(baseline)), value, ink)
}

func rightText(c *canvas, face font.Face, right, baseline int, value string, ink color.Gray) {
	c.textPx(face, c.px(float64(right))-font.MeasureString(face, value).Ceil(), c.px(float64(baseline)), value, ink)
}

func dottedLine(c *canvas, x0, x1, y int, ink color.Gray) {
	for x := x0; x <= x1; x += 2 {
		rectangle(c, x, y, x+1, y+1, ink)
	}
}

func speed(bytes *float64) string {
	if bytes == nil {
		return "--"
	}
	switch value := *bytes; {
	case value >= 1e6:
		return fmt.Sprintf("%.1f MB/s", value/1e6)
	case value >= 1e3:
		return fmt.Sprintf("%.0f KB/s", value/1e3)
	default:
		return fmt.Sprintf("%.0f B/s", value)
	}
}

func capacity(used, total *float64) string {
	if used == nil || total == nil {
		return ""
	}
	return fmt.Sprintf("%.1f/%.1f GB", *used, *total)
}

// drawBattery draws a small vertical battery icon filled to the charge level.
func drawBattery(img *canvas, t theme, x, y int, level *int, plugged bool) {
	outline(img, image.Rect(x, y+3, x+16, y+34), t.frame)
	outline(img, image.Rect(x+1, y+4, x+15, y+33), t.frame)
	rectangle(img, x+5, y, x+11, y+3, t.frame)
	if level != nil {
		fill := 25 * clampPercent(*level) / 100
		rectangle(img, x+3, y+31-fill, x+13, y+31, t.frame)
	}
	if plugged {
		// Lightning bolt cut out of the fill.
		bolt := t.panel
		if level == nil || *level < 50 {
			bolt = t.frame
		}
		for i := 0; i < 6; i++ {
			rectangle(img, x+9-i/2, y+10+i, x+11-i/2, y+11+i, bolt)
			rectangle(img, x+7-i/2, y+16+i, x+9-i/2, y+17+i, bolt)
		}
		rectangle(img, x+5, y+16, x+11, y+17, bolt)
	}
}

// drawStats replaces the sample numbers baked into the backdrop with live PC values.
// A nil stats value (PC offline) shows empty gauges and dashes.
func (a *artwork) drawStats(img *canvas, t theme, s *pcStats) {
	if s == nil {
		s = &pcStats{}
	}
	values := [4]*int{s.CPU, s.GPU, s.RAM, s.VRAM}
	histories := [4][]int{s.History.CPU, s.History.GPU, s.History.RAM, s.History.VRAM}
	capacities := [4]string{"", "", capacity(s.RAMUsed, s.RAMTotal), capacity(s.VRAMUsed, s.VRAMTotal)}
	for i, col := range metricColumns {
		// Gauge and its percentage.
		rectangle(img, int(col.cx-gaugeOuter)-2, int(col.cy-gaugeOuter)-2, int(col.cx+gaugeOuter)+3, int(col.cy+gaugeOuter)+3, t.panel)
		drawGauge(img, t, col.cx, col.cy, values[i])
		label := "--"
		if values[i] != nil {
			label = fmt.Sprintf("%d%%", clampPercent(*values[i]))
		}
		// Drawn twice, one pixel apart, to match the bold numbers in the artwork.
		centerText(img, a.title, col.cx, 572, label, t.ink)
		centerText(img, a.title, col.cx+1, 572, label, t.ink)
		// Capacity line such as "21.8/32.0 GB".
		rectangle(img, col.left+1, 603, col.right, 617, t.panel)
		if capacities[i] != "" {
			centerText(img, a.tiny, col.cx, 614, a.fitText(a.tiny, capacities[i], col.right-col.left-4), t.ink)
		}
		drawSpark(img, t, col.sparkL, col.sparkR, histories[i])
	}

	// Battery block replaces the temperature block (Windows has no standard temperature API).
	rectangle(img, 449, 501, 584, 562, t.panel)
	plugged := s.Plugged != nil && *s.Plugged
	drawBattery(img, t, 461, 510, s.Battery, plugged)
	textAt(img, a.small, 485, 523, "BAT", t.ink)
	level := "--"
	if s.Battery != nil {
		level = fmt.Sprintf("%d%%", clampPercent(*s.Battery))
	}
	rightText(img, a.small, 575, 523, level, t.ink)
	dottedLine(img, 485, 575, 532, t.line)
	state := "--"
	switch {
	case s.Battery == nil && s.Plugged != nil:
		state = "AC POWER"
	case s.Battery != nil && plugged && *s.Battery >= 100:
		state = "FULL"
	case s.Battery != nil && plugged:
		state = "CHARGING"
	case s.Battery != nil:
		state = "ON BATT"
	}
	rightText(img, a.small, 575, 548, state, t.ink)

	// Network speeds.
	rectangle(img, 449, 592, 584, 631, t.panel)
	rightText(img, a.small, 575, 606, "↓ "+speed(s.Down), t.ink)
	rightText(img, a.small, 575, 626, "↑ "+speed(s.Up), t.ink)
}
