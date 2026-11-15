package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A captured header opens the same context-menu rows as a hit target, at the
// pointer. At a window edge the surface clamps instead of flipping to the row's
// other side, so the rows that stay visible do not reorder their geometry.
func TestContextMenuAtKeepsItsOriginAndDenseRows(t *testing.T) {
	m := &contextModel{hosts: []string{"alpha"}}
	var menu *ui.MenuItems
	at := gfx.Point{X: 45, Y: 267}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Button(c, "alpha")
			ui.ContextMenuAt(c, ui.Button(c, "Header"), &m.open, at, func(items *ui.MenuItems) {
				menu = items
				items.RowHeight(24)
				menu.Item("One")
				menu.Item("Two")
				menu.Separator()
				if menu.Item("Connect").Clicked() {
					m.connected = "alpha"
				}
			})
		})
	})
	if m.open {
		t.Fatal("a closed menu must not declare its rows")
	}
	m.open = true
	d.repaint()
	first, second := d.hit("One").Bounds, d.hit("Two").Bounds
	if first.X != at.X+4 || first.Y != at.Y+4 || first.Height != 24 || second.Y-first.Y != 24 {
		t.Fatalf("dense rows are %v and %v, want origin %v plus the surface inset", first, second, at)
	}
	if menu == nil {
		t.Fatal("the menu did not expose its public per-menu options")
	}
	d.click("Connect")
	if m.connected != "alpha" || m.open {
		t.Fatalf("choosing Connect left connected=%q open=%v", m.connected, m.open)
	}

	m.open, at = true, gfx.Point{X: 45, Y: 147}
	d.repaint()
	first, second = d.hit("One").Bounds, d.hit("Two").Bounds
	if first.Y != at.Y+4 || second.Y-first.Y != 24 {
		t.Fatalf("the menu flipped above the pointer: %v and %v, want %v and 24px apart", first, second, at)
	}
}
