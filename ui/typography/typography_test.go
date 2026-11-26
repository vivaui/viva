package typography_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/typography"
)

var long = strings.Repeat("Keeps scene nodes and repaints only when the content changes. ", 8)

func TestWarmPaintAllocatesNothing(t *testing.T) {
	tx := typography.New()
	spec := typography.Spec{Rows: 2, Expandable: true, Copyable: true}
	view := func(c ui.C) { tx.View(c, long, spec).FillWidth() }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 360}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}
