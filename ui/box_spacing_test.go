package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A single-edge form changes only its side: the pair forms still set theirs,
// and a later call sets just the side it names.
func TestSingleEdgePaddingKeepsTheOtherSides(t *testing.T) {
	var view View
	var child, after Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100})
	f.Column().Children(func() {
		f.Column().PadX(3).PadTop(2).PadBottom(4).PadLeft(5).Children(func() {
			child = f.Slot().Height(10)
		})
		after = f.Slot().Height(10)
	})
	f.End()
	near(t, "padded child", child.Rect(), gfx.Rect{X: 5, Y: 2, Width: 92, Height: 10})
	near(t, "after", after.Rect(), gfx.Rect{X: 0, Y: 16, Width: 100, Height: 10})
}

// The four single-edge margin forms place the element and leave its siblings to
// follow the margins, as the Sides form does.
func TestSingleEdgeMarginKeepsTheOtherSides(t *testing.T) {
	var view View
	var child, after Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100})
	f.Column().Children(func() {
		child = f.Slot().Height(10).MarginTop(6).MarginRight(7).MarginBottom(8).MarginLeft(9)
		after = f.Slot().Height(10)
	})
	f.End()
	near(t, "inset child", child.Rect(), gfx.Rect{X: 9, Y: 6, Width: 84, Height: 10})
	near(t, "after", after.Rect(), gfx.Rect{X: 0, Y: 24, Width: 100, Height: 10})
}
