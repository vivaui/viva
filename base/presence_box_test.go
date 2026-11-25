package base_test

import (
	"testing"
	"time"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// An element with a Presence that is no longer painted stays on screen as a
// picture that fades out, and is gone when the exit has run.
func TestBoxPresenceFadesAnElementOutAfterItIsRemoved(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	presence := gfx.Presence{Motion: gfx.Motion{Duration: 100 * time.Millisecond, Easing: gfx.EaseLinear}, Fade: true}
	shown := true
	draw := func(c C) {
		Column(c).Fill().Children(func() {
			if shown {
				Column(c).Size(100, 40).Background(gfx.Color{Red: 1, Alpha: 1}).Hit(gfx.Item(7), "item").Presence(presence)
			}
		})
	}
	bounds := gfx.Rect{Width: 200, Height: 200}
	alpha := func(at time.Duration) (float32, bool) {
		now = at
		canvas.Reset()
		canvas.BeginFrame()
		page.Draw(canvas, bounds, draw)
		canvas.EndFrame()
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color.Red == 1 {
				return rectangle.Color.Alpha, true
			}
		}
		return 0, false
	}
	alpha(0)
	if got, ok := alpha(time.Second); !ok || got != 1 {
		t.Fatalf("shown element alpha = %v painted=%v, want opaque", got, ok)
	}
	shown = false
	if got, ok := alpha(2 * time.Second); !ok || got < 0.99 {
		t.Fatalf("frame after removal = %v painted=%v, want the element still there", got, ok)
	}
	if got, ok := alpha(2*time.Second + 50*time.Millisecond); !ok || got <= 0 || got >= 1 || !canvas.Animating() {
		t.Fatalf("halfway = %v painted=%v animating=%v, want it fading", got, ok, canvas.Animating())
	}
	if _, ok := alpha(3 * time.Second); ok || canvas.Animating() {
		t.Fatalf("after the exit the element is still painted=%v animating=%v", ok, canvas.Animating())
	}
	for _, hit := range canvas.Hits() {
		if hit.ID == gfx.Item(7) {
			t.Fatal("a leaving element declared a hit region")
		}
	}
}

// A row that leaves a list with Presence is drawn fading where it was while the
// rows below it close up.
func TestListRowPresenceKeepsADeletedRowFadingOut(t *testing.T) {
	theme := gfx.LightTheme()
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	presence := gfx.Presence{Motion: gfx.Motion{Duration: 100 * time.Millisecond, Easing: gfx.EaseLinear}, Fade: true}
	var list ListState
	list.Key = func(i int) int { return i + 1 }
	first := 1
	rows := 3
	draw := func(c C) {
		list.Key = func(i int) int { return first + i }
		Column(c).Fill().Children(func() {
			List(c, &list, rows, func(row ListRow) {
				colour := gfx.Color{Red: 0.1 * float32(row.Key), Alpha: 1}
				row.Element.Height(40).Background(colour).Presence(presence)
			}).Grow(1).MinHeight(0)
		})
	}
	bounds := gfx.Rect{Width: 200, Height: 300}
	frame := func(at time.Duration) {
		now = at
		canvas.Reset()
		canvas.BeginFrame()
		page.Draw(canvas, bounds, draw)
		canvas.EndFrame()
	}
	painted := func(key int) (float32, bool) {
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color.Red > 0.1*float32(key)-0.01 && rectangle.Color.Red < 0.1*float32(key)+0.01 {
				return rectangle.Color.Alpha, true
			}
		}
		return 0, false
	}
	frame(0)
	frame(time.Second)
	first, rows = 2, 2 // the first row is deleted
	frame(2 * time.Second)
	if alpha, ok := painted(1); !ok || alpha < 0.99 {
		t.Fatalf("deleted row alpha = %v painted=%v, want still drawn", alpha, ok)
	}
	frame(2*time.Second + 50*time.Millisecond)
	if alpha, ok := painted(1); !ok || alpha <= 0 || alpha >= 1 {
		t.Fatalf("deleted row halfway = %v painted=%v, want fading", alpha, ok)
	}
	frame(3 * time.Second)
	if _, ok := painted(1); ok {
		t.Fatal("the deleted row is still drawn after its exit")
	}
	if _, ok := painted(2); !ok {
		t.Fatal("the remaining row disappeared")
	}
}
