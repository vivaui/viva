//go:build darwin && !ios

package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/darwin"
)

func TestCodeEditorShapedHighlightSkipsClipsOutsideTheViewport(t *testing.T) {
	const count = 120
	const letter = "\u05d0"
	f := newCodeFixture(strings.Repeat(letter, count))
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi = true
	f.bounds.Width = f.widthFor(8)
	f.snapshot.Highlight = func(_, source string, dst []CodeEditorRun) []CodeEditorRun {
		for i := 0; i < len(source); i += len(letter) {
			color := gfx.RGB(220, 40, 40)
			if i/len(letter)%2 == 1 {
				color = gfx.RGB(40, 40, 220)
			}
			dst = append(dst, CodeEditorRun{Start: i, End: i + len(letter), Color: color})
		}
		return dst
	}
	f.paint()
	labels := f.canvas.Texts()
	if len(labels) < 3 || len(labels) > 10 {
		t.Fatalf("painted %d labels for an eight-cell viewport", len(labels))
	}
	for _, label := range labels {
		if label.Value != f.snapshot.Value || !label.HasClip || label.ClipOf.Width <= 0 {
			t.Errorf("nonvisible or split shaped label: clip=%+v, bytes=%d", label.ClipOf, len(label.Value))
		}
	}
}

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
	before, ok := f.editor.CaretBounds()
	if !ok {
		t.Fatal("shaped caret missing before applying styles")
	}
	comment := gfx.RGB(30, 110, 65)
	f.snapshot.HighlightLine = func(_ string, index int, source string, dst []CodeEditorRun) []CodeEditorRun {
		if index != 0 || source != value {
			t.Fatalf("highlight got line %d %q; want the logical source", index, source)
		}
		return append(dst, CodeEditorRun{Start: len(prefix), End: len(value) - 2, Color: comment, Italic: true})
	}
	f.paint()
	if after, ok := f.editor.CaretBounds(); !ok || after != before {
		t.Fatalf("applying an italic style moved the remembered caret: before %+v, after %+v, %v", before, after, ok)
	}
	styled := false
	for _, label := range f.canvas.Texts() {
		runs := f.canvas.TextRuns(label)
		if len(runs) != 1 || !runs[0].Ligatures || label.Value != value {
			t.Fatalf("highlight split the fitted run: label=%q runs=%+v", label.Value, runs)
		}
		styled = styled || (label.HasClip && runs[0].Color == comment && runs[0].Italic)
	}
	if !styled {
		t.Fatal("the fitted Arabic range was not drawn in the requested style")
	}
}

func TestCodeEditorCoreTextWrapKeepsBothRowsOfTheBoundary(t *testing.T) {
	const first = "\u05d0\u05d1 "
	const value = first + "cd"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi = true
	f.snapshot.Wrap = true
	f.bounds.Width = f.widthFor(3)
	f.paint()
	rows := f.editor.rows
	if len(rows) != 2 || int(rows[0].end) != len(first) {
		t.Fatalf("expected a soft wrap after %q: %+v", first, rows)
	}
	labels := f.canvas.Texts()
	if len(labels) != 2 || labels[0].Value != first || labels[1].Value != "\u200fcd" {
		t.Fatalf("the continuation should inherit the RTL paragraph direction: %+v", labels)
	}
	for index, row := range rows {
		carets := f.editor.shapedRow(row)
		if len(carets) == 0 {
			t.Fatalf("row %d did not retain shaped geometry", index)
		}
		x := carets[0].Leading
		inside := carets[1].Trailing
		if index == 0 {
			x = carets[len(carets)-1].Trailing
			inside = carets[len(carets)-2].Leading
		}
		left, top := f.editor.content.X, f.editor.content.Y
		line := f.editor.lineHeight
		point := gfx.Point{X: left + x + 0.1*(inside-x), Y: top + (float32(index)+0.5)*line}
		at, ok := f.editor.OffsetAt(point)
		if !ok || at != len(first) {
			t.Fatalf("hit row %d: byte %d, %v; want %d", index, at, ok, len(first))
		}
		f.snapshot.Caret = at
		f.paint()
		caret, ok := f.editor.CaretBounds()
		if !ok || !near(caret.Y, top+float32(index)*line) || !near(caret.X, left+x) {
			t.Errorf("hit row %d painted caret %+v, %v; want (%g,%g)", index, caret, ok, left+x, top+float32(index)*line)
		}
	}
	f.snapshot.Wrap = false
	f.bounds.Width = f.widthFor(12)
	f.paint()
	if labels := f.canvas.Texts(); len(labels) != 1 || labels[0].Value != value {
		t.Errorf("switching wrap off retained a cached continuation: %+v", labels)
	}
}

func TestCodeEditorCoreTextTabUsesOnlySourceOffsets(t *testing.T) {
	const value = "\u05d0\u05d1\tx"
	const before, after = len("\u05d0\u05d1"), len("\u05d0\u05d1\t")
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi = true
	f.snapshot.TabWidth = 8
	f.paint()
	row := f.editor.rows[0]
	carets := f.editor.shapedRow(row)
	if len(carets) != 5 || carets[2].Offset != before || carets[3].Offset != after || carets[4].Offset != len(value) {
		t.Fatalf("expanded tab must not be hit as multiple source bytes: %+v", carets)
	}
	labels := f.canvas.Texts()
	if len(labels) != 1 || labels[0].Value != "\u05d0\u05d1      x" {
		t.Fatalf("expected the expanded text as one shaped label: %+v", labels)
	}
	lead, trail := carets[2].Leading, carets[3].Trailing
	for _, check := range []struct {
		fraction float32
		want     int
	}{{0.2, before}, {0.8, after}} {
		left, top := f.editor.content.X, f.editor.content.Y
		point := gfx.Point{X: left + lead + (trail-lead)*check.fraction, Y: top + 0.5*f.editor.lineHeight}
		if at, ok := f.editor.OffsetAt(point); !ok || at != check.want {
			t.Errorf("hit at %.2f of tab = %d, %v; want %d", check.fraction, at, ok, check.want)
		}
	}
	f.snapshot.TabWidth = 4
	f.paint()
	if labels := f.canvas.Texts(); len(labels) != 1 || labels[0].Value != "\u05d0\u05d1  x" {
		t.Errorf("changing the tab width retained the old shaped label: %+v", labels)
	}
}

func TestCodeEditorCoreTextWrapInheritsLTRBeforeHebrew(t *testing.T) {
	const value = "ab \u05d0\u05d1"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi, f.snapshot.Wrap, f.snapshot.ShowWrapSymbol = true, true, true
	f.bounds.Width = f.widthFor(4)
	f.paint()
	var label, marker bool
	for _, text := range f.canvas.Texts() {
		label = label || text.Value == "\u200e\u05d0\u05d1"
		marker = marker || text.Value == "\u21aa"
	}
	if !label || !marker {
		t.Errorf("inherited LTR label=%t, wrap marker=%t; both should paint", label, marker)
	}
}
