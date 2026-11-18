package ui_test

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A wheel over a Popover is the panel's. It scrolls a list in the panel, and when that
// list is at its end, or the pointer is on the panel's padding, it stops at the panel
// instead of scrolling the page area painted under it. The panel hangs off a toolbar
// button and is declared before the page area, as a title bar's menu is, so the area
// comes later in the page than the panel's list. Outside the panel the wheel scrolls the
// page as before: a Popover is not modal.
func TestAWheelOverAPopoverStaysInThePanel(t *testing.T) {
	open := true
	var list, area ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			projects := ui.Button(c, "Projects")
			ui.PopoverWith(c, projects, ui.PopoverSpec{Width: 200}, &open, func() {
				list = ui.Scroll(c).FillWidth().MaxHeight(150).Children(func() {
					for range 20 {
						ui.Slot(c).Height(30)
					}
				})
			})
			area = ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				for range 40 {
					ui.Slot(c).Height(30)
				}
			})
		})
	})
	wheel := func(x, y, dy float32) {
		t.Helper()
		if err := d.driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: dy, Precise: true}); err != nil {
			t.Fatal(err)
		}
		d.repaint()
	}
	panel, page := list.ScrollState(), area.ScrollState()
	r := list.Rect()
	if pageRect := area.Rect(); r.Y < pageRect.Y || r.Y+r.Height > pageRect.Y+pageRect.Height {
		t.Fatalf("the panel's list %v is not over the page area %v", r, pageRect)
	}
	x, y := r.X+r.Width/2, r.Y+r.Height/2

	wheel(x, y, 40)
	if panel.Offset != 40 || page.Offset != 0 {
		t.Fatalf("a wheel over the panel's list moved it to %v and the page under it to %v, want 40 and 0", panel.Offset, page.Offset)
	}
	wheel(x, y, 5000)
	if panel.Offset != panel.Max() {
		t.Fatalf("the panel's list is at %v, want its end %v", panel.Offset, panel.Max())
	}
	wheel(x, y, 60)
	if page.Offset != 0 {
		t.Errorf("a wheel over the panel's list at its end scrolled the page under the panel to %v", page.Offset)
	}
	padding := pageTheme.Metrics.PadSurface
	wheel(x, r.Y-padding/2, 60)
	if page.Offset != 0 {
		t.Errorf("a wheel over the panel's padding scrolled the page under the panel to %v", page.Offset)
	}
	pageRect := area.Rect()
	wheel(r.X+r.Width+padding+40, pageRect.Y+pageRect.Height-20, 30)
	if page.Offset != 30 {
		t.Errorf("a wheel over the page beside the panel moved it to %v, want 30", page.Offset)
	}
}

// A Table that scrolls itself takes the wheel inside a Popover as it does on the page,
// and one painted under the panel does not: past the panel's table's end the wheel
// stops at the panel. Beside the panel the page's table scrolls.
func TestATableInAPopoverTakesTheWheelAndTheTableUnderItDoesNot(t *testing.T) {
	rows := make(ui.TableRows, 60)
	for i := range rows {
		rows[i] = []string{"row"}
	}
	open := true
	var inner, under ui.TableState
	var panelTable, pageTable ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			projects := ui.Button(c, "Projects")
			ui.PopoverWith(c, projects, ui.PopoverSpec{Width: 200}, &open, func() {
				panelTable = ui.Table(c, &inner, ui.TableSpec{
					Columns: []components.TableColumn{{}}, Rows: gfx.Item(1).Scope(), Source: &rows,
				}).FillWidth().Height(150)
			})
			pageTable = ui.Table(c, &under, ui.TableSpec{
				Columns: []components.TableColumn{{}}, Rows: gfx.Item(2).Scope(), Source: &rows,
			}).FillWidth().Height(320)
		})
	})
	wheel := func(x, y, dy float32) {
		t.Helper()
		if err := d.driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: dy, Precise: true}); err != nil {
			t.Fatal(err)
		}
		d.repaint()
	}
	r, pageRect := panelTable.Rect(), pageTable.Rect()
	if r.Y < pageRect.Y || r.Y+r.Height > pageRect.Y+pageRect.Height {
		t.Fatalf("the panel's table %v is not over the page's %v", r, pageRect)
	}
	x, y := r.X+r.Width/2, r.Y+r.Height/2

	wheel(x, y, 40)
	if inner.Scroll.Offset != 40 || under.Scroll.Offset != 0 {
		t.Fatalf("a wheel over the panel's table moved it to %v and the table under the panel to %v, want 40 and 0", inner.Scroll.Offset, under.Scroll.Offset)
	}
	wheel(x, y, 50000)
	wheel(x, y, 60)
	if inner.Scroll.Offset != inner.Scroll.Max() || under.Scroll.Offset != 0 {
		t.Errorf("past its end the panel's table is at %v (end %v) and the table under the panel at %v, want 0", inner.Scroll.Offset, inner.Scroll.Max(), under.Scroll.Offset)
	}
	wheel(pageRect.X+pageRect.Width-20, pageRect.Y+pageRect.Height-20, 30)
	if under.Scroll.Offset != 30 {
		t.Errorf("a wheel over the page's table beside the panel moved it to %v, want 30", under.Scroll.Offset)
	}
}

// A Modal keeps the wheel from the page under its scrim, over its surface and beside it.
func TestAWheelOverAModalLeavesThePageUnderIt(t *testing.T) {
	open := true
	var area ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			area = ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				for range 40 {
					ui.Slot(c).Height(30)
				}
			})
		})
		ui.Modal(c, &open, "Rename", func() { ui.Text(c, "A short body") }, nil)
	})
	page := area.ScrollState()
	r := area.Rect()
	for _, at := range [][2]float32{{r.X + r.Width/2, r.Y + r.Height/2}, {r.X + 10, r.Y + r.Height - 10}} {
		if err := d.driver.Scroll(gfx.ScrollEvent{X: at[0], Y: at[1], DeltaY: 60, Precise: true}); err != nil {
			t.Fatal(err)
		}
		d.repaint()
		if page.Offset != 0 {
			t.Fatalf("a wheel at %v with a Modal open scrolled the page under it to %v", at, page.Offset)
		}
	}
}
