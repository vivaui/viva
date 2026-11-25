package timepicker_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/timepicker"
)

func at(hour, minute, second int) timepicker.Value {
	return timepicker.Value{Hour: hour, Minute: minute, Second: second, Valid: true}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	value := at(9, 30, 0)
	p := timepicker.New().Seconds()
	view := func(c ui.C) { p.View(c, "Start", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 420}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	value := at(9, 30, 0)
	p := timepicker.New().Seconds()
	view := func(c ui.C) { p.View(c, "Start", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 420}, view)
		canvas.EndFrame()
	}
}
