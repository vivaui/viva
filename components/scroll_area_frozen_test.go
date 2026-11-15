package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A frozen area scrolls its content around the pinned band: the offset's
// maximum keeps the unfrozen value — the offset never moves what is pinned —
// while the translated content moves by the pinned width and the offset
// together, so the caller paints the scrolling columns of a frozen table into
// the window the seam leaves.
func TestFrozenScrollAreaScrollsAroundThePinnedBand(t *testing.T) {
	area := BeginScrollArea(nil, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 200, Height: 100}, gfx.Size{Width: 500, Height: 400}).
		Frozen(60).Offset(120, 0)
	geometry := area.Geometry()
	// 300 is the unfrozen 500-200: the offset reaches exactly what an
	// unfrozen area's would, and the translated content keeps the unfrozen
	// origin, because the caller paints the pinned columns itself over it.
	if geometry.MaxOffsetX != 300 {
		t.Fatalf("MaxOffsetX = %v, want the unfrozen 300", geometry.MaxOffsetX)
	}
	if geometry.Content.X != 10-120 {
		t.Fatalf("the content starts at %v, want the unfrozen origin 10 - the offset 120", geometry.Content.X)
	}
	// The horizontal bar belongs to the scrolling window: its track starts at
	// the pinned band's far edge — less the corner a vertical bar takes — and
	// its thumb spans exactly what scrolls.
	if geometry.HorizontalTrack.X != 10+60 || geometry.HorizontalTrack.Width != 190-60 {
		t.Fatalf("the horizontal track is %+v, want it past the pinned 60 px of the 190 the corner leaves", geometry.HorizontalTrack)
	}
	scrolled := geometry.HorizontalTrack.Width
	if thumb := geometry.HorizontalThumb; thumb.X < geometry.HorizontalTrack.X+1 || thumb.X+thumb.Width > geometry.HorizontalTrack.X+scrolled {
		t.Fatalf("the horizontal thumb is %+v, want it inside the scrolling window of %v px alone", thumb, scrolled)
	}
	// The offset reaches every scrolled pixel of content while never moving
	// the pinned ones: a drag to the thumb's far end is the whole content's.
	area = area.Offset(geometry.MaxOffsetX, 0)
	end := area.Geometry()
	if end.Content.X+end.Content.Width != 10+200 {
		t.Fatalf("at the end the content reaches %v, want the viewport's far edge", end.Content.X+end.Content.Width)
	}
}

// The drag mapping stays the inverse of the thumb: a press at some offset maps
// back to the same offset, and moving the thumb to the end of the scrolling
// window scrolls to the end of the content.
func TestFrozenScrollAreaDragMapsWhatTheThumbSpans(t *testing.T) {
	area := BeginScrollArea(nil, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 200, Height: 100}, gfx.Size{Width: 500, Height: 400}).
		Frozen(60).HorizontalScrollbar(gfx.Item(1), "Columns", "")
	geometry := area.Geometry()
	if got := geometry.HorizontalOffsetForThumb(geometry.HorizontalThumb.X); got != 0 {
		t.Fatalf("the resting thumb maps back to %v, want 0", got)
	}
	thumbAt := func(offset float32) float32 {
		return area.Offset(offset, 0).Geometry().HorizontalThumb.X
	}
	for _, offset := range []float32{60, 150, 300} {
		if geometry := area.Offset(offset, 0).Geometry(); geometry.OffsetX != offset {
			t.Fatalf("offset %v clamped to %v", offset, geometry.OffsetX)
		}
		if back := geometry.HorizontalOffsetForThumb(thumbAt(offset)); back-offset > 0.01 || offset-back > 0.01 {
			t.Fatalf("thumb for %v maps back to %v", offset, back)
		}
	}
}

// A frozen area without a horizontal overflow, with nothing pinned, or with
// the whole viewport pinned, is the unfrozen area it always was: same
// geometry, byte for byte.
func TestFrozenScrollAreaWithoutOverflowOrBandKeepsUnfrozenGeometry(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 20, Width: 200, Height: 100}
	cases := []struct {
		name    string
		frozen  float32
		content gfx.Size
	}{
		{"nothing pinned", 0, gfx.Size{Width: 500, Height: 400}},
		{"no overflow", 60, gfx.Size{Width: 200, Height: 400}},
		{"overflow shorter than the band", 300, gfx.Size{Width: 240, Height: 400}},
		{"whole viewport pinned", 200, gfx.Size{Width: 500, Height: 400}},
	}
	for _, c := range cases {
		plain := BeginScrollArea(nil, theme, bounds, c.content).Offset(120, 40)
		frozen := BeginScrollArea(nil, theme, bounds, c.content).Frozen(c.frozen).Offset(120, 40)
		if got, want := frozen.Geometry(), plain.Geometry(); got != want {
			t.Fatalf("%s: frozen geometry is %+v, want the unfrozen %+v", c.name, got, want)
		}
	}
}
