package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Disabled is the declaration's as well as the snapshot's: a field disabled by a
// condition after it was declared paints dimmed, is not a tab stop and takes no
// press, as the other controls do.
func TestDisabledTextFieldMarksItsHitDisabled(t *testing.T) {
	var view View
	canvas := testCanvas()
	id := tree.ID(page, "disabled-field")
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
		f.Input(components.InputSnapshot{ID: id, Label: "Name", Value: "Gopher"}).Disabled(true)
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].ID != id {
		t.Fatalf("hits = %+v", hits)
	}
	if !hits[0].Disabled || hits[0].Focusable {
		t.Fatalf("a disabled field's hit = %+v, want disabled and not focusable", hits[0])
	}
}

// A field's own Disabled and ReadOnly are the snapshot's, and a read-only field has
// no caret; ReadOnly cleared leaves the caret a focused field paints.
func TestFieldStateReachesTheSnapshot(t *testing.T) {
	canvas := testCanvas()
	id := tree.ID(page, "readonly-field")
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, id, true)
	f := &Frame{canvas: canvas}
	field := Field{ReadOnly: true}
	snapshot := field.snapshot(f, id, "Identifier", "")
	if !snapshot.ReadOnly || snapshot.ShowCaret {
		t.Fatalf("read-only snapshot = %+v, want read-only without a caret", snapshot)
	}

	field.ReadOnly = false
	snapshot = field.snapshot(f, id, "Identifier", "")
	if !snapshot.ShowCaret {
		t.Fatal("an editable focused field paints no caret")
	}

	field.Disabled = true
	snapshot = field.snapshot(f, id, "Identifier", "")
	if !snapshot.Disabled || snapshot.ShowCaret {
		t.Fatalf("disabled snapshot = %+v, want disabled without a caret", snapshot)
	}
}

// ReadOnly is an option on the declared field, so a TextField bound to a string,
// which the program has no Field for, can be read-only: the platform is told, the
// hit stays focusable and no editor opens.
func TestReadOnlyTextFieldOptionReachesTheHit(t *testing.T) {
	var view View
	canvas := testCanvas()
	id := tree.ID(page, "readonly-option")
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
		f.Input(components.InputSnapshot{ID: id, Label: "Identifier", Value: "viva"}).ReadOnly(true)
	})
	hits := canvas.Hits()
	if len(hits) != 1 || !hits[0].ReadOnly || hits[0].Disabled {
		t.Fatalf("read-only hit = %+v, want read-only and enabled", hits)
	}
	if !hits[0].Focusable {
		t.Fatal("a read-only field is not focusable")
	}
}
