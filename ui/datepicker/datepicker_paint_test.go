package datepicker_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/datepicker"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.Local) }

var today = func() time.Time { return time.Date(2026, 11, 25, 15, 30, 0, 0, time.Local) }

func TestWarmPaintAllocatesNothing(t *testing.T) {
	value := date(2026, 11, 18)
	d := datepicker.New().Clock(today)
	view := func(c ui.C) { d.View(c, "Due", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 460}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	value := date(2026, 11, 18)
	d := datepicker.New().Clock(today)
	view := func(c ui.C) { d.View(c, "Due", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 460}, view)
		canvas.EndFrame()
	}
}
