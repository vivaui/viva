package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

type shapedCodeTestEngine struct{ gfx.TextEngine }

func (shapedCodeTestEngine) FittedMonoCarets(dst []gfx.FittedMonoCaret, value string, style gfx.TextStyle, span float32) ([]gfx.FittedMonoCaret, bool) {
	if value != "a\u05d0\u05d1z" {
		return dst, false
	}
	// Visual order is a bet alef z. The boundary before alef is at 1/4
	// after the Latin letter and at 3/4 before the RTL letter.
	return append(dst,
		gfx.FittedMonoCaret{Offset: 0, Leading: 0, Trailing: 0},
		gfx.FittedMonoCaret{Offset: 1, Leading: span * 0.75, Trailing: span * 0.25},
		gfx.FittedMonoCaret{Offset: 3, Leading: span * 0.5, Trailing: span * 0.5},
		gfx.FittedMonoCaret{Offset: 5, Leading: span * 0.75, Trailing: span * 0.25},
		gfx.FittedMonoCaret{Offset: 6, Leading: span, Trailing: span},
	), true
}

func TestCodeEditorRetainsTheClickedShapedCaretEdge(t *testing.T) {
	f := newCodeFixture("a\u05d0\u05d1z")
	f.canvas.SetTextEngine(shapedCodeTestEngine{gfx.ReferenceTextEngine()})
	f.snapshot.ShapedBidi = true
	f.paint()
	cell, _, line := f.cell()
	left := f.editor.content.X
	y := f.editor.content.Y + line/2
	for _, column := range []float32{1, 3} {
		at, ok := f.editor.OffsetAt(gfx.Point{X: left + (column-0.1)*cell, Y: y})
		if !ok || at != 1 {
			t.Fatalf("hit near column %g = %d, %v; want byte 1", column, at, ok)
		}
		f.snapshot.Caret = at
		f.paint()
		if caret, ok := f.editor.CaretBounds(); !ok || !near(caret.X, left+column*cell) {
			t.Errorf("click near %g: IME caret %+v, %v; want X=%g", column, caret, ok, left+column*cell)
		}
	}
	f.snapshot.ShapedBidi = false
	f.paint()
	if caret, ok := f.editor.CaretBounds(); !ok || !near(caret.X, left+cell) {
		t.Errorf("turning off shape kept an alternate edge: %+v, %v", caret, ok)
	}
}

func TestCodeEditorShapedSelectionAndCompositionUseVisualSpans(t *testing.T) {
	f := newCodeFixture("a\u05d0\u05d1z")
	f.canvas.SetTextEngine(shapedCodeTestEngine{gfx.ReferenceTextEngine()})
	f.snapshot.ShapedBidi = true
	f.snapshot.SelectionEnd = 3
	f.snapshot.CompositionStart, f.snapshot.CompositionEnd = 3, 5
	f.paint()
	cell, _, _ := f.cell()
	left := f.editor.content.X
	spans := f.fills(f.theme.Selection)
	if len(spans) != 2 || !near(spans[0].X, left) || !near(spans[0].Width, cell) ||
		!near(spans[1].X, left+2*cell) || !near(spans[1].Width, cell) {
		t.Fatalf("selection spanned the wrong visual cells: %+v", spans)
	}
	found := false
	for _, fill := range f.fills(f.theme.Foreground) {
		if fill.Height == 1 && near(fill.X, left+cell) && near(fill.Width, cell) {
			found = true
		}
	}
	if !found {
		t.Fatal("composition did not underline the middle visual cell")
	}
}
