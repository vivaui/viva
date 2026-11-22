//go:build !dev && !vivadev

package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A look on an element with no identity changes nothing: no state can be
// reported for it, and release builds ignore the look instead of failing.
func TestAnElementWithoutIdentityIgnoresItsLooks(t *testing.T) {
	canvas := styleCanvas(styleRow, styleRow, styleRow, true)
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		box = f.Row().Size(50, 50).Background(colorRest).Hover(Look{}.Background(colorHover)).Focus(Look{}.Ring(3, colorRing))
	})
	if got := fillOf(t, canvas, box); got != colorRest {
		t.Errorf("fill is %+v, want the resting one", got)
	}
	if count, _, _ := ringTriangles(canvas); count != 0 {
		t.Error("a ring was painted")
	}
}
