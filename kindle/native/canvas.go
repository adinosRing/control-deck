package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// canvas draws the dashboard at the screen's real resolution.
//
// All layout code works in the 600 × 800 design grid ("logical pixels"); the
// canvas multiplies by scale at the last moment, so lines, shapes and text
// are rasterised natively at, for example, 1072 × 1429 on a Paperwhite 3
// instead of being drawn small and stretched afterwards.
type canvas struct {
	img   *image.Gray
	scale float64
}

func newCanvas(scale float64) *canvas {
	w, h := scaledSize(scale)
	return &canvas{img: image.NewGray(image.Rect(0, 0, w, h)), scale: scale}
}

func scaledSize(scale float64) (int, int) {
	return int(math.Round(width * scale)), int(math.Round(height * scale))
}

// px converts a logical coordinate to a device pixel.
func (c *canvas) px(v float64) int { return int(math.Round(v * c.scale)) }

// unit is the device-pixel size of one logical pixel, at least 1.
func (c *canvas) unit() int { return max(1, c.px(1)) }

func (c *canvas) fillPx(r image.Rectangle, ink color.Gray) {
	draw.Draw(c.img, r, image.NewUniform(ink), image.Point{}, draw.Src)
}

func rectangle(c *canvas, x0, y0, x1, y1 int, ink color.Gray) {
	c.fillPx(image.Rect(c.px(float64(x0)), c.px(float64(y0)), c.px(float64(x1)), c.px(float64(y1))), ink)
}

// outline draws a one-logical-pixel border inside rect, scaled to a crisp whole number of device pixels.
func outline(c *canvas, rect image.Rectangle, ink color.Gray) {
	x0, y0 := c.px(float64(rect.Min.X)), c.px(float64(rect.Min.Y))
	x1, y1 := c.px(float64(rect.Max.X)), c.px(float64(rect.Max.Y))
	w := c.unit()
	c.fillPx(image.Rect(x0, y0, x1, y0+w), ink)
	c.fillPx(image.Rect(x0, y1-w, x1, y1), ink)
	c.fillPx(image.Rect(x0, y0, x0+w, y1), ink)
	c.fillPx(image.Rect(x1-w, y0, x1, y1), ink)
}

func (c *canvas) textPx(face font.Face, x, baseline int, value string, ink color.Gray) {
	drawer := font.Drawer{Dst: c.img, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, baseline)}
	drawer.DrawString(value)
}

// textAt draws text with its origin at a logical position. Faces are already sized for the canvas scale.
func textAt(c *canvas, face font.Face, x, baseline int, value string, ink color.Gray) {
	c.textPx(face, c.px(float64(x)), c.px(float64(baseline)), value, ink)
}

// line draws a straight line of the given logical thickness, natively at device resolution.
func (c *canvas) line(x0, y0, x1, y1, thickness float64, ink color.Gray) {
	sx0, sy0, sx1, sy1 := x0*c.scale, y0*c.scale, x1*c.scale, y1*c.scale
	t := max(1, int(math.Round(thickness*c.scale)))
	steps := int(math.Ceil(math.Max(math.Abs(sx1-sx0), math.Abs(sy1-sy0)))) + 1
	for i := 0; i <= steps; i++ {
		f := float64(i) / float64(steps)
		x := int(math.Round(sx0 + (sx1-sx0)*f))
		y := int(math.Round(sy0 + (sy1-sy0)*f))
		c.fillPx(image.Rect(x, y, x+t, y+t), ink)
	}
}

// triangleRight fills a play-style triangle: full height halfHeight×2 at x0, narrowing to a point at x1.
func (c *canvas) triangleRight(x0, x1, cy, halfHeight float64, ink color.Gray) {
	px0, px1 := c.px(x0), c.px(x1)
	for x := px0; x < px1; x++ {
		f := (float64(x) + .5 - x0*c.scale) / ((x1 - x0) * c.scale)
		h := halfHeight * (1 - f) * c.scale
		top := int(math.Round(cy*c.scale - h))
		bottom := int(math.Round(cy*c.scale + h))
		c.fillPx(image.Rect(x, top, x+1, bottom), ink)
	}
}

// measure returns the logical width of value, rounded up.
func (a *artwork) measure(face font.Face, value string) int {
	return int(math.Ceil(float64(font.MeasureString(face, value).Ceil()) / a.scale))
}

// fitText shortens value with "..." until it fits width logical pixels.
func (a *artwork) fitText(face font.Face, value string, width int) string {
	if a.measure(face, value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		shortened := string(runes) + "..."
		if a.measure(face, shortened) <= width {
			return shortened
		}
	}
	return ""
}

// drawKindleBattery replaces the sample battery in the status bar with the Kindle's real charge.
func (a *artwork) drawKindleBattery(c *canvas, t theme, level int, charging bool) {
	level = max(0, min(100, level))
	rectangle(c, 505, 5, 590, 30, t.paper)
	rightText(c, a.small, 547, 22, fmt.Sprintf("%d%%", level), t.ink)
	// Battery body, terminal nub and charge level.
	outline(c, image.Rect(556, 12, 577, 24), t.ink)
	rectangle(c, 577, 15, 580, 21, t.ink)
	rectangle(c, 558, 14, 558+17*level/100, 22, t.ink)
	if charging {
		// While charging, show a clear lightning bolt; the percentage still gives the level.
		rectangle(c, 558, 14, 575, 22, t.paper)
		c.polygon([][2]float64{{568.5, 12.5}, {562, 18.6}, {566, 18.6}, {564.5, 23.5}, {571.5, 17.2}, {567.4, 17.2}, {569.5, 12.5}}, t.ink)
	}
}

// polygon fills a closed shape given in logical coordinates, at device resolution.
func (c *canvas) polygon(points [][2]float64, ink color.Gray) {
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range points {
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	for py := int(math.Floor(minY * c.scale)); py <= int(math.Ceil(maxY*c.scale)); py++ {
		y := (float64(py) + .5) / c.scale
		var xs []float64
		for i := range points {
			p, q := points[i], points[(i+1)%len(points)]
			if (p[1] <= y) != (q[1] <= y) {
				xs = append(xs, p[0]+(y-p[1])/(q[1]-p[1])*(q[0]-p[0]))
			}
		}
		sort.Float64s(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			c.fillPx(image.Rect(int(math.Round(xs[i]*c.scale)), py, int(math.Round(xs[i+1]*c.scale)), py+1), ink)
		}
	}
}
