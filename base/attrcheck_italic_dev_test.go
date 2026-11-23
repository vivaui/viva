//go:build dev || vivadev

package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Italic is the Text's own type. An element that shapes no italic text reports it
// in the dev build instead of dropping it, as it reports Mono on an element that
// paints no monospace text.
func TestItalicOnAnElementThatShapesNoTextIsReported(t *testing.T) {
	drawBase(func(f *Frame) {
		f.Column().Children(func() {
			f.Text("emphasis").Italic()
		})
	})
	expectAttributePanic(t, func(f *Frame) {
		f.Column().Italic()
	}, "Column", "Italic")
	expectAttributePanic(t, func(f *Frame) {
		f.Button(gfx.Item(9), "Save").Italic()
	}, "Button", "Italic")
}
