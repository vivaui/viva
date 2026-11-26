package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A filterable column has a button at the end of its heading, separate from the
// heading's own press: a press on it is reported as the column's index in Columns
// (with or without a column of checkboxes in front), and does not sort.
func TestFilterButtonReportsItsColumn(t *testing.T) {
	for _, checkedTable := range []bool{false, true} {
		var state ui.TableState
		var checked ui.Selection
		filtered, sorted := -1, -1
		columns := titledColumns(nil, []string{"Host", "Port"}, []bool{true, true})
		columns[1].Filterable = true
		spec := ui.TableSpec{Source: wholeRows{}, Columns: columns}
		if checkedTable {
			spec.Checked = &checked
		}
		d := openPage(t, func(c ui.C) {
			table := ui.Table(c, &state, spec).Width(400).Height(200)
			if column, ok := table.FilterClicked(); ok {
				filtered = column
			}
			if column, ok := table.HeaderClicked(); ok {
				sorted = column
			}
		})
		d.clickHit(d.hit("Filter"))
		if filtered != 1 || sorted != -1 {
			t.Fatalf("checked=%v: filter reported column %d and the heading sorted %d; want 1 and none", checkedTable, filtered, sorted)
		}
		if node := d.hit("Filter"); node.Selected {
			t.Fatal("a column that is not filtered paints its filter button as on")
		}
	}
}

// Filtered paints the button as one that is on, and HeaderRect says where the heading
// is so the application can anchor its choice there.
func TestFilteredColumnAndHeaderRect(t *testing.T) {
	var state ui.TableState
	columns := titledColumns(nil, []string{"Host", "Port"}, nil)
	columns[0].Filterable, columns[0].Filtered = true, true
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{Source: wholeRows{}, Columns: columns}).Width(400).Height(200)
	})
	if !d.hit("Filter").Selected {
		t.Fatal("a filtered column's button is not painted as on")
	}
	rect, ok := state.HeaderRect(0)
	filter := d.hit("Filter")
	if !ok || filter.Bounds.X < rect.X || filter.Bounds.X+filter.Bounds.Width > rect.X+rect.Width+0.5 || filter.Bounds.Y < rect.Y {
		t.Fatalf("HeaderRect(0) = %+v, %v; the filter button %+v is not inside it", rect, ok, filter.Bounds)
	}
	if _, ok := state.HeaderRect(5); ok {
		t.Fatal("HeaderRect answered for a column that does not exist")
	}
}
