package columnresizeview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/columnresizeview"
)

func testTheme() *gfx.Theme {
	theme := gfx.LightTheme()
	return &theme
}

func drawPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 640, Height: 240}, view)
	canvas.EndFrame()
}

func TestViewPaintsCellsAndDividers(t *testing.T) {
	first, second := gfx.ID{}, gfx.ID{}
	var columns columnresizeview.Columns
	var painted int
	view := func(c ui.C) {
		first, second = ui.ID(c), ui.ID(c)
		columns = columnresizeview.Columns{
			Widths: []float32{260, 90, 140}, Minimums: []float32{120, 54, 76},
			Fits: []float32{180, 64, 100}, Gap: 8,
			DividerIDs: []gfx.ID{first, second}, DividerLabels: []string{"Resize name", "Resize size"},
			Cell: func(d *ui.DrawContext, index int, bounds gfx.Rect) {
				painted++
				d.Canvas.Fill(bounds, d.Theme.Muted)
			},
		}
		columnresizeview.View(c, "Files", &columns).FillWidth().Height(180)
	}
	page := ui.NewPage(ui.PageConfig{View: view, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	drawPage(page, canvas, view)
	if painted != 3 {
		t.Fatalf("painted %d cells, want 3", painted)
	}
	if len(columns.Columns()) != 3 {
		t.Fatalf("columns = %d, want 3", len(columns.Columns()))
	}
	var found bool
	for _, hit := range canvas.Hits() {
		if hit.ID == first && hit.Label == "Resize name" {
			found = true
		}
	}
	if !found {
		t.Fatal("the first divider declared no hit")
	}
	left, right, ok := columns.Edit(0, columns.Columns()[0].X+220)
	if !ok || left+right != 350 || left < 120 || right < 54 {
		t.Fatalf("edit = %g/%g ok=%v, want a clamped pair summing to 350", left, right, ok)
	}
	if l, r, ok := columns.Fit(1); !ok || l+r != 230 {
		t.Fatalf("fit = %g/%g ok=%v, want a pair summing to 230", l, r, ok)
	}
}

func BenchmarkViewWarmPaint(b *testing.B) {
	widths := []float32{260, 90, 140}
	minimums := []float32{120, 54, 76}
	cell := func(*ui.DrawContext, int, gfx.Rect) {}
	var columns columnresizeview.Columns
	view := func(c ui.C) {
		columns.Widths, columns.Minimums, columns.Gap, columns.Cell = widths, minimums, 8, cell
		columnresizeview.View(c, "Files", &columns).FillWidth().Height(180)
	}
	page := ui.NewPage(ui.PageConfig{View: view, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}
