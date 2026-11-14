package ui

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// cellTable is one table under test: its state, its rows, and a paint that
// draws it into a fresh canvas at the given size.
type cellTable struct {
	rows  TableRows
	state TableState
	spec  TableSpec
	view  View
}

func newCellTable(rows int, spec TableSpec) *cellTable {
	t := &cellTable{rows: make(TableRows, rows)}
	for i := range t.rows {
		t.rows[i] = []string{"a", "b", "c", "d", "e", "f"}
	}
	spec.Source = &t.rows
	t.spec = spec
	return t
}

func (t *cellTable) paint(size gfx.Rect) *gfx.Canvas {
	canvas := testCanvas()
	t.view.Draw(canvas, gfx.LightTheme(), size, func(f *Frame) {
		f.Column().Fill().Children(func() {
			f.Table(&t.state, t.spec).Fill()
		})
	})
	return canvas
}

func (t *cellTable) wantCell(t0 *testing.T, row, column int, want gfx.Rect) {
	t0.Helper()
	got, ok := t.state.CellRect(row, column)
	if !ok {
		t0.Fatalf("CellRect(%d, %d) reports no cell, want %+v", row, column, want)
	}
	near(t0, fmt.Sprintf("CellRect(%d, %d)", row, column), got, want)
}

var cellColumns = []components.TableColumn{{Width: 120}, {Width: 90}, {Width: 100}}

// The painted cells answer with the geometry the rows were painted at: each
// column where its cell lies, and each row one row height down from the first
// visible one, counted from the body's top under a header.
func TestTableCellsAreWhereTheRowsWerePainted(t *testing.T) {
	const width, height = float32(400), float32(300)
	table := newCellTable(20, TableSpec{
		Rows: tableScope, Columns: cellColumns, Headers: []string{"A", "B", "C"},
	})
	table.paint(gfx.Rect{Width: width, Height: height})
	rowH := components.TableRowHeight(gfx.LightTheme())
	if rowH != table.state.cellRowHeight {
		t.Fatalf("recorded row height %v, want the painted %v", table.state.cellRowHeight, rowH)
	}
	// The table is taller than it needs and so scrolls; its first row is painted
	// under the header, at the top of the body.
	headH := components.TableHeaderHeight(gfx.LightTheme())
	top := headH
	for _, column := range []int{0, 1, 2} {
		x := float32(0)
		for c := 0; c < column; c++ {
			x += cellColumns[c].Width
		}
		for _, row := range []int{0, 1, 5} {
			table.wantCell(t, row, column, gfx.Rect{X: x, Y: top + float32(row)*rowH, Width: cellColumns[column].Width, Height: rowH})
		}
	}
}

// A table that scrolls its rows keeps the source indices: the same cell moves
// up by the offset, the rows scrolled off answer false, and the rows scrolled
// into view answer.
func TestTableCellsFollowTheScroll(t *testing.T) {
	table := newCellTable(100, TableSpec{Rows: tableScope, Columns: cellColumns})
	table.paint(gfx.Rect{Width: 400, Height: 160})
	rowH := components.TableRowHeight(gfx.LightTheme())
	table.wantCell(t, 0, 0, gfx.Rect{X: 0, Y: 0, Width: 120, Height: rowH})

	table.state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 10 * rowH, Precise: true})
	if table.state.Scroll.Offset != 10*rowH {
		t.Fatalf("offset %v, want ten rows of %v", table.state.Scroll.Offset, rowH)
	}
	table.paint(gfx.Rect{Width: 400, Height: 160})
	table.wantCell(t, 10, 0, gfx.Rect{X: 0, Y: 0, Width: 120, Height: rowH})
	if _, ok := table.state.CellRect(0, 0); ok {
		t.Error("a row scrolled above the body still reports a cell")
	}
	table.state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 1e9})
	table.paint(gfx.Rect{Width: 400, Height: 160})
	if _, ok := table.state.CellRect(99, 2); !ok {
		t.Error("the last row, on screen at the end, reports no cell")
	}
	if _, ok := table.state.CellRect(100, 0); ok {
		t.Error("a row past the table's rows reports a cell")
	}
}

// A cell is the whole cell, not the strip its clip leaves visible: a row half
// scrolled out at the top keeps its full height.
func TestTableCellRectIsNotCutByTheClip(t *testing.T) {
	table := newCellTable(100, TableSpec{Rows: tableScope, Columns: cellColumns})
	rowH := components.TableRowHeight(gfx.LightTheme())
	table.paint(gfx.Rect{Width: 400, Height: 160})
	table.state.Scroll.Wheel(gfx.ScrollEvent{DeltaY: 5*rowH + rowH/2, Precise: true})
	table.paint(gfx.Rect{Width: 400, Height: 160})
	// Rows 5 and 6 straddle the top of the body; both answer their whole height,
	// row 5 starting above it.
	table.wantCell(t, 5, 0, gfx.Rect{X: 0, Y: -rowH / 2, Width: 120, Height: rowH})
	table.wantCell(t, 6, 0, gfx.Rect{X: 0, Y: rowH / 2, Width: 120, Height: rowH})
}

// A column's cell comes back where the column was painted: a fixed width as
// declared, the last one taking what is left. Changing the widths and the row
// height moves the next paint's cells with them.
func TestTableCellsFollowTheColumnAndRowSizes(t *testing.T) {
	rowH := components.TableRowHeight(gfx.LightTheme())
	table := newCellTable(10, TableSpec{Rows: tableScope, Columns: cellColumns})
	table.paint(gfx.Rect{Width: 500, Height: 300})
	table.wantCell(t, 1, 1, gfx.Rect{X: 120, Y: rowH, Width: 90, Height: rowH})

	// The last column has a width of its own, so the table ends at 310.
	table.wantCell(t, 1, 2, gfx.Rect{X: 210, Y: rowH, Width: 100, Height: rowH})

	const wide, tall = float32(160), float32(34)
	table.spec.Columns = []components.TableColumn{{Width: wide}, {Width: 90}, {Width: 100}}
	table.spec.RowHeight = tall
	table.paint(gfx.Rect{Width: 500, Height: 300})
	table.wantCell(t, 2, 1, gfx.Rect{X: wide, Y: 2 * tall, Width: 90, Height: tall})
}

// Two tables of one paint keep their own cells: each state answers for the
// table it was declared in, never for the other's.
func TestTableCellsDoNotCrossTables(t *testing.T) {
	first := newCellTable(20, TableSpec{Rows: tableScope, Columns: cellColumns})
	second := newCellTable(20, TableSpec{Rows: tableScope, Columns: []components.TableColumn{{Width: 60}, {Width: 60}, {Width: 60}}})
	canvas := testCanvas()
	var view View
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 600}, func(f *Frame) {
		f.Column().Fill().Gap(8).Children(func() {
			f.Table(&first.state, first.spec).Height(200)
			f.Table(&second.state, second.spec).Height(200)
		})
	})
	if rect, ok := first.state.CellRect(0, 0); !ok || rect.Width != 120 {
		t.Errorf("the first table's cell is %+v, %v; want its own 120 wide column", rect, ok)
	}
	if rect, ok := second.state.CellRect(0, 0); !ok || rect.Width != 60 {
		t.Errorf("the second table's cell is %+v, %v; want its own 60 wide column", rect, ok)
	}
}

// A frozen column answers where the band was pinned, and a column scrolling
// under it keeps its scrolled place; the frozen ones do not move with OffsetX.
func TestFrozenColumnCellsStayPinned(t *testing.T) {
	table := newCellTable(20, TableSpec{Rows: tableScope, Columns: wideSixColumns, Headers: sixTitles, FrozenColumns: 2})
	table.paint(gfx.Rect{Width: 400, Height: 300})
	frozen, ok := table.state.CellRect(0, 0)
	if !ok || frozen.X != 0 || frozen.Width != 120 {
		t.Fatalf("frozen cell is %+v, %v; want the first 120 px column at the left edge", frozen, ok)
	}
	scrolled, ok := table.state.CellRect(0, 2)
	if !ok || scrolled.X != 240 {
		t.Fatalf("the scrolling column is at %+v, %v; want it at 240", scrolled, ok)
	}
	table.state.Scroll.Wheel(gfx.ScrollEvent{DeltaX: 100, Precise: true})
	table.paint(gfx.Rect{Width: 400, Height: 300})
	if moved, ok := table.state.CellRect(0, 0); !ok || moved != frozen {
		t.Errorf("the frozen cell moved to %+v, want it at %+v", moved, frozen)
	}
	if moved, ok := table.state.CellRect(0, 2); !ok || moved.X != 140 {
		t.Errorf("the scrolling column sits at %+v, want it at 140", moved)
	}
}

// A table in a ScrollArea scrolls with the area: its cells are painted where
// the area's last paint put them, which is a window coordinate, and the header
// keeps its height above them.
func TestTableCellsInAScrollAreaFollowTheArea(t *testing.T) {
	table := newCellTable(1000, TableSpec{Rows: tableScope, Columns: cellColumns, Headers: []string{"A", "B", "C"}})
	rowH, headH := components.TableRowHeight(gfx.LightTheme()), components.TableHeaderHeight(gfx.LightTheme())
	paint := func() {
		canvas := testCanvas()
		table.view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 200}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.ScrollArea(listID).Grow(1).Children(func() {
					f.Text("above").Height(30)
					f.Table(&table.state, table.spec)
				})
			})
		})
	}
	paint()
	table.wantCell(t, 0, 0, gfx.Rect{X: 0, Y: 30 + headH, Width: 120, Height: rowH})
	table.view.ScrollState(listID).Offset = 30 + headH + 10*rowH
	paint()
	// The header sticks to the area's top; its rows have scrolled under it, and
	// row 10 lies behind the sticky header, so the first cell it paints is 11.
	if _, ok := table.state.CellRect(10, 0); ok {
		t.Error("a row scrolled behind the sticky header reports a cell")
	}
	table.wantCell(t, 11, 0, gfx.Rect{X: 0, Y: rowH, Width: 120, Height: rowH})
	if _, ok := table.state.CellRect(0, 0); ok {
		t.Error("a row scrolled above the area still reports a cell")
	}
}

// Before any paint, and for columns the table does not have, there is no cell.
func TestTableCellsAreUnsetBeforeAPaint(t *testing.T) {
	var state TableState
	if rect, ok := state.CellRect(0, 0); ok || rect != (gfx.Rect{}) {
		t.Fatalf("an unpainted table answers %+v, %v", rect, ok)
	}
	table := newCellTable(4, TableSpec{Rows: tableScope, Columns: cellColumns})
	table.paint(gfx.Rect{Width: 400, Height: 300})
	if _, ok := table.state.CellRect(0, 3); ok {
		t.Error("a column the table does not paint reports a cell")
	}
	if _, ok := table.state.CellRect(-1, 0); ok {
		t.Error("a negative row reports a cell")
	}
}
