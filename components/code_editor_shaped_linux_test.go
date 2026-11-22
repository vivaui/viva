//go:build linux && !android

package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/linux"
)

func TestCodeEditorPangoRTLAlignsPaintAndPlacement(t *testing.T) {
	engine := linux.TextEngine()
	if engine == nil {
		t.Skip("Pango unavailable")
	}
	testCodeEditorRTLAlignsPaintAndPlacement(t, engine)
}

func TestCodeEditorPangoRTLKeyboardSelectionAcrossWrap(t *testing.T) {
	engine := linux.TextEngine()
	if engine == nil {
		t.Skip("Pango unavailable")
	}
	testCodeEditorRTLKeyboardSelectionAcrossWrap(t, engine)
}

func TestCodeEditorPangoKeepsTheClickedEdgeUnderHighlighting(t *testing.T) {
	const left, rtl, right = "a\u0301\U00010437 ", "\u0633\u0644\u0627\u0645", " z"
	const value = left + rtl + right
	f := newCodeFixture(value)
	engine := linux.TextEngine()
	if engine == nil {
		t.Skip("Pango unavailable")
	}
	f.canvas.SetTextEngine(engine)
	f.snapshot.ShapedBidi = true
	f.paint()
	carets := f.editor.shapedRow(f.editor.rows[0])
	if len(carets) != 10 || carets[3].Offset != len(left) || carets[3].Leading-carets[3].Trailing < f.editor.cellWidth {
		t.Fatalf("missing separate visual edges before Arabic: %+v", carets)
	}
	color := gfx.RGB(30, 110, 65)
	f.snapshot.Highlight = func(_, source string, dst []CodeEditorRun) []CodeEditorRun {
		if source != value {
			t.Fatalf("highlighter got %q; want %q", source, value)
		}
		return append(dst, CodeEditorRun{Start: len(left), End: len(left + rtl), Color: color, Italic: true})
	}
	for _, edge := range []float32{carets[3].Trailing, carets[3].Leading} {
		x := f.editor.content.X + edge
		y := f.editor.content.Y + f.editor.lineHeight/2
		at, ok := f.editor.OffsetAt(gfx.Point{X: x - 0.25, Y: y})
		if !ok || at != len(left) {
			t.Fatalf("click at %g = %d, %v; want byte %d", x-0.25, at, ok, len(left))
		}
		f.snapshot.Caret = at
		f.paint()
		if caret, ok := f.editor.CaretBounds(); !ok || !near(caret.X, x) {
			t.Errorf("caret after highlighting = %+v, %v; want X=%g", caret, ok, x)
		}
	}
	styled := false
	for _, label := range f.canvas.Texts() {
		runs := f.canvas.TextRuns(label)
		if label.Value != value || len(runs) != 1 || !runs[0].Ligatures {
			t.Fatalf("highlight split the fitted run: text %q, runs %+v", label.Value, runs)
		}
		styled = styled || (label.HasClip && runs[0].Italic && runs[0].Color == color)
	}
	if !styled {
		t.Fatal("Arabic highlight did not paint an italic clip")
	}
}

func TestCodeEditorPangoWrapAndTabKeepSourceOffsets(t *testing.T) {
	const first, rest = "\u05d0\u05d1\u05d2\u05d3\u05d4 ", "ab\tcd"
	f := newCodeFixture(first + rest)
	engine := linux.TextEngine()
	if engine == nil {
		t.Skip("Pango unavailable")
	}
	f.canvas.SetTextEngine(engine)
	f.snapshot.ShapedBidi, f.snapshot.Wrap, f.snapshot.TabWidth = true, true, 4
	f.bounds.Width = f.widthFor(6)
	f.paint()
	rows := f.editor.rows
	if len(rows) != 2 || int(rows[0].end) != len(first) || int(rows[1].end) != len(first+rest) {
		t.Fatalf("expected one soft wrap before %q: %+v", rest, rows)
	}
	labels := f.canvas.Texts()
	if len(labels) != 2 || labels[0].Value != first || labels[1].Value != "\u200fab  cd" {
		t.Fatalf("continuation should expand the tab and inherit RTL direction: %+v", labels)
	}
	carets := f.editor.shapedRow(rows[1])
	if len(carets) != 6 || carets[2].Offset != 2 || carets[3].Offset != 3 {
		t.Fatalf("the direction mark and expanded spaces must not add source offsets: %+v", carets)
	}
	for i, fraction := range []float32{0.2, 0.8} {
		a, b := carets[2].Leading, carets[3].Trailing
		point := gfx.Point{X: f.editor.content.X + a + fraction*(b-a), Y: f.editor.content.Y + 1.5*f.editor.lineHeight}
		if at, ok := f.editor.OffsetAt(point); !ok || at != len(first)+2+i {
			t.Errorf("hit at %g of tab = %d, %v; want byte %d", fraction, at, ok, len(first)+2+i)
		}
	}
	f.snapshot.Direction = gfx.DirectionLTR
	f.paint()
	labels = f.canvas.Texts()
	if len(labels) != 2 || labels[0].Value != "\u200e"+first || labels[1].Value != "ab  cd" {
		t.Fatalf("forced LTR retained the old paragraph direction: %+v", labels)
	}
	if carets := f.editor.shapedRow(f.editor.rows[1]); len(carets) != 0 {
		t.Errorf("LTR continuation kept its old RTL geometry: %+v", carets)
	}
}
