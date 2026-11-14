package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// A Field bound to a multi-selection editor paints every selection: adding one
// to the editor shows up as another band of selection color, without the field
// being told anything but the editor.
func TestFieldPaintsMultipleSelections(t *testing.T) {
	editor := text.NewEditor("abcdef")
	var field ui.Field
	field.Bind(editor)
	search := gfx.Item(9)
	view := func(c ui.C) {
		ui.TextFieldID(c, search, &field, "Search", "").Width(200)
	}
	d := openPage(t, view)
	bands := func() int {
		d.repaint()
		count := 0
		for _, primitive := range d.driver.Snapshot().Primitives {
			if primitive.Kind == app.RectanglePrimitive && primitive.Color == pageTheme.Selection {
				count++
			}
		}
		return count
	}
	editor.SetSelection(text.Range{Start: 0, End: 1})
	if got := bands(); got != 1 {
		t.Fatalf("one selection painted %d bands", got)
	}
	editor.AddSelection(text.Range{Start: 3, End: 5})
	if got := bands(); got != 2 {
		t.Fatalf("two selections painted %d bands", got)
	}
	editor.ClearSecondarySelections()
	if got := bands(); got != 1 {
		t.Fatalf("after clearing: %d bands", got)
	}
}

// The multi-selection paint is allocation-free like the single-selection one:
// the field copies the editor's spans into its own scratch.
func TestFieldMultiSelectionPaintAllocatesNothing(t *testing.T) {
	editor := text.NewEditor("abcdef")
	editor.SetSelections([]text.Range{{Start: 0, End: 1}, {Start: 3, End: 5}}, 1)
	var field ui.Field
	field.Bind(editor)
	search := gfx.Item(9)
	d := openPage(t, func(c ui.C) {
		ui.TextFieldID(c, search, &field, "Search", "").Width(200)
	})
	d.repaint()
	if allocations := testing.AllocsPerRun(100, d.repaint); allocations != 0 {
		t.Fatalf("%v allocations a repaint", allocations)
	}
}
