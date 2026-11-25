package divider_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/divider"
)

func measure(value string, size, weight float32) float32 { return float32(len(value)) * size * 0.55 }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 100}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	canvas.SetReduceMotion(true)
	draw(page, canvas, view)
	return page, canvas
}

// The label sits in the middle, or near the start or the end, with line on both
// sides of it in the middle and a short run of line before it at an end.
func TestLabelAlignment(t *testing.T) {
	at := func(align divider.Align) float32 {
		view := func(c ui.C) {
			ui.Column(c).Pad(0).Children(func() { divider.View(c, "or", divider.Spec{Align: align}) })
		}
		_, canvas := open(t, view)
		for _, text := range canvas.Texts() {
			if text.Value == "or" {
				return text.Bounds.X + text.Bounds.Width/2
			}
		}
		t.Fatal("the label was not painted")
		return 0
	}
	center, start, end := at(divider.Center), at(divider.Start), at(divider.End)
	if center < 190 || center > 210 {
		t.Fatalf("centered label at %v in a 400 wide row, want about 200", center)
	}
	if !(start < center && center < end) {
		t.Fatalf("label positions start %v, center %v, end %v are not in order", start, center, end)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	view := func(c ui.C) { divider.View(c, "or", divider.Spec{Align: divider.Start}) }
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}
