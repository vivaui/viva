package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
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

func TestAlertDialogKeepsTheWheelFromTheTableBehindIt(t *testing.T) {
	var tree gfx.Tree
	scope := tree.Sub(tree.Feature("alert-wheel"), tree.Name("alert"))
	rows := make(ui.TableRows, 60)
	for i := range rows {
		rows[i] = []string{"row"}
	}
	var state ui.TableState
	var table ui.Box
	open := true
	d := openPageWith(t, ui.PageConfig{Tree: &tree, View: func(c ui.C) {
		table = ui.Table(c, &state, ui.TableSpec{
			Columns: []components.TableColumn{{}}, Rows: tree.Sub(scope, tree.Name("rows")), Source: &rows,
		}).Fill()
		if open {
			ui.AlertDialog(c, ui.AlertDialogSpec{
				Title: "Discard changes?", Description: "This cannot be undone.",
				Scope: scope.At(0), ActionID: scope.At(1), CancelID: scope.At(2),
				ActionLabel: "Discard", CancelLabel: "Cancel",
			})
		}
	}}, app.Config{})
	for _, at := range [][2]float32{{240, 240}, {10, 470}} {
		if err := d.driver.Scroll(gfx.ScrollEvent{X: at[0], Y: at[1], DeltaY: 60, Precise: true}); err != nil {
			t.Fatal(err)
		}
		if state.Scroll.Offset != 0 {
			t.Errorf("a wheel at %v through the AlertDialog moved the table to %v", at, state.Scroll.Offset)
		}
	}
	open = false
	d.repaint()
	x, y := center(table.Rect())
	if err := d.driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: 60, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if state.Scroll.Offset != 60 {
		t.Errorf("after closing the AlertDialog the table is at %v, want 60", state.Scroll.Offset)
	}
}

// A non-modal drawer owns only its surface; a modal drawer owns its scrim too.
// Both keep a wheel over their header, footer or exhausted body off the page.
func TestDrawerKeepsTheWheelOverItsPanel(t *testing.T) {
	for _, nonModal := range []bool{false, true} {
		name := "modal"
		if nonModal {
			name = "non-modal"
		}
		t.Run(name, func(t *testing.T) {
			open := true
			var behind ui.TableState
			rows := make(ui.TableRows, 60)
			for i := range rows {
				rows[i] = []string{"row"}
			}
			var inside, footer ui.Box
			d := openPage(t, func(c ui.C) {
				ui.Table(c, &behind, ui.TableSpec{
					Columns: []components.TableColumn{{}}, Rows: gfx.Item(1).Scope(), Source: &rows,
				}).Fill()
				ui.Drawer(c, &open, "Inspector", ui.DrawerSpec{Size: 240, NonModal: nonModal}, func() {
					inside = ui.Scroll(c).FillWidth().Height(120).Shrink(0).Children(func() {
						ui.Slot(c).Height(1000).Shrink(0)
					})
				}, func() { footer = ui.Button(c, "Done") })
			})
			if err := d.driver.Advance(time.Second); err != nil {
				t.Fatal(err)
			}
			wheel := func(x, y, dy float32) {
				t.Helper()
				if err := d.driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: dy, Precise: true}); err != nil {
					t.Fatal(err)
				}
				d.repaint()
			}
			x, y := center(inside.Rect())
			wheel(x, y, 40)
			if got := inside.ScrollState().Offset; got != 40 || behind.Scroll.Offset != 0 {
				t.Fatalf("a wheel in the drawer moved its body to %v and the page to %v, want 40 and 0", got, behind.Scroll.Offset)
			}
			wheel(x, y, 5000)
			wheel(x, y, 60)
			wheel(x, 10, 60) // header, outside the scroll body
			fx, fy := center(footer.Rect())
			wheel(fx, fy, 60)
			if got := inside.ScrollState(); got.Offset != got.Max() || behind.Scroll.Offset != 0 {
				t.Errorf("past the drawer body's end: body %v (max %v), page %v, want page 0", got.Offset, got.Max(), behind.Scroll.Offset)
			}
			wheel(10, 240, 30)
			want := float32(0)
			if nonModal {
				want = 30
			}
			if behind.Scroll.Offset != want {
				t.Errorf("beside the drawer: page offset %v, want %v", behind.Scroll.Offset, want)
			}
		})
	}
}

// These widgets have wheel routes of their own, outside wheelArea's focus-trap
// filter. Each still scrolls inside a dialog, and resumes on the page after close.
func TestDialogKeepsTheWheelFromIndependentRoutesBehindIt(t *testing.T) {
	for _, kind := range []string{"table", "textarea", "sideways-list"} {
		t.Run(kind, func(t *testing.T) {
			var boxes [2]ui.Box
			var tables [2]ui.TableState
			var fields [2]ui.Field
			lists := [2]ui.ListState{{RowWidth: 800}, {RowWidth: 800}}
			for i := range fields {
				fields[i].SetText(strings.Repeat("a line of text\n", 60))
				fields[i].Select(0, 0)
			}
			rows := make(ui.TableRows, 60)
			for i := range rows {
				rows[i] = []string{"row"}
			}
			declare := func(c ui.C, i int) ui.Box {
				switch kind {
				case "table":
					return ui.Table(c, &tables[i], ui.TableSpec{
						Columns: []components.TableColumn{{}}, Rows: gfx.Item(i + 1).Scope(), Source: &rows,
					})
				case "textarea":
					return ui.TextArea(c, &fields[i], "Notes", "")
				default:
					return ui.List(c, &lists[i], 60, func(row ui.ListRow) {
						row.Element.Height(30).Children(func() { ui.Text(row.C, "row") })
					})
				}
			}
			offset := func(i int) float32 {
				switch kind {
				case "table":
					return tables[i].Scroll.Offset
				case "textarea":
					return fields[i].GetArea().Scroll
				default:
					return lists[i].ScrollX()
				}
			}
			open := true
			d := openPage(t, func(c ui.C) {
				boxes[0] = declare(c, 0).Fill()
				ui.Modal(c, &open, "Details", func() {
					boxes[1] = declare(c, 1).FillWidth().Height(150).Shrink(0)
				}, nil)
			})
			if err := d.driver.Advance(time.Second); err != nil {
				t.Fatal(err)
			}
			wheel := func(x, y, delta float32) {
				t.Helper()
				event := gfx.ScrollEvent{X: x, Y: y, DeltaY: delta, Precise: true}
				if kind == "sideways-list" {
					event.DeltaX, event.DeltaY = delta, 0
				}
				if err := d.driver.Scroll(event); err != nil {
					t.Fatal(err)
				}
				d.repaint()
			}
			x, y := center(boxes[1].Rect())
			wheel(x, y, 40)
			if offset(1) != 40 || offset(0) != 0 {
				t.Fatalf("a wheel over the dialog moved its %s to %v and the page to %v, want 40 and 0", kind, offset(1), offset(0))
			}
			wheel(x, y, 50000)
			wheel(x, y, 60)
			wheel(10, 470, 60) // scrim
			if offset(0) != 0 {
				t.Errorf("a wheel past the dialog's %s or over its scrim moved the page to %v", kind, offset(0))
			}
			open = false
			d.repaint()
			x, y = center(boxes[0].Rect())
			wheel(x, y, 60)
			if offset(0) != 60 {
				t.Errorf("after closing the dialog the page's %s is at %v, want 60", kind, offset(0))
			}
		})
	}
}

// Padding and exhausted panel areas consume the wheel before the host fallback.
// ScrollAt is the exception: its owner still receives the event once.
func TestPopoverWheelFallbackBelongsOnlyToScrollAt(t *testing.T) {
	open, manual := true, false
	var area ui.Box
	var page *ui.Page
	calls := 0
	d := openPageWith(t, ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			anchor := ui.Button(c, "Projects")
			ui.Popover(c, anchor, &open, func() {
				if manual {
					area = ui.ScrollAt(c, 0)
				} else {
					area = ui.Scroll(c)
				}
				area.FillWidth().Height(100).Children(func() { ui.Slot(c).Height(1000) })
			})
		})
	}}, app.Config{OnScroll: func(event gfx.ScrollEvent) gfx.EventResult {
		calls++
		return gfx.EventResult{Handled: true}
	}})
	page = d.page
	x, y := center(area.Rect())
	event := gfx.ScrollEvent{X: x, Y: y, DeltaY: 5000, Precise: true}
	page.Wheel(event)
	d.repaint()
	for _, at := range []gfx.ScrollEvent{event, {X: x, Y: area.Rect().Y - 2, DeltaY: 60, Precise: true}} {
		if result := page.RouteWheel(at); !result.Handled || result.Repaint {
			t.Errorf("RouteWheel at an exhausted panel or its padding returned %+v", result)
		}
		if result := page.Wheel(at); !result.Handled || calls != 0 {
			t.Errorf("Wheel over the panel returned %+v with %d fallback calls", result, calls)
		}
	}
	manual = true
	d.repaint()
	if result := page.RouteWheel(event); result.Handled || calls != 0 {
		t.Fatalf("RouteWheel over ScrollAt returned %+v with %d fallback calls", result, calls)
	}
	if result := page.Wheel(event); !result.Handled || calls != 1 {
		t.Fatalf("Wheel over ScrollAt returned %+v with %d fallback calls, want 1", result, calls)
	}
}
