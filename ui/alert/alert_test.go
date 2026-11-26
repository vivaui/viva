package alert_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/alert"
)

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]alert.Spec{
		"plain":  {Title: "Saved", Description: "All changes were written.", Status: ui.StatusSuccess},
		"full":   {Title: "Update", Description: "A new version is ready.", Action: "Review", Closable: true, Status: ui.StatusWarning},
		"banner": {Title: "Offline", Banner: true, Status: ui.StatusDanger},
	} {
		t.Run(name, func(t *testing.T) {
			a := alert.New()
			view := func(c ui.C) { a.View(c, spec).FillWidth() }
			theme := gfx.LightTheme()
			page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
			canvas := &gfx.Canvas{}
			canvas.SetReduceMotion(true)
			draw := func() {
				canvas.BeginFrame()
				canvas.Reset()
				page.Draw(canvas, gfx.Rect{Width: 480, Height: 240}, view)
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
	a := alert.New()
	spec := alert.Spec{Title: "Update", Description: "A new version is ready.", Action: "Review", Closable: true, Status: ui.StatusWarning}
	view := func(c ui.C) { a.View(c, spec).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 480, Height: 240}, view)
		canvas.EndFrame()
	}
}
