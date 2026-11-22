package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func testCodeEditorRTLAlignsPaintAndPlacement(t *testing.T, engine gfx.TextEngine) {
	t.Helper()
	const short = "\u05d0\u05d1\u05d2"
	const value = short + "\n"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(engine)
	f.snapshot.ShapedBidi, f.snapshot.Direction = true, gfx.DirectionRTL
	f.snapshot.SelectionEnd = len(short)
	f.snapshot.CompositionStart, f.snapshot.CompositionEnd = 2, 4
	f.paint()
	labels := f.canvas.Texts()
	if len(labels) != 1 || labels[0].Bounds.X < f.editor.content.X+f.editor.content.Width/2 {
		t.Fatalf("short RTL label was not aligned to the right: %+v", labels)
	}
	label := labels[0].Bounds
	end := label.X + label.Width
	selection := f.fills(f.theme.Selection)
	if len(selection) != 1 || !near(selection[0].X, label.X) || !near(selection[0].Width, label.Width) {
		t.Errorf("selection did not move with the label: %+v; want %+v", selection, label)
	}
	underline := false
	for _, fill := range f.fills(f.theme.Foreground) {
		if fill.Height == 1 && fill.X >= label.X && fill.X+fill.Width <= end+0.01 {
			underline = true
		}
	}
	if !underline {
		t.Error("composition underline did not move with the label")
	}
	y := f.editor.content.Y + f.editor.lineHeight/2
	if at, ok := f.editor.OffsetAt(gfx.Point{X: end - 0.25, Y: y}); !ok || at != 0 {
		t.Fatalf("hit at RTL line start = %d, %t; want byte 0", at, ok)
	}
	f.paint()
	if caret, ok := f.editor.CaretBounds(); !ok || !near(caret.X, end) {
		t.Errorf("IME anchor = %+v, %t; want X=%g", caret, ok, end)
	}
	f.snapshot.Caret = f.editor.Move(value, 0, TextareaMoveDown)
	f.paint()
	if caret, ok := f.editor.CaretBounds(); f.snapshot.Caret != len(value) || !ok || !near(caret.X, end) {
		t.Errorf("movement to the blank RTL row: offset %d, caret %+v, %t; want byte %d, X=%g", f.snapshot.Caret, caret, ok, len(value), end)
	}

	f.snapshot.Value, f.snapshot.Caret = value+strings.Repeat(short, 30), 0
	f.paint()
	shift := f.editor.cellWidth / 2
	if f.editor.ScrollX() < shift {
		t.Fatal("the caret on a short RTL row did not follow the wide document's right edge")
	}
	f.editor.ScrollBy(-shift, 0)
	f.paint()
	label = f.canvas.Texts()[0].Bounds
	if at, ok := f.editor.OffsetAt(gfx.Point{X: label.X + label.Width - 0.25, Y: y}); !ok || at != 0 {
		t.Errorf("click after horizontal scroll = %d, %t; want byte 0", at, ok)
	}
	if !near(label.X+label.Width, end+shift) {
		t.Errorf("right edge after horizontal scroll = %g; want %g", label.X+label.Width, end+shift)
	}
}

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

func TestCodeEditorShapedHighlightKeepsTheFullRunAndSelectionStyle(t *testing.T) {
	const value = "a\u05d0\u05d1z"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(shapedCodeTestEngine{gfx.ReferenceTextEngine()})
	f.snapshot.ShapedBidi = true
	keyword := gfx.RGB(230, 60, 40)
	f.snapshot.Highlight = func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		if line != value {
			t.Errorf("highlighter got %q; want logical source %q", line, value)
		}
		return append(dst, CodeEditorRun{Start: 0, End: 3, Color: keyword, Italic: true})
	}
	f.snapshot.SelectionStart, f.snapshot.SelectionEnd = 1, 3
	f.paint()
	labels := f.canvas.Texts()
	cell, _, _ := f.cell()
	left := f.editor.content.X
	if len(labels) != 4 {
		t.Fatalf("painted %d labels; want four disjoint visual styles", len(labels))
	}
	for i, want := range []struct {
		color  gfx.Color
		italic bool
	}{{keyword, true}, {f.theme.Foreground, false}, {f.theme.SelectionForeground, true}, {f.theme.Foreground, false}} {
		label := labels[i]
		runs := f.canvas.TextRuns(label)
		if label.Value != value || len(runs) != 1 || !runs[0].Ligatures || runs[0].Bytes != int32(len(value)) {
			t.Fatalf("clip %d changed the shaped source: label=%q runs=%+v", i, label.Value, runs)
		}
		if !label.HasClip || !near(label.ClipOf.X, left+float32(i)*cell) || !near(label.ClipOf.Width, cell) ||
			runs[0].Color != want.color || runs[0].Italic != want.italic {
			t.Errorf("clip %d: bounds=%+v run=%+v; want column %d with %+v", i, label.ClipOf, runs[0], i, want)
		}
	}
}
