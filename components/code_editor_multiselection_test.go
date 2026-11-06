package components

import (
	"testing"

	"github.com/vivaui/viva/text"
)

func TestCodeEditorPaintsDisjointSelectionSpansWithoutSelectingGaps(t *testing.T) {
	fixture := newCodeFixture("abc\ndef\nghi")
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.snapshot.SelectionRanges = []text.Span{{Start: 1, End: 2}, {Start: 5, End: 6}, {Start: 9, End: 10}}
	fixture.paint()
	cell, _, rowHeight := fixture.cell()
	fills := fixture.fills(fixture.theme.Selection)
	if len(fills) != 3 {
		t.Fatalf("selection fills = %+v", fills)
	}
	for index, fill := range fills {
		if !near(fill.X, fixture.editor.content.X+cell) || !near(fill.Y, fixture.editor.content.Y+float32(index)*rowHeight) || !near(fill.Width, cell) {
			t.Fatalf("selection row %d = %+v", index, fill)
		}
	}
	selectedRuns := 0
	for _, label := range fixture.canvas.Texts() {
		for _, run := range fixture.canvas.TextRuns(label) {
			if run.Color == fixture.theme.SelectionForeground {
				selectedRuns++
			}
		}
	}
	if selectedRuns != 3 {
		t.Fatalf("selected foreground runs = %d", selectedRuns)
	}
}

func TestCodeEditorPaintsMultipleSelectionsOnOneLineSeparately(t *testing.T) {
	fixture := newCodeFixture("abcdef")
	fixture.snapshot.SelectionRanges = []text.Span{{Start: 0, End: 1}, {Start: 3, End: 5}}
	fixture.paint()
	cell, _, _ := fixture.cell()
	fills := fixture.fills(fixture.theme.Selection)
	if len(fills) != 2 || !near(fills[0].Width, cell) || !near(fills[1].X, fixture.editor.content.X+3*cell) || !near(fills[1].Width, 2*cell) {
		t.Fatalf("separate selections = %+v", fills)
	}
}
