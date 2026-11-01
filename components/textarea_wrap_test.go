package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

const wrapTestText = "the quick brown fox jumps over the lazy dog and keeps running through the forest until night"

type wrapBox struct {
	t      *testing.T
	canvas ui.Canvas
	layout TextareaLayout
	bounds ui.Rect
	theme  ui.Theme
}

func newWrapBox(t *testing.T, height float32) *wrapBox {
	return &wrapBox{t: t, bounds: ui.Rect{X: 10, Y: 20, Width: 200, Height: height}, theme: ui.LightTheme()}
}

func (box *wrapBox) paint(snapshot TextareaSnapshot) {
	box.canvas.Reset()
	snapshot.ID, snapshot.Label, snapshot.Layout = ui.Item(1), "Notes", &box.layout
	if snapshot.Caret != 0 || snapshot.SelectionEnd != 0 {
		snapshot.Focused, snapshot.ShowCaret = true, true
	}
	PaintTextarea(&box.canvas, box.theme, box.bounds, snapshot)
}

// rows are the visual rows of text at the width of the box, as byte ranges.
func (box *wrapBox) rows(text string) []ui.TextRow {
	return box.canvas.WrapRows(nil, text, box.theme.Metrics.FontSizeM, ui.WeightRegular, box.bounds.Width-2*inputInsetX)
}

func (box *wrapBox) caret() ui.Rect {
	rect, ok := box.layout.CaretBounds()
	if !ok {
		box.t.Fatal("the caret is not in view")
	}
	return rect
}

func TestTextareaWrapsLongLinesInsteadOfClippingThem(t *testing.T) {
	box := newWrapBox(t, 120)
	box.paint(TextareaSnapshot{Value: wrapTestText})
	texts := box.canvas.Texts()
	if len(texts) < 3 {
		t.Fatalf("a %d-byte line painted %d row(s)", len(wrapTestText), len(texts))
	}
	var joined []string
	for _, text := range texts {
		if text.Bounds.Y < box.bounds.Y || width(box.canvas, text.Value) > box.bounds.Width-2*inputInsetX {
			t.Errorf("row %q does not fit the box", text.Value)
		}
		joined = append(joined, text.Value)
	}
	if strings.Join(joined, " ") != wrapTestText {
		t.Errorf("wrapping changed the text: %q", joined)
	}

	box.paint(TextareaSnapshot{Value: wrapTestText, NoWrap: true})
	if rows := box.canvas.Texts(); len(rows) != 1 || rows[0].Value != wrapTestText {
		t.Errorf("NoWrap painted %d row(s)", len(rows))
	}
}

func width(canvas ui.Canvas, value string) float32 {
	return canvas.TextWidthWeight(value, 13, ui.WeightRegular)
}

// A press lands on the visual row under it, wherever the row's text begins in
// the value, and the caret then paints there.
func TestTextareaPlacesTheCaretWhereAPressLands(t *testing.T) {
	box := newWrapBox(t, 120)
	box.paint(TextareaSnapshot{Value: wrapTestText})
	rows := box.rows(wrapTestText)
	for index, row := range rows[:3] {
		y := box.bounds.Y + textareaPadY + float32(index)*20 + 10
		offset, ok := box.layout.OffsetAt(ui.Point{X: box.bounds.X + inputInsetX + 1, Y: y})
		if !ok || offset != row.Start {
			t.Fatalf("a press at the start of row %d put the caret at %d (%v), want %d", index, offset, ok, row.Start)
		}
	}
	// Past the end of a row that wrapped is the end of its text, not the start of the next.
	offset, _ := box.layout.OffsetAt(ui.Point{X: box.bounds.X + box.bounds.Width, Y: box.bounds.Y + textareaPadY + 10})
	if want := len(strings.TrimRight(wrapTestText[rows[0].Start:rows[0].End], " ")); offset != want {
		t.Errorf("a press past the end of row 0 put the caret at %d, want %d", offset, want)
	}
	box.paint(TextareaSnapshot{Value: wrapTestText, Caret: offset})
	if got := box.caret().Y; got != box.bounds.Y+textareaPadY {
		t.Errorf("the caret at the end of row 0 is painted at y=%v, on another row", got)
	}
	// And below the last row is the last row.
	last, _ := box.layout.OffsetAt(ui.Point{X: box.bounds.X + box.bounds.Width, Y: box.bounds.Y + box.bounds.Height})
	if last != len(wrapTestText) && last < rows[len(rows)-1].Start {
		t.Errorf("a press below the text put the caret at %d", last)
	}
}

func TestTextareaVerticalMovesKeepTheColumn(t *testing.T) {
	const text = "aaaaaaaaaa\nbb\ncccccccccc"
	box := newWrapBox(t, 120)
	box.paint(TextareaSnapshot{Value: text})
	caret := 8 // column 8 of the first row
	caret = box.layout.Move(text, caret, TextareaMoveDown)
	if caret != len("aaaaaaaaaa\nbb") {
		t.Fatalf("down from column 8 into the short row went to %d", caret)
	}
	caret = box.layout.Move(text, caret, TextareaMoveDown)
	if caret != len("aaaaaaaaaa\nbb\n")+8 {
		t.Fatalf("down again went to %d: the column of 8 was lost", caret)
	}
	caret = box.layout.Move(text, caret, TextareaMoveUp)
	caret = box.layout.Move(text, caret, TextareaMoveUp)
	if caret != 8 {
		t.Fatalf("up twice went to %d, want 8", caret)
	}
	if got := box.layout.Move(text, caret, TextareaMoveUp); got != 0 {
		t.Errorf("up from the first row went to %d, want the start of the text", got)
	}
	if got := box.layout.Move(text, len(text)-3, TextareaMoveDown); got != len(text) {
		t.Errorf("down from the last row went to %d, want the end of the text", got)
	}
}

func TestTextareaHomeAndEndAreTheEndsOfTheVisualRow(t *testing.T) {
	box := newWrapBox(t, 120)
	box.paint(TextareaSnapshot{Value: wrapTestText})
	rows := box.rows(wrapTestText)
	second := rows[1]
	caret := second.Start + 4
	start := box.layout.Move(wrapTestText, caret, TextareaMoveLineStart)
	if start != second.Start {
		t.Fatalf("Home went to %d, want the start of the row at %d", start, second.Start)
	}
	end := box.layout.Move(wrapTestText, caret, TextareaMoveLineEnd)
	if end <= caret || end > second.End {
		t.Fatalf("End went to %d from %d; the row is %+v", end, caret, second)
	}
	box.paint(TextareaSnapshot{Value: wrapTestText, Caret: end})
	if got, want := box.caret().Y, box.bounds.Y+textareaPadY+ui.LightTheme().Metrics.FontSizeM+7; got != want {
		t.Errorf("the caret after End is on y=%v, want the second row at %v", got, want)
	}
	// The end of a row that wrapped without a space is also the start of the next
	// row, and End keeps the caret on the first.
	const run = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	box.paint(TextareaSnapshot{Value: run})
	long := box.rows(run)
	if len(long) < 2 {
		t.Fatal("the long word did not wrap")
	}
	end = box.layout.Move(run, 2, TextareaMoveLineEnd)
	box.paint(TextareaSnapshot{Value: run, Caret: end})
	if end != long[0].End || box.caret().Y != box.bounds.Y+textareaPadY {
		t.Errorf("End on an unbroken word: caret %d at y=%v, want %d on the first row", end, box.caret().Y, long[0].End)
	}
	// Typing on, or arrowing elsewhere, no longer holds it there.
	box.paint(TextareaSnapshot{Value: run, Caret: end + 1})
	if box.caret().Y != box.bounds.Y+textareaPadY+ui.LightTheme().Metrics.FontSizeM+7 {
		t.Errorf("one character on, the caret is still on the first row")
	}
}

func TestTextareaSelectionRunsAcrossRows(t *testing.T) {
	box := newWrapBox(t, 120)
	box.paint(TextareaSnapshot{Value: wrapTestText, SelectionStart: 3, SelectionEnd: len(wrapTestText) - 3})
	var selected []float32
	for index, rectangle := range box.canvas.Rectangles() {
		// The first rectangle is the field's border, which a focused field
		// paints in the same accent color.
		if index > 0 && rectangle.Color == box.theme.Selection {
			selected = append(selected, rectangle.Bounds.Y)
		}
	}
	rows := len(box.rows(wrapTestText))
	if len(selected) != rows {
		t.Fatalf("selection filled %d rows' worth of highlight, text has %d rows", len(selected), rows)
	}
}

func TestTextareaScrollsToTheCaret(t *testing.T) {
	var lines []string
	for index := range 29 {
		lines = append(lines, "line "+string(rune('a'+index%26))+string(rune('a'+index/26)))
	}
	text := strings.Join(lines, "\n") + "\nthe last one"
	box := newWrapBox(t, 100) // five rows
	box.paint(TextareaSnapshot{Value: text, Caret: len(text)})
	if caret := box.caret(); caret.Y < box.bounds.Y || caret.Y+caret.Height > box.bounds.Y+box.bounds.Height {
		t.Fatalf("the caret on the last row is at %+v, outside the box", caret)
	}
	if texts := box.canvas.Texts(); len(texts) > 7 || texts[len(texts)-1].Value != "the last one" {
		t.Errorf("painted %d rows, the last %q", len(texts), texts[len(texts)-1].Value)
	}
	box.paint(TextareaSnapshot{Value: text, Caret: 0})
	if texts := box.canvas.Texts(); texts[0].Bounds.Y < box.bounds.Y {
		t.Errorf("back at the start the first row is above the box: %+v", texts[0].Bounds)
	}
	// A wheel moves the box and the next paint leaves it until the caret moves.
	box.layout.ScrollBy(200)
	box.paint(TextareaSnapshot{Value: text, Caret: 0})
	if first := box.canvas.Texts()[0].Value; first == "line aa" {
		t.Errorf("a wheel scroll was undone by the next paint: the first row is %q", first)
	}
}

func TestTextareaContentHeightGrowsWithTheText(t *testing.T) {
	box := newWrapBox(t, 120)
	short := box.layout.ContentHeight(&box.canvas, box.theme, "one line", 200, false)
	long := box.layout.ContentHeight(&box.canvas, box.theme, wrapTestText, 200, false)
	lines := box.layout.ContentHeight(&box.canvas, box.theme, "1\n2\n3\n4\n5\n6", 200, false)
	if short != 64 || long <= short || lines <= long {
		t.Errorf("heights %v, %v, %v: a short text is the 64 minimum and more rows are taller", short, long, lines)
	}
}

func TestTextareaWrapsAndAllocatesNothingOnceWarm(t *testing.T) {
	box := newWrapBox(t, 120)
	snapshot := TextareaSnapshot{Value: wrapTestText, Caret: 30, SelectionStart: 5, SelectionEnd: 50}
	box.paint(snapshot)
	if allocations := testing.AllocsPerRun(100, func() { box.paint(snapshot) }); allocations != 0 {
		t.Errorf("paint with a layout allocates %v times", allocations)
	}
	snapshot.Layout = nil
	if allocations := testing.AllocsPerRun(100, func() {
		box.canvas.Reset()
		PaintTextarea(&box.canvas, box.theme, box.bounds, snapshot)
	}); allocations != 0 {
		t.Errorf("paint without a layout allocates %v times", allocations)
	}
}

func BenchmarkPaintTextareaWrapped(b *testing.B) {
	var canvas ui.Canvas
	var layout TextareaLayout
	theme := ui.LightTheme()
	text := strings.Repeat(wrapTestText+"\n", 8)
	snapshot := TextareaSnapshot{ID: ui.Item(1), Value: text, Focused: true, ShowCaret: true, Caret: 300, Layout: &layout}
	bounds := ui.Rect{Width: 360, Height: 160}
	canvas.Reset()
	PaintTextarea(&canvas, theme, bounds, snapshot)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.Reset()
		PaintTextarea(&canvas, theme, bounds, snapshot)
	}
}

func TestTextareaMovementForKey(t *testing.T) {
	for _, test := range []struct {
		key       ui.Key
		modifiers ui.Modifiers
		want      TextareaMovement
		extend    bool
	}{
		{ui.KeyUp, 0, TextareaMoveUp, false},
		{ui.KeyDown, ui.ModifierShift, TextareaMoveDown, true},
		{ui.KeyHome, 0, TextareaMoveLineStart, false},
		{ui.KeyEnd, ui.ModifierShift, TextareaMoveLineEnd, true},
		{ui.KeyLeft, ui.ModifierMeta, TextareaMoveLineStart, false},
		{ui.KeyUp, ui.ModifierMeta, TextareaMoveTextStart, false},
		{ui.KeyEnd, ui.ModifierControl, TextareaMoveTextEnd, false},
		{ui.KeyPageDown, 0, TextareaMovePageDown, false},
		{ui.KeyLeft, 0, TextareaMoveNone, false},
		{ui.KeyEnter, 0, TextareaMoveNone, false},
	} {
		if got, extend := TextareaMovementForKey(test.key, test.modifiers); got != test.want || extend != test.extend {
			t.Errorf("key %v with %v = (%v, %v), want (%v, %v)", test.key, test.modifiers, got, extend, test.want, test.extend)
		}
	}
}
