package main

import (
	"image"
	"image/color"
	"testing"
	"time"
)

func TestConnectedDashboardUsesFixedPanels(t *testing.T) {
	art, err := openArtwork()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, name := range []string{"dark", "light"} {
		model := newDashboard(name, now)
		model.connectedUI, model.pcOnline = true, true
		model.youtube = &youtubeVideo{Title: "Test track", Duration: 180, Playing: true}
		frame := art.render(model, now)
		original := art.backdrops[name]
		for _, panel := range []image.Rectangle{image.Rect(9, 332, 382, 457), image.Rect(9, 466, 591, 667)} {
			for y := panel.Min.Y; y < panel.Max.Y; y++ {
				for x := panel.Min.X; x < panel.Max.X; x++ {
					expected := color.GrayModel.Convert(original.At(x, y)).(color.Gray)
					if frame.GrayAt(x, y) != expected {
						t.Fatalf("%s fixed panel changed at %d,%d", name, x, y)
					}
				}
			}
		}
	}
}

func TestKT4TouchRelease(t *testing.T) {
	for _, release := range []struct {
		kind, code uint16
		value      int
	}{
		{evKey, btnTouch, 0}, {evAbs, absTracking, -1},
	} {
		decoder := touchDecoder{multiTouch: true, horizontal: axisRange{0, 599}, vertical: axisRange{0, 799}, primary: -1}
		decoder.decode(evAbs, absTracking, 4)
		decoder.decode(evAbs, absMtX, 424)
		decoder.decode(evAbs, absMtY, 62)
		if _, ok := decoder.decode(evSync, synReport, 0); ok {
			t.Fatal("first contact was mistaken for a completed tap")
		}
		decoder.decode(release.kind, release.code, release.value)
		point, ok := decoder.decode(evSync, synReport, 0)
		if !ok || point != image.Pt(424, 62) {
			t.Fatalf("unexpected tap result %v (%t)", point, ok)
		}
		decoder.decode(evKey, btnTouch, 0)
		if _, ok := decoder.decode(evSync, synReport, 0); ok {
			t.Fatal("duplicated touch release produced another tap")
		}
	}
}

func TestDashboardControlsAndRender(t *testing.T) {
	now := time.Date(2026, 11, 18, 19, 0, 0, 0, time.UTC)
	model := newDashboard("dark", now)
	art, err := openArtwork()
	if err != nil {
		t.Fatal(err)
	}
	before := art.render(model, now)
	exit, flash := model.tap(image.Pt(424, 62), now)
	after := art.render(model, now)
	if exit || !flash || model.theme != light || before.GrayAt(0, 0) == after.GrayAt(0, 0) {
		t.Fatal("theme tap did not switch the artwork")
	}
	model.tap(image.Pt(270, 267), now)
	model.tap(image.Pt(196, 267), now)
	model.tap(image.Pt(53, 302), now)
	if model.track != 1 || model.playing || model.volume != 0 {
		t.Fatal("demo controls did not update their state")
	}
	model.tap(image.Pt(545, 740), now)
	if model.dialogTitle != "TERMINAL" {
		t.Fatal("shortcut dialog missing")
	}
	model.tap(image.Pt(420, 460), now)
	if model.dialogTitle != "" {
		t.Fatal("shortcut dialog did not close")
	}
	model.tap(image.Pt(542, 59), now)
	if exit, _ := model.tap(image.Pt(165, 460), now); !exit {
		t.Fatal("exit confirmation ignored")
	}
}
