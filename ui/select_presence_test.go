package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// An open Select inside an enclosing presence (a dialog, drawer or popover)
// closes only its own popup: what is painted after the select is still the
// enclosing surface's, not the popup's entrance.
func TestOpenSelectInsideAPresenceLeavesTheRestOfThePageAlone(t *testing.T) {
	var now time.Duration
	canvas := &gfx.Canvas{}
	canvas.SetClock(func() time.Duration { return now })
	canvas.SetMeasurer(func(value string, size, _ float32) float32 { return float32(len(value)) * size / 2 })
	theme := gfx.LightTheme()
	choice := 0
	choices := []int{0, 1, 2}
	label := func(index int) string { return [...]string{"A", "B", "C"}[index] }
	view := func(c ui.C) { ui.Select(c, &choice, choices, label).Size(120, 32) }
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	bounds := gfx.Rect{Width: 320, Height: 240}
	outer, presence := gfx.Item(1), gfx.Presence{Motion: gfx.Motion{Duration: 100 * time.Millisecond}, Fade: true}
	sentinel := gfx.Rect{X: 280, Y: 200, Width: 10, Height: 10}

	canvas.BeginFrame()
	canvas.BeginPresence(outer, 0, presence, gfx.Point{})
	page.Draw(canvas, bounds, view)
	canvas.EndPresence()
	canvas.EndFrame()
	var trigger gfx.ID
	for _, hit := range canvas.Hits() {
		if hit.Role == gfx.RoleCombobox {
			trigger = hit.ID
		}
	}
	if trigger.IsZero() {
		t.Fatal("the select painted no trigger")
	}

	// The enclosing presence has settled; the select opens in an event pass.
	now = time.Second
	canvas.Reset()
	canvas.BeginEventPass(trigger, gfx.ClickInfo{})
	page.Draw(canvas, bounds, view)
	canvas.EndEventPass()

	canvas.Reset()
	canvas.BeginFrame()
	canvas.BeginPresence(outer, 0, presence, gfx.Point{})
	page.Draw(canvas, bounds, view)
	canvas.Fill(sentinel, gfx.Color{Red: 1, Alpha: 1})
	canvas.EndPresence()
	canvas.EndFrame()
	rects := canvas.Rectangles()
	if len(rects) == 0 {
		t.Fatal("nothing was painted")
	}
	if last := rects[len(rects)-1]; last.Bounds != sentinel || last.Color.Alpha != 1 {
		t.Fatalf("the rectangle painted after the open select = %+v, want the settled sentinel at full alpha", last)
	}
}
