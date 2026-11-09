package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// TextFieldID edits an editor the application keeps, under the identity the
// application gave it: the page's input routing and a test address that name,
// and what is typed reaches the same editor the application reads.
func TestTextFieldIDEditsTheBoundEditorUnderItsExplicitName(t *testing.T) {
	editor := text.NewEditor("")
	var field ui.Field
	field.Bind(editor)
	search := gfx.Item(9)
	d := openPage(t, func(c ui.C) {
		ui.TextFieldID(c, search, &field, "Search", "").NavigationKeys()
	})
	hit := d.hit("Search")
	if hit.ID != search {
		t.Fatalf("hit identity = %v, want the explicit %v", hit.ID, search)
	}
	if !hit.NavigationKeys {
		t.Fatal("NavigationKeys did not reach the field's hit")
	}
	d.click("Search")
	if d.focused() != "Search" {
		t.Fatalf("focus = %q", d.focused())
	}
	d.typeText("hello")
	if editor.String() != "hello" || field.Text() != "hello" {
		t.Fatalf("editor %q, field %q", editor.String(), field.Text())
	}
}

// A bound field kept in the model repaints without allocating, and reports a change
// only for an edit.
func TestTextFieldIDKeptInTheModelIsStable(t *testing.T) {
	editor := text.NewEditor("")
	var field ui.Field
	field.Bind(editor)
	search := gfx.Item(9)
	changed := 0
	d := openPage(t, func(c ui.C) {
		if ui.TextFieldID(c, search, &field, "Search", "").Changed() {
			changed++
		}
	})
	d.click("Search")
	d.typeText("a")
	d.typeText("b")
	if changed != 2 {
		t.Fatalf("Changed reported %d times for 2 edits", changed)
	}
	before := changed
	d.repaint()
	d.repaint()
	if changed != before {
		t.Fatalf("Changed reported on repaints without an edit: %d", changed-before)
	}
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// What the editor holds when it is bound is not an edit; one typed after is.
func TestFieldBindIsNotAnEdit(t *testing.T) {
	editor := text.NewEditor("saved")
	var field ui.Field
	field.Bind(editor)
	search := gfx.Item(9)
	changed := 0
	d := openPage(t, func(c ui.C) {
		if ui.TextFieldID(c, search, &field, "Search", "").Changed() {
			changed++
		}
	})
	d.repaint()
	if changed != 0 {
		t.Fatalf("binding an editor holding text reported a change %d times", changed)
	}
	d.click("Search")
	d.typeText("!")
	if changed != 1 || editor.String() != "saved!" {
		t.Fatalf("changed %d, editor %q", changed, editor.String())
	}
}
