package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// A margin below a row is room the row takes between itself and the next one, so
// the content a list scrolls to must hold it: measured by its rectangle alone, the
// wheel stops short of the end and the last rows never come into view.
func TestListReachesItsLastRowWhenEveryRowSetsAMargin(t *testing.T) {
	const rows, rowHeight, rowGap, window = 17, 44, 4, 602
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	var list ListState
	var last Box
	draw := func(c C) {
		last = Box{}
		Column(c).Fill().Children(func() {
			List(c, &list, rows, func(row ListRow) {
				box := row.Element.Height(rowHeight).MarginBottom(rowGap)
				if row.Index == rows-1 {
					last = box
				}
			}).Grow(1).MinHeight(0)
		})
	}
	bounds := gfx.Rect{Width: 300, Height: window}
	page.Draw(canvas, bounds, draw)
	// The wheel is how a reader reaches the end: 60 rows' worth is more than the
	// content under a correct height, so the last paint shows the true end.
	for range 60 {
		page.Draw(canvas, bounds, draw)
		page.Wheel(gfx.ScrollEvent{X: 150, Y: window / 2, DeltaY: 60, Precise: true})
	}
	page.Draw(canvas, bounds, draw)
	rect := last.Rect()
	if rect.Height == 0 {
		t.Fatalf("the last of %d rows was never placed", rows)
	}
	if rect.Y+rect.Height > window {
		t.Fatalf("the wheel left the last row at %v, past the bottom of the list, %d", rect, window)
	}
	if !list.AtEnd() {
		t.Fatalf("the list says it is not at its end while showing the last row")
	}
}
