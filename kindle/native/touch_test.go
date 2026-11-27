package main

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"
)

func TestConnectedDashboardUsesFixedPanels(t *testing.T) {
	art, err := openArtwork(1)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, name := range []string{"dark", "light"} {
		model := newDashboard(name, now)
		model.connectedUI, model.pcOnline = true, true
		model.youtube = &youtubeVideo{Title: "Test track", Duration: 180, Playing: true}
		frame := art.render(model, now)
		original := art.backdrop(name)
		// Panel frames and headings stay exactly as drawn in the backdrop.
		for _, panel := range []image.Rectangle{
			image.Rect(9, 332, 382, 358), image.Rect(389, 91, 528, 122), image.Rect(389, 451, 591, 458),
			image.Rect(9, 466, 591, 500), image.Rect(9, 655, 591, 667),
		} {
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

func TestConnectedDashboardShowsLiveStats(t *testing.T) {
	art, err := openArtwork(1)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cpu, ram, battery, plugged := 12, 91, 40, true
	model := newDashboard("dark", now)
	model.connectedUI, model.pcOnline = true, true
	offline := art.render(model, now)
	model.stats = &pcStats{CPU: &cpu, RAM: &ram, Battery: &battery, Plugged: &plugged}
	model.stats.History.CPU = []int{0, 50, 100}
	live := art.render(model, now)
	changed := func(area image.Rectangle) bool {
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if offline.GrayAt(x, y) != live.GrayAt(x, y) {
					return true
				}
			}
		}
		return false
	}
	for name, area := range map[string]image.Rectangle{
		"CPU gauge": image.Rect(35, 527, 111, 603), "CPU sparkline": image.Rect(28, 619, 117, 653),
		"RAM gauge": image.Rect(248, 527, 324, 603), "battery": image.Rect(449, 501, 584, 562),
	} {
		if !changed(area) {
			t.Errorf("%s did not show the live value", name)
		}
	}
	if changed(image.Rect(141, 527, 217, 603)) {
		t.Error("GPU gauge changed although the PC reported no GPU")
	}
}

func TestDateFollowsTheDay(t *testing.T) {
	art, err := openArtwork(1)
	if err != nil {
		t.Fatal(err)
	}
	friday := time.Date(2026, 11, 27, 9, 41, 0, 0, time.Local)
	thursday := time.Date(2026, 12, 3, 22, 7, 0, 0, time.Local)
	model := newDashboard("dark", friday)
	first, second := art.render(model, friday), art.render(model, thursday)
	for name, area := range map[string]image.Rectangle{
		"day": image.Rect(30, 390, 102, 442), "weekday": image.Rect(118, 364, 374, 401), "month": image.Rect(29, 368, 103, 390),
	} {
		same := true
		for y := area.Min.Y; y < area.Max.Y && same; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if first.GrayAt(x, y) != second.GrayAt(x, y) {
					same = false
					break
				}
			}
		}
		if same {
			t.Errorf("%s did not change with the date", name)
		}
	}
}

func TestTodoTapSendsTheTappedTask(t *testing.T) {
	now := time.Now()
	model := newDashboard("dark", now)
	model.connectedUI, model.pcOnline = true, true
	model.todo = &todoList{Items: []todoItem{{Line: 0, Text: "first"}, {Line: 4, Text: "second task"}}, Total: 2}
	model.tap(image.Pt(480, todoTop+todoRowHeight+20), now)
	if model.pending == nil || model.pending.Action != "todo-toggle" || model.pending.Value != 4 || model.pending.Text != "second task" {
		t.Fatalf("unexpected action %+v", model.pending)
	}
	model.pending = nil
	model.tap(image.Pt(480, todoTop+5*todoRowHeight+20), now)
	if model.pending != nil {
		t.Fatalf("tapping an empty row sent %+v", model.pending)
	}
	model.pcOnline = false
	model.tap(image.Pt(480, todoTop+20), now)
	if model.pending != nil {
		t.Fatalf("tapping while offline sent %+v", model.pending)
	}
}

func TestTodoWrapsLongTasks(t *testing.T) {
	art, err := openArtwork(1)
	if err != nil {
		t.Fatal(err)
	}
	lines := art.wrapTwoLines(art.week, "Write the operating systems group project report and send it to everyone", todoTextWidth)
	if len(lines) != 2 || !strings.HasSuffix(lines[1], "...") {
		t.Fatalf("unexpected wrap %q", lines)
	}
	for _, line := range lines {
		if width := art.measure(art.week, line); width > todoTextWidth {
			t.Fatalf("%q is %d px wide", line, width)
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
	art, err := openArtwork(1)
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
	if model.dialogTitle != "YOUTUBE MUSIC" {
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

func TestViewportFitsPaperwhite(t *testing.T) {
	w, h, ok := parseFBInkState("FBInk 1.25 ... viewWidth=1072;viewHeight=1448;screenWidth=1088;screenHeight=1448;")
	if !ok || w != 1072 || h != 1448 {
		t.Fatalf("parsed %d x %d %v", w, h, ok)
	}
	view := newViewport(w, h)
	art, err := openArtwork(view.scale)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	native := art.render(newDashboard("dark", now), now)
	// Drawn at full resolution, not stretched: the frame is already the scaled size.
	if native.Bounds() != image.Rect(0, 0, view.dstW, view.dstH) || view.dstW != 1072 {
		t.Fatalf("frame drawn at %v, want %dx%d", native.Bounds(), view.dstW, view.dstH)
	}
	frame := view.fit(native, dark.paper)
	if frame.Bounds() != image.Rect(0, 0, 1072, 1448) {
		t.Fatalf("frame is %v", frame.Bounds())
	}
	// A tap in the middle of the EXIT button lands on EXIT in dashboard coordinates.
	exitOnScreen := image.Pt(view.offX+int(545*view.scale), view.offY+int(60*view.scale))
	panel := image.Pt(exitOnScreen.X*width/w, exitOnScreen.Y*height/h)
	got := view.fromTouch(panel)
	if got.X < 538 || got.X > 552 || got.Y < 54 || got.Y > 66 {
		t.Fatalf("EXIT tap mapped to %v", got)
	}
	if plain := newViewport(600, 800); !plain.native() || plain.fromTouch(image.Pt(10, 20)) != image.Pt(10, 20) {
		t.Fatal("600x800 screens must be unchanged")
	}
}

func TestStatusBarShowsKindleBattery(t *testing.T) {
	art, err := openArtwork(1)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	model := newDashboard("dark", now)
	sample := art.render(model, now)
	model.kindleBattery = 12
	low := art.render(model, now)
	model.kindleBattery = 95
	high := art.render(model, now)
	differs := func(a, b *image.Gray, area image.Rectangle) bool {
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if a.GrayAt(x, y) != b.GrayAt(x, y) {
					return true
				}
			}
		}
		return false
	}
	if !differs(sample, low, image.Rect(505, 5, 590, 30)) || !differs(low, high, image.Rect(556, 12, 580, 24)) {
		t.Fatal("status bar did not show the real battery level")
	}
}

func TestTodayTextFitsBesideWeather(t *testing.T) {
	// Check at both the original size and a Paperwhite 3's, since font widths round differently.
	for _, screen := range [][2]int{{600, 800}, {1072, 1448}} {
		view := newViewport(screen[0], screen[1])
		art, err := openArtwork(view.scale)
		if err != nil {
			t.Fatal(err)
		}
		c := &canvas{scale: view.scale}
		// The longest weekday and date must stay clear of the weather column (dotted divider at x≈261).
		wednesday := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
		model := newDashboard("dark", wednesday)
		model.connectedUI, model.pcOnline = true, true
		temp, code := 24.0, 2
		model.weather = &weatherInfo{Temp: &temp, Code: &code, IsDay: true, Place: "Bengaluru"}
		frame := art.render(model, wednesday)
		for y := c.px(364); y < c.px(449); y++ {
			for x := c.px(258); x < c.px(260); x++ {
				// Only ink the app drew counts; the artwork's own soft edges are fine.
				if got := frame.GrayAt(x, y); got != dark.panel && got != art.backdrop("dark").At(x, y) {
					t.Fatalf("%dx%d: date text reaches the weather column at (%d,%d)", screen[0], screen[1], x, y)
				}
			}
		}
		for _, label := range []string{"PARTLY CLOUDY", "THUNDERSTORM", "MOSTLY CLEAR"} {
			if got := art.fitText(art.tiny, label, 102); got != label {
				t.Errorf("%dx%d: %q was shortened to %q", screen[0], screen[1], label, got)
			}
		}
	}
}

func TestQuickLaunchButtons(t *testing.T) {
	model := newDashboard("dark", time.Now())
	model.connectedUI, model.pcOnline = true, true
	for x, want := range map[int]string{60: "Cyberpunk", 540: "YouTube Music", 440: "YouTube"} {
		model.pending = nil
		model.tap(image.Pt(x, 750), time.Now())
		if model.pending == nil || model.pending.Name != want {
			t.Errorf("tap at x=%d sent %+v, want %q", x, model.pending, want)
		}
	}
}
