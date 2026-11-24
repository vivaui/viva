package base_test

import (
	"testing"
	"time"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// Taking a row out of a list slides the rows below it up instead of letting them
// jump, and a row's place is measured without the scroll.
func TestListGlideSlidesRowsAfterOneIsRemoved(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	var list ListState
	list.Key = func(i int) int { return i + 1 }
	list.Glide = gfx.Motion{Duration: 100 * time.Millisecond, Easing: gfx.EaseLinear}
	rows := 3
	var second float32
	var secondID gfx.ID
	draw := func(c C) {
		Column(c).Fill().Children(func() {
			List(c, &list, rows, func(row ListRow) {
				row.Element.Height(40).Background(gfx.Color{Red: 1, Alpha: 1})
				if row.Key == 2 {
					secondID = row.ID
				}
			}).Grow(1).MinHeight(0)
		})
	}
	bounds := gfx.Rect{Width: 200, Height: 300}
	frame := func(at time.Duration) float32 {
		now = at
		canvas.Reset()
		canvas.BeginFrame()
		page.Draw(canvas, bounds, draw)
		canvas.EndFrame()
		second = 0
		for _, hit := range canvas.Hits() {
			if hit.ID == secondID {
				second = hit.Bounds.Y
			}
		}
		return second
	}
	frame(0)
	start := frame(10 * time.Millisecond)
	if start != 40 {
		t.Fatalf("second row at %v, want 40", start)
	}
	// Remove the first row: the old second row is now the first and the layout
	// puts it at 0, but the eye still sees it at 40.
	rows = 2
	list.Key = func(i int) int { return i + 2 }
	if got := frame(time.Second); got != 40 {
		t.Fatalf("frame after removal = %v, want the old place 40", got)
	}
	if got := frame(time.Second + 50*time.Millisecond); got <= 0 || got >= 40 || !canvas.Animating() {
		t.Fatalf("halfway = %v animating=%v, want between 0 and 40", got, canvas.Animating())
	}
	if got := frame(2 * time.Second); got != 0 || canvas.Animating() {
		t.Fatalf("arrived = %v animating=%v", got, canvas.Animating())
	}
}
