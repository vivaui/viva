package ui_test

import (
	"testing"
	"time"

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
	if d.focused() != "Rename" {
		t.Fatalf("Down from the anchor focused %q", d.focused())
	}
	d.key(gfx.KeyUp) // loops to the last
	if d.focused() != "Delete" {
		t.Fatalf("Up from the first focused %q", d.focused())
	}
	d.key(gfx.KeyUp)
	d.key(gfx.KeyEnter)
	if m.copied != 1 || m.open {
		t.Fatalf("Enter on Copy: copied=%d open=%v", m.copied, m.open)
	}
	if err := d.driver.Advance(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d.focused() != "More" {
		t.Fatalf("the focus went to %q, not back to the anchor", d.focused())
	}
}

// Typing a letter in an open Menu goes to the item it begins, the same letter again
// to the next such item, and a pause starts the word anew.
func TestMenuTypeahead(t *testing.T) {
	m := &menuModel{}
	d := openPage(t, m.page)
	d.click("More")
	typed := func(letter rune, want string) {
		t.Helper()
		// A window offers a printable key to OnShortcut when nothing edits text.
		if _, err := d.driver.ShortcutChord(0, letter); err != nil {
			t.Fatal(err)
		}
		if d.focused() != want {
			t.Fatalf("%q went to %q, not %q", letter, d.focused(), want)
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
	if d.focused() != "Delete" {
		t.Fatalf("Down from Rename went to %q, not past the disabled Copy", d.focused())
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

func (d *pageDriver) rest(label string, for_ time.Duration) {
	d.t.Helper()
	if err := d.driver.Move(center(d.hit(label).Bounds)); err != nil {
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
	if d.focused() != "Move to" {
		t.Fatalf("on %q", d.focused())
	}
	d.key(gfx.KeyRight)
	if d.focused() != "Production" {
		t.Fatalf("Right went to %q", d.focused())
	}
	d.key(gfx.KeyDown)
	if d.focused() != "Staging" {
		t.Fatalf("Down in the submenu went to %q", d.focused())
	}
	d.key(gfx.KeyLeft)
	if d.focused() != "Move to" || d.has("Production") {
		t.Fatalf("Left went to %q, submenu shown %v", d.focused(), d.has("Production"))
	}
	d.key(gfx.KeyEnter)
	if d.focused() != "Production" {
		t.Fatalf("Enter went to %q", d.focused())
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
