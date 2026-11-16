package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// A textarea paints every selection of a multi-selection editor: ranges on
// different rows each get their band, and two on one row do not join.
func TestTextareaPaintsEverySelection(t *testing.T) {
	theme := gfx.LightTheme()
	theme.Selection = gfx.RGB(1, 2, 3)
	paint := func(snapshot TextareaSnapshot) []gfx.Rect {
		var canvas gfx.Canvas
		bounds := gfx.Rect{Width: 300, Height: 120}
		PaintTextarea(&canvas, theme, bounds, snapshot)
		var fills []gfx.Rect
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Selection {
				fills = append(fills, rectangle.Bounds)
			}
		}
		return fills
	}
	snapshot := TextareaSnapshot{Value: "abc\ndef\nghi", SelectionStart: 0, SelectionEnd: 1,
		SelectionRanges: []text.Span{{Start: 0, End: 1}, {Start: 4, End: 5}, {Start: 8, End: 9}}}
	fills := paint(snapshot)
	if len(fills) != 3 {
		t.Fatalf("selection fills = %+v, want one per row", fills)
	}
	if fills[0].Y >= fills[1].Y || fills[1].Y >= fills[2].Y {
		t.Fatalf("fills are not one per row: %+v", fills)
	}
	// Two ranges on the first row stay two bands.
	fills = paint(TextareaSnapshot{Value: "abcdef", SelectionRanges: []text.Span{{Start: 0, End: 1}, {Start: 3, End: 5}}})
	if len(fills) != 2 || fills[1].X <= fills[0].X+fills[0].Width {
		t.Fatalf("one-row fills = %+v, want two separate bands", fills)
	}
	if fills[1].Width <= fills[0].Width {
		t.Fatalf("the two-character band is not wider than the one-character one: %+v", fills)
	}
	// The single-selection path is untouched by an empty SelectionRanges.
	single := paint(TextareaSnapshot{Value: "abcdef", SelectionStart: 0, SelectionEnd: 5})
	if len(single) != 1 {
		t.Fatalf("single selection = %+v", single)
	}
}
