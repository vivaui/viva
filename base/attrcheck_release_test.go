//go:build !dev && !vivadev

package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Release builds do not run the attribute rules: a declaration that the dev
// build reports is declared as before (the attribute has no effect), so tests
// and applications that rely on that keep building and running.
func TestAttributesThatDoNothingAreNotCheckedInRelease(t *testing.T) {
	var view View
	view.Draw(styleCanvas(gfx.ID{}, gfx.ID{}, gfx.ID{}, false), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
		f.Stack().Gap(4)
		f.Row().Hover(Look{}.Background(colorHover))
		f.Checkbox(gfx.Item(1), true).Pad(4)
	})
}
