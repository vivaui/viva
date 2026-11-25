package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// A Field places its value where Alignment says: a centred value starts further
// in than one at the field's start.
func TestFieldAlignmentPlacesTheValue(t *testing.T) {
	valueX := func(align gfx.Alignment) float32 {
		t.Helper()
		var editor text.Editor
		editor.Insert("30")
		field := ui.Field{Alignment: align}
		field.Bind(&editor)
		d := openPage(t, func(c ui.C) {
			ui.TextField(c, &field, "Steps", "", ui.WithID(ui.ID(c))).Size(64, 32)
		})
		for _, painted := range d.driver.Snapshot().Texts {
			if painted.Value == "30" {
				return painted.Bounds.X
			}
		}
		t.Fatal("the value was not painted")
		return 0
	}
	left, centre := valueX(gfx.AlignStart), valueX(gfx.AlignCenter)
	if centre <= left {
		t.Fatalf("a centred value starts at %v, not after the left one at %v", centre, left)
	}
}
