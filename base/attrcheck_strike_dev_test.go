//go:build dev || vivadev

package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Strike is the Text's own type, as Italic and Mono are. An element that draws no
// struck text of its own reports it in the dev build instead of dropping it.
func TestStrikeOnAnElementThatDrawsNoTextIsReported(t *testing.T) {
	drawBase(func(f *Frame) {
		f.Column().Children(func() {
			f.Text("paid").Strike()
		})
	})
	expectAttributePanic(t, func(f *Frame) {
		f.Column().Strike()
	}, "Column", "Strike")
	expectAttributePanic(t, func(f *Frame) {
		f.Button(gfx.Item(9), "Save").Strike()
	}, "Button", "Strike")
}
