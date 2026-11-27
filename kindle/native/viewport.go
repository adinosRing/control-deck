package main

import (
	"image"
	"image/color"
	"math"
	"regexp"
	"strconv"

	xdraw "golang.org/x/image/draw"
)

// viewport scales the 600 × 800 dashboard to the Kindle's real screen
// (for example 1072 × 1448 on a Paperwhite 3) and maps taps back.
type viewport struct {
	screenW, screenH int
	scale            float64
	offX, offY       int
	dstW, dstH       int
}

func newViewport(screenW, screenH int) viewport {
	if screenW <= 0 || screenH <= 0 {
		screenW, screenH = width, height
	}
	scale := math.Min(float64(screenW)/width, float64(screenH)/height)
	dstW, dstH := int(math.Round(width*scale)), int(math.Round(height*scale))
	return viewport{screenW, screenH, scale, (screenW - dstW) / 2, (screenH - dstH) / 2, dstW, dstH}
}

func (v viewport) native() bool { return v.screenW == width && v.screenH == height }

// fit scales a dashboard frame to the screen, centring it on the paper colour.
func (v viewport) fit(frame *image.Gray, paper color.Gray) image.Image {
	if v.native() {
		return frame
	}
	screen := image.NewGray(image.Rect(0, 0, v.screenW, v.screenH))
	xdraw.Draw(screen, screen.Bounds(), image.NewUniform(paper), image.Point{}, xdraw.Src)
	target := image.Rect(v.offX, v.offY, v.offX+v.dstW, v.offY+v.dstH)
	xdraw.ApproxBiLinear.Scale(screen, target, frame, frame.Bounds(), xdraw.Src, nil)
	return screen
}

// fromTouch converts a tap, already scaled from the touch panel's range to 600 × 800,
// into dashboard coordinates on the scaled, centred frame.
func (v viewport) fromTouch(point image.Point) image.Point {
	if v.native() {
		return point
	}
	sx := float64(point.X) * float64(v.screenW) / width
	sy := float64(point.Y) * float64(v.screenH) / height
	return image.Pt(int((sx-float64(v.offX))/v.scale), int((sy-float64(v.offY))/v.scale))
}

var fbinkSize = regexp.MustCompile(`viewWidth=(\d+);?\s*viewHeight=(\d+)`)

// parseFBInkState reads the screen size from the output of `fbink -e`.
func parseFBInkState(output string) (int, int, bool) {
	match := fbinkSize.FindStringSubmatch(output)
	if match == nil {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(match[1])
	h, errH := strconv.Atoi(match[2])
	if errW != nil || errH != nil || w < 300 || h < 300 {
		return 0, 0, false
	}
	if w > h {
		w, h = h, w // the dashboard is portrait
	}
	return w, h, true
}
