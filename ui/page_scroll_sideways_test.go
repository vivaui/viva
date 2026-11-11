package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ui.Scroll scrolls sideways on a page as a Frame's ScrollArea does: the page's wheel
// moves it across, and a steady paint with its scrollbars allocates nothing.
func TestPageSidewaysScrollTakesTheWheelAndAllocatesNothing(t *testing.T) {
	var row ui.Box
	var across float32
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			area := ui.Scroll(c).Grow(1).Sideways().Scrollbars().Children(func() {
				row = ui.Slot(c).Width(1500).Height(30)
			})
			across = area.ScrollState().OffsetX
		})
	}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 2 {
		drawPage(page, canvas, view)
	}
	result := page.Wheel(gfx.ScrollEvent{X: 10, Y: 10, DeltaY: 100, Precise: true, Modifiers: gfx.ModifierShift})
	if !result.Handled || !result.Repaint {
		t.Fatalf("Shift with the wheel over the area gave %+v, want it handled and repainted", result)
	}
	drawPage(page, canvas, view)
	if row.Rect().X != -100 || across != 100 {
		t.Errorf("the row is at x %v and OffsetX is %v, want -100 and 100", row.Rect().X, across)
	}
	wheel := gfx.ScrollEvent{X: 10, Y: 10, DeltaX: 1, Precise: true}
	if allocs := testing.AllocsPerRun(20, func() { drawPage(page, canvas, view); page.Wheel(wheel) }); allocs != 0 {
		t.Fatalf("a steady paint of a sideways area allocates %v times", allocs)
	}
}
