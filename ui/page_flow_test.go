package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// ui.Flow wraps on a page as Frame.Flow does, and a steady paint allocates nothing.
func TestPageFlowWrapsAndAllocatesNothing(t *testing.T) {
	var cells [12]ui.Box
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.Flow(c).Gap(10).LineGap(5).Children(func() {
				for i := range cells {
					cells[i] = ui.Slot(c).Width(90).Height(20)
				}
			})
		})
	}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 2 {
		drawPage(page, canvas, view)
	}
	// 600 wide: six 90 wide slots 10 apart take 590, so two lines of six.
	if got := cells[5].Rect(); got.X != 500 || got.Y != 0 {
		t.Errorf("the sixth slot is at %+v, want x=500 on the first line", got)
	}
	if got := cells[6].Rect(); got.X != 0 || got.Y != 25 {
		t.Errorf("the seventh slot is at %+v, want x=0 on the second line, y=25", got)
	}
	if allocs := testing.AllocsPerRun(20, func() { drawPage(page, canvas, view) }); allocs != 0 {
		t.Fatalf("a steady paint of a flow allocates %v times", allocs)
	}
}
