package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// A field bound to a multi-selection editor paints every selection on its one
// line, not the primary's band stretched over the gap between them.
func TestInputPaintsEverySelectionOnOneLine(t *testing.T) {
	theme := gfx.LightTheme()
	theme.Selection = gfx.RGB(1, 2, 3)
	var canvas gfx.Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len(value)) * size / 2
	})
	bounds := gfx.Rect{X: 10, Y: 4, Width: 400, Height: 32}
	PaintInputContent(&canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "Name", Value: "abcdef", Focused: true, ShowCaret: true, Caret: 5,
		SelectionRanges: []text.Span{{Start: 0, End: 1}, {Start: 3, End: 5}},
	})
	var fills []gfx.Rect
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == theme.Selection {
			fills = append(fills, rectangle.Bounds)
		}
	}
	if len(fills) != 2 {
		t.Fatalf("selection fills = %+v", fills)
	}
	charWidth := float32(14) / 2
	origin := fills[0].X
	if !near(fills[0].Width, charWidth) {
		t.Fatalf("first fill = %+v", fills[0])
	}
	if !near(fills[1].X, origin+3*charWidth) || !near(fills[1].Width, 2*charWidth) {
		t.Fatalf("second fill = %+v, want x=%v width=%v", fills[1], origin+3*charWidth, 2*charWidth)
	}
}

// SelectionRanges replaces SelectionStart/End for painting, and an empty one
// keeps the single-selection path exactly as it was.
func TestInputSelectionRangesReplaceTheSingleSelection(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(snapshot InputSnapshot) []gfx.Rect {
		var canvas gfx.Canvas
		canvas.SetMeasurer(func(value string, size, weight float32) float32 {
			return float32(len(value)) * size / 2
		})
		bounds := gfx.Rect{X: 10, Y: 4, Width: 400, Height: 32}
		snapshot.ID, snapshot.Label = gfx.Item(1), "Name"
		PaintInputContent(&canvas, theme, bounds, bounds, snapshot)
		var fills []gfx.Rect
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Selection {
				fills = append(fills, rectangle.Bounds)
			}
		}
		return fills
	}
	// The wide single selection is one band; the two ranges are two.
	wide := paint(InputSnapshot{Value: "abcdef", SelectionStart: 0, SelectionEnd: 5})
	narrow := paint(InputSnapshot{Value: "abcdef", SelectionStart: 0, SelectionEnd: 1,
		SelectionRanges: []text.Span{{Start: 0, End: 1}, {Start: 3, End: 5}}})
	if len(wide) != 1 || len(narrow) != 2 {
		t.Fatalf("single = %+v, ranges = %+v", wide, narrow)
	}
	if !near(narrow[0].X, wide[0].X) || !near(narrow[0].Width, wide[0].Width/5) {
		t.Fatalf("the first range is not the first character of the single selection: %+v vs %+v", narrow[0], wide[0])
	}
	// An empty ranges slice is the single selection again.
	single := paint(InputSnapshot{Value: "abcdef", SelectionStart: 0, SelectionEnd: 5})
	if len(single) != 1 || single[0] != wide[0] {
		t.Fatalf("empty ranges changed the paint: %+v", single)
	}
}
