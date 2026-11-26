package transfer_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/transfer"
)

func TestWarmPaintAllocatesNothing(t *testing.T) {
	items := []transfer.Item{{Title: "alpha"}, {Title: "beta", Checked: true}, {Title: "gamma", Target: true}}
	tr := transfer.New()
	view := func(c ui.C) { tr.View(c, "Groups", transfer.Spec{Items: items}).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 640, Height: 420}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	items := make([]transfer.Item, 200)
	for i := range items {
		items[i] = transfer.Item{Title: "host", Target: i%3 == 0, Checked: i%5 == 0}
	}
	tr := transfer.New()
	view := func(c ui.C) { tr.View(c, "Groups", transfer.Spec{Items: items}).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 640, Height: 420}, view)
		canvas.EndFrame()
	}
}
