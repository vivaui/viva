package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// At generation wrap the Frame is retired, so a Box of the old generation never
// names a node of the new one.
func TestFrameGenerationWrapRetiresOldBoxes(t *testing.T) {
	var view View
	canvas := &gfx.Canvas{}
	var old Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		old = f.Row()
	})
	if !old.Valid() {
		t.Fatal("a Box is invalid before the wrap")
	}
	view.frame.gen = ^uint32(0)
	var fresh Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		fresh = f.Row()
	})
	if old.Valid() {
		t.Fatal("a Box survived the generation wrap")
	}
	if !fresh.Valid() {
		t.Fatal("a Box of the new generation is invalid")
	}
}
