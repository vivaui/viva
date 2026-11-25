package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// rowChecks are the checkboxes of the rows of the last paint, top to bottom.
func (d *pageDriver) rowChecks() []gfx.Hit {
	var out []gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == "Select row" && !hit.IsLabel {
			out = append(out, hit)
		}
	}
	return out
}

func (d *pageDriver) clickHit(hit gfx.Hit) {
	d.t.Helper()
	if err := d.driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		d.t.Fatal(err)
	}
}

// A table given Checked has a checkbox for each row and one in the header: the
// row's ticks are the keys of the Selection, the header ticks or clears them all,
// and it is mixed while only some are ticked.
func TestTableCheckedTicksRowsAndHeader(t *testing.T) {
	var state ui.TableState
	var checked ui.Selection
	changes := 0
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{
			Source: wholeRows{}, Checked: &checked,
			Columns: titledColumns(nil, []string{"Host", "Port"}, nil),
		}).Width(400).Height(200)
		if state.CheckedChanged() {
			changes++
		}
	})
	boxes := d.rowChecks()
	if len(boxes) != 3 {
		t.Fatalf("row checkboxes = %d, want one for each of the 3 rows", len(boxes))
	}
	d.clickHit(boxes[1])
	if !checked.Has(1) || checked.Len() != 1 || changes != 1 {
		t.Fatalf("after a tick on row 1: checked %v (len %d), changes %d", checked.Has(1), checked.Len(), changes)
	}
	if state.Selected() != -1 {
		t.Fatalf("a tick selected row %d; ticks and the selected row are separate", state.Selected())
	}
	header := d.hit("Select all rows")
	if header.Checked || !header.Mixed {
		t.Fatalf("header with one of three ticked: checked %v, mixed %v; want mixed", header.Checked, header.Mixed)
	}
	d.clickHit(header)
	if checked.Len() != 3 {
		t.Fatalf("%d rows ticked after the header was pressed with some ticked, want all 3", checked.Len())
	}
	if h := d.hit("Select all rows"); !h.Checked || h.Mixed {
		t.Fatalf("header with every row ticked: checked %v, mixed %v", h.Checked, h.Mixed)
	}
	d.clickHit(d.hit("Select all rows"))
	if checked.Len() != 0 {
		t.Fatalf("%d rows ticked after the header was pressed with all ticked, want none", checked.Len())
	}
}

// The column of checkboxes is in front of the application's columns and is not
// one of them: Source.Cell and CellRect keep the application's indexes, and the
// first column's text starts after the checkboxes.
func TestTableCheckedKeepsTheApplicationsColumnIndexes(t *testing.T) {
	var state ui.TableState
	var checked ui.Selection
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{
			Source: wholeRows{}, Checked: &checked,
			Columns: titledColumns(nil, []string{"Host", "Port"}, nil),
		}).Width(400).Height(200)
	})
	at := map[string]float32{}
	for _, label := range d.driver.Snapshot().Labels {
		at[label.Value] = label.Bounds.X
	}
	if at["alpha"] < 40 {
		t.Fatalf("the first column's text starts at %v, inside the 40 pixel column of checkboxes", at["alpha"])
	}
	cell, ok := state.CellRect(0, 0)
	if !ok || cell.X < 40 {
		t.Fatalf("CellRect(0, 0) = %+v, %v; want the application's first column, after the checkboxes", cell, ok)
	}
	port, ok := state.CellRect(0, 1)
	if !ok || port.X <= cell.X {
		t.Fatalf("CellRect(0, 1) = %+v, %v; want the second column right of the first", port, ok)
	}
}

// A sortable column is sorted by its own index, with a column of checkboxes in front.
func TestTableCheckedSortsByTheApplicationsColumn(t *testing.T) {
	var state ui.TableState
	var checked ui.Selection
	clicked := -1
	d := openPage(t, func(c ui.C) {
		table := ui.Table(c, &state, ui.TableSpec{
			Source: wholeRows{}, Checked: &checked,
			Columns: titledColumns(nil, []string{"Host", "Port"}, []bool{true, true}),
		}).Width(400).Height(200)
		if column, ok := table.HeaderClicked(); ok {
			clicked = column
		}
	})
	d.click("Port")
	if clicked != 1 {
		t.Fatalf("a click on the Port heading reported column %d, want 1", clicked)
	}
}

// Checked rows repaint without allocating.
func TestTableCheckedRepaintsWithoutAllocating(t *testing.T) {
	var state ui.TableState
	var checked ui.Selection
	checked.Add(1)
	rows := ui.TableRows{{"alpha", ""}, {"beta", ":22"}, {"gamma", ""}} // cells made once: wholeRows builds them in each call
	columns := titledColumns(nil, []string{"Host", "Port"}, nil)
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{Source: &rows, Checked: &checked, Columns: columns}).Width(400).Height(200)
	})
	d.repaint()
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// With frozen columns and sideways scrolling the rows are painted in two passes;
// the checkbox of a row still ticks once for one click.
func TestTableCheckedWithFrozenColumnsTicksOnce(t *testing.T) {
	var state ui.TableState
	var checked ui.Selection
	rows := ui.TableRows{{"alpha", "a", "b"}, {"beta", "c", "d"}, {"gamma", "e", "f"}}
	columns := titledColumns([]ui.TableColumn{{Width: 120}, {Width: 160}, {Width: 160}}, []string{"Host", "Port", "User"}, nil)
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{Source: &rows, Checked: &checked, Columns: columns, FrozenColumns: 1}).Width(300).Height(200)
	})
	boxes := d.rowChecks()
	if len(boxes) == 0 {
		t.Fatal("no row checkboxes painted")
	}
	d.clickHit(boxes[len(boxes)-1])
	if checked.Len() != 1 {
		t.Fatalf("%d rows ticked after one click in a table painted in two passes, want 1", checked.Len())
	}
}
