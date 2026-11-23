package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A dropdown menu sizes its rows by DropdownMenuOptions.RowHeight, the way a
// context menu sizes its own by ContextMenuOptions.RowHeight, and falls back to
// the theme's control height when it is zero.
func TestDropdownMenuRowsFollowTheRowHeightOption(t *testing.T) {
	compact := gfx.LightTheme()
	compact.Metrics = compact.Metrics.WithDensity(gfx.DensityCompact)
	for _, test := range []struct {
		name      string
		theme     gfx.Theme
		rowHeight float32
		want      float32
	}{
		{"default density", gfx.LightTheme(), 0, 32},
		{"compact density", compact, 0, 26},
		{"RowHeight wins over the theme", compact, 24, 24},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			menu := BeginDropdownMenu(&canvas, test.theme, gfx.Rect{X: 10, Y: 20, Width: 160, Height: 400}, DropdownMenuOptions{
				Layer: 1, Scope: gfx.Item(2), DismissID: gfx.Item(3), RowHeight: test.rowHeight,
			})
			first := menu.AddItem(DropdownMenuItem{ID: gfx.Item(11), Label: "One"})
			second := menu.AddItem(DropdownMenuItem{ID: gfx.Item(12), Label: "Two"})
			if first.Height != test.want || second.Height != test.want || menu.NextItemBounds().Height != test.want {
				t.Fatalf("rows = %+v %+v next %+v, want %v high", first, second, menu.NextItemBounds(), test.want)
			}
			if got := DropdownMenuItemHeightFor(test.theme, DropdownMenuItem{Label: "One"}, test.rowHeight); got != test.want {
				t.Fatalf("DropdownMenuItemHeightFor = %v, want %v: the helper a caller lays a menu out with must agree with AddItem", got, test.want)
			}
		})
	}
}

// The option packs the rows the menu reserves, not just the ones it paints: a
// separator still takes its own height, and the next row starts right below it.
func TestDropdownMenuRowHeightOptionPacksItemsDenser(t *testing.T) {
	var canvas gfx.Canvas
	menu := BeginDropdownMenu(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 160, Height: 4*24 + 9 + 8}, DropdownMenuOptions{
		Layer: 1, Scope: gfx.Item(2), DismissID: gfx.Item(3), RowHeight: 24,
	})
	first := menu.AddItem(DropdownMenuItem{ID: gfx.Item(11), Label: "One"})
	menu.AddSeparator()
	second := menu.AddItem(DropdownMenuItem{ID: gfx.Item(12), Label: "Two"})
	third := menu.AddItem(DropdownMenuItem{ID: gfx.Item(13), Label: "Three"})
	if first.Height != 24 || second.Y != first.Y+24+DropdownMenuSeparatorHeight || third.Y != second.Y+24 || menu.NextItemBounds().Height != 24 {
		t.Fatalf("rows = %+v %+v %+v next %+v", first, second, third, menu.NextItemBounds())
	}
}
