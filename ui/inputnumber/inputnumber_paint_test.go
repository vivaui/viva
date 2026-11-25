package inputnumber_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputnumber"
)

func TestWarmPaintAllocatesNothing(t *testing.T) {
	value := 12.5
	n := inputnumber.New().Step(0.5).Min(0).Max(100)
	view := func(c ui.C) { n.View(c, "Port", &value).Width(200) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 360, Height: 200}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	value := 12.5
	n := inputnumber.New().Step(0.5).Min(0).Max(100)
	view := func(c ui.C) { n.View(c, "Port", &value).Width(200) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 360, Height: 200}, view)
		canvas.EndFrame()
	}
}
