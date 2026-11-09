package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestCodeEditorPaintsAdditionalCaretInVirtualSpace(t *testing.T) {
	fixture := newCodeFixture("ab\nc\nd")
	fixture.snapshot.Caret = 0
	fixture.snapshot.CaretColor = gfx.RGB(12, 34, 56)
	fixture.snapshot.ExtraCarets = []CodeEditorCaret{{Offset: 4, VirtualCells: 2}}
	fixture.paint()
	cell, _, lineHeight := fixture.cell()
	fills := fixture.fills(fixture.snapshot.CaretColor)
	if len(fills) != 2 {
		t.Fatalf("primary and secondary caret fills = %+v", fills)
	}
	if !near(fills[1].X, fixture.editor.content.X+3*cell) || !near(fills[1].Y, fixture.editor.content.Y+lineHeight+(lineHeight-fills[1].Height)/2) {
		t.Fatalf("virtual secondary caret = %+v", fills[1])
	}
	if bounds, ok := fixture.editor.CaretBounds(); !ok || !near(bounds.X, fixture.editor.content.X) {
		t.Fatalf("additional caret changed native primary caret: %+v, %v", bounds, ok)
	}
}
