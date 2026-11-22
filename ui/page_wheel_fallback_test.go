package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// RouteWheel is the page's own routing: it scrolls what it owns and leaves the
// OnScroll fallback to Wheel. A host whose own OnScroll forwards the wheel to the
// page calls it, so the page cannot hand the same event back to that handler.
func TestRouteWheelLeavesTheOnScrollFallbackToWheel(t *testing.T) {
	var area ui.Box
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			area = ui.Scroll(c).Height(100).Shrink(0).Children(func() {
				ui.Slot(c).Height(1000).Shrink(0)
			})
			ui.Slot(c).Grow(1)
		})
	}
	page := ui.NewPage(ui.PageConfig{Root: "route-wheel", View: view})
	calls := 0
	ui.Configure(page, app.Config{OnScroll: func(gfx.ScrollEvent) gfx.EventResult {
		calls++
		return gfx.EventResult{Handled: true, Repaint: true}
	}})
	canvas := benchCanvas()
	drawPage(page, canvas, view)
	r := area.Rect()
	over := gfx.ScrollEvent{X: r.X + 10, Y: r.Y + 10, DeltaY: 80, Precise: true}
	if result := page.RouteWheel(over); !result.Handled || !result.Repaint {
		t.Fatalf("RouteWheel over the area gave %+v, want it handled and repainted", result)
	}
	if calls != 0 {
		t.Fatalf("RouteWheel called the OnScroll fallback %d times", calls)
	}
	drawPage(page, canvas, view)
	if area.ScrollState().Offset == 0 {
		t.Fatal("RouteWheel did not move the page's own area")
	}
	outside := gfx.ScrollEvent{X: r.X + 10, Y: r.Y + r.Height + 40, DeltaY: 10, Precise: true}
	if result := page.Wheel(outside); !result.Handled || calls != 1 {
		t.Fatalf("Wheel outside every area gave %+v after %d fallback calls, want the fallback once", result, calls)
	}
	steady := gfx.ScrollEvent{X: r.X + 10, Y: r.Y + 10, DeltaX: 1, Precise: true}
	if allocs := testing.AllocsPerRun(20, func() { drawPage(page, canvas, view); page.RouteWheel(steady) }); allocs != 0 {
		t.Fatalf("a steady paint and RouteWheel allocates %v times", allocs)
	}
}
