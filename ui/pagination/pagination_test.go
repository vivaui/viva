package pagination_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/pagination"
)

func TestWarmPaintAllocatesNothing(t *testing.T) {
	state := &pagination.State{PageSize: 10, Page: 5}
	spec := pagination.Spec{Total: 500, ShowTotal: true, SizeOptions: []int{10, 20, 50}, Jumper: true}
	view := func(c ui.C) { pagination.View(c, "Pages", state, spec) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 900, Height: 160}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	state := &pagination.State{PageSize: 10, Page: 5}
	spec := pagination.Spec{Total: 500, ShowTotal: true}
	view := func(c ui.C) { pagination.View(c, "Pages", state, spec) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 900, Height: 160}, view)
		canvas.EndFrame()
	}
}
