package base_test

import (
	"testing"
	"time"

	. "github.com/vivaui/viva/base"

	"github.com/vivaui/viva/gfx"
)

// A row of keyed boxes: when the first is taken out, the second slides to the
// place the layout gives it instead of jumping there.
func TestGlideSlidesAnElementTheLayoutMoved(t *testing.T) {
	var canvas gfx.Canvas
	var view View
	bounds := gfx.Rect{Width: 300, Height: 40}
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	motion := gfx.Motion{Duration: 100 * time.Millisecond, Easing: gfx.EaseLinear}
	green := gfx.Color{Green: 1, Alpha: 1}
	frame := func(at time.Duration, first bool) float32 {
		now = at
		canvas.Reset()
		canvas.BeginFrame()
		view.Draw(&canvas, gfx.LightTheme(), bounds, func(f *Frame) {
			f.Row().Children(func() {
				if first {
					f.Column().Size(50, 20).Background(gfx.Color{Red: 1, Alpha: 1}).Glide(motion, gfx.Point{}).Hit(gfx.Item(1), "a")
				}
				f.Column().Size(50, 20).Background(green).Glide(motion, gfx.Point{}).Hit(gfx.Item(2), "b")
			})
		})
		canvas.EndFrame()
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == green {
				return rectangle.Bounds.X
			}
		}
		t.Fatal("the second box was not painted")
		return 0
	}
	start := frame(0, true)
	if start <= 0 {
		t.Fatalf("second box at %v, want it beside the first", start)
	}
	if got := frame(time.Second, false); got != start {
		t.Fatalf("frame that closes the first = %v, want the old place %v", got, start)
	}
	if got := frame(time.Second+50*time.Millisecond, false); got <= 0 || got >= start || !canvas.Animating() {
		t.Fatalf("halfway = %v animating=%v, want between 0 and %v", got, canvas.Animating(), start)
	}
	if got := frame(2*time.Second, false); got != 0 || canvas.Animating() {
		t.Fatalf("arrived = %v animating=%v", got, canvas.Animating())
	}
}
