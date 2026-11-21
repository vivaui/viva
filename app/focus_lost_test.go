//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestFocusLostIsReportedAfterKeysAreReleased(t *testing.T) {
	var keys []gfx.KeyEvent
	lost := 0
	heldWhenLost := -1
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult { keys = append(keys, event); return gfx.EventResult{} },
		OnFocusLost: func() gfx.EventResult {
			lost++
			heldWhenLost = len(keys)
			return gfx.EventResult{Repaint: true}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.LoseFocus(); err != nil {
		t.Fatal(err)
	}
	if lost != 1 {
		t.Fatalf("OnFocusLost called %d times, want 1", lost)
	}
	if heldWhenLost != len(keys) {
		t.Fatalf("OnFocusLost ran before the held keys were released: %d of %d key events", heldWhenLost, len(keys))
	}
}
