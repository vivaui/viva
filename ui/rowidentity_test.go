package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type rowIdentityModel struct {
	rows   ui.TableRows
	first  ui.TableState
	second ui.TableState
	target gfx.ID
	row    int
	ok     bool
}

func newRowIdentityModel(rows int) *rowIdentityModel {
	m := &rowIdentityModel{rows: make(ui.TableRows, rows), row: -1}
	for i := range m.rows {
		m.rows[i] = []string{fmt.Sprintf("source-%d", i)}
	}
	return m
}

func (m *rowIdentityModel) table(c ui.C, state *ui.TableState) {
	ui.Table(c, state, ui.TableSpec{
		Columns:   []components.TableColumn{{}},
		Source:    &m.rows,
		RowHeight: 20,
	}).Height(100)
}

func (m *rowIdentityModel) page(c ui.C) {
	m.table(c, &m.first)
}

func (m *rowIdentityModel) pointer(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerDown {
		return gfx.EventResult{}
	}
	m.target = event.Target
	m.row, m.ok = event.Target.In(m.first.Rows())
	return gfx.EventResult{}
}

func (m *rowIdentityModel) press(t *testing.T, d *pageDriver, row int) {
	t.Helper()
	x, y := center(d.hit(fmt.Sprintf("source-%d", row)).Bounds)
	m.ok = false
	if err := d.driver.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if !m.ok {
		t.Fatalf("a press on source row %d did not reach the table's row scope; target: %v", row, m.target)
	}
	if m.row != row {
		t.Fatalf("pointer routed to source row %d, want %d", m.row, row)
	}
	id, ok := m.first.RowID(row)
	if !ok || id != m.target || id.Scope() != m.first.Rows() {
		t.Fatalf("RowID(%d) = %v, %v; pointer target = %v, Rows = %v", row, id, ok, m.target, m.first.Rows())
	}
}

func declaredRowHits(d *pageDriver, scope gfx.Scope) map[int]gfx.ID {
	ids := make(map[int]gfx.ID)
	for _, hit := range d.driver.Snapshot().Hits {
		if row, ok := hit.ID.In(scope); ok && !hit.IsLabel {
			ids[row] = hit.ID
		}
	}
	return ids
}

func TestTableRowIDRoutesRealPointerAcrossScroll(t *testing.T) {
	m := newRowIdentityModel(50)
	d := openPageWith(t, ui.PageConfig{View: m.page}, app.Config{OnPointer: m.pointer})
	m.press(t, d, 0)
	firstRowID := m.target
	m.press(t, d, 3)
	if _, ok := m.first.RowID(20); ok {
		t.Fatal("RowID names an undeclared row below the table's viewport")
	}

	at := d.hit("source-3").Bounds
	if err := d.driver.Scroll(gfx.ScrollEvent{X: at.X + 1, Y: at.Y + 1, DeltaY: 200, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.first.RowID(0); ok {
		t.Fatal("RowID still names row 0 after the second page replaces the first")
	}
	m.press(t, d, 10)
	m.press(t, d, 14)
	if firstRowID == m.target {
		t.Fatal("a second-page source row reused the old first-page identity")
	}
	hits := declaredRowHits(d, m.first.Rows())
	for row, id := range hits {
		if got, ok := m.first.RowID(row); !ok || got != id {
			t.Fatalf("declared source row %d has RowID %v, %v, hit %v", row, got, ok, id)
		}
	}
	if len(hits) == 0 {
		t.Fatal("the second page declares no row hits")
	}
	for _, row := range []int{-1, 50, 100} {
		if id, ok := m.first.RowID(row); ok || !id.IsZero() {
			t.Fatalf("RowID(%d) = %v, %v, want zero, false", row, id, ok)
		}
	}
}

func TestTableRowsIdentitiesArePerTableAndStableInAnOuterScroll(t *testing.T) {
	m := newRowIdentityModel(40)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				m.table(c, &m.first)
				m.table(c, &m.second)
			})
		})
	})
	firstID, firstOK := m.first.RowID(0)
	secondID, secondOK := m.second.RowID(0)
	if !firstOK || !secondOK || m.first.Rows().IsZero() || m.second.Rows().IsZero() {
		t.Fatal("the default scopes do not expose declared rows")
	}
	if firstID == secondID || m.first.Rows() == m.second.Rows() {
		t.Fatal("two tables on one page share row identities")
	}
	if row, ok := firstID.In(m.second.Rows()); ok || row != 0 {
		t.Fatal("the first table's row belongs to the second table's scope")
	}
	for _, hit := range declaredRowHits(d, m.first.Rows()) {
		if hit == secondID {
			t.Fatal("the default scope passed to Table changed its row identity")
		}
	}

	at := d.hit("source-1").Bounds
	if err := d.driver.Scroll(gfx.ScrollEvent{X: at.X + 1, Y: at.Y + 1, DeltaY: 20, Precise: true}); err != nil {
		t.Fatal(err)
	}
	movedID, movedOK := m.first.RowID(0)
	if !movedOK || movedID != firstID {
		t.Fatal("a row's identity changed as an outer scroll moved it on screen")
	}
	if row, ok := movedID.In(m.second.Rows()); ok || row != 0 {
		t.Fatal("a moved row entered another table's scope")
	}

	d.repaint()
	if next, ok := m.first.RowID(0); !ok || next != firstID {
		t.Fatal("the first table's identity changed after a repaint with unchanged layout")
	}
}

func TestTableRowIDLastDeclarationAndZeroState(t *testing.T) {
	var zero ui.TableState
	if !zero.Rows().IsZero() {
		t.Fatal("a zero state has a row scope before its first paint")
	}
	if id, ok := zero.RowID(0); ok || !id.IsZero() {
		t.Fatal("a zero state has a row identity before its first paint")
	}

	m := newRowIdentityModel(3)
	tree := new(gfx.Tree)
	page := tree.Feature("row-identity-test")
	rows := tree.Sub(page, tree.Name("rows"))
	other := tree.Sub(page, tree.Name("other"))
	second := false
	var state ui.TableState
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{Rows: rows, Source: &m.rows, RowHeight: 20})
		if second {
			ui.Table(c, &state, ui.TableSpec{Rows: other, Source: &m.rows, RowHeight: 20})
		}
	})
	firstID, ok := state.RowID(0)
	if !ok || state.Rows() != rows {
		t.Fatal("an explicitly named table did not publish its declared scope")
	}
	second = true
	d.repaint()
	lastID, ok := state.RowID(0)
	if !ok || state.Rows() != other || lastID == firstID {
		t.Fatal("the accessors do not describe the last table's declaration")
	}

	m.rows = m.rows[:1]
	d.repaint()
	if id, ok := state.RowID(1); ok || !id.IsZero() {
		t.Fatal("a row removed from the source retains its old identity")
	}
	if id, ok := state.RowID(0); !ok || id != lastID {
		t.Fatal("the remaining source row's identity was not preserved")
	}
}
