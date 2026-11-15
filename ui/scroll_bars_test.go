package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Box.Bars hands the style to the area's own state, which is what the paint of the bars
// reads, so a product whose bars are its own measures them itself. The area is declared
// without Box.Scrollbars here: this checks the plumbing, and the paint path reads the
// same field.
func TestBarsReachTheAreasScrollState(t *testing.T) {
	var view View
	var canvas gfx.Canvas
	style := components.ScrollBarStyle{Size: 11, HoverSize: 5, Inset: 1, MinThumb: 20}
	view.Draw(&canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		f.Column().Fill().Children(func() {
			b := f.ScrollArea(listID).Grow(1).Bars(style)
			state := b.areaState()
			if state == nil {
				t.Fatal("the scroll area has no state")
			}
			if state.barStyle != style {
				t.Fatalf("bar style = %+v, want %+v", state.barStyle, style)
			}
			f.Slot().Height(10)
		})
	})
}

// An area that sets no style keeps Viva's own geometry, so nothing changes for a page
// that never asks for its own bars.
func TestBarsDefaultToTheSharedGeometry(t *testing.T) {
	var view View
	var canvas gfx.Canvas
	view.Draw(&canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		f.Column().Fill().Children(func() {
			b := f.ScrollArea(listID).Grow(1)
			state := b.areaState()
			if state == nil {
				t.Fatal("the scroll area has no state")
			}
			if state.barStyle != (components.ScrollBarStyle{}) {
				t.Fatalf("bar style = %+v, want the zero style", state.barStyle)
			}
			f.Slot().Height(10)
		})
	})
}
