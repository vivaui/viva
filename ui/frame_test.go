package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestBoundsIsWhatTheCallerLaidTheFrameOutIn: custom painting needs the frame's
// own rectangle before End (a child's is known only after).
func TestBoundsIsWhatTheCallerLaidTheFrameOutIn(t *testing.T) {
	var view View
	var canvas gfx.Canvas
	want := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 200}
	var got gfx.Rect
	view.Draw(&canvas, gfx.LightTheme(), want, func(f *Frame) {
		got = f.Bounds()
	})
	if got != want {
		t.Fatalf("Bounds() = %+v, want %+v", got, want)
	}
}
