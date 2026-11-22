package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Pagination paints the component's items and reports the one a click chose;
// a disabled control and the current page declare no target.
func TestPaginationReportsTheItemChosen(t *testing.T) {
	var chosen gfx.ID
	items := []ui.PaginationItem{
		{ID: gfx.Item(1), Label: "Previous", Kind: ui.PaginationPrevious, Disabled: true},
		{Label: "3", Kind: ui.PaginationPage, Current: true},
		{ID: gfx.Item(2), Label: "Next", Kind: ui.PaginationNext},
	}
	var row ui.Box
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(10).Children(func() {
			row = ui.Pagination(c, items)
			if id, ok := row.Chosen(); ok {
				chosen = id
			}
		})
	}
	d := openPage(t, view)
	if d.has("Previous") {
		t.Error("a disabled previous control declared a target")
	}
	if d.has("3") {
		t.Error("the current page declared a target")
	}
	next := d.hit("Next")
	if next.Role != gfx.RoleButton {
		t.Errorf("Next is a %v, want a button", next.Role)
	}
	if next.Bounds.X+next.Bounds.Width > row.Rect().X+row.Rect().Width+0.01 {
		t.Errorf("Next ends at %v, past the row's %v", next.Bounds.X+next.Bounds.Width, row.Rect().X+row.Rect().Width)
	}
	d.click("Next")
	if chosen != gfx.Item(2) {
		t.Errorf("a click on Next reported %v, want its identity", chosen)
	}
}
