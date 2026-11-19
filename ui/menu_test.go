package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type menuModel struct {
	open                     bool
	renamed, deleted, copied int
	locked                   bool
}

func (m *menuModel) page(c ui.C) {
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		more := ui.Button(c, "More").Outline()
		if more.Clicked() {
			m.open = !m.open
		}
		ui.Menu(c, more, &m.open, func(menu *ui.MenuItems) {
			menu.Label("Host")
			if menu.Item("Rename").Clicked() {
				m.renamed++
			}
			if menu.Item("Copy").Disabled(m.locked).Clicked() {
				m.copied++
			}
			menu.Separator()
			if menu.Item("Delete").Destructive().Clicked() {
				m.deleted++
			}
		})
		ui.Button(c, "After")
	})
}

// A Menu opens from its anchor, chooses an item by click or by the arrows and Enter,
// closes as it does, and gives the focus back to the anchor.
func TestMenuChoosesAnItem(t *testing.T) {
	m := &menuModel{}
	d := openPage(t, m.page)
	d.click("More")
	if !m.open || !d.has("Rename") || !d.page.Overlay() {
		t.Fatalf("open=%v shown=%v overlay=%v", m.open, d.has("Rename"), d.page.Overlay())
	}
	if more, menu := d.hit("More").Bounds, d.hit("Rename").Bounds; menu.Y < more.Y+more.Height {
		t.Fatalf("the menu at %v is not under its anchor at %v", menu, more)
	}
	d.click("Delete")
	if m.deleted != 1 || m.open || d.has("Rename") {
		t.Fatalf("deleted=%d open=%v", m.deleted, m.open)
	}

	d.click("More")
	d.key(gfx.KeyDown) // the first item
	if d.Focused() != "Rename" {
		t.Fatalf("Down from the anchor focused %q", d.Focused())
	}
	d.key(gfx.KeyUp) // loops to the last
	if d.Focused() != "Delete" {
		t.Fatalf("Up from the first focused %q", d.Focused())
	}
	d.key(gfx.KeyUp)
	d.key(gfx.KeyEnter)
	if m.copied != 1 || m.open {
		t.Fatalf("Enter on Copy: copied=%d open=%v", m.copied, m.open)
	}
	if err := d.driver.Advance(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d.Focused() != "More" {
		t.Fatalf("the focus went to %q, not back to the anchor", d.Focused())
	}
}

// Typing a letter in an open Menu goes to the item it begins, the same letter again
// to the next such item, and a pause starts the word anew.
func TestMenuTypeahead(t *testing.T) {
	m := &menuModel{}
	d := openPage(t, m.page)
	d.click("More")
	typed := func(letter rune, Want string) {
		t.Helper()
		// A window offers a printable key to OnShortcut when nothing edits text.
		if _, err := d.driver.ShortcutChord(0, letter); err != nil {
			t.Fatal(err)
		}
		if d.Focused() != Want {
			t.Fatalf("%q went to %q, not %q", letter, d.Focused(), Want)
		}
	}
	typed('d', "Delete")
	if err := d.driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	typed('r', "Rename")
	if err := d.driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	m.locked = true
	d.repaint()
	typed('c', "Rename") // Copy is disabled: no item begins with c
	d.key(gfx.KeyEnter)
	if m.renamed != 1 || m.copied != 0 {
		t.Fatalf("renamed=%d copied=%d", m.renamed, m.copied)
	}
}

// The arrows pass over a disabled item, and a click on it chooses nothing.
func TestMenuDisabledItem(t *testing.T) {
	m := &menuModel{locked: true}
	d := openPage(t, m.page)
	d.click("More")
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	if d.Focused() != "Delete" {
		t.Fatalf("Down from Rename went to %q, not past the disabled Copy", d.Focused())
	}
	d.click("Copy")
	if m.copied != 0 {
		t.Fatal("a disabled item was chosen")
	}
}

// Escape, Tab and a press outside close the menu and choose nothing.
func TestMenuCloses(t *testing.T) {
	for _, close := range []string{"escape", "tab", "outside"} {
		t.Run(close, func(t *testing.T) {
			m := &menuModel{}
			d := openPage(t, m.page)
			d.click("More")
			d.key(gfx.KeyDown)
			switch close {
			case "escape":
				d.key(gfx.KeyEscape)
			case "tab":
				d.key(gfx.KeyTab)
			case "outside":
				if err := d.driver.Click(5, 400); err != nil {
					t.Fatal(err)
				}
			}
			if m.open || m.renamed+m.deleted+m.copied != 0 {
				t.Fatalf("open=%v chosen=%d", m.open, m.renamed+m.deleted+m.copied)
			}
		})
	}
}

func BenchmarkMenuOpen(b *testing.B) {
	m := &menuModel{open: true}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	var now time.Duration
	canvas.SetClock(func() time.Duration { return now })
	b.ReportAllocs()
	for b.Loop() {
		now += 16 * time.Millisecond
		drawPage(page, canvas, m.page)
	}
}

type contextModel struct {
	hosts          []string
	connected      string
	wrap           bool
	open           bool
	renames, opens int
}

func (m *contextModel) page(c ui.C) {
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		for _, h := range m.hosts {
			row := ui.Button(c.Key(h), h)
			ui.ContextMenu(c, row, func(menu *ui.MenuItems) {
				if menu.Item("Connect").Clicked() {
					m.connected = h
				}
				if menu.Item("Rename").Keys(ui.Primary, "r").Clicked() {
					m.renames++
				}
				menu.Separator()
				if menu.Item("Wrap lines").Checked(m.wrap).Clicked() {
					m.wrap = !m.wrap
				}
			})
		}
	})
}

// A ContextMenu opens at the pointer on a secondary press of its target, acts for
// that target, and keeps the item kinds a menu has.
func TestContextMenuOpensAtThePointer(t *testing.T) {
	m := &contextModel{hosts: []string{"alpha", "beta"}}
	d := openPage(t, m.page)
	beta := d.hit("beta").Bounds
	x, y := beta.X+5, beta.Y+beta.Height/2
	if err := d.driver.Press(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	connect := d.hit("Connect").Bounds
	if !d.page.Overlay() || connect.X < x-1 || connect.Y < y-1 {
		t.Fatalf("overlay %v, the menu at %v is not at the pointer (%v, %v)", d.page.Overlay(), connect, x, y)
	}
	if hit := d.hit("Wrap lines"); hit.Checked {
		t.Fatal("an unchecked item reads as checked")
	}
	d.click("Wrap lines")
	if !m.wrap || d.has("Connect") {
		t.Fatalf("wrap %v, open %v", m.wrap, d.has("Connect"))
	}
	if err := d.driver.Press(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if !d.hit("Wrap lines").Checked {
		t.Fatal("a checked item does not read as checked")
	}
	d.click("Connect")
	if m.connected != "beta" {
		t.Fatalf("connected %q, not the row pressed", m.connected)
	}
}

// Shift+F10 opens the focused target's menu; Tab stays in it, Escape closes it.
func TestContextMenuKeyboard(t *testing.T) {
	m := &contextModel{hosts: []string{"alpha"}}
	d := openPage(t, m.page)
	d.click("alpha")
	if err := d.driver.Key(gfx.KeyF10, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if !d.has("Connect") {
		t.Fatal("Shift+F10 did not open the menu")
	}
	d.key(gfx.KeyDown)
	d.key(gfx.KeyTab)
	if !d.has("Connect") {
		t.Fatal("Tab closed a context menu")
	}
	d.key(gfx.KeyEscape)
	if d.has("Connect") {
		t.Fatal("Escape did not close the menu")
	}
}

type submenuModel struct {
	open  bool
	moved string
}

func (m *submenuModel) page(c ui.C) {
	ui.Column(c).Pad(16).Children(func() {
		more := ui.Button(c, "More").Outline()
		if more.Clicked() {
			m.open = !m.open
		}
		ui.Menu(c, more, &m.open, func(menu *ui.MenuItems) {
			menu.Item("Rename")
			menu.Sub("Move to", func(to *ui.MenuItems) {
				for _, group := range [...]string{"Production", "Staging"} {
					if to.Item(group).Clicked() {
						m.moved = group
					}
				}
			})
			menu.Item("Delete")
		})
	})
}

func (d *pageDriver) rest(Label string, for_ time.Duration) {
	d.t.Helper()
	if err := d.driver.Move(center(d.hit(Label).Bounds)); err != nil {
		d.t.Fatal(err)
	}
	if err := d.driver.Advance(for_); err != nil {
		d.t.Fatal(err)
	}
}

// A submenu opens beside its row as the pointer rests there, a sibling row the
// pointer rests on closes it, and choosing in it closes the whole menu.
func TestSubmenuPointer(t *testing.T) {
	m := &submenuModel{}
	d := openPage(t, m.page)
	d.click("More")
	d.rest("Move to", 150*time.Millisecond)
	if !d.has("Production") {
		t.Fatal("resting on the row did not open its submenu")
	}
	row, first := d.hit("Move to").Bounds, d.hit("Production").Bounds
	if first.X < row.X+row.Width-8 || first.Y+first.Height < row.Y || first.Y > row.Y+row.Height {
		t.Fatalf("the submenu at %v is not beside its row at %v", first, row)
	}
	d.rest("Rename", 150*time.Millisecond)
	if d.has("Production") {
		t.Fatal("resting on another row did not close the submenu")
	}
	d.click("Move to")
	if !d.has("Staging") {
		t.Fatal("a click on the row did not open its submenu")
	}
	d.click("Staging")
	if m.moved != "Staging" || m.open || d.has("Rename") {
		t.Fatalf("moved %q, menu open %v", m.moved, m.open)
	}
	// It opens again without the submenu, even in the next paint.
	m.open = true
	d.repaint()
	if !d.has("Rename") || d.has("Production") {
		t.Fatal("the menu opened again with its submenu")
	}
}

// Right on the row opens the submenu with its first row focused, Left goes back to
// the row, and Escape closes every menu.
func TestSubmenuKeyboard(t *testing.T) {
	m := &submenuModel{}
	d := openPage(t, m.page)
	d.click("More")
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	if d.Focused() != "Move to" {
		t.Fatalf("on %q", d.Focused())
	}
	d.key(gfx.KeyRight)
	if d.Focused() != "Production" {
		t.Fatalf("Right went to %q", d.Focused())
	}
	d.key(gfx.KeyDown)
	if d.Focused() != "Staging" {
		t.Fatalf("Down in the submenu went to %q", d.Focused())
	}
	d.key(gfx.KeyLeft)
	if d.Focused() != "Move to" || d.has("Production") {
		t.Fatalf("Left went to %q, submenu shown %v", d.Focused(), d.has("Production"))
	}
	d.key(gfx.KeyEnter)
	if d.Focused() != "Production" {
		t.Fatalf("Enter went to %q", d.Focused())
	}
	d.key(gfx.KeyEscape)
	if m.open || d.has("Rename") || d.has("Production") {
		t.Fatal("Escape did not close the menus")
	}
	if m.moved != "" {
		t.Fatalf("moved to %q", m.moved)
	}
}

// A ContextMenu's submenu behaves as a Menu's.
func TestContextSubmenu(t *testing.T) {
	var chosen string
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			row := ui.Button(c, "web-1")
			ui.ContextMenu(c, row, func(menu *ui.MenuItems) {
				menu.Item("Connect")
				menu.Sub("Copy", func(copy *ui.MenuItems) {
					if copy.Item("Address").Clicked() {
						chosen = "address"
					}
				})
			})
		})
	})
	r := d.hit("web-1").Bounds
	if err := d.driver.ClickButton(r.X+10, r.Y+10, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	d.rest("Copy", 150*time.Millisecond)
	d.click("Address")
	if chosen != "address" || d.has("Connect") {
		t.Fatalf("chose %q, menu shown %v", chosen, d.has("Connect"))
	}
}

// A menu with its submenu open repaints without allocating.
func TestSubmenuRepaintAllocates(t *testing.T) {
	m := &submenuModel{}
	d := openPage(t, m.page)
	d.click("More")
	d.rest("Move to", 150*time.Millisecond)
	if !d.has("Production") {
		t.Fatal("no submenu")
	}
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// menuOptionsModel exercises the menu calls that shape its surface: a second line
// under an item, its width range, and opening over or beside the anchor.
type menuOptionsModel struct {
	open            bool
	above           bool
	beside          bool
	least, greatest float32
	description     string
	Label           string
	picked          int
}

func (m *menuOptionsModel) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		if m.above {
			ui.Slot(c).Height(200).Shrink(0)
		}
		more := ui.Button(c, "More")
		if more.Clicked() {
			m.open = !m.open
		}
		ui.Menu(c, more, &m.open, func(menu *ui.MenuItems) {
			menu.WidthRange(m.least, m.greatest)
			if m.above {
				menu.Above()
			}
			if m.beside {
				menu.Beside()
			}
			label := m.Label
			if label == "" {
				label = "Model"
			}
			item := menu.Item(label)
			if m.description != "" {
				item.Description(m.description)
			}
			if item.Clicked() {
				m.picked++
			}
			menu.Item("Other")
		})
	})
}

// An item with a description paints it under the label, grows its row, and still
// chooses the one action.
func TestMenuItemDescription(t *testing.T) {
	m := &menuOptionsModel{description: "What answers"}
	d := openPage(t, m.page)
	d.click("More")
	if len(d.textsNamed("What answers")) == 0 {
		t.Fatal("the description was not painted")
	}
	if d.hit("Model").Bounds.Height <= d.hit("Other").Bounds.Height {
		t.Fatalf("the described row is %v tall, the plain one %v", d.hit("Model").Bounds.Height, d.hit("Other").Bounds.Height)
	}
	d.click("Model")
	if m.picked != 1 || m.open {
		t.Fatalf("picked=%d open=%v", m.picked, m.open)
	}
}

// WidthRange sets the surface's least and greatest width; rows that do not fit the
// greatest are still one hit with their whole label.
func TestMenuWidthRange(t *testing.T) {
	t.Run("least", func(t *testing.T) {
		m := &menuOptionsModel{least: 220}
		d := openPage(t, m.page)
		d.click("More")
		if got := d.hit("Model").Bounds.Width; got < 200 {
			t.Fatalf("the surface is %v wide, want at least the 220 it was given", got)
		}
	})
	t.Run("greatest", func(t *testing.T) {
		m := &menuOptionsModel{greatest: 160, Label: "A model name that is far too long for a narrow menu"}
		d := openPage(t, m.page)
		d.click("More")
		if got := d.hit(m.Label).Bounds.Width; got > 160 {
			t.Fatalf("the surface is %v wide, want at most the 160 it was given", got)
		}
	})
}

// Beside opens the menu right of its anchor, its first row level with the anchor
// rather than under it, and a menu that stops asking for it opens under it again.
func TestMenuBeside(t *testing.T) {
	m := &menuOptionsModel{beside: true}
	d := openPage(t, m.page)
	d.click("More")
	anchor, item := d.hit("More").Bounds, d.hit("Model").Bounds
	if item.X < anchor.X+anchor.Width || item.Y > anchor.Y+anchor.Height {
		t.Fatalf("the menu at %v is not beside its anchor at %v", item, anchor)
	}
	d.key(gfx.KeyEscape)
	m.beside = false
	d.click("More")
	if item := d.hit("Model").Bounds; item.Y < anchor.Y+anchor.Height {
		t.Fatalf("the menu at %v is not under its anchor at %v once Beside is gone", item, anchor)
	}
}

// Above puts the menu over the anchor even where it would fit under it.
func TestMenuAbove(t *testing.T) {
	m := &menuOptionsModel{above: true}
	d := openPage(t, m.page)
	d.click("More")
	anchor, item := d.hit("More").Bounds, d.hit("Model").Bounds
	if item.Y+item.Height > anchor.Y+0.5 {
		t.Fatalf("the menu at %v is not over its anchor at %v", item, anchor)
	}
}

type tallMenuModel struct {
	open   bool
	chosen string
}

// A menu taller than the window scrolls: the keyboard reaches its last item and
// brings it into view, the wheel moves it, and choosing the last item works.
func TestMenuTallerThanTheWindowScrollsToItsLastItem(t *testing.T) {
	m := &tallMenuModel{open: true}
	names := []string{"One", "Two", "Three", "Four", "Five", "Six", "Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve"}
	d := openPageWith(t, ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			anchor := ui.Button(c, "More")
			ui.Menu(c, anchor, &m.open, func(menu *ui.MenuItems) {
				for _, name := range names {
					if menu.Item(name).Clicked() {
						m.chosen = name
					}
				}
			})
		})
	}}, app.Config{Width: 300, Height: 160})
	window := gfx.Rect{Width: 300, Height: 160}
	inside := func(Label string) bool {
		for _, hit := range d.driver.Snapshot().Hits {
			if hit.Label == Label && hit.Role == gfx.RoleMenuItem {
				return hit.Bounds.Y >= 0 && hit.Bounds.Y+hit.Bounds.Height <= window.Height
			}
		}
		return false
	}
	if !inside("One") || inside("Twelve") {
		t.Fatalf("the first item is shown %v and the last %v before anything scrolled", inside("One"), inside("Twelve"))
	}
	d.key(gfx.KeyDown)
	d.key(gfx.KeyEnd)
	if !inside("Twelve") {
		t.Fatal("End did not bring the last item into view")
	}
	d.click("Twelve")
	if m.chosen != "Twelve" || m.open {
		t.Fatalf("chosen %q, open %v", m.chosen, m.open)
	}
}

// A context menu opened at a point is modal: the arrows move through it while the
// focus is still on the control that was focused when it opened.
func TestContextMenuAtTakesTheArrowsFromTheFocusedControl(t *testing.T) {
	m := &contextModel{hosts: []string{"alpha"}}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Button(c, "Anchor")
			ui.ContextMenuAt(c, ui.Button(c, "Header"), &m.open, gfx.Point{X: 100, Y: 100}, func(Items *ui.MenuItems) {
				Items.Item("One")
				if Items.Item("Two").Clicked() {
					m.connected = "two"
				}
			})
		})
	})
	d.click("Anchor")
	m.open = true
	d.repaint()
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	d.key(gfx.KeyEnter)
	if m.connected != "two" || m.open {
		t.Fatalf("connected %q, open %v: the arrows did not reach the menu", m.connected, m.open)
	}
}

func TestContextMenuAtNamedByAZeroSizeSlotTakesTheArrows(t *testing.T) {
	m := &contextModel{hosts: []string{"alpha"}}
	d := openPage(t, func(c ui.C) {
		ui.PopupLayer(c, 600)
		anchor := ui.Slot(c).Absolute().Left(100).Top(100).Size(0, 0).HitWith(ui.ID(c), "", gfx.HitOptions{SkipFocus: true, SkipTab: true})
		ui.ContextMenuAt(c, anchor, &m.open, gfx.Point{X: 100, Y: 100}, func(Items *ui.MenuItems) {
			Items.RowHeight(24)
			Items.Item("One")
			if Items.Item("Two").Clicked() {
				m.connected = "two"
			}
		})
	})
	m.open = true
	d.repaint()
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	d.key(gfx.KeyEnter)
	if m.connected != "two" || m.open {
		t.Fatalf("connected %q, open %v: the arrows did not reach the menu", m.connected, m.open)
	}
}

// A Content row takes room in the menu and is painted by the caller; the arrows pass over it.
func TestMenuContentRow(t *testing.T) {
	m := &menuModel{open: true}
	painted := gfx.Rect{}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			more := ui.Button(c, "More")
			ui.Menu(c, more, &m.open, func(menu *ui.MenuItems) {
				menu.Item("One")
				menu.Content(40, func(Surface *gfx.Canvas, bounds gfx.Rect, Palette gfx.Theme) {
					painted = bounds
					Surface.Hit(gfx.Item(77), bounds)
				})
				if menu.Item("Two").Clicked() {
					m.renamed++
				}
			})
		})
	})
	one, two := d.hit("One").Bounds, d.hit("Two").Bounds
	if painted.Height != 40 || two.Y-one.Y < 40 {
		t.Fatalf("content %v, rows %v and %v: the row took no room", painted, one, two)
	}
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	d.key(gfx.KeyEnter)
	if m.renamed != 1 {
		t.Fatalf("the arrows did not pass over the content row to Two (%d)", m.renamed)
	}
}

// A Trailing control has a hit of its own at the row's end: pressing it does not choose the row.
func TestMenuItemTrailingControl(t *testing.T) {
	m := &menuModel{open: true}
	var eye gfx.Rect
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			more := ui.Button(c, "More")
			ui.Menu(c, more, &m.open, func(menu *ui.MenuItems) {
				if menu.Item("One").Trailing(24, func(Surface *gfx.Canvas, bounds gfx.Rect, Palette gfx.Theme) {
					eye = bounds
					Surface.HitWithOptions(gfx.Item(88), bounds, "Hide", gfx.HitOptions{Role: gfx.RoleButton})
				}).Clicked() {
					m.renamed++
				}
			})
		})
	})
	row := d.hit("One").Bounds
	if eye.Width != 24 || eye.X+eye.Width > row.X+row.Width {
		t.Fatalf("trailing slot %v not at the end of row %v", eye, row)
	}
	d.click("Hide")
	if m.renamed != 0 {
		t.Fatal("pressing the trailing control chose the row")
	}
}

// A ContentPage row takes room in the menu and declares its controls in the page:
// a swatch there is clicked without closing the menu.
func TestMenuPageContentRow(t *testing.T) {
	chosen := false
	open := true
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			more := ui.Button(c, "More")
			ui.Menu(c, more, &open, func(menu *ui.MenuItems) {
				menu.Item("One")
				menu.ContentPage(40, func(c ui.C, _ ui.Rect) {
					if ui.ColorSwatch(c, gfx.RGB(220, 38, 38), chosen, "Red").Clicked() {
						chosen = true
					}
				})
				menu.Item("Two")
			})
		})
	})
	one, two := d.hit("One").Bounds, d.hit("Two").Bounds
	if two.Y-one.Y < 40 {
		t.Fatalf("the page row took no room: One %v, Two %v", one, two)
	}
	d.click("Red")
	if !chosen || !open {
		t.Fatalf("the swatch chose %v, menu open %v", chosen, open)
	}
}

// MenuItem.ID is the row's identity: a caller can anchor a context menu or a
// part to the row it declared, without knowing the menu's numbering.
func TestMenuItemIDNamesItsRow(t *testing.T) {
	open := true
	var row gfx.ID
	d := openPage(t, func(c ui.C) {
		anchor := ui.Button(c, "More")
		ui.Menu(c, anchor, &open, func(menu *ui.MenuItems) {
			row = menu.Item("Rename").ID()
		})
	})
	if row.IsZero() {
		t.Fatal("MenuItem.ID is zero")
	}
	if hit := d.hit("Rename"); hit.ID != row {
		t.Fatalf("the row's hit is %v, ID says %v", hit.ID, row)
	}
}
