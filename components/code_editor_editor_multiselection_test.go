package components

import (
	"testing"

	"github.com/vivaui/viva/text"
)

// A code editor driven from a text.Editor's selections paints every one of
// them: SelectionSpansInto is what the snapshot's SelectionRanges is for.
func TestCodeEditorPaintsAnEditorsEverySelection(t *testing.T) {
	editor := text.NewEditor("abc\ndef\nghi")
	editor.SetSelections([]text.Range{{Start: 1, End: 2}, {Start: 5, End: 6}, {Start: 9, End: 10}}, 1)
	fixture := newCodeFixture(editor.String())
	fixture.snapshot.SelectionRanges = editor.SelectionSpans()
	fixture.snapshot.Caret = editor.ByteOffset(editor.Caret())
	fixture.paint()
	fills := fixture.fills(fixture.theme.Selection)
	if len(fills) != 3 {
		t.Fatalf("selection fills = %+v, want one per selection", fills)
	}
	_, _, rowHeight := fixture.cell()
	cell, _, _ := fixture.cell()
	for index, fill := range fills {
		if !near(fill.X, fixture.editor.content.X+cell) || !near(fill.Y, fixture.editor.content.Y+float32(index)*rowHeight) || !near(fill.Width, cell) {
			t.Fatalf("selection row %d = %+v", index, fill)
		}
	}
	// The primary is the middle range, and only it has a caret.
	if bounds, ok := fixture.editor.CaretBounds(); !ok || !near(bounds.Y, fixture.editor.content.Y+rowHeight) {
		t.Fatalf("caret = %+v, %v", bounds, ok)
	}
}

// Two selections on one line paint as two bands, and the caret sits on the
// primary one.
func TestCodeEditorPaintsTwoEditorSelectionsOnOneLine(t *testing.T) {
	editor := text.NewEditor("abcdef")
	editor.SetSelections([]text.Range{{Start: 0, End: 1}, {Start: 3, End: 5}}, 1)
	fixture := newCodeFixture(editor.String())
	fixture.snapshot.SelectionRanges = editor.SelectionSpans()
	fixture.snapshot.Caret = editor.ByteOffset(editor.Caret())
	fixture.paint()
	cell, _, _ := fixture.cell()
	fills := fixture.fills(fixture.theme.Selection)
	if len(fills) != 2 || !near(fills[0].Width, cell) || !near(fills[1].X, fixture.editor.content.X+3*cell) || !near(fills[1].Width, 2*cell) {
		t.Fatalf("two selections = %+v", fills)
	}
	if bounds, ok := fixture.editor.CaretBounds(); !ok || !near(bounds.X, fixture.editor.content.X+5*cell) {
		t.Fatalf("caret = %+v, %v, want the primary's end", bounds, ok)
	}
}

// SelectionSpans and Selections describe the same set, so a view that converts
// through the editor and one that converts rune indices agree.
func TestSelectionSpansMatchSelections(t *testing.T) {
	editor := text.NewEditor("a中b")
	editor.SetSelections([]text.Range{{Start: 0, End: 1}, {Start: 2, End: 3}}, 0)
	spans := editor.SelectionSpans()
	if len(spans) != 2 || spans[0] != (text.Span{Start: 0, End: 1}) || spans[1] != (text.Span{Start: 4, End: 5}) {
		t.Fatalf("spans = %+v", spans)
	}
	dst := make([]text.Span, 0, 2)
	if allocations := testing.AllocsPerRun(100, func() { dst = editor.SelectionSpansInto(dst) }); allocations != 0 {
		t.Fatalf("SelectionSpansInto allocated %v times with a reused scratch", allocations)
	}
}
