package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// frozenSpec is the spec of a 400x300 table of six 120-wide columns with the
// first two frozen: 240 pinned at the left edge, 480 scrolling past them.
func frozenSpec(state *TableState, rows TableSource) TableSpec {
	return TableSpec{
		Rows: tableScope, Header: headerScope, Columns: wideSixColumns, Headers: sixTitles,
		Sortable: allSortableSix, Source: rows, FrozenColumns: 2,
	}
}

// textX reports where the last text equal to value is painted: the pass a
// table bands its columns into paints several times, and the last is the one
// that showed. A return of false means no such text. The scrolled pass of a
// banded table is the first, so its caller pairs this with firstX below.
func textX(canvas *gfx.Canvas, value string) (float32, bool) {
	x, seen := float32(0), false
	for _, text := range canvas.Texts() {
		if text.Value == value {
			x, seen = text.Bounds.X, true
		}
	}
	return x, seen
}

// firstX reports where the first text equal to value is painted.
func firstX(canvas *gfx.Canvas, value string) (float32, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text.Bounds.X, true
		}
	}
	return 0, false
}

// The frozen columns stay pinned at the table's left edge while the rest
// scroll under OffsetX, the separator stays where it was, and MaxX keeps
// the unfrozen value: the offset covers exactly the columns that move.
func TestFrozenColumnsStayPutWhileTheRestScroll(t *testing.T) {
	rows := make(TableRows, 100)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc", "dd", "ee", "ff"}
	}
	var state TableState
	canvas := testCanvas()
	var view View
	paint := func() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, frozenSpec(&state, &rows)).Fill()
			})
		})
	}
	paint()
	if max := state.Scroll.MaxX(); max != 320 { // 720 declared in 400 shown, as unfrozen
		t.Fatalf("MaxX = %v, want the unfrozen 320", max)
	}
	frozenHeader, _ := textX(canvas, "A")
	frozenCell, _ := textX(canvas, "aa")
	if frozenHeader != 8 || frozenCell != 8 { // the theme's cell padding
		t.Fatalf("the frozen header sits at %v and its first cell at %v, want both at the table's left edge plus its padding", frozenHeader, frozenCell)
	}
	scrolledHeader, _ := firstX(canvas, "C")
	scrolledCell, _ := firstX(canvas, "cc")
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 200, Precise: true})
	paint()
	if state.Scroll.OffsetX != 200 {
		t.Fatalf("OffsetX = %v, want 200", state.Scroll.OffsetX)
	}
	if x, _ := textX(canvas, "A"); x != frozenHeader {
		t.Errorf("the frozen header moved to %v, want it at %v", x, frozenHeader)
	}
	if x, _ := textX(canvas, "aa"); x != frozenCell {
		t.Errorf("the frozen cell moved to %v, want it at %v", x, frozenCell)
	}
	if x, _ := firstX(canvas, "C"); x != scrolledHeader-200 {
		t.Errorf("the scrolled header sits at %v, want %v", x, scrolledHeader-200)
	}
	if x, _ := firstX(canvas, "cc"); x != scrolledCell-200 {
		t.Errorf("the scrolled cell sits at %v, want %v", x, scrolledCell-200)
	}
}

// The horizontal scrollbar of a frozen table keeps the drag the component
// maps: the bar belongs to the scrolling window past the frozen columns, and
// a thumb dragged across it scrolls exactly what the offset can reach.
func TestFrozenTableScrollbarDragsWhatScrolls(t *testing.T) {
	rows := make(TableRows, 20)
	var state TableState
	canvas := testCanvas()
	var view View
	paint := func() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, func() TableSpec {
					spec := frozenSpec(&state, &rows)
					spec.HorizontalScrollbar = gfx.Item(901)
					return spec
				}()).Fill()
			})
		})
	}
	paint()
	bar := hitByID(canvas, gfx.Item(901))
	if bar.ID.IsZero() {
		t.Fatal("the horizontal scrollbar is not painted")
	}
	// The track starts past the 240 px of frozen columns and spans the 160
	// px of scrolling window, less the corner of the vertical bar.
	if bar.Bounds.X < 240 {
		t.Fatalf("the horizontal bar starts at %v, past the frozen columns at 240", bar.Bounds.X)
	}
	deliver := func(kind gfx.PointerEventKind, x, y float32) bool {
		return canvas.DeliverPointer(gfx.PointerEvent{Kind: kind, Button: gfx.PointerButtonPrimary, Target: gfx.Item(901), Captured: kind != gfx.PointerDown, X: x, Y: y}, gfx.Size{Width: 400, Height: 300}, func(*gfx.Canvas, gfx.Size) { paint() })
	}
	x, y := bar.Bounds.X+2, bar.Bounds.Y+bar.Bounds.Height/2
	if ok := deliver(gfx.PointerDown, x, y); !ok {
		t.Fatal("a press on the horizontal bar was not taken")
	}
	if !deliver(gfx.PointerMove, x+50, y) {
		t.Fatal("the drag of the bar was not taken")
	}
	deliver(gfx.PointerUp, x+50, y)
	paint()
	if got := state.Scroll.OffsetX; got <= 0 || got > 320 {
		t.Errorf("dragging the bar left the columns at %v, want some of the 320 px of range", got)
	}
}

// The rows of a frozen table keep the identities the same table has
// unfrozen: every row hit has its source row, and RowID answers for them.
func TestFrozenTableKeepsRowIdentities(t *testing.T) {
	rows := make(TableRows, 100)
	paint := func(frozen int) (*TableState, []gfx.Hit) {
		var state TableState
		canvas := testCanvas()
		var view View
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				spec := frozenSpec(&state, &rows)
				spec.FrozenColumns = frozen
				f.Table(&state, spec).Fill()
			})
		})
		return &state, canvas.Hits()
	}
	plainState, _ := paint(0)
	frozenState, frozenHits := paint(2)
	if plainState.Rows() != frozenState.Rows() {
		t.Fatalf("the frozen table named its rows %v, want the unfrozen %v", frozenState.Rows(), plainState.Rows())
	}
	if plainState.rowFirst != frozenState.rowFirst || plainState.rowEnd != frozenState.rowEnd {
		t.Fatalf("the frozen table painted rows %d..%d, want the unfrozen %d..%d",
			frozenState.rowFirst, frozenState.rowEnd, plainState.rowFirst, plainState.rowEnd)
	}
	rowNamed := func(hits []gfx.Hit, row int) bool {
		for _, hit := range hits {
			if index, ok := hit.ID.In(tableScope); ok && index == row {
				return true
			}
		}
		return false
	}
	for row := int(plainState.rowFirst); row < int(plainState.rowEnd); row++ {
		_, ok := frozenState.RowID(row)
		if !rowNamed(frozenHits, row) {
			t.Errorf("row %d of the frozen table declares no hit", row)
		}
		if !ok {
			t.Errorf("RowID does not answer for row %d of the frozen layout", row)
		}
	}
}

// A press on a frozen column's row still names that row after the table has
// scrolled across: the overlay's hits stay above the scrolled ones, and the
// row under the frozen band's clip is the one the pointer rests on.
func TestFrozenColumnHitsKeepTheirRowsAfterScrolling(t *testing.T) {
	rows := make(TableRows, 100)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc", "dd", "ee", "ff"}
	}
	var state TableState
	canvas := testCanvas()
	var view View
	paint := func() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, frozenSpec(&state, &rows)).Fill()
			})
		})
	}
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 150, Precise: true})
	paint()
	// Every row hit of the frozen band keeps its row's own bounds: none of
	// them follows the scrolled columns out from under the band.
	seen := map[int]bool{}
	for _, hit := range canvas.Hits() {
		index, ok := hit.ID.In(tableScope)
		if !ok {
			continue
		}
		if seen[index] {
			continue // one hit a row arrives twice, clipped to each band
		}
		if hit.Bounds.Width > 240.01 {
			t.Errorf("row %d reaches %v px across, past the frozen band and into the scrolled columns", index, hit.Bounds.Width)
		}
		seen[index] = true
	}
	// The front-most hit at a point inside the frozen band is the row itself —
	// the overlay's, not a scrolled one underneath.
	for _, hit := range canvas.Hits() {
		if _, ok := hit.ID.In(tableScope); !ok {
			continue
		}
		if hit.Bounds.X != 0 || hit.Bounds.Width != 240 {
			continue
		}
		row, _ := hit.ID.In(tableScope)
		x, y := hit.Bounds.X+10, hit.Bounds.Y+hit.Bounds.Height/2
		front := gfx.ID{}
		hits := canvas.Hits()
		for i := len(hits) - 1; i >= 0; i-- {
			region := hits[i]
			if region.IsLabel || region.Backdrop || region.ID.IsZero() {
				continue
			}
			if x >= region.Bounds.X && x <= region.Bounds.Right() && y >= region.Bounds.Y && y <= region.Bounds.Bottom() {
				front = region.ID
				break
			}
		}
		if index, ok := front.In(tableScope); !ok || index != row {
			t.Fatalf("the front-most hit over frozen row %d is %v, want the row itself", row, front)
		}
		break
	}
}

// A warm frozen table paints without allocating: the two bands cost no more
// a frame than the wide table's one.
func TestFrozenTableAllocatesNothingOnceWarm(t *testing.T) {
	rows := &countingRows{n: 100}
	var state TableState
	canvas := testCanvas()
	var view View
	paint := func() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				spec := frozenSpec(&state, rows)
				spec.Scrollbar, spec.HorizontalScrollbar = gfx.Item(900), gfx.Item(901)
				f.Table(&state, spec).Fill()
			})
		})
	}
	paint()
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 40, Precise: true})
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 90, Precise: true})
	for range 3 {
		paint()
	}
	if allocs := testing.AllocsPerRun(100, paint); allocs != 0 {
		t.Fatalf("a warm frozen table allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkFrozenTable(b *testing.B) {
	rows := &countingRows{n: 1000}
	var state TableState
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 60, Precise: true})
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 3000, Precise: true})
	canvas := testCanvas()
	view := View{}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				spec := frozenSpec(&state, rows)
				spec.Scrollbar, spec.HorizontalScrollbar = gfx.Item(900), gfx.Item(901)
				f.Table(&state, spec).Fill()
			})
		})
	}
}

// A table whose FrozenColumns pins nothing — zero, negative, or every column,
// leaving nothing to scroll — paints exactly the canvas it did before the
// field existed: rectangles, texts and hits, end to end.
func TestTableWithNothingFrozenPaintsAsItAlwaysDid(t *testing.T) {
	rows := make(TableRows, 50)
	paint := func(columns int) *gfx.Canvas {
		canvas := testCanvas()
		var view View
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				spec := frozenSpec(&TableState{}, &rows)
				spec.FrozenColumns = columns
				f.Table(&TableState{}, spec).Fill()
			})
		})
		return canvas
	}
	plain := paint(0)
	for _, count := range []int{-3, 0, 6} { // 6 would pin every column: as wide as the content, so nothing scrolls and nothing freezes
		sameCanvas(t, "count that pins nothing", plain, paint(count))
	}
}
