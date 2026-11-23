package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestCodeEditorShapedResizeKeepsManualScroll(t *testing.T) {
	for _, mode := range []struct {
		name      string
		direction gfx.TextDirection
	}{{"LTR", gfx.DirectionLTR}, {"RTL", gfx.DirectionRTL}} {
		t.Run(mode.name, func(t *testing.T) {
			f := newCodeFixture(strings.Repeat(strings.Repeat("a\u05d0\u05d1z", 30)+"\n", 40))
			f.snapshot.ShapedBidi, f.snapshot.Direction = true, mode.direction
			f.bounds.Width = f.widthFor(24)
			f.paint()
			f.editor.SetScrollX(40)
			f.editor.SetScrollY(80)
			f.bounds.Width = f.widthFor(30)
			f.paint()
			if x, y := f.editor.ScrollX(), f.editor.ScrollY(); x != 40 || y != 80 {
				t.Fatalf("resize changed scroll to (%g, %g); want (40, 80)", x, y)
			}
			// A real direction change still reveals the unchanged caret.
			f.snapshot.Direction = gfx.DirectionRTL
			if mode.direction == gfx.DirectionRTL {
				f.snapshot.Direction = gfx.DirectionLTR
			}
			f.bounds.Width = f.widthFor(32)
			f.paint()
			if y := f.editor.ScrollY(); y != 0 {
				t.Fatalf("direction change left the caret above the viewport: ScrollY = %g", y)
			}
		})
	}
}

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

func TestCodeEditorCollapsesSelectionToTheVisualEdge(t *testing.T) {
	const value = "a\u05d0\u05d1z"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(shapedCodeTestEngine{gfx.ReferenceTextEngine()})
	f.snapshot.ShapedBidi, f.snapshot.Direction = true, gfx.DirectionLTR
	f.paint()
	cell, _, _ := f.cell()
	left := f.editor.content.X
	for _, step := range []struct {
		right  bool
		offset int
		column float32
	}{{false, 3, 2}, {true, 1, 3}} {
		at, ok := f.editor.CollapseHorizontal(value, 1, 3, step.right)
		if !ok || at != step.offset {
			t.Fatalf("right=%v collapsed to byte %d, %v; want %d", step.right, at, ok, step.offset)
		}
		f.snapshot.Caret = at
		f.paint()
		if caret, shown := f.editor.CaretBounds(); !shown || !near(caret.X, left+step.column*cell) {
			t.Fatalf("right=%v collapsed caret = %+v; want column %g", step.right, caret, step.column)
		}
	}
}

func TestCodeEditorHorizontalMovementKeepsShapedAffinity(t *testing.T) {
	const value = "a\u05d0\u05d1z"
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(shapedCodeTestEngine{gfx.ReferenceTextEngine()})
	f.snapshot.ShapedBidi = true
	f.paint()
	cell, _, line := f.cell()
	left, top := f.editor.content.X, f.editor.content.Y
	f.snapshot.Caret, _ = f.editor.OffsetAt(gfx.Point{X: left + 0.9*cell, Y: top + line/2})
	f.paint()
	for _, step := range []struct {
		right  bool
		offset int
		column float32
	}{{true, 3, 2}, {true, 1, 3}, {true, len(value), 4}, {true, len(value), 4}, {false, 5, 3}, {false, 3, 2}, {false, 5, 1}, {false, 0, 0}} {
		offset, ok := f.editor.MoveHorizontal(value, f.snapshot.Caret, step.right)
		if !ok || offset != step.offset {
			t.Fatalf("right=%t from %d got byte %d, %t; want %d", step.right, f.snapshot.Caret, offset, ok, step.offset)
		}
		f.snapshot.Caret = offset
		f.paint()
		if caret, ok := f.editor.CaretBounds(); !ok || !near(caret.X, left+step.column*cell) {
			t.Fatalf("right=%t to %d drew %+v, %t; want column %g", step.right, offset, caret, ok, step.column)
		}
	}
	if _, ok := f.editor.MoveHorizontal(value+"!", 0, true); ok {
		t.Fatal("movement accepted stale painted geometry")
	}
}

func testCodeEditorRTLKeyboardSelectionAcrossWrap(t *testing.T, engine gfx.TextEngine) {
	t.Helper()
	const value = "\u05d0\u05d1\u05d2\u05d3"
	doc := text.NewEditor(value)
	doc.SetSelection(text.Range{})
	f := newCodeFixture(value)
	f.canvas.SetTextEngine(engine)
	f.snapshot.ShapedBidi, f.snapshot.Wrap, f.snapshot.Direction = true, true, gfx.DirectionRTL
	f.bounds.Width = f.widthFor(2)
	paint := func() gfx.Rect {
		t.Helper()
		f.snapshot.Value, f.snapshot.Caret = doc.String(), doc.ByteOffset(doc.Caret())
		range_ := doc.Selection()
		f.snapshot.SelectionStart, f.snapshot.SelectionEnd = doc.ByteOffset(range_.Start), doc.ByteOffset(range_.End)
		f.paint()
		caret, ok := f.editor.CaretBounds()
		if !ok {
			t.Fatal("native caret anchor missing")
		}
		return caret
	}
	move := func(want int, extend bool) gfx.Rect {
		t.Helper()
		at, ok := f.editor.MoveHorizontal(doc.String(), doc.ByteOffset(doc.Caret()), false)
		if !ok || at != want {
			t.Fatalf("Left from rune %d returned byte %d, %t; want byte %d", doc.Caret(), at, ok, want)
		}
		doc.MoveTo(doc.RuneIndex(at), extend)
		return paint()
	}
	start, first, wrap := paint(), move(2, false), move(4, false)
	if first.X >= start.X || wrap.X >= first.X || wrap.Y != start.Y {
		t.Fatalf("Left did not traverse the first RTL row: %+v, %+v, %+v", start, first, wrap)
	}
	next := move(4, false)
	if next.Y <= wrap.Y || next.X <= wrap.X {
		t.Fatalf("soft wrap lost the second visual position of the same byte: before %+v, after %+v", wrap, next)
	}
	selected := move(6, true)
	if selected.Y != next.Y || selected.X >= next.X || doc.Selection() != (text.Range{Start: 2, End: 3}) || len(f.fills(f.theme.Selection)) != 1 {
		t.Fatalf("Shift+Left did not select the next RTL grapheme on the wrapped row: %+v, selection %+v", selected, doc.Selection())
	}
	doc.Insert("x")
	if doc.String() != "\u05d0\u05d1x\u05d3" || !doc.Undo() || doc.String() != value || doc.Selection() != (text.Range{Start: 2, End: 3}) {
		t.Fatalf("edit and Undo did not preserve the logical selection: text %q, range %+v", doc.String(), doc.Selection())
	}
}

func testCodeEditorRTLWithDefaultEditorDecorations(t *testing.T, engine gfx.TextEngine) {
	t.Helper()
	const first, second = "\t\u05d0\u05d1", "x\x01\u05d2\u05d3"
	f := newCodeFixture(first + "\n" + second)
	f.canvas.SetTextEngine(engine)
	f.snapshot.ShapedBidi, f.snapshot.ShowIndentGuides, f.snapshot.Direction = true, true, gfx.DirectionRTL
	f.snapshot.TabWidth = 4
	f.snapshot.InvisibleColor = gfx.RGB(38, 79, 126)
	gutter := gfx.RGB(200, 230, 210)
	f.snapshot.Decorations = &CodeEditorDecorations{GutterWidth: 8, GutterBackground: gutter}
	f.snapshot.Representations = []CodeRepresentation{{Character: '\x01', Label: "SOH", Blob: true}}
	f.paint()
	rows := f.editor.rows
	if len(rows) != 2 || len(f.editor.shapedRow(rows[0])) != 4 || len(f.editor.shapedRow(rows[1])) != 0 {
		t.Fatal("settings hid shaped geometry on the ordinary row or shaped the represented control")
	}
	caret, shown := f.editor.CaretBounds()
	guide := f.snapshot.InvisibleColor
	guide.Alpha *= 0.5
	marks := f.fills(guide)
	if !shown || len(marks) != 1 || !near(marks[0].X, caret.X) || len(f.fills(gutter)) != 1 {
		t.Errorf("indent guide or gutter lost: caret %+v, %t; guides %+v; lanes %+v", caret, shown, marks, f.fills(gutter))
	}
	f.snapshot.Representations = []CodeRepresentation{{Character: '\t', Label: "TAB", Blob: true}}
	f.paint()
	rows = f.editor.rows
	if len(f.editor.shapedRow(rows[0])) != 0 || len(f.editor.shapedRow(rows[1])) == 0 {
		t.Error("changing the representation table retained stale shaped rows")
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
