package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A menu opened against a rectangle of the application's own surface opens
// below it, flips above it at the bottom of the window, and works as a Menu.
func TestMenuAtOpensAgainstARectangle(t *testing.T) {
	open, renamed := true, false
	anchor := ui.Rect{X: 40, Y: 40, Width: 80, Height: 24}
	d := openPage(t, func(c ui.C) {
		ui.Text(c, "surface")
		ui.MenuAt(c, &open, anchor, func(menu *ui.MenuItems) {
			if menu.Item("Rename").Clicked() {
				renamed = true
			}
			menu.Item("Delete")
		})
	})
	if item := d.hit("Rename"); item.Bounds.Y < anchor.Y+anchor.Height {
		t.Fatalf("a menu under room opened at %v, over its anchor %v", item.Bounds, anchor)
	}

	anchor.Y = 450 // the window is 480 tall: no room below
	d.repaint()
	if item := d.hit("Delete"); item.Bounds.Y+item.Bounds.Height > anchor.Y {
		t.Fatalf("a menu at the bottom opened at %v, not above its anchor %v", item.Bounds, anchor)
	}
	d.click("Rename")
	if !renamed || open {
		t.Fatalf("choosing an item: renamed %v, still open %v", renamed, open)
	}
}
