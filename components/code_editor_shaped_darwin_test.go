//go:build darwin && !ios

package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/darwin"
)

func TestCodeEditorCoreTextCarriesBidiOverrideAcrossSoftBreak(t *testing.T) {
	testCodeEditorBidiOverrideAcrossSoftBreak(t, darwin.SharedTextEngine())
}

func TestCodeEditorCoreTextShapesShortRowsOfLongParagraph(t *testing.T) {
	const prefix = "\u05d0\u05d1 "
	f := newCodeFixture(prefix + strings.Repeat("abc ", 1200))
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi, f.snapshot.Wrap, f.snapshot.Direction = true, true, gfx.DirectionRTL
	f.bounds.Width = f.widthFor(24)
	f.paint()
	if at, ok := f.editor.MoveHorizontal(f.snapshot.Value, 0, false); !ok || at != 2 {
		t.Fatalf("visual Left on the first short row = %d, %t; want the next Hebrew boundary at byte 2", at, ok)
	}
	row := f.editor.rows[1]
	if _, ok := f.editor.MoveHorizontal(f.snapshot.Value, int(row.start), true); !ok {
		t.Fatal("a Latin continuation lost the paragraph's forced RTL layout")
	}
}

func TestCodeEditorCoreTextKeepsLongUnwrappedBidiGeometry(t *testing.T) {
	value := strings.Repeat("\u05d0\u05d1", 2048) + "abc"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi, f.snapshot.Direction = true, gfx.DirectionRTL
	f.paint()
	before, shown := f.editor.CaretBounds()
	if !shown || f.editor.ScrollX() < f.editor.MaxScrollX()/2 {
		t.Fatalf("RTL line start did not reveal the right end: caret %+v, scroll %g of %g", before, f.editor.ScrollX(), f.editor.MaxScrollX())
	}
	if at, ok := f.editor.MoveHorizontal(value, 0, false); !ok || at != 2 {
		t.Fatalf("Left on the long RTL line = %d, %v; want byte 2", at, ok)
	}
	f.snapshot.Caret = 2
	f.paint()
	after, shown := f.editor.CaretBounds()
	if !shown || after.X >= before.X {
		t.Fatalf("Left moved from %+v to %+v", before, after)
	}
}

func TestCodeEditorCoreTextVisibleSymbolsKeepRTLGeometry(t *testing.T) {
	const line = "\u05d0 \t\u05d1"
	f := newCodeFixture(line + "\n\n")
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi, f.snapshot.Direction = true, gfx.DirectionRTL
	marker := gfx.RGB(33, 104, 171)
	f.snapshot.InvisibleColor = marker
	f.paint()
	before, ok := f.editor.CaretBounds()
	if !ok || before.X <= f.bounds.X+f.bounds.Width/2 {
		t.Fatalf("expected a right-aligned RTL caret: %+v", before)
	}
	rowLabel := f.canvas.Texts()[0].Bounds
	f.snapshot.ShowWhitespace, f.snapshot.ShowEOL = true, true
	f.paint()
	if caret, shown := f.editor.CaretBounds(); !shown || !near(caret.X, before.X) {
		t.Fatalf("showing symbols moved the RTL caret from %+v to %+v", before, caret)
	}
	marks := f.fills(marker)
	if len(marks) != 4 {
		t.Fatalf("wanted one space dot and one tab arrow: %+v", marks)
	}
	for _, mark := range marks {
		if mark.X <= rowLabel.X || mark.X+mark.Width >= rowLabel.X+rowLabel.Width {
			t.Fatalf("whitespace mark painted outside the shaped run: %+v, row %+v", mark, rowLabel)
		}
	}
	if marks[2].X >= marks[1].X+marks[1].Width/2 || marks[3].X != marks[2].X {
		t.Fatalf("tab arrow does not point left: %+v", marks[1:])
	}
	var endings []gfx.Rect
	for _, label := range f.canvas.Texts() {
		if label.Value == "LF" {
			endings = append(endings, label.Bounds)
		}
	}
	if len(endings) != 2 || endings[0].X+endings[0].Width >= rowLabel.X || !near(endings[1].X+endings[1].Width, before.X-2) {
		t.Fatalf("expected left-side LF markers for the text and empty rows: %+v", endings)
	}
	if caret, handled := f.editor.MoveHorizontal(f.snapshot.Value, 0, false); !handled || caret != 2 {
		t.Fatalf("visible symbols changed visual Left: %d, %t", caret, handled)
	}
}

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

func TestCodeEditorCoreTextSwitchesParagraphDirection(t *testing.T) {
	const value = "a \u05d0\u05d1"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi = true
	f.paint()
	auto, ok := f.editor.CaretBounds()
	if !ok {
		t.Fatal("automatic caret missing")
	}
	f.snapshot.Direction = gfx.DirectionRTL
	f.paint()
	labels := f.canvas.Texts()
	if len(labels) != 1 || labels[0].Value != "\u200f"+value {
		t.Fatalf("forced RTL did not change the fitted paragraph: %+v", labels)
	}
	rtl, ok := f.editor.CaretBounds()
	if !ok || rtl.X <= auto.X {
		t.Fatalf("paragraph direction did not update the caret: auto=%+v RTL=%+v", auto, rtl)
	}
	if at, ok := f.editor.OffsetAt(gfx.Point{X: rtl.X + 0.25, Y: rtl.Y + rtl.Height/2}); !ok || at != 0 {
		t.Errorf("clicked visual start = %d, %t; want the original byte 0", at, ok)
	}
	f.snapshot.Direction = gfx.DirectionLTR
	f.paint()
	if ltr, ok := f.editor.CaretBounds(); !ok || ltr != auto {
		t.Errorf("switch back to LTR returned stale caret %+v, %t; want %+v", ltr, ok, auto)
	}
}

func TestCodeEditorCoreTextForcedLTRAppliesToEveryWrappedRow(t *testing.T) {
	const first, second = "\u05d0\u05d1 ", "\u05d2\u05d3"
	f := newCodeFixture(first + second)
	f.canvas.SetTextEngine(darwin.SharedTextEngine())
	f.snapshot.ShapedBidi, f.snapshot.Wrap, f.snapshot.Direction = true, true, gfx.DirectionLTR
	f.bounds.Width = f.widthFor(3)
	f.paint()
	labels := f.canvas.Texts()
	if len(labels) != 2 || labels[0].Value != "\u200e"+first || labels[1].Value != "\u200e"+second {
		t.Fatalf("wrapped rows did not use the forced direction: %+v", labels)
	}
	for _, row := range f.editor.rows {
		carets := f.editor.shapedRow(row)
		if len(carets) == 0 || carets[0].Offset != 0 || carets[len(carets)-1].Offset != int(row.end-row.start) {
			t.Errorf("direction mark escaped into source offsets: %+v", carets)
		}
	}
}

func TestCodeEditorCoreTextRTLAlignsPaintAndPlacement(t *testing.T) {
	testCodeEditorRTLAlignsPaintAndPlacement(t, darwin.SharedTextEngine())
}

func TestCodeEditorCoreTextRTLKeyboardSelectionAcrossWrap(t *testing.T) {
	testCodeEditorRTLKeyboardSelectionAcrossWrap(t, darwin.SharedTextEngine())
}

func TestCodeEditorCoreTextRTLWithDefaultEditorDecorations(t *testing.T) {
	testCodeEditorRTLWithDefaultEditorDecorations(t, darwin.SharedTextEngine())
}
