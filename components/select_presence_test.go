package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// A single select's list enters and leaves over frames, as a Select's does,
// instead of appearing and vanishing at once.
func TestSingleSelectListAnimates(t *testing.T) {
	var canvas gfx.Canvas
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	state := &SingleSelectState{Open: true}
	state.SetOptions([]SingleSelectOption{{Value: "alpha", Label: "alpha"}, {Value: "beta", Label: "beta"}})
	paint := func() {
		canvas.Reset()
		canvas.BeginFrame()
		PaintSingleSelect(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 10, Width: 160, Height: 32}, state, SingleSelectOptions{
			TriggerID: gfx.Item(1), DismissID: gfx.Item(2), Scope: gfx.Item(3),
			Viewport: gfx.Rect{Width: 400, Height: 400},
		})
		canvas.EndFrame()
	}
	paint()
	if _, animating := canvas.NextFrame(); !animating {
		t.Fatal("an opening list asked for no next frame")
	}
	for range 30 {
		now += 16 * time.Millisecond
		paint()
	}
	if _, animating := canvas.NextFrame(); animating {
		t.Fatal("the list was still animating half a second after it opened")
	}
}
