package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A caller can match the bars to its own chrome: the track width, the inset and the
// least thumb come from the requested style, and the thumb keeps the track less both
// insets.
func TestScrollAreaBarsStyleChangesTracksAndThumbs(t *testing.T) {
	var canvas gfx.Canvas
	area := BeginScrollArea(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, gfx.Size{Width: 200, Height: 1000}).
		Bars(ScrollBarStyle{Size: 11, Inset: 2, MinThumb: 20})
	geometry := area.Geometry()
	if geometry.VerticalTrack.Width != 11 {
		t.Fatalf("track width = %v, want 11", geometry.VerticalTrack.Width)
	}
	if geometry.Inset != 2 {
		t.Fatalf("inset = %v, want 2", geometry.Inset)
	}
	if got := geometry.VerticalThumb.Width; got != 11-2*2 {
		t.Fatalf("thumb width = %v, want the track less both insets", got)
	}
	// The drag inversion uses the same inset the area drew with.
	if top := geometry.VerticalOffsetForThumb(geometry.VerticalTrack.Y + 2); top != 0 {
		t.Fatalf("offset at the track's start = %v, want 0", top)
	}
}

// A scroll area that sets no style keeps Viva's own bar geometry.
func TestScrollAreaBarsDefaultToTheSharedGeometry(t *testing.T) {
	var canvas gfx.Canvas
	area := BeginScrollArea(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, gfx.Size{Width: 200, Height: 1000})
	geometry := area.Geometry()
	if geometry.VerticalTrack.Width != scrollAreaBarSize {
		t.Fatalf("track width = %v, want %v", geometry.VerticalTrack.Width, scrollAreaBarSize)
	}
	if geometry.Inset != scrollAreaBarInset {
		t.Fatalf("inset = %v, want %v", geometry.Inset, scrollAreaBarInset)
	}
}
