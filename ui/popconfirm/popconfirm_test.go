package popconfirm_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/popconfirm"
)

func TestWarmPaintAllocatesNothing(t *testing.T) {
	pc := popconfirm.New()
	pc.Open()
	var c0 ui.C
	_ = c0
	view := func(c ui.C) {
		trigger := ui.Button(c, "Remove")
		pc.View(c, trigger, popconfirm.Spec{Title: "Remove?", Description: "No undo.", Danger: true})
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 480, Height: 320}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}
