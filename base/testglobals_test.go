package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

var (
	tree   = gfx.Tree{}
	page   = tree.Feature("flex")
	save   = tree.ID(page, "save")
	omit   = tree.ID(page, "cancel")
	rowID  = tree.ID(page, "row")
	listID = tree.ID(page, "list")
)

func testCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	return canvas
}

func near(t *testing.T, Name string, got, Want gfx.Rect) {
	t.Helper()
	d := func(a, b float32) bool { return a-b > 0.01 || b-a > 0.01 }
	if d(got.X, Want.X) || d(got.Y, Want.Y) || d(got.Width, Want.Width) || d(got.Height, Want.Height) {
		t.Errorf("%s = %+v, want %+v", Name, got, Want)
	}
}
