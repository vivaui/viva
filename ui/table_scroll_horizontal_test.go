package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// wideFourColumns is four explicitly 120-wide columns: 480 in a table given
// less, so it scrolls them left and right. Package-level slices, like
// tableColumns, because a slice literal built inside a paint allocates every
// frame.
var wideFourColumns = []components.TableColumn{{Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}}

var wideSixColumns = []components.TableColumn{{Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}}

var fourTitles = []string{"A", "B", "C", "D"}
var sixTitles = []string{"A", "B", "C", "D", "E", "F"}
var allSortableSix = []bool{true, true, true, true, true, true}

// A table whose explicit columns do not fit the room it was given scrolls them
// left and right, and its header rides along with its rows.
func TestTableScrollsItsColumnsWhenTheyDoNotFit(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Size{Width: 320, Height: 240}
	rows := make(TableRows, 50)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc", "dd"}
	}
	var state TableState
	var view View
	canvas := testCanvas()
	// headerX and rowX are read from cell hits: the header is not sortable
	// here, so the header texts are the witnesses of where the header sits.
	var headerX, rowX float32
	paint := func() {
		view.Draw(canvas, theme, gfx.Rect{Width: viewport.Width, Height: viewport.Height}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, TableSpec{Rows: tableScope, Header: headerScope, Columns: wideFourColumns, Headers: fourTitles, Sortable: []bool{true, false, false, false}, Source: &rows}).Size(viewport.Width, viewport.Height)
			})
		})
		headerX, rowX = headerAndRowX(canvas)
	}
	paint()
	if max := state.Scroll.MaxX(); max != 160 { // 480 declared in 320 shown
		t.Fatalf("MaxX = %v, want 160", max)
	}
	beforeHeader, beforeRow := headerX, rowX
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 60, Precise: true})
	canvas.Reset()
	paint()
	if state.Scroll.OffsetX != 60 {
		t.Errorf("OffsetX = %v, want 60", state.Scroll.OffsetX)
	}
	if moved := headerX - beforeHeader; moved != -60 {
		t.Errorf("the header moved %v, want it left by 60", moved)
	}
	if moved := rowX - beforeRow; moved != -60 {
		t.Errorf("the body moved %v, want it left by 60 with the header", moved)
	}

	// Out of range on either side stays in bounds.
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 1e5, Precise: true})
	canvas.Reset()
	paint()
	if got := state.Scroll.OffsetX; got != state.Scroll.MaxX() {
		t.Errorf("past the end OffsetX = %v, want %v", got, state.Scroll.MaxX())
	}
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: -1e5, Precise: true})
	canvas.Reset()
	paint()
	if got := state.Scroll.OffsetX; got != 0 {
		t.Errorf("before the start OffsetX = %v, want 0", got)
	}
}

func headerAndRowX(canvas *gfx.Canvas) (headerX, rowX float32) {
	for _, text := range canvas.Texts() {
		if text.Value == "B" {
			headerX = text.Bounds.X
		}
		if text.Value == "bb" {
			rowX = text.Bounds.X
		}
	}
	return
}

// The wheel moves the columns of a wide table and its rows at once: a
// horizontal trackpad delta (or the wheel with Shift held, as for a Sideways
// area) moves the columns while the plain wheel still moves the rows.
func TestTableWheelScrollsBothWays(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Size{Width: 320, Height: 240}
	rows := make(TableRows, 200)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc", "dd"}
	}
	var state TableState
	var view View
	canvas := testCanvas()
	paint := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: viewport.Width, Height: viewport.Height}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, TableSpec{Rows: tableScope, Columns: wideFourColumns, Headers: fourTitles, Source: &rows}).Size(viewport.Width, viewport.Height)
			})
		})
	}
	paint()
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 30, Precise: true})
	paint()
	if state.Scroll.OffsetX != 30 || state.Scroll.Offset != 0 {
		t.Errorf("a horizontal delta left the table at (%v, %v), want (30, 0)", state.Scroll.OffsetX, state.Scroll.Offset)
	}
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 30, Precise: true, Modifiers: gfx.ModifierShift})
	paint()
	if state.Scroll.OffsetX != 60 {
		t.Errorf("Shift with the wheel moved the columns to %v, want 60", state.Scroll.OffsetX)
	}
	if state.Scroll.Offset != 0 {
		t.Errorf("a horizontal gesture also moved the rows to %v, want 0", state.Scroll.Offset)
	}
	before := state.Scroll.Offset
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 40, Precise: true})
	paint()
	if state.Scroll.Offset == before {
		t.Error("the plain wheel no longer scrolls the rows")
	}
}

// A table whose columns fit (the last flexible, or all of them explicit and
// room still left) keeps the layout it always had: no horizontal offset, and
// the last column takes whatever is left.
func TestTableThatFitsDoesNotScrollAcross(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Size{Width: 320, Height: 240}
	rows := make(TableRows, 50)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc"}
	}
	cases := []struct {
		name    string
		columns []components.TableColumn
	}{
		{"flexible last", []components.TableColumn{{Width: 100}, {Width: 100}, {}}},
		{"all explicit", []components.TableColumn{{Width: 60}, {Width: 60}, {Width: 60}}},
	}
	for _, c := range cases {
		headers := []string{"A", "B", "C"}
		var state TableState
		var view View
		canvas := testCanvas()
		paint := func() {
			canvas.Reset()
			view.Draw(canvas, theme, gfx.Rect{Width: viewport.Width, Height: viewport.Height}, func(f *Frame) {
				f.Column().Fill().Children(func() {
					f.Table(&state, TableSpec{Rows: tableScope, Columns: c.columns, Headers: headers, Source: &rows}).Size(viewport.Width, viewport.Height)
				})
			})
		}
		paint()
		if max := state.Scroll.MaxX(); max != 0 {
			t.Errorf("%s: a fitting table scrolls across, MaxX %v", c.name, max)
		}
		state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 30, Precise: true})
		paint()
		if state.Scroll.OffsetX != 0 {
			t.Errorf("%s: the wheel moved a fitting table to %v", c.name, state.Scroll.OffsetX)
		}
		if state.Scroll.ContentWidth != state.Scroll.ViewportWidth {
			t.Errorf("%s: content is %v wide in a %v viewport, want them equal", c.name, state.Scroll.ContentWidth, state.Scroll.ViewportWidth)
		}
	}
}

// A table inside a ScrollArea is as tall as its rows and as wide as the area
// makes it; the area scrolls left and right when it is Sideways, and the table
// leaves the horizontal offset to it.
func TestTableInAScrollAreaLeavesAcrossToTheArea(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Size{Width: 300, Height: 200}
	rows := make(TableRows, 50)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc", "dd"}
	}
	for _, sideways := range []bool{false, true} {
		var state TableState
		var view View
		canvas := testCanvas()
		paint := func() {
			canvas.Reset()
			view.Draw(canvas, theme, gfx.Rect{Width: viewport.Width, Height: viewport.Height}, func(f *Frame) {
				f.Column().Fill().Children(func() {
					area := f.ScrollArea(listID).Grow(1)
					if sideways {
						area = area.Sideways()
					}
					area.Children(func() {
						f.Table(&state, TableSpec{Rows: tableScope, Columns: wideFourColumns, Headers: fourTitles, Source: &rows})
					})
				})
			})
		}
		paint()
		area := view.ScrollState(listID)
		want := float32(300)
		if sideways {
			want = 480
		}
		if area.ContentWidth != want {
			t.Errorf("sideways %v: the area's content is %v wide, want %v", sideways, area.ContentWidth, want)
		}
		// However wide its columns are, a table in an area leaves the horizontal
		// offset to it: it never moves itself across, sideways area or not.
		if state.Scroll.OffsetX != 0 {
			t.Errorf("sideways %v: the table moved itself across to %v", sideways, state.Scroll.OffsetX)
		}
	}
}

// The horizontal scrollbar drags like the vertical one: a press on the thumb
// holds it, a drag moves the columns with it, and the release lets go.
func TestTableHorizontalScrollbarDrags(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Size{Width: 320, Height: 240}
	rows := make(TableRows, 10)
	for i := range rows {
		rows[i] = []string{"aa", "bb", "cc", "dd"}
	}
	var state TableState
	var view View
	canvas := testCanvas()
	paint := func(*gfx.Canvas, gfx.Size) {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: viewport.Width, Height: viewport.Height}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, TableSpec{
					Rows: tableScope, Header: headerScope, Columns: wideFourColumns,
					Headers: fourTitles, Sortable: []bool{true, true, true, true}, Source: &rows,
					HorizontalScrollbar: gfx.Item(901),
				}).Size(viewport.Width, viewport.Height)
			})
		})
	}
	paint(canvas, gfx.Size{})
	// A 320 viewport shows the first two 120 columns whole and the third cut at
	// its right edge; the fourth is off screen and declares no hit. The header
	// scissored by the same clip as the body shows three.
	if hits := tableHeaderHits(canvas, headerScope); len(hits) != 3 {
		t.Fatalf("the header shows %d cells in a 320 viewport showing 320 of 480 px, want the 2 whole and 1 cut", len(hits))
	}
	hbar := hitByID(canvas, gfx.Item(901))
	thumb := hbar.Bounds
	deliver := func(kind gfx.PointerEventKind, x, y float32) bool {
		return canvas.DeliverPointer(gfx.PointerEvent{Kind: kind, Button: gfx.PointerButtonPrimary, Target: gfx.Item(901), Captured: kind != gfx.PointerDown, X: x, Y: y}, gfx.Size{Width: viewport.Width, Height: viewport.Height}, paint)
	}
	x, y := thumb.X+2, thumb.Y+thumb.Height/2
	if ok := deliver(gfx.PointerDown, x, y); !ok {
		t.Fatal("a press on the horizontal bar was not taken")
	}
	if !deliver(gfx.PointerMove, x+40, y) {
		t.Fatal("the drag of the bar was not taken")
	}
	if got := state.Scroll.OffsetX; got <= 0 || got > 160 {
		t.Errorf("dragging the bar left the columns at %v, want some of the 160 px of range", got)
	}
	deliver(gfx.PointerUp, x+40, y)
	paint(canvas, gfx.Size{})
	if state.Scroll.Dragging() {
		t.Error("the bar is still dragging after the release")
	}
}

func hitByID(canvas *gfx.Canvas, id gfx.ID) gfx.Hit {
	var hit gfx.Hit
	for _, h := range canvas.Hits() {
		if h.ID == id {
			return h
		}
	}
	return hit
}

func tableHeaderHits(canvas *gfx.Canvas, scope gfx.Scope) []gfx.Hit {
	var hits []gfx.Hit
	for _, hit := range canvas.Hits() {
		if hit.ID.Scope() == scope {
			hits = append(hits, hit)
		}
	}
	return hits
}

// A warm wide table paints without allocating: the horizontal scroll path adds
// no per-frame allocations to the vertical one.
func TestWideTableAllocatesNothingOnceWarm(t *testing.T) {
	theme := gfx.LightTheme()
	rows := &countingRows{n: 100}
	var view View
	var state TableState
	canvas := testCanvas()
	paint := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, TableSpec{Rows: tableScope, Header: headerScope, Columns: wideSixColumns, Headers: sixTitles, Sortable: allSortableSix, Striped: true, Source: rows, Scrollbar: gfx.Item(900), HorizontalScrollbar: gfx.Item(901)}).Fill()
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
		t.Fatalf("a warm wide table allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkWideTable(b *testing.B) {
	theme := gfx.LightTheme()
	rows := &countingRows{n: 1000}
	var view View
	var state TableState
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 60, Precise: true})
	state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 3000, Precise: true})
	canvas := testCanvas()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, TableSpec{Rows: tableScope, Header: headerScope, Columns: wideSixColumns, Headers: sixTitles, Sortable: allSortableSix, Striped: true, Source: rows, Scrollbar: gfx.Item(900), HorizontalScrollbar: gfx.Item(901)}).Fill()
			})
		})
	}
}
