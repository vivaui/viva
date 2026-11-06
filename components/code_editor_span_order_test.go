package components

import (
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// The spans are documented as sorted and disjoint, but the paint walks them with
// one forward cursor, so an out-of-order caller must still get every caret and
// every selection rather than silently losing the ones after a backwards step.
func TestCodeEditorPaintsOutOfOrderExtraCaretsAndSelectionRanges(t *testing.T) {
	fixture := newCodeFixture("abc\ndef\nghi")
	fixture.snapshot.Caret = 0
	fixture.snapshot.CaretColor = ui.RGB(12, 34, 56)
	// Rows out of order, and two carets on the same row out of order.
	fixture.snapshot.ExtraCarets = []CodeEditorCaret{{Offset: 9}, {Offset: 1}, {Offset: 5}, {Offset: 6}}
	fixture.paint()
	cell, _, lineHeight := fixture.cell()
	carets := fixture.fills(fixture.snapshot.CaretColor)
	// The primary caret at offset 0 plus the four secondary ones.
	if len(carets) != 5 {
		t.Fatalf("caret fills = %d (%+v), want the primary plus every secondary", len(carets), carets)
	}
	wantRows := []float32{0, 0, 1, 1, 2}
	seen := make(map[float32]int)
	for _, caret := range carets {
		row := (caret.Y - fixture.editor.content.Y) / lineHeight
		seen[float32(int(row+0.5))]++
	}
	for _, row := range wantRows {
		if seen[row] == 0 {
			t.Fatalf("no caret painted on row %v; fills = %+v", row, carets)
		}
	}
	// Column 1 on row 0 is the secondary caret at offset 1.
	if !near(carets[1].X, fixture.editor.content.X+cell) && !near(carets[2].X, fixture.editor.content.X+cell) {
		t.Fatalf("offset 1 caret is not at column 1: %+v", carets)
	}

	unsorted := newCodeFixture("abcdef")
	unsorted.snapshot.SelectionRanges = []text.Span{{Start: 3, End: 5}, {Start: 0, End: 1}}
	unsorted.paint()
	single := newCodeFixture("abcdef")
	single.snapshot.SelectionRanges = []text.Span{{Start: 0, End: 1}, {Start: 3, End: 5}}
	single.paint()
	if got, want := len(unsorted.fills(unsorted.theme.Selection)), len(single.fills(single.theme.Selection)); got != want {
		t.Fatalf("out-of-order spans painted %d selections, sorted ones %d", got, want)
	}
}

// A span that runs past the text, or one that is empty or reversed, must not
// paint anything rather than reaching beyond the last row.
func TestCodeEditorClampsSelectionRangesPastTheText(t *testing.T) {
	fixture := newCodeFixture("abc\ndef")
	fixture.snapshot.SelectionRanges = []text.Span{{Start: 0, End: 9999}}
	fixture.paint()
	fills := fixture.fills(fixture.theme.Selection)
	if len(fills) == 0 {
		t.Fatal("clamped span painted nothing at all")
	}
	for _, fill := range fills {
		if fill.X+fill.Width > fixture.editor.content.X+fixture.editor.content.Width+1 {
			t.Fatalf("clamped selection ran past the content: %+v", fill)
		}
	}

	// A reversed span is a forward selection, exactly as a reversed
	// SelectionStart/SelectionEnd pair is: both go through textareaRange. Only a
	// span with nothing in it selects nothing.
	empty := newCodeFixture("abc\ndef")
	empty.snapshot.SelectionRanges = []text.Span{{Start: 2, End: 2}}
	empty.paint()
	if fills := empty.fills(empty.theme.Selection); len(fills) != 0 {
		t.Fatalf("an empty span painted %+v", fills)
	}

	reversed := newCodeFixture("abcdef")
	reversed.snapshot.SelectionRanges = []text.Span{{Start: 4, End: 1}}
	reversed.paint()
	same := newCodeFixture("abcdef")
	same.snapshot.SelectionRanges = []text.Span{{Start: 1, End: 4}}
	same.paint()
	if got, want := len(reversed.fills(reversed.theme.Selection)), len(same.fills(same.theme.Selection)); got != want || want == 0 {
		t.Fatalf("reversed span painted %d selections, the same span forward %d", got, want)
	}
}
