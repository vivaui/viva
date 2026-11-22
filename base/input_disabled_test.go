package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Disabled is the declaration's as well as the snapshot's: a field disabled by a
// condition after it was declared is not a tab stop and takes no press, as the other
// controls do.
func TestDisabledFieldMarksItsHitDisabled(t *testing.T) {
	var view View
	canvas := testCanvas()
	id := gfx.Item(5)
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
		f.Input(InputSnapshot{ID: id, Label: "Name", Value: "Gopher"}).Disabled(true)
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].ID != id {
		t.Fatalf("hits = %+v", hits)
	}
	if !hits[0].Disabled || hits[0].Focusable {
		t.Fatalf("a disabled field's hit = %+v, want disabled and not focusable", hits[0])
	}
}
