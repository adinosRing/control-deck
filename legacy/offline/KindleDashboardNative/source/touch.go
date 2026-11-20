package main

import "image"

const (
	evSync      = 0
	evKey       = 1
	evAbs       = 3
	synReport   = 0
	synDropped  = 3
	btnTouch    = 0x14a
	absX        = 0
	absY        = 1
	absSlot     = 0x2f
	absMtX      = 0x35
	absMtY      = 0x36
	absTracking = 0x39
)

type axisRange struct{ min, max int }

func (axis axisRange) scale(value, size int) int {
	return (max(axis.min, min(value, axis.max)) - axis.min) * (size - 1) / (axis.max - axis.min)
}

type touchDecoder struct {
	multiTouch           bool
	horizontal, vertical axisRange
	slot, primary        int
	x, y                 int
	hasX, hasY, released bool
}

func (d *touchDecoder) decode(kind, code uint16, value int) (image.Point, bool) {
	switch kind {
	case evAbs:
		if code == absSlot {
			d.slot = value
		}
		if d.primary < 0 && (code == absTracking && value >= 0 || code == absX || code == absY || code == absMtX || code == absMtY) {
			d.primary = d.slot
		}
		if d.slot != d.primary {
			break
		}
		switch {
		case code == absTracking && value == -1:
			d.released = true
		case code == absMtX && d.multiTouch || code == absX && !d.multiTouch:
			d.x, d.hasX = value, true
		case code == absMtY && d.multiTouch || code == absY && !d.multiTouch:
			d.y, d.hasY = value, true
		}
	case evKey:
		if code == btnTouch && value == 0 {
			d.released = true
		}
	case evSync:
		if code == synDropped {
			d.primary, d.slot = -1, 0
			d.hasX, d.hasY, d.released = false, false, false
		}
		if code == synReport && d.released {
			ok := d.hasX && d.hasY
			point := image.Pt(d.horizontal.scale(d.x, width), d.vertical.scale(d.y, height))
			d.primary = -1
			d.hasX, d.hasY, d.released = false, false, false
			return point, ok
		}
	}
	return image.Point{}, false
}
