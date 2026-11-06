package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCodeEditorOffsetWithVirtualCellsAtHardLineEnd(t *testing.T) {
	fixture := newCodeFixture("ab\nc\n")
	fixture.paint()
	cell, _, lineHeight := fixture.cell()
	point := ui.Point{X: fixture.editor.content.X + 4*cell, Y: fixture.editor.content.Y + lineHeight*1.5}
	offset, virtual, ok := fixture.editor.OffsetWithVirtualAt(point)
	if !ok || offset != 4 || virtual != 3 {
		t.Fatalf("hard-line position = %d + %d virtual cells, %v", offset, virtual, ok)
	}
	if plain, ok := fixture.editor.OffsetAt(point); !ok || plain != offset {
		t.Fatalf("ordinary placement changed: %d, %v", plain, ok)
	}
	point.X = 1e20
	if _, virtual, ok := fixture.editor.OffsetWithVirtualAt(point); !ok || virtual != 1_000_000-1 {
		t.Fatalf("distant virtual column was not bounded: %d, %v", virtual, ok)
	}
}

func TestCodeEditorOffsetWithVirtualCellsDoesNotExtendSoftWrap(t *testing.T) {
	fixture := newCodeFixture("abcdefgh")
	fixture.bounds.Width = fixture.widthFor(3)
	fixture.snapshot.Wrap = true
	fixture.paint()
	cell, _, lineHeight := fixture.cell()
	point := ui.Point{X: fixture.editor.content.X + 6*cell, Y: fixture.editor.content.Y + lineHeight/2}
	offset, virtual, ok := fixture.editor.OffsetWithVirtualAt(point)
	if !ok || offset != int(fixture.editor.rows[0].end) || virtual != 0 {
		t.Fatalf("soft-wrap position = %d + %d virtual cells, %v", offset, virtual, ok)
	}
}
