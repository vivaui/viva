package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A Popover's InitialFocusID asks for the keyboard as the panel opens, not on
// every paint, and asks again only after the panel was closed.
func TestPopoverRequestsInitialFocusOnce(t *testing.T) {
	var view View
	canvas := &gfx.Canvas{}
	theme := gfx.LightTheme()
	bounds := gfx.Rect{Width: 300, Height: 200}
	anchorID, scope, focusID := gfx.Item(2), gfx.Item(3), gfx.Item(4)
	draw := func(open bool) {
		canvas.Reset()
		view.Draw(canvas, theme, bounds, func(f *Frame) {
			anchor := f.Button(anchorID, "More")
			if open {
				f.Popover(anchor, PopoverSpec{Scope: scope, InitialFocusID: focusID})
			}
		})
	}

	draw(true)
	if got := canvas.FocusRequest(); got != focusID {
		t.Fatalf("the open panel asks to focus %v, want %v", got, focusID)
	}
	draw(true)
	if got := canvas.FocusRequest(); !got.IsZero() {
		t.Fatalf("the panel asked again while it stayed open: %v", got)
	}
	draw(false)
	draw(true)
	if got := canvas.FocusRequest(); got != focusID {
		t.Fatalf("the reopened panel asks to focus %v, want %v", got, focusID)
	}
}
