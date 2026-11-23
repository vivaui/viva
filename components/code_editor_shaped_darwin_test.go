//go:build darwin && !ios

package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/darwin"
)

func TestCodeEditorCoreTextKeepsFittedArabicCaretEdges(t *testing.T) {
	const prefix = "a\u0301\U00010437 "
	const value = prefix + "\u0633\u0644\u0627\u0645 z"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi = true
	f.paint()
	row := f.editor.rows[0]
	carets := f.editor.shapedRow(row)
	if len(carets) != 10 {
		t.Fatalf("fitted carets did not preserve the ten grapheme boundaries: %+v", carets)
	}
	edge := carets[3]
	if edge.Offset != len(prefix) || edge.Leading-edge.Trailing < f.editor.cellWidth {
		t.Fatalf("expected separate leading and trailing positions for byte %d: %+v", len(prefix), edge)
	}
	labels := f.canvas.Texts()
	if len(labels) != 1 || labels[0].Value != value {
		t.Fatalf("expected one intact shaped label: %+v", labels)
	}
	runs := f.canvas.TextRuns(labels[0])
	if len(runs) != 1 || !runs[0].Ligatures || int(runs[0].Bytes) != len(value) || runs[0].Cells != row.cells {
		t.Fatalf("expected the measured run to be fitted without subruns: %+v", runs)
	}
	for _, x := range []float32{edge.Trailing, edge.Leading} {
		want := f.editor.content.X + x
		y := f.editor.content.Y + f.editor.lineHeight/2
		at, ok := f.editor.OffsetAt(gfx.Point{X: want - 0.25, Y: y})
		if !ok || at != len(prefix) {
			t.Fatalf("click at %g: byte %d, %v; want %d", want-0.25, at, ok, len(prefix))
		}
		f.snapshot.Caret = at
		f.paint()
		if got, ok := f.editor.CaretBounds(); !ok || !near(got.X, want) {
			t.Errorf("IME placement at byte %d: %+v, %v; want X=%g", at, got, ok, want)
		}
	}
}
