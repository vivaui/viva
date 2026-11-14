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
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(20).Children(func() {
			area = ui.Scroll(c).Size(300, 200).Scrollbars().Children(func() {
				ui.Slot(c).Height(2000)
			})
		})
	}
	d := openPageWith(t, ui.PageConfig{View: view, ScrollbarSize: 10}, app.Config{})
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

// An application-owned vertical bar can replace a table's anonymous hover
// overlay without losing the table's geometry, wheel, or horizontal overlay.
func TestTableHideHoverScrollbarsOmitsOnlyTheAnonymousVerticalThumb(t *testing.T) {
	for _, test := range []struct {
		name   string
		hide   bool
		gutter float32
	}{
		{name: "default overlay", gutter: 0},
		{name: "default over a gutter", gutter: 10},
		{name: "hidden overlay", hide: true, gutter: 0},
		{name: "hidden overlay over a gutter", hide: true, gutter: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := make(ui.TableRows, 100)
			for i := range rows {
				rows[i] = []string{"row", "a", "b", "c"}
			}
			var state ui.TableState
			var table ui.Box
			view := func(c ui.C) {
				ui.Column(c).Children(func() {
					table = ui.Table(c, &state, ui.TableSpec{
						Columns: []components.TableColumn{{Width: 120}, {Width: 120}, {Width: 120}, {Width: 120}},
						Headers: []string{"A", "B", "C", "D"}, Rows: gfx.Item(1).Scope(), Source: &rows,
						ScrollbarSize: 10, ScrollbarGutter: test.gutter,
						HideHoverScrollbars: test.hide,
					}).Size(320, 200)
				})
			}
			d := openPage(t, view)
			d.driver.Move(150, 100) // rest over the overflowing table
			bounds := table.Rect()
			bounds.Y += components.TableHeaderHeight(pageTheme)
			bounds.Height -= components.TableHeaderHeight(pageTheme)
			layout := components.BeginScrollArea(nil, pageTheme, bounds, gfx.Size{Width: 480, Height: float32(len(rows)) * components.TableRowHeight(pageTheme)}).
				ScrollbarSize(10).HoverScrollbars(true)
			if test.gutter > 0 {
				layout = layout.ReserveScrollbars()
			}
			geometry := layout.Geometry()
			if geometry.MaxOffsetX <= 0 || geometry.MaxOffsetY <= 0 {
				t.Fatalf("the table does not overflow both ways: %+v", geometry)
			}
			haveVertical, haveHorizontal := false, false
			for _, primitive := range d.driver.Snapshot().Primitives {
				if primitive.Kind != app.RectanglePrimitive || primitive.Radius <= 0 {
					continue
				}
				switch primitive.Bounds {
				case geometry.VerticalThumb:
					haveVertical = true
				case geometry.HorizontalThumb:
					haveHorizontal = true
				}
			}
			if haveVertical == test.hide {
				t.Errorf("vertical thumb painted = %v with HideHoverScrollbars %v", haveVertical, test.hide)
			}
			if !haveHorizontal {
				t.Error("the horizontal thumb disappeared")
			}
			if err := d.driver.Scroll(gfx.ScrollEvent{X: 150, Y: 100, DeltaY: 100, Precise: true}); err != nil {
				t.Fatal(err)
			}
			if state.Scroll.Offset == 0 {
				t.Error("hiding the overlay stopped the table's wheel")
			}
		})
	}
}

// An explicitly owned scrollbar is not an anonymous hover ornament: the opt-out
// leaves it painted, hit, draggable, and able to move the table.
func TestTableHideHoverScrollbarsKeepsAnExplicitVerticalBarInteractive(t *testing.T) {
	rows := make(ui.TableRows, 100)
	for i := range rows {
		rows[i] = []string{"row", "state"}
	}
	var state ui.TableState
	view := func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{
			Columns: []components.TableColumn{{Width: 200}, {Width: 0}},
			Rows:    gfx.Item(1).Scope(), Source: &rows,
			Scrollbar: gfx.Item(900), ScrollbarLabel: "Table rows",
			ScrollbarSize: 10, HideHoverScrollbars: true,
		}).Size(320, 200)
	}
	d := openPage(t, view)
	d.driver.Move(150, 100)
	bar := d.hit("Table rows")
	at := gfx.Point{X: bar.Bounds.X + bar.Bounds.Width/2, Y: bar.Bounds.Y + 20}
	if err := d.driver.Press(at.X, at.Y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(at.X, at.Y+50); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(at.X, at.Y+50, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if state.Scroll.Offset == 0 {
		t.Fatal("the explicit vertical scrollbar did not drag the table")
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
