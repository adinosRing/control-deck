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

// drawToday fills the date column of the TODAY panel. The weather column to its right is drawn by drawWeather.
func (a *artwork) drawToday(img *canvas, t theme, now time.Time) {
	const textLeft, textWidth = 121, 138
	// Calendar tile.
	rectangle(img, 28, 368, 104, 390, t.frame)
	inkText(img, a.month, strings.ToUpper(now.Format("Jan")), alignCenter, 65, 384, t.panel, 1)
	rectangle(img, 30, 390, 102, 442, t.panel)
	inkText(img, a.day, fmt.Sprint(now.Day()), alignCenter, 65.5, 429, t.ink, 1)
	// Weekday: long names such as WEDNESDAY switch to a smaller size so they don't reach the weather column.
	weekday := strings.ToUpper(now.Format("Monday"))
	face := a.weekday
	if a.measure(face, weekday)+1 > textWidth {
		face = a.weekdaySmall
	}
	rectangle(img, 118, 364, 260, 400, t.panel)
	inkText(img, face, weekday, alignLeft, textLeft, 390, t.ink, 1)
	// Full date, or a short month name ("30 SEP 2026") when the full one is too long.
	date := strings.ToUpper(now.Format("2 January 2006"))
	if a.measure(a.date, date) > textWidth {
		date = strings.ToUpper(now.Format("2 Jan 2006"))
	}
	rectangle(img, 118, 400, 260, 421, t.panel)
	inkText(img, a.date, date, alignLeft, textLeft, 412, t.ink, 0)
	_, week := now.ISOWeek()
	rectangle(img, 118, 424, 260, 449, t.panel)
	inkText(img, a.week, fmt.Sprintf("WEEK %d · DAY %d", week, now.YearDay()), alignLeft, 120.5, 438, t.muted, 0)
}
