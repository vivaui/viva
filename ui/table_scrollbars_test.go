package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A declarative scroll area's bars are the hover style: hidden until the pointer
// reaches the area, sized by ScrollbarSize, and dragged like the component's.
func TestScrollAreaBarsFollowThePointerAtTheirSize(t *testing.T) {
	var area ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Pad(20).Children(func() {
			area = ui.Scroll(c).Size(300, 200).Scrollbars().ScrollbarSize(10).Children(func() {
				ui.Slot(c).Height(2000)
			})
		})
	})
	if d.has("Vertical scrollbar") {
		t.Fatal("the bar is painted with the pointer away from the area")
	}
	r := area.Rect()
	d.driver.Move(r.X+r.Width/2, r.Y+r.Height/2)
	bar := d.hit("Vertical scrollbar")
	if bar.Bounds.Width != 10 {
		t.Fatalf("bar width = %v, want 10", bar.Bounds.Width)
	}
	if err := d.driver.Scroll(gfx.ScrollEvent{X: r.X + 10, Y: r.Y + 10, DeltaY: 100, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if area.ScrollState().Offset == 0 {
		t.Fatal("the wheel did not move the area")
	}
	bar = d.hit("Vertical scrollbar")
	if err := d.driver.Press(bar.Bounds.X+bar.Bounds.Width/2, bar.Bounds.Y+bar.Bounds.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	before := area.ScrollState().Offset
	if err := d.driver.Move(bar.Bounds.X+bar.Bounds.Width/2, bar.Bounds.Y+bar.Bounds.Height/2+40); err != nil {
		t.Fatal(err)
	}
	if area.ScrollState().Offset == before {
		t.Fatal("dragging the bar did not move the area")
	}
	if err := d.driver.Release(bar.Bounds.X+bar.Bounds.Width/2, bar.Bounds.Y+bar.Bounds.Height/2+40, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	d.driver.Move(2, 2)
	if d.has("Vertical scrollbar") {
		t.Fatal("the bar stayed after the pointer left the area")
	}
}

// A table given a smaller height than its rows scrolls them itself, with the
// bar in the gutter it is asked for, and keeps the pointer from the rows while
// the bar is held. Config.OnKey hands it the keys, as the application does.
func TestTableScrollsItselfWithAHoverBarInItsGutter(t *testing.T) {
	rows := make(ui.TableRows, 300)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("row %03d", i), "running"}
	}
	rowScope := gfx.Item(1).Scope()
	var tableState ui.TableState
	var table ui.Box
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(20).Children(func() {
			table = ui.Table(c, &tableState, ui.TableSpec{
				Rows: rowScope, Header: gfx.Item(2).Scope(),
				Columns: []components.TableColumn{{Width: 200}, {Width: 0}},
				Headers: []string{"Name", "State"}, Source: &rows,
				Scrollbar: gfx.Item(900), ScrollbarLabel: "Table rows",
				ScrollbarSize: 10, ScrollbarGutter: 10,
			}).Size(400, 160)
		})
	}
	d := openPageWith(t, ui.PageConfig{View: view}, app.Config{OnKey: func(event gfx.KeyEvent) gfx.EventResult {
		return tableState.Key(event)
	}})
	if d.has("Table rows") {
		t.Fatal("the bar is painted with the pointer away from the table")
	}
	r := table.Rect()
	d.driver.Move(r.X+20, r.Y+components.TableHeaderHeight(pageTheme)+10)
	bar := d.hit("Table rows")
	if bar.Bounds.Width != 10 {
		t.Fatalf("bar width = %v, want 10", bar.Bounds.Width)
	}
	rowHit := func(row int) (gfx.Hit, bool) {
		for _, hit := range d.driver.Snapshot().Hits {
			if hit.ID == rowScope.At(row) {
				return hit, true
			}
		}
		return gfx.Hit{}, false
	}
	first, ok := rowHit(0)
	if !ok {
		t.Fatal("the first row is not painted")
	}
	if first.Bounds.Width != 400-10 {
		t.Fatalf("row width = %v, want the table less its gutter", first.Bounds.Width)
	}
	if err := d.driver.Scroll(gfx.ScrollEvent{X: r.X + 20, Y: r.Y + 30, DeltaY: 3000, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if tableState.Scroll.Offset == 0 {
		t.Fatal("the wheel did not scroll the table")
	}
	if err := d.driver.Click(r.X+20, r.Y+components.TableHeaderHeight(pageTheme)+10); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Key(gfx.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	if tableState.Selected() != len(rows)-1 {
		t.Fatalf("End selected row %d of %d", tableState.Selected(), len(rows))
	}
	if tableState.Scroll.Offset != tableState.Scroll.Max() {
		t.Fatalf("End left the table at %v of %v", tableState.Scroll.Offset, tableState.Scroll.Max())
	}

	// A held bar owns the pointer: no row takes a hit or a hover meanwhile.
	bar = d.hit("Table rows")
	at := gfx.Point{X: bar.Bounds.X + bar.Bounds.Width/2, Y: bar.Bounds.Y + 20}
	if err := d.driver.Press(at.X, at.Y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	d.driver.Move(r.X+20, at.Y)
	rowsOnScreen := 0
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.ID.Scope() == rowScope && hit.ID != gfx.Item(900) {
			rowsOnScreen++
		}
	}
	if rowsOnScreen != 0 {
		t.Fatalf("%d rows take the pointer under a held bar", rowsOnScreen)
	}
	if err := d.driver.Release(r.X+20, at.Y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
}
