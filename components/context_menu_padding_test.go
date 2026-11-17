package components

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// A menu remembers the row the pointer focused. That row must stay unlit while
// the pointer is anywhere else, including the menu's own padding and separators,
// which declare no hit of their own: a pointer travelling over them is on the
// menu, not on the page behind it.
func TestMenuRowStaysUnlitOverTheMenusOwnPadding(t *testing.T) {
	var tree gfx.Tree
	feature := tree.Feature("menu-padding")
	scope := gfx.Item(64)
	theme := gfx.DarkTheme()
	var rows []gfx.Rect
	var menuBounds, separator gfx.Rect
	driver, err := app.NewDriver(app.Config{
		Width: 320, Height: 240, Tree: &tree,
		Paint: func(c *gfx.Canvas, size gfx.Size) {
			menu := BeginContextMenu(c, theme, gfx.Rect{X: 20, Y: 20, Width: 200, Height: 160}, ContextMenuOptions{
				Layer: 7, Scope: scope, DismissID: tree.ID(feature, "dismiss"), Viewport: gfx.Rect{Width: 320, Height: 240},
			})
			menuBounds = menu.Bounds()
			rows = rows[:0]
			for i := 0; i < 4; i++ {
				rows = append(rows, menu.AddItem(ContextMenuItem{ID: tree.ID(feature, string(rune('a'+i))), Label: "Row"}))
			}
			separator = menu.AddSeparator()
			menu.End()
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	move := func(x, y float32) {
		t.Helper()
		if err := driver.Move(x, y); err != nil {
			t.Fatal(err)
		}
	}
	row := rows[3]
	rowTinted := func() bool {
		t.Helper()
		for _, primitive := range driver.Snapshot().Primitives {
			if primitive.Color == theme.Accent && primitive.Bounds == row {
				return true
			}
		}
		return false
	}
	// Hovering the row makes it the menu's current row.
	move(row.X+row.Width/2, row.Y+row.Height/2)
	move(row.X+row.Width/2+1, row.Y+row.Height/2)
	if !rowTinted() {
		t.Fatal("hovering the row did not tint it: the test cannot see the state it checks")
	}

	// The pointer leaves the menu entirely.
	move(300, 230)
	if rowTinted() {
		t.Fatal("the row is still tinted with the pointer outside the menu")
	}

	// It comes back over the menu's top padding, which is not a row.
	move(menuBounds.X+40, menuBounds.Y+2)
	if rowTinted() {
		t.Fatal("the row relit as the pointer entered the menu's padding")
	}

	// And over the separator, also not a row. The pointer lights a row first, so
	// the frame after the move is a repaint that had the row lit: the assertion is
	// about the separator putting it out, not about the padding result again.
	if separator.Height <= 0 {
		t.Fatal("the menu laid out no separator")
	}
	move(row.X+row.Width/2, row.Y+row.Height/2)
	move(row.X+row.Width/2+1, row.Y+row.Height/2)
	if !rowTinted() {
		t.Fatal("the row did not relight before the separator: the test cannot see the state it checks")
	}
	separatorY := separator.Y + separator.Height/2
	if separatorY < row.Y+row.Height || separatorY >= row.Y+row.Height+separator.Height {
		t.Fatalf("the separator's y %v is not inside the separator row %+v", separatorY, separator)
	}
	move(separator.X+40, separatorY)
	if rowTinted() {
		t.Fatal("the row relit as the pointer crossed the menu's separator")
	}

	// Returning to the row lights it again.
	move(row.X+row.Width/2, row.Y+row.Height/2)
	move(row.X+row.Width/2+1, row.Y+row.Height/2)
	if !rowTinted() {
		t.Fatal("the row is no longer tinted after the pointer returned to it")
	}
}

// A key press moves the menu's current row while the pointer rests anywhere,
// the menu's padding included. That row must stay lit: the surface only says
// where the pointer is, it does not take the menu away from the keyboard.
func TestMenuRowKeepsVisibleKeyboardFocusOverTheMenusOwnPadding(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	// paint is one frame. The pointer rests, so the caller paints twice: the first
	// frame is the one that crossed onto the spot, and the second is the frame that
	// rests there, which is the frame the rule is about.
	paint := func(hovered gfx.ID, role gfx.Role, focused gfx.ID, onSurface, focusVisible bool) int {
		t.Helper()
		canvas.Reset()
		canvas.SetInteraction(hovered, gfx.ID{}, focused, focusVisible)
		canvas.SetHoveredRole(role)
		canvas.SetPointerOnSurface(onSurface)
		menu := BeginContextMenu(&canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 220, Height: 160}, ContextMenuOptions{
			Layer: 7, Scope: gfx.Item(64),
		})
		menu.AddItem(ContextMenuItem{ID: gfx.Item(11), Label: "One"})
		menu.AddItem(ContextMenuItem{ID: gfx.Item(12), Label: "Two"})
		menu.End()
		tinted := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Accent && rectangle.Radius == theme.Metrics.RadiusItem() {
				tinted++
			}
		}
		return tinted
	}
	rest := func(hovered gfx.ID, role gfx.Role, focused gfx.ID, onSurface, focusVisible bool) int {
		t.Helper()
		paint(hovered, role, focused, onSurface, focusVisible)
		return paint(hovered, role, focused, onSurface, focusVisible)
	}

	// The pointer rests on the padding (no hit at all) and the keyboard moved the
	// current row: Down must leave that row lit.
	if got := rest(gfx.ID{}, 0, gfx.Item(11), true, true); got != 1 {
		t.Fatalf("the keyboard's row tinted %d rows with the pointer on the padding", got)
	}
	// The same pointer position without visible focus lights nothing: the surface
	// still keeps a remembered row from relighting.
	if got := rest(gfx.ID{}, 0, gfx.Item(11), true, false); got != 0 {
		t.Fatalf("a row with no visible focus tinted %d rows over the padding", got)
	}
	// Caller content under the pointer is no different: visible keyboard focus keeps
	// its row lit wherever the pointer rests.
	if got := rest(gfx.Item(99), gfx.RoleRadio, gfx.Item(11), false, true); got != 1 {
		t.Fatalf("the keyboard's row tinted %d rows with the pointer on a radio", got)
	}
	// And without visible focus that same frame lights nothing, which is the
	// behaviour the colour row depends on.
	if got := rest(gfx.Item(99), gfx.RoleRadio, gfx.Item(11), false, false); got != 0 {
		t.Fatalf("a row without visible focus tinted %d rows over a radio", got)
	}
}
