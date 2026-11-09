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
