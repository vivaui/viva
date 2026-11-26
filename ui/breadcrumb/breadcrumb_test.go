package breadcrumb_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/breadcrumb"
)

var places = []breadcrumb.Item{
	{Label: "Home"}, {Label: "Hosts"}, {Label: "Production"}, {Label: "build-01"}, {Label: "Logs"},
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]breadcrumb.Spec{
		"all":       {Items: places},
		"collapsed": {Items: places, Max: 3, Separator: ">"},
	} {
		t.Run(name, func(t *testing.T) {
			b := breadcrumb.New()
			view := func(c ui.C) { b.View(c, "Location", spec) }
			theme := gfx.LightTheme()
			page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
			canvas := &gfx.Canvas{}
			canvas.SetReduceMotion(true)
			draw := func() {
				canvas.BeginFrame()
				canvas.Reset()
				page.Draw(canvas, gfx.Rect{Width: 640, Height: 200}, view)
				canvas.EndFrame()
			}
			draw()
			if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
				t.Fatalf("warm paint allocated %v times, want 0", allocations)
			}
		})
	}
}

func BenchmarkViewPaint(b *testing.B) {
	path := breadcrumb.New()
	view := func(c ui.C) { path.View(c, "Location", breadcrumb.Spec{Items: places, Max: 4}) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 640, Height: 200}, view)
		canvas.EndFrame()
	}
}
