package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"golang.org/x/image/font"
)

// Ink-box anchoring keeps the live text where the browser placed the sample
// text in the backdrop, whatever the font's side bearings are.
type anchor int

const (
	alignLeft anchor = iota
	alignCenter
	alignRight
)

// inkText draws value so that its visible ink sits at x (left edge, centre or right edge) with its lowest
// pixel row at bottom, both in logical pixels. bold > 0 repeats the text that many logical pixels to the right
// to match the bold weight used in the artwork. Positions are worked out in device pixels, so it stays exact
// at any scale.
func inkText(c *canvas, face font.Face, value string, align anchor, x float64, bottom int, ink color.Gray, bold int) {
	bounds, _ := font.BoundString(face, value)
	minX, maxX := bounds.Min.X.Floor(), bounds.Max.X.Ceil()
	boldPx := c.px(float64(bold))
	left := c.px(x) - minX
	switch align {
	case alignCenter:
		left = c.px(x+.5) - (minX+maxX+boldPx)/2
	case alignRight:
		left = c.px(x+1) - maxX - boldPx
	}
	baseline := c.px(float64(bottom+1)) - bounds.Max.Y.Ceil()
	// Thicken sideways only: shifting vertically too would close small gaps, turning a "2" into an "8".
	for dx := 0; dx <= boldPx; dx++ {
		c.textPx(face, left+dx, baseline, value, ink)
	}
}

// drawToday fills the TODAY panel with the current date.
func (a *artwork) drawToday(img *canvas, t theme, now time.Time) {
	// TODAY panel: calendar tile, weekday, full date, ISO week and day of the year.
	rectangle(img, 29, 368, 103, 390, t.frame)
	inkText(img, a.month, strings.ToUpper(now.Format("Jan")), alignCenter, 65, 384, t.panel, 1)
	rectangle(img, 30, 390, 102, 442, t.panel)
	inkText(img, a.day, fmt.Sprint(now.Day()), alignCenter, 65.5, 428, t.ink, 1)
	rectangle(img, 118, 364, 374, 401, t.panel)
	inkText(img, a.weekday, strings.ToUpper(now.Format("Monday")), alignLeft, 122, 391, t.ink, 1)
	rectangle(img, 118, 402, 374, 422, t.panel)
	inkText(img, a.date, strings.ToUpper(now.Format("2 January 2006")), alignLeft, 121, 415, t.ink, 0)
	_, week := now.ISOWeek()
	rectangle(img, 118, 428, 374, 449, t.panel)
	inkText(img, a.week, fmt.Sprintf("WEEK %d · DAY %d", week, now.YearDay()), alignLeft, 120, 442, t.muted, 0)
}
