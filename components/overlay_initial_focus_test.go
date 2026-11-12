package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A sheet, drawer, and confirmation can name the control that takes focus when
// they open, as a dialog already can.
func TestSheetDrawerAndAlertDialogSetInitialFocus(t *testing.T) {
	var sheetCanvas gfx.Canvas
	BeginSheet(&sheetCanvas, gfx.LightTheme(), gfx.Rect{Width: 800, Height: 600}, SheetOptions{
		Scope: gfx.Item(1), DismissID: gfx.Item(2), InitialFocusID: gfx.Item(3),
	}).Paint("Title", "Description")
	if sheetCanvas.InitialFocus() != gfx.Item(3) {
		t.Fatalf("sheet initial focus = %v", sheetCanvas.InitialFocus())
	}

	var drawerCanvas gfx.Canvas
	BeginDrawer(&drawerCanvas, gfx.LightTheme(), gfx.Rect{Width: 800, Height: 600}, DrawerOptions{
		Scope: gfx.Item(4), DismissID: gfx.Item(5), InitialFocusID: gfx.Item(6),
	}).Paint("Title", "Description")
	if drawerCanvas.InitialFocus() != gfx.Item(6) {
		t.Fatalf("drawer initial focus = %v", drawerCanvas.InitialFocus())
	}

	var alertCanvas gfx.Canvas
	BeginAlertDialog(&alertCanvas, gfx.LightTheme(), gfx.Rect{Width: 800, Height: 600}, AlertDialogOptions{
		Scope: gfx.Item(7), ActionID: gfx.Item(8), CancelID: gfx.Item(9), InitialFocusID: gfx.Item(8),
	}).Paint("Title", "Description", "Delete", "Cancel")
	if alertCanvas.InitialFocus() != gfx.Item(8) {
		t.Fatalf("alert initial focus = %v", alertCanvas.InitialFocus())
	}
}
