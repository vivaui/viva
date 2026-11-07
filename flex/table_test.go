package flex

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

var (
	tableScope  = tree.Sub(page, tree.Name("table-rows"))
	headerScope = tree.Sub(page, tree.Name("table-headers"))
)

var (
	tableTitles  = []string{"A", "B", "C"}
	allSortable  = []bool{true, true, true}
	tableColumns = []components.TableColumn{{Width: 120}, {Width: 90, Alignment: components.TableAlignCenter}, {Alignment: components.TableAlignRight}}
)

func sameCanvas(t *testing.T, name string, flexed, direct *ui.Canvas) {
	t.Helper()
	if a, b := flexed.Rectangles(), direct.Rectangles(); len(a) != len(b) {
		t.Fatalf("%s: flex painted %d rectangles, the component %d", name, len(a), len(b))
	} else {
		for i := range a {
			if a[i].Bounds != b[i].Bounds || a[i].Color != b[i].Color {
				t.Errorf("%s: rectangle %d is %+v, the component's %+v", name, i, a[i], b[i])
			}
		}
	}
	if a, b := flexed.Texts(), direct.Texts(); len(a) != len(b) {
		t.Fatalf("%s: flex painted %d texts, the component %d", name, len(a), len(b))
	} else {
		for i := range a {
			if a[i].Value != b[i].Value || a[i].Bounds != b[i].Bounds || a[i].Style != b[i].Style || a[i].ClipOf != b[i].ClipOf {
				t.Errorf("%s: text %d is %+v, the component's %+v", name, i, a[i], b[i])
			}
		}
	}
	if a, b := flexed.Hits(), direct.Hits(); len(a) != len(b) {
		t.Fatalf("%s: flex declared %d hits, the component %d", name, len(a), len(b))
	} else {
		for i := range a {
			if a[i].ID != b[i].ID || a[i].Bounds != b[i].Bounds || a[i].Label != b[i].Label {
				t.Errorf("%s: hit %d is %+v, the component's %+v", name, i, a[i], b[i])
			}
		}
	}
}

// A Table looks exactly like the components.Table it is painted by: the same
// rectangles, texts and hit regions as the calls an application would make, in a
// table that is exactly as tall as its rows and in one that is taller.
func TestTableIsPaintedLikeTheComponent(t *testing.T) {
	theme := ui.LightTheme()
	titles := []string{"Name", "State", "Total"}
	data := TableRows{{"Ada", "Active", "$42"}, {"Grace", "Idle", "$7"}, {"Linus", "Active", "$1,000"}, {"Ken", "Away", "$0"}}
	for _, height := range []float32{components.TableHeaderHeight(theme) + 4*components.TableRowHeight(theme), 300} {
		for _, hovered := range []int{-1, 1} {
			var state TableState
			state.Select(2)
			flexed, direct := testCanvas(), testCanvas()
			if hovered >= 0 {
				flexed.SetInteraction(tableScope.At(hovered), ui.ID{}, ui.ID{}, false)
				direct.SetInteraction(tableScope.At(hovered), ui.ID{}, ui.ID{}, false)
			}
			var view View
			view.Draw(flexed, theme, ui.Rect{Width: 320, Height: height}, func(f *Frame) {
				f.Column().Children(func() {
					f.Table(&state, TableSpec{Rows: tableScope, Columns: tableColumns, Headers: titles, Source: &data}).Height(height)
				})
			})
			table := components.BeginTable(direct, theme, ui.Rect{Width: 320, Height: height}, tableColumns)
			table.Header(titles)
			for i, row := range data {
				table.Row(tableScope.At(i), row, i == 2)
			}
			sameCanvas(t, fmt.Sprintf("height %v hovered %d", height, hovered), flexed, direct)
		}
	}
}

// The typography options reach both the body rows and the header: flex paints the
// same cells as a component table given the same size and padding.
func TestTableSpecTypographyMatchesTheComponent(t *testing.T) {
	theme := ui.LightTheme()
	const size, padding = float32(12.5), float32(12)
	titles := []string{"Name", "State"}
	data := TableRows{{"Ada", "Active"}, {"Grace", "Idle"}}
	columns := []components.TableColumn{{Width: 140}, {Width: 100}}
	height := components.TableHeaderHeight(theme) + 2*components.TableRowHeight(theme)

	flexed, direct := testCanvas(), testCanvas()
	var state TableState
	var view View
	view.Draw(flexed, theme, ui.Rect{Width: 240, Height: height}, func(f *Frame) {
		f.Column().Children(func() {
			f.Table(&state, TableSpec{
				Rows: tableScope, Columns: columns, Headers: titles, Source: &data,
				FontSize: size, CellPadding: padding,
			}).Height(height)
		})
	})
	table := components.BeginTable(direct, theme, ui.Rect{Width: 240, Height: height}, columns).
		FontSize(size).CellPadding(padding)
	table.Header(titles)
	for i, row := range data {
		table.Row(tableScope.At(i), row, false)
	}
	sameCanvas(t, "typography", flexed, direct)
}

func TestTableSpecFontSizeSizesTheSortArrow(t *testing.T) {
	theme := ui.LightTheme()
	const size = float32(12.5)
	data := TableRows{{"b", "2"}, {"a", "1"}}
	columns := []components.TableColumn{{Width: 140}, {Width: 100}}
	cells := []string{"Name", "N"}
	sortable := []bool{true, false}
	var state TableState
	state.SortBy(0)
	canvas := testCanvas()
	var view View
	view.Draw(canvas, theme, ui.Rect{Width: 240, Height: 160}, func(f *Frame) {
		f.Column().Children(func() {
			f.Table(&state, TableSpec{
				Rows: tableScope, Header: headerScope, Columns: columns, Headers: cells,
				Sortable: sortable, Source: &data, FontSize: size,
			}).Height(160)
		})
	})

	arrow := false
	for _, text := range canvas.Texts() {
		if text.Value != "↑" {
			continue
		}
		arrow = true
		if text.Style.Size != size {
			t.Fatalf("sort arrow is painted at %v px, want the table's %v", text.Style.Size, size)
		}
	}
	if !arrow {
		t.Fatal("the sorted header painted no arrow")
	}
}

type countingRows struct {
	n     int
	reads int
}

func (c *countingRows) Len() int { return c.n }
func (c *countingRows) Cell(row, column int) string {
	c.reads++
	return "r"
}

// Ten thousand rows inside a ScrollArea cost the rows on screen: they are the only
// ones read and painted, wherever the area is scrolled to.
func TestTableInAScrollAreaPaintsOnlyTheRowsOnScreen(t *testing.T) {
	theme := ui.LightTheme()
	rows := &countingRows{n: 10000}
	var view View
	var table Box
	var state TableState
	draw := func(canvas *ui.Canvas) {
		view.Draw(canvas, theme, ui.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.ScrollArea(listID).Grow(1).Children(func() {
					table = f.Table(&state, TableSpec{Rows: tableScope, Columns: tableColumns[:2], Headers: []string{"A", "B"}, Source: rows})
				})
			})
		})
	}
	rowH, headH := components.TableRowHeight(theme), components.TableHeaderHeight(theme)
	total := headH + 10000*rowH
	for _, offset := range []float32{0, 5000 * rowH, total - 300} {
		view.ScrollState(listID).Offset = offset
		rows.reads = 0
		canvas := testCanvas()
		draw(canvas)
		if content := view.ScrollState(listID).Content; content < 10000*rowH {
			t.Fatalf("the area's content is %v, want the 10001 rows of the table", content)
		}
		visible := 0
		for _, hit := range canvas.Hits() {
			if id := hit.ID; id.Scope() == tableScope {
				visible++
			}
		}
		if visible == 0 || visible > 10 || rows.reads > 10*2 {
			t.Errorf("at offset %v: %d rows on screen, %d cells read, want under 10 rows", offset, visible, rows.reads)
		}
	}
	if table.Rect().Height != total {
		t.Errorf("the table is %v tall, want header and rows", table.Rect().Height)
	}
}

// A table given a height of its own keeps its header and scrolls its rows under it.
func TestTableWithAHeightScrollsItsRowsUnderTheHeader(t *testing.T) {
	theme := ui.LightTheme()
	rows := &countingRows{n: 100}
	var state TableState
	draw := func() *ui.Canvas {
		canvas := testCanvas()
		var view View
		view.Draw(canvas, theme, ui.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Children(func() {
				f.Table(&state, TableSpec{Rows: tableScope, Columns: tableColumns[:2], Headers: []string{"A", "B"}, Source: rows}).Height(200)
			})
		})
		return canvas
	}
	draw()
	content, viewport := 100*components.TableRowHeight(theme), 200-components.TableHeaderHeight(theme)
	if state.Scroll.Content != content || state.Scroll.Viewport != viewport {
		t.Fatalf("content %v viewport %v, want %v and %v", state.Scroll.Content, state.Scroll.Viewport, content, viewport)
	}
	state.Scroll.Wheel(ui.ScrollEvent{DeltaY: 1e9})
	canvas := draw()
	if state.Scroll.Offset != content-viewport {
		t.Errorf("scrolled to %v, want the end %v", state.Scroll.Offset, content-viewport)
	}
	last := -1
	for _, hit := range canvas.Hits() {
		if index, ok := hit.ID.In(tableScope); ok {
			last = max(last, index)
		}
	}
	if last != 99 {
		t.Errorf("the last visible row is %d, want 99", last)
	}
	header := false
	for _, text := range canvas.Texts() {
		header = header || text.Value == "A" && text.Bounds.Y == 0
	}
	if !header {
		t.Error("the header is not at the top")
	}
}

func TestTableHeaderClicksSortAndShowTheArrow(t *testing.T) {
	theme := ui.LightTheme()
	data := TableRows{{"b", "2"}, {"a", "1"}}
	var state TableState
	draw := func(canvas *ui.Canvas) (column, row int) {
		column, row = -1, -1
		var view View
		view.Draw(canvas, theme, ui.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Children(func() {
				tbl := f.Table(&state, TableSpec{Rows: tableScope, Header: headerScope, Columns: tableColumns[:2], Headers: []string{"Name", "N"}, Sortable: []bool{true, false}, Source: &data})
				if c, ok := tbl.HeaderClicked(); ok {
					column = c
				}
				if r, ok := tbl.RowClicked(); ok {
					row = r
				}
			})
		})
		return
	}
	canvas := testCanvas()
	draw(canvas)
	headers := 0
	for _, hit := range canvas.Hits() {
		if hit.ID.Scope() == headerScope {
			headers++
		}
	}
	if headers != 1 {
		t.Fatalf("%d sortable header cells, want 1", headers)
	}
	for _, text := range canvas.Texts() {
		if text.Value == "↑" || text.Value == "↓" {
			t.Fatal("an arrow is shown before anything is sorted")
		}
	}
	canvas = testCanvas()
	canvas.BeginEventPass(headerScope.At(0))
	if column, _ := draw(canvas); column != 0 {
		t.Fatalf("the click on the Name header reported column %d", column)
	}
	state.SortBy(0)
	canvas = testCanvas()
	draw(canvas)
	arrows := 0
	for _, text := range canvas.Texts() {
		if text.Value == "↑" {
			arrows++
		}
	}
	if arrows != 1 {
		t.Errorf("an ascending sort shows %d arrows, want 1", arrows)
	}
	state.SortBy(0)
	if column, descending, ok := state.Sort(); column != 0 || !descending || !ok {
		t.Errorf("a second click gives %d %v %v, want a descending sort of column 0", column, descending, ok)
	}
	canvas = testCanvas()
	canvas.BeginEventPass(tableScope.At(1))
	if _, row := draw(canvas); row != 1 {
		t.Errorf("the click on the second row reported %d", row)
	}
}

func TestTableAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	rows := &countingRows{n: 10000}
	var state TableState
	state.SortBy(1)
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, ui.Rect{Width: 500, Height: 400}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.ScrollArea(listID).Grow(1).Children(func() {
					f.Table(&state, TableSpec{Rows: tableScope, Header: headerScope, Columns: tableColumns, Headers: tableTitles, Sortable: allSortable, Striped: true, Source: rows})
				})
			})
		})
	}
	run()
	view.ScrollState(listID).Offset = 20000
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm table of ten thousand rows allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkTenThousandRowTable(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := ui.LightTheme()
	rows := &countingRows{n: 10000}
	var state TableState
	view.ScrollState(listID).Offset = 50000
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, ui.Rect{Width: 800, Height: 600}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.ScrollArea(listID).Grow(1).Children(func() {
					f.Table(&state, TableSpec{Rows: tableScope, Columns: tableColumns, Headers: tableTitles, Source: rows})
				})
			})
		})
	}
}

// With a row focused, the arrow, page, Home and End keys move the selection and the
// keyboard focus, keep the selected row on screen, and do nothing for other focus.
func TestTableKeysMoveTheSelectionAndScrollItIntoView(t *testing.T) {
	theme := ui.LightTheme()
	rows := &countingRows{n: 100}
	var state TableState
	state.Select(0)
	draw := func() *ui.Canvas {
		canvas := testCanvas()
		var view View
		view.Draw(canvas, theme, ui.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Children(func() {
				f.Table(&state, TableSpec{Rows: tableScope, Columns: tableColumns[:2], Headers: []string{"A", "B"}, Source: rows}).Height(200)
			})
		})
		return canvas
	}
	draw()
	press := func(key ui.Key, focused int) ui.EventResult {
		t.Helper()
		result := state.Key(ui.KeyEvent{Key: key, Target: tableScope.At(focused)})
		draw()
		return result
	}
	if got := state.Key(ui.KeyEvent{Key: ui.KeyDown, Target: omit}); got.Handled || state.Selected() != 0 {
		t.Fatal("a key for another control moved the selection")
	}
	result := press(ui.KeyDown, 0)
	if state.Selected() != 1 || !result.Handled || result.Focus != tableScope.At(1) {
		t.Errorf("Down: selected %d, result %+v, want row 1 selected and focused", state.Selected(), result)
	}
	press(ui.KeyUp, 0)
	press(ui.KeyUp, 0)
	if state.Selected() != 0 {
		t.Errorf("Up at the top selected %d, want to stay at 0", state.Selected())
	}
	if result := press(ui.KeyUnknown, 0); result.Handled {
		t.Error("a key the table does not use was reported handled")
	}
	press(ui.KeyEnd, 0)
	if state.Selected() != 99 || state.Scroll.Offset != state.Scroll.Max() {
		t.Errorf("End: selected %d scrolled to %v, want row 99 and the end %v", state.Selected(), state.Scroll.Offset, state.Scroll.Max())
	}
	press(ui.KeyPageUp, 99)
	rowH := components.TableRowHeight(theme)
	if page := int(state.Scroll.Viewport / rowH); state.Selected() > 99-2 || state.Selected() < 99-page-1 {
		t.Errorf("PageUp from the last row selected %d, want about one screenful (%d rows) up", state.Selected(), page)
	}
	selected := state.Selected()
	if top := float32(selected) * rowH; top < state.Scroll.Offset || top+rowH > state.Scroll.Offset+state.Scroll.Viewport {
		t.Errorf("row %d is outside the viewport at offset %v", selected, state.Scroll.Offset)
	}
	press(ui.KeyHome, selected)
	if state.Selected() != 0 || state.Scroll.Offset != 0 {
		t.Errorf("Home: selected %d offset %v, want row 0 at the top", state.Selected(), state.Scroll.Offset)
	}
}

// A table as tall as its rows in a ScrollArea keeps its header at the top of the
// area as the rows scroll under it, and a key that selects a row below the fold
// scrolls the area to it.
func TestTableInAScrollAreaKeepsItsHeaderAndKeysScrollTheArea(t *testing.T) {
	theme := ui.LightTheme()
	rows := &countingRows{n: 1000}
	var state TableState
	var view View
	draw := func() *ui.Canvas {
		canvas := testCanvas()
		view.Draw(canvas, theme, ui.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.ScrollArea(listID).Grow(1).Children(func() {
					f.Text("above the table").Height(40)
					f.Table(&state, TableSpec{Rows: tableScope, Header: headerScope, Columns: tableColumns[:2], Headers: []string{"A", "B"}, Sortable: []bool{true, true}, Source: rows})
				})
			})
		})
		return canvas
	}
	canvas := draw()
	for _, hit := range canvas.Hits() {
		if hit.ID.Scope() == headerScope && hit.Bounds.Y != 40 {
			t.Errorf("before scrolling the header is at %v, want where the table starts, 40", hit.Bounds.Y)
		}
	}
	view.ScrollState(listID).Offset = 5000
	canvas = draw()
	headers := 0
	for _, hit := range canvas.Hits() {
		if hit.ID.Scope() == headerScope {
			headers++
			if hit.Bounds.Y != 0 {
				t.Errorf("a scrolled table's header is at %v, want the top of the area", hit.Bounds.Y)
			}
		}
		if _, ok := hit.ID.In(tableScope); ok && hit.Bounds.Y < components.TableHeaderHeight(theme) {
			t.Errorf("row hit %+v reaches under the header", hit.Bounds)
		}
	}
	if headers != 2 {
		t.Errorf("%d header cells while scrolled, want 2", headers)
	}

	view.ScrollState(listID).Offset = 0
	draw()
	state.Select(10)
	state.Key(ui.KeyEvent{Key: ui.KeyPageDown, Target: tableScope.At(10)})
	draw()
	draw() // the area follows one frame after the table asked
	selected := state.Selected()
	rowH, headH := components.TableRowHeight(theme), components.TableHeaderHeight(theme)
	rowY := 40 + headH + float32(selected)*rowH - view.ScrollState(listID).Offset
	if selected <= 10 || rowY < headH || rowY+rowH > 200 {
		t.Errorf("selected row %d is at y=%v in a 200 tall area scrolled to %v, want it fully visible below the header", selected, rowY, view.ScrollState(listID).Offset)
	}
}
