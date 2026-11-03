package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestFullscreenIsDistinctFromMaximizeAndSuppressesFrameInput(t *testing.T) {
	var states []bool
	win := &window{frame: frameState{fullscreen: 1, maximizedVertical: 2, maximizedHorizontal: 3}}
	win.fullscreenHandler = func(fullscreen bool) {
		if win.fullscreen.Load() != fullscreen {
			t.Fatal("state callback saw stale fullscreen state")
		}
		states = append(states, fullscreen)
	}
	if win.observeWindowState([]uint32{1}) {
		t.Fatal("fullscreen was reported as maximized")
	}
	if !win.fullscreen.Load() {
		t.Fatal("fullscreen atom did not update native state")
	}
	win.frame.pending = true
	win.observeWindowState([]uint32{1})
	if len(states) != 1 {
		t.Fatal("unchanged native property produced another callback")
	}
	config := platform.Config{WindowMode: platform.WindowCustomTitlebar,
		PressAt: func(float32, float32) platform.PressKind { return platform.PressMove },
	}
	if win.beginWindowMove(config, &xcbInputEvent{Detail: xcbButtonLeft}) {
		t.Fatal("fullscreen edge/titlebar press was consumed as a frame operation")
	}
	if !win.observeWindowState([]uint32{2, 3}) || win.fullscreen.Load() {
		t.Fatal("leaving fullscreen lost the independent maximized state")
	}
	if win.frame.pending || len(states) != 2 || states[1] {
		t.Fatalf("exit retained a pending drag or stale callbacks: %v", states)
	}
}

func BenchmarkObservedFullscreenState(b *testing.B) {
	win := &window{frame: frameState{fullscreen: 1, maximizedVertical: 2, maximizedHorizontal: 3}}
	atoms := []uint32{1}
	b.ReportAllocs()
	for b.Loop() {
		win.observeWindowState(atoms)
	}
}
