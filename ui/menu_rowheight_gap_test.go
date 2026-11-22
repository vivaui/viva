package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A menu opened by a trigger is a dropdown, not a context menu, and its rows stay
// the theme's control height whatever RowHeight asks for.
//
// Two halves of the same gap:
//
//   - ui/menu_paint.go's rowExtent reads GetRowSize() only inside `if m.GetContext()`
//     (the context menu), and its dropdown branch calls
//     components.DropdownMenuItemHeight, which has no row height at all.
//   - the dropdown branch of the painter never hands a row height to
//     components.DropdownMenuOptions, which has no field to carry one — even though
//     the menuSurfaceOptions underneath it has read `rowHeight` all along
//     (components/dropdown_menu.go: itemHeight / menuItemHeight).
//
// This is the same request TestContextMenuRowHeightOptionPacksItemsDenser makes of
// a context menu, one menu kind over, which is why it is a fair thing to ask for.
func TestDropdownMenuTakesTheRowHeight(t *testing.T) {
	open := true
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			more := ui.Button(c, "More")
			ui.Menu(c, more, &open, func(Items *ui.MenuItems) {
				Items.RowHeight(24)
				Items.Item("One")
				Items.Item("Two")
			})
		})
	})
	one, two := d.hit("One").Bounds, d.hit("Two").Bounds
	if one.Height != 24 {
		t.Fatalf("row is %v high, want the 24 RowHeight asked for (a dropdown row keeps the theme's height)", one.Height)
	}
	if got := two.Y - one.Y; got != 24 {
		t.Fatalf("rows are %v apart, want 24", got)
	}
}
