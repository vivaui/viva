package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func columnResizeFixture(canvas *gfx.Canvas) ColumnResize {
	return BeginColumnResize(canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 500, Height: 28},
		[]float32{280, 80, 120}).
		Dividers([]gfx.ID{gfx.Item(11), gfx.Item(12)}).
		Minimums([]float32{120, 40, 60}).
		Fits([]float32{200, 64, 100}).
		Gap(8)
}

func TestColumnResizeLaysOutColumnsAndDividers(t *testing.T) {
	var canvas gfx.Canvas
	columns := make([]gfx.Rect, 3)
	dividers := make([]gfx.Rect, 2)
	hits := make([]gfx.Rect, 2)
	layout := columnResizeFixture(&canvas).Paint(columns, dividers, hits, []string{"Resize name", "Resize size"})
	if layout.Columns[0] != (gfx.Rect{X: 10, Y: 20, Width: 280, Height: 28}) {
		t.Fatalf("name column = %+v", layout.Columns[0])
	}
	if layout.Columns[1] != (gfx.Rect{X: 298, Y: 20, Width: 80, Height: 28}) {
		t.Fatalf("size column = %+v", layout.Columns[1])
	}
	if layout.Dividers[0] != (gfx.Rect{X: 294, Y: 20, Width: 1, Height: 28}) {
		t.Fatalf("first divider = %+v", layout.Dividers[0])
	}
	if layout.Hits[0] != (gfx.Rect{X: 292.5, Y: 20, Width: 4, Height: 28}) {
		t.Fatalf("first hit = %+v", layout.Hits[0])
	}
	if len(canvas.Rectangles()) != 2 || len(canvas.Hits()) != 2 {
		t.Fatalf("column resize commands = %d rectangles, %d hits", len(canvas.Rectangles()), len(canvas.Hits()))
	}
	for index, hit := range canvas.Hits() {
		if hit.Role != gfx.RoleSeparator || hit.Cursor != gfx.PointerCursorResizeHorizontal || hit.Label != []string{"Resize name", "Resize size"}[index] {
			t.Fatalf("hit %d = %+v", index, hit)
		}
	}
}

func TestColumnResizeDragKeepsBothColumnsUsable(t *testing.T) {
	var canvas gfx.Canvas
	resize := columnResizeFixture(&canvas)
	edit, ok := resize.Edit(0, 300)
	if !ok || edit.Left != 286 || edit.Right != 74 {
		t.Fatalf("drag to 300 = %+v, %v; want 286/74", edit, ok)
	}
	edit, _ = resize.Edit(0, 1000)
	if edit.Left != 320 || edit.Right != 40 {
		t.Fatalf("drag past the end = %+v, want the 40 px right minimum", edit)
	}
	edit, _ = resize.Edit(0, 0)
	if edit.Left != 120 || edit.Right != 240 {
		t.Fatalf("drag past the start = %+v, want the 120 px left minimum", edit)
	}
	edit, _ = resize.Edit(1, 1000)
	if edit.Left != 140 || edit.Right != 60 {
		t.Fatalf("second divider drag = %+v, want the 60 px right minimum", edit)
	}
	if _, ok := resize.Edit(2, 100); ok {
		t.Fatal("an edit past the last divider was accepted")
	}
	if _, ok := resize.Edit(-1, 100); ok {
		t.Fatal("a negative divider was accepted")
	}
}

func TestColumnResizeFitUsesTheContentWidth(t *testing.T) {
	var canvas gfx.Canvas
	resize := columnResizeFixture(&canvas)
	edit, ok := resize.Fit(0)
	if !ok || edit.Left != 200 || edit.Right != 160 {
		t.Fatalf("fit name = %+v, %v; want 200/160", edit, ok)
	}
	edit, _ = resize.Fit(1)
	if edit.Left != 64 || edit.Right != 136 {
		t.Fatalf("fit size = %+v, want 64/136", edit)
	}
	fitted := columnResizeFixture(&canvas).Fits([]float32{1000, 1, 100})
	edit, _ = fitted.Fit(0)
	if edit.Left != 320 || edit.Right != 40 {
		t.Fatalf("oversized fit = %+v, want the 40 px right minimum", edit)
	}
	edit, _ = fitted.Fit(1)
	if edit.Left != 40 || edit.Right != 160 {
		t.Fatalf("undersized fit = %+v, want the 40 px left minimum", edit)
	}
}

func TestColumnResizeHighlightsWhenHoveredOrPressed(t *testing.T) {
	paint := func(hovered, pressed uint32) []gfx.Rectangle {
		var canvas gfx.Canvas
		canvas.SetInteraction(gfx.Item(int(hovered)), gfx.Item(int(pressed)), gfx.ID{}, false)
		resize := columnResizeFixture(&canvas)
		columns := make([]gfx.Rect, 3)
		dividers := make([]gfx.Rect, 2)
		hits := make([]gfx.Rect, 2)
		resize.Paint(columns, dividers, hits, []string{"name", "size"})
		return canvas.Rectangles()
	}
	theme := gfx.LightTheme()
	rest := paint(0, 0)
	hover := paint(11, 0)
	press := paint(11, 11)
	// The highlight is painted before the next divider, not after every line.
	highlight := func(rectangles []gfx.Rectangle) gfx.Rectangle {
		for _, rectangle := range rectangles {
			if rectangle.Bounds.Width == 3 {
				return rectangle
			}
		}
		return gfx.Rectangle{}
	}
	if len(hover) != len(rest)+1 || highlight(hover).Color != theme.Ring || highlight(hover).Bounds.Width != 3 {
		t.Fatalf("hovered divider = %+v (resting %d rectangles)", hover, len(rest))
	}
	if len(press) != len(rest)+1 || highlight(press).Color != theme.Primary {
		t.Fatalf("pressed divider = %+v, want a primary line", press)
	}
}

func BenchmarkColumnResizePaint(b *testing.B) {
	var canvas gfx.Canvas
	resize := columnResizeFixture(&canvas)
	columns := make([]gfx.Rect, 3)
	dividers := make([]gfx.Rect, 2)
	hits := make([]gfx.Rect, 2)
	labels := []string{"name", "size"}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		canvas.Reset()
		resize.Paint(columns, dividers, hits, labels)
	}
}

func TestColumnResizePaintAllocatesNothing(t *testing.T) {
	var canvas gfx.Canvas
	resize := columnResizeFixture(&canvas)
	columns := make([]gfx.Rect, 3)
	dividers := make([]gfx.Rect, 2)
	hits := make([]gfx.Rect, 2)
	labels := []string{"name", "size"}
	paint := func() {
		canvas.Reset()
		resize.Paint(columns, dividers, hits, labels)
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("column resize paint allocations = %v", allocations)
	}
}
