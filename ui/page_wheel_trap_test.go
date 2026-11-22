package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A focus trap owns the wheel as it owns the pointer: while a dialog is open, a
// scroll area painted outside its scope takes no wheel, wherever it lies under
// the pointer, and the dialog's own area still scrolls. Once the dialog is gone
// the page's areas scroll again.
func TestWheelOverAnAreaBehindATrappedDialogGoesNowhere(t *testing.T) {
	var tree gfx.Tree
	root := tree.Feature("wheel-trap")
	scope := tree.Sub(root, tree.Name("scope"))
	var behind, inside ui.Box
	open := true
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			behind = ui.Scroll(c).Grow(1).Children(func() {
				ui.Slot(c).Height(2000).Shrink(0)
			})
		})
		if !open {
			return
		}
		dialog := ui.Dialog(c, ui.DialogSpec{Title: "Trap", Scope: scope.ID(tree.Name("scope")), Width: 320, Height: 300})
		dialog.Body(func() {
			inside = ui.Scroll(c).Height(80).Shrink(0).Children(func() {
				ui.Slot(c).Height(1000).Shrink(0)
			})
		})
	}
	page := ui.NewPage(ui.PageConfig{Tree: &tree, Root: "wheel-trap", View: view})
	canvas := benchCanvas()
	drawPage(page, canvas, view)

	offset := func(b ui.Box) float32 { return b.ScrollState().Offset }
	at := func(b ui.Box) gfx.ScrollEvent {
		r := b.Rect()
		return gfx.ScrollEvent{X: r.X + 10, Y: r.Y + 10, DeltaY: 60, Precise: true}
	}

	// The page's area is under the pointer at its top-left, away from the centred
	// dialog surface.
	drawPage(page, canvas, view)
	page.Wheel(at(behind))
	drawPage(page, canvas, view)
	if offset(behind) != 0 {
		t.Fatalf("a wheel over the page's area behind an open dialog scrolled it to %v", offset(behind))
	}

	page.Wheel(at(inside))
	drawPage(page, canvas, view)
	if offset(inside) == 0 {
		t.Fatal("the dialog's own area did not take the wheel")
	}

	open = false
	drawPage(page, canvas, view)
	page.Wheel(at(behind))
	drawPage(page, canvas, view)
	if offset(behind) == 0 {
		t.Fatal("the page's area did not scroll again once the dialog closed")
	}
}
