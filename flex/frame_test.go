package flex

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// TestBoundsIsWhatTheCallerLaidTheFrameOutIn: custom painting needs the frame's
// own rectangle before End (a child's is known only after).
func TestBoundsIsWhatTheCallerLaidTheFrameOutIn(t *testing.T) {
	var view View
	var canvas ui.Canvas
	want := ui.Rect{X: 10, Y: 20, Width: 300, Height: 200}
	var got ui.Rect
	view.Draw(&canvas, ui.LightTheme(), want, func(f *Frame) {
		got = f.Bounds()
	})
	if got != want {
		t.Fatalf("Bounds() = %+v, want %+v", got, want)
	}
}
