package base

import "github.com/vivaui/viva/gfx"

// CheckedChanged reports whether the paint being declared delivered a tick or an
// untick in the table's column of checkboxes (TableSpec.Checked). Like Changed, it
// is true in the paint that finds the click and false in the paint after it.
func (s *TableState) CheckedChanged() bool { return s.checkedChanged }

// GetLead is 1 when the table has a column of checkboxes in front of its columns.
func (t *TableDecl) GetLead() int { return t.lead }

// ToggleRow ticks a row that is not ticked and unticks one that is.
func (t *TableDecl) ToggleRow(id gfx.ID, row int) {
	if sel := t.spec.Checked; sel != nil && t.state.checkDone != id {
		t.state.checkDone = id
		if sel.Has(row) {
			sel.Remove(row)
		} else {
			sel.Add(row)
		}
		t.state.checkedChanged = true
	}
}

// ToggleAll ticks every row when some are not ticked, and clears them when all are.
func (t *TableDecl) ToggleAll(id gfx.ID) {
	sel := t.spec.Checked
	if sel == nil || t.state.checkDone == id {
		return
	}
	t.state.checkDone = id
	if t.AllChecked() {
		sel.Clear()
	} else {
		for row := 0; row < t.rows; row++ {
			sel.Add(row)
		}
	}
	t.state.checkedChanged = true
}

// AllChecked reports whether the table has rows and every one is ticked: the
// Selection holds as many keys as there are rows. The keys of a table's Selection
// are its row numbers, and the application takes the ones of rows that are gone out.
func (t *TableDecl) AllChecked() bool {
	return t.spec.Checked != nil && t.rows > 0 && t.spec.Checked.Len() >= t.rows
}

// SomeChecked reports whether any row is ticked.
func (t *TableDecl) SomeChecked() bool { return t.spec.Checked != nil && t.spec.Checked.Len() > 0 }

// checks is the scope of a table's checkboxes, inside the scope of its rows: the
// header's is item 0 and the checkbox of row r is item r+1.
func checks(canvas *gfx.Canvas, rows gfx.Scope) gfx.Scope {
	return canvas.Tree().Sub(rows, gfx.Index(1<<30))
}

// CheckID names the checkbox of a row.
func CheckID(canvas *gfx.Canvas, rows gfx.Scope, row int) gfx.ID {
	return checks(canvas, rows).At(row + 1)
}

// HeaderCheckID names the checkbox of the header.
func HeaderCheckID(canvas *gfx.Canvas, rows gfx.Scope) gfx.ID {
	return checks(canvas, rows).At(0)
}

// applyCheckClick applies the click this paint is delivering, when it is on one of
// the table's checkboxes, as the declaration reads the click on a row. The rows
// that can have been clicked are the ones the last paint put on screen.
func (f *Frame) applyCheckClick(t *TableDecl) {
	if f.Surface == nil {
		return
	}
	id, ok := f.Surface.ClickTarget()
	if !ok || id.IsZero() {
		return
	}
	if id == HeaderCheckID(f.Surface, t.spec.Rows) {
		t.ToggleAll(id)
		return
	}
	for row := int(t.state.rowFirst); row < int(t.state.rowEnd); row++ {
		if id == CheckID(f.Surface, t.spec.Rows, row) {
			t.ToggleRow(id, row)
			return
		}
	}
}
