//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func TestNativeFullscreenWiringRepaintsAWindowWithoutApplicationPaint(t *testing.T) {
	fullscreen := false
	var events []string
	normalized, err := (Config{Title: "fullscreen", Width: 600, Height: 400,
		WindowMode: WindowCustomTitlebar,
		OnFullscreen: func(value bool) {
			fullscreen = value
			events = append(events, "fullscreen")
		},
		OnResize: func(ui.Size) {
			if !fullscreen {
				t.Fatal("native resize was delivered without the fullscreen callback")
			}
			events = append(events, "resize")
		},
	}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainter(&ui.Canvas{}, &normalized)
	if err != nil {
		t.Fatal(err)
	}
	native, frames := nativeWindowConfig(&normalized, painter)
	if frames != nil {
		t.Fatal("a window without application paint started an animation timer")
	}
	if native.OnFullscreen == nil || native.OnResize == nil || native.OnRedraw == nil {
		t.Fatal("a window without application paint did not wire fullscreen, resize and redraw")
	}
	native.OnFullscreen(true)
	if _, err := native.OnResize(platform.Size{Width: 1920, Height: 1080, Scale: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := native.OnRedraw(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0] != "fullscreen" || events[1] != "resize" {
		t.Fatalf("native callback events = %v", events)
	}
}

func TestDriverFullscreenRestoresPlacementAndOrdersCallbacks(t *testing.T) {
	origin := ui.Point{X: 120, Y: 80}
	var driver *Driver
	var events []string
	var observed bool
	config := Config{Title: "fullscreen", Width: 600, Height: 400, Origin: &origin,
		WindowMode: WindowCustomTitlebar,
		OnFullscreen: func(fullscreen bool) {
			observed = fullscreen
			if driver.Fullscreen() != fullscreen {
				t.Fatal("state was not updated before callback")
			}
			events = append(events, "fullscreen")
		},
		OnResize: func(ui.Size) {
			if driver != nil {
				if driver.Fullscreen() != observed {
					t.Fatal("resize ran before fullscreen state callback")
				}
				events = append(events, "resize")
			}
		},
		OnFrameChange: func(ui.Rect) {
			if driver.Fullscreen() {
				t.Fatal("fullscreen geometry was reported as a restorable placement")
			}
			events = append(events, "frame")
		},
	}
	var err error
	driver, err = NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	original := driver.Frame()
	monitor := ui.Rect{X: -1920, Width: 1920, Height: 1080}
	if err := driver.SetFullscreenMonitor(monitor); err != nil {
		t.Fatal(err)
	}
	if err := driver.SetMaximized(true); err != nil {
		t.Fatal(err)
	}
	events = nil
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if driver.Frame() != monitor || !driver.Maximized() {
		t.Fatalf("fullscreen frame/state = %+v, maximized %v", driver.Frame(), driver.Maximized())
	}
	if len(events) != 2 || events[0] != "fullscreen" || events[1] != "resize" {
		t.Fatalf("entry events = %v", events)
	}
	before := len(events)
	if err := driver.SetFullscreen(true); err != nil || len(events) != before {
		t.Fatalf("repeated request produced events: %v, %v", err, events)
	}
	if err := driver.Resize(1280, 720); err != nil {
		t.Fatal(err)
	}
	if err := driver.SetFullscreenMonitor(ui.Rect{X: 1920, Width: 1280, Height: 720}); err != nil {
		t.Fatal(err)
	}
	monitor.X = 1920
	if err := driver.MoveWindow(500, 500); err != nil {
		t.Fatal(err)
	}
	if driver.Frame().X != monitor.X {
		t.Fatal("fullscreen window was draggable")
	}
	events = nil
	if err := driver.SetFullscreen(false); err != nil {
		t.Fatal(err)
	}
	if driver.Frame() != original || !driver.Maximized() {
		t.Fatalf("restore frame/state = %+v, want %+v and maximized", driver.Frame(), original)
	}
	if len(events) != 3 || events[0] != "fullscreen" || events[1] != "resize" || events[2] != "frame" {
		t.Fatalf("exit events = %v", events)
	}
}

func TestFullscreenRemovesFrameworkButtonsAndMoveSemantics(t *testing.T) {
	driver, err := NewDriver(Config{Title: "fullscreen", Width: 600, Height: 400,
		WindowMode: WindowCustomTitlebar,
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			canvas.MoveRegion(ui.Rect{Width: size.Width, Height: 40})
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if driver.PressAt(100, 20) != PressMove {
		t.Fatal("normal titlebar was not draggable")
	}
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if driver.PressAt(100, 20) == PressMove || driver.PressAt(100, 20) == PressMoveOnDrag {
		t.Fatal("fullscreen titlebar retained move semantics")
	}
	if controlsDrawnByViva {
		if _, ok := driver.HitTest(driver.Frame().Width-20, 20); ok {
			t.Fatal("framework caption button remains interactive in fullscreen")
		}
	}
	if err := driver.SetFullscreen(false); err != nil {
		t.Fatal(err)
	}
	if driver.PressAt(100, 20) != PressMove {
		t.Fatal("normal titlebar did not regain move semantics")
	}
	if controlsDrawnByViva {
		if _, ok := driver.HitTest(driver.Frame().Width-20, 20); !ok {
			t.Fatal("framework caption controls did not return after fullscreen")
		}
	}
}

func BenchmarkDriverFullscreenTransition(b *testing.B) {
	driver, err := NewDriver(Config{Title: "fullscreen", Width: 600, Height: 400}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	for b.Loop() {
		if err := driver.SetFullscreen(true); err != nil {
			b.Fatal(err)
		}
		if err := driver.SetFullscreen(false); err != nil {
			b.Fatal(err)
		}
	}
}

func TestFullscreenCallbackCanRequestExitWithoutCorruptingRestore(t *testing.T) {
	var driver *Driver
	var states []bool
	var err error
	driver, err = NewDriver(Config{Title: "fullscreen", Width: 600, Height: 400,
		OnFullscreen: func(fullscreen bool) {
			states = append(states, fullscreen)
			if fullscreen {
				if err := driver.SetFullscreen(false); err != nil {
					t.Fatal(err)
				}
			}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	original := driver.Frame()
	if err := driver.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if driver.Fullscreen() || driver.Frame() != original || len(states) != 2 || !states[0] || states[1] {
		t.Fatalf("reentrant exit left invalid state/frame: %v, %+v, %v", driver.Fullscreen(), driver.Frame(), states)
	}
}
