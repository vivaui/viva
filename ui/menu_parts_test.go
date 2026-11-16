package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// An unpainted menu keeps its behavior and leaves every visible part to the
// application: the surface, the rows, and the row rectangles the app paints in.
func TestAnUnpaintedMenuDrawsItsOwnPartsAndKeepsItsHits(t *testing.T) {
	theme := gfx.LightTheme()
	panelColor := gfx.RGB(20, 30, 40)
	rowColor := gfx.RGB(50, 60, 70)
	page := NewPage(PageConfig{Theme: &theme})
	canvas := testCanvas()
	open := true
	var rows []MenuItemParts
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 300}, func(c C) {
		anchor := Button(c, "Open")
		Menu(c, anchor, &open, func(menu *MenuItems) {
			menu.Unpainted().
				DrawPanel(func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme) {
					canvas.Fill(bounds, panelColor)
				}).
				DrawItem(func(parts MenuItemParts) {
					rows = append(rows, parts)
					parts.Canvas.Fill(parts.Rect, rowColor)
				})
			menu.Item("One")
			menu.Item("Two")
		})
	})
	if len(rows) != 2 || rows[0].Label != "One" || rows[1].Label != "Two" {
		t.Fatalf("rows = %+v, want two actions", rows)
	}
	if rows[0].Rect.Height <= 0 {
		t.Errorf("the first row has no rectangle: %+v", rows[0].Rect)
	}
	painted := make(map[gfx.Color]bool)
	for _, rectangle := range canvas.Rectangles() {
		painted[rectangle.Color] = true
	}
	if !painted[panelColor] || !painted[rowColor] {
		t.Fatalf("the menu's parts were not painted: %d rectangles", len(canvas.Rectangles()))
	}
	items := 0
	for _, hit := range canvas.Hits() {
		if hit.Role == gfx.RoleMenuItem {
			items++
		}
	}
	if items != 2 {
		t.Fatalf("%d menu item hits, want 2", items)
	}
}
