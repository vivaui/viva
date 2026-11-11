package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A Popover's InitialFocusID asks for the keyboard as the panel opens, not on
// every paint, and asks again after the panel was closed or moved its target.
func TestPopoverRequestsInitialFocusOnce(t *testing.T) {
	var view View
	canvas := &gfx.Canvas{}
	theme := gfx.LightTheme()
	bounds := gfx.Rect{Width: 300, Height: 200}
	anchorID, scope, focusID := gfx.Item(2), gfx.Item(3), gfx.Item(4)
	otherID := gfx.Item(5)
	draw := func(open bool, id gfx.ID) {
		canvas.Reset()
		view.Draw(canvas, theme, bounds, func(f *Frame) {
			anchor := f.Button(anchorID, "More")
			if open {
				f.Popover(anchor, PopoverSpec{Scope: scope, InitialFocusID: id})
			}
		})
	}

	draw(true, focusID)
	if got := canvas.FocusRequest(); got != focusID {
		t.Fatalf("the open panel asks to focus %v, want %v", got, focusID)
	}
	draw(true, focusID)
	if got := canvas.FocusRequest(); !got.IsZero() {
		t.Fatalf("the panel asked again while it stayed open: %v", got)
	}
	// A panel that moves its InitialFocusID while it stays open, as a bulk
	// action appearing over a list does, asks once more for the new target.
	draw(true, otherID)
	if got := canvas.FocusRequest(); got != otherID {
		t.Fatalf("the panel that changed its target asks to focus %v, want %v", got, otherID)
	}
	draw(true, otherID)
	if got := canvas.FocusRequest(); !got.IsZero() {
		t.Fatalf("the panel asked again for the target it already has: %v", got)
	}
	draw(false, focusID)
	draw(true, focusID)
	if got := canvas.FocusRequest(); got != focusID {
		t.Fatalf("the reopened panel asks to focus %v, want %v", got, focusID)
	}
}
