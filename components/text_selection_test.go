package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// texts stand for painted surfaces laid out one under another, ten pixels each.
var selectionTexts = []string{"alpha beta", "gamma", "", "delta epsilon"}

func selectionAt(point ui.Point) (TextPoint, bool) {
	surface := int(point.Y / 10)
	if point.Y < 0 || surface >= len(selectionTexts) {
		return TextPoint{}, false
	}
	return TextPoint{Surface: surface, Offset: min(int(point.X), len(selectionTexts[surface]))}, true
}

func selectionText(surface int, part MarkdownSelection) string {
	text := []byte(selectionTexts[surface])
	start, end := part.Range(text)
	return string(text[start:end])
}

func selectionLength(surface int) int { return len(selectionTexts[surface]) }

func press(x, y float32) ui.PointerEvent {
	return ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: x, Y: y}
}

func drag(x, y float32) ui.PointerEvent {
	return ui.PointerEvent{Kind: ui.PointerMove, Button: ui.PointerButtonPrimary, X: x, Y: y, Captured: true}
}

func release(x, y float32) ui.PointerEvent {
	return ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: x, Y: y, Captured: true}
}

func TestTextSelectionDragsFromOneSurfaceIntoAnother(t *testing.T) {
	var selection TextSelection
	if result := selection.Pointer(press(6, 1), selectionAt); !result.Handled || !result.Changed || !selection.Dragging() {
		t.Fatalf("press = %+v dragging=%v", result, selection.Dragging())
	}
	selection.Pointer(drag(3, 31), selectionAt)
	result := selection.Pointer(release(5, 31), selectionAt)
	if !result.Handled || selection.Dragging() {
		t.Fatalf("release = %+v dragging=%v", result, selection.Dragging())
	}
	want := "beta\ngamma\ndelta"
	if got := selection.Text(len(selectionTexts), "\n", selectionText); got != want {
		t.Fatalf("copied %q, want %q", got, want)
	}
}

func TestTextSelectionDraggedBackwardCopiesTheSameText(t *testing.T) {
	var forward, backward TextSelection
	forward.Select(TextPoint{Surface: 0, Offset: 6}, TextPoint{Surface: 3, Offset: 5})
	backward.Select(TextPoint{Surface: 3, Offset: 5}, TextPoint{Surface: 0, Offset: 6})
	a := forward.Text(len(selectionTexts), "|", selectionText)
	b := backward.Text(len(selectionTexts), "|", selectionText)
	if a != b || a != "beta|gamma|delta" {
		t.Fatalf("forward %q, backward %q", a, b)
	}
}

func TestTextSelectionInKeepsTheDragOrderWithinOneSurface(t *testing.T) {
	var selection TextSelection
	selection.Select(TextPoint{Surface: 1, Offset: 4}, TextPoint{Surface: 1, Offset: 1})
	if got := selection.In(1); got != (MarkdownSelection{Anchor: 4, Focus: 1}) {
		t.Fatalf("In = %+v: a drag upward was turned round", got)
	}
	if got := selection.In(0); got != (MarkdownSelection{}) {
		t.Fatalf("a surface outside the selection covers %+v", got)
	}
}

func TestTextSelectionCoversWholeSurfacesBetweenItsEnds(t *testing.T) {
	var selection TextSelection
	selection.Select(TextPoint{Surface: 0, Offset: 6}, TextPoint{Surface: 3, Offset: 5})
	for _, test := range []struct {
		surface int
		want    string
	}{{0, "beta"}, {1, "gamma"}, {2, ""}, {3, "delta"}} {
		if got := selectionText(test.surface, selection.In(test.surface)); got != test.want {
			t.Errorf("surface %d covers %q, want %q", test.surface, got, test.want)
		}
	}
}

func TestTextSelectionPressOnNothingIsLeftToTheCaller(t *testing.T) {
	var selection TextSelection
	selection.Select(TextPoint{Surface: 0, Offset: 1}, TextPoint{Surface: 0, Offset: 4})
	if result := selection.Pointer(press(0, 500), selectionAt); result.Handled || result.Changed {
		t.Fatalf("press off every surface = %+v", result)
	}
	if selection.Empty() {
		t.Fatal("a press the selection did not take cleared it")
	}
	secondary := press(1, 1)
	secondary.Button = ui.PointerButtonSecondary
	if result := selection.Pointer(secondary, selectionAt); result.Handled {
		t.Fatalf("a secondary press = %+v", result)
	}
}

func TestTextSelectionClickWithoutDragSelectsNothing(t *testing.T) {
	var selection TextSelection
	selection.Pointer(press(2, 1), selectionAt)
	selection.Pointer(release(2, 1), selectionAt)
	if !selection.Empty() || selection.Dragging() || selection != (TextSelection{}) {
		t.Fatalf("a click left %+v", selection)
	}
}

func TestTextSelectionMovesWithoutAHeldButtonDoNothing(t *testing.T) {
	var selection TextSelection
	selection.Pointer(press(2, 1), selectionAt)
	hover := drag(8, 1)
	hover.Captured = false
	if result := selection.Pointer(hover, selectionAt); result.Handled || selection.Focus.Offset != 2 {
		t.Fatalf("a hovering move = %+v focus=%+v", result, selection.Focus)
	}
	var idle TextSelection
	if result := idle.Pointer(drag(8, 1), selectionAt); result.Handled {
		t.Fatalf("a move with no drag = %+v", result)
	}
}

func TestTextSelectionKeepsWhatWasSelectedWhenThePointerLeavesTheWindow(t *testing.T) {
	var selection TextSelection
	selection.Pointer(press(2, 1), selectionAt)
	selection.Pointer(drag(7, 1), selectionAt)
	exit := ui.PointerEvent{Kind: ui.PointerExit}
	if result := selection.Pointer(exit, selectionAt); !result.Handled || selection.Dragging() || selection.Empty() {
		t.Fatalf("exit = %+v selection=%+v", result, selection)
	}
}

func TestTextSelectionSelectAllSkipsEmptySurfacesAtTheEnds(t *testing.T) {
	var selection TextSelection
	selection.SelectAll(len(selectionTexts), selectionLength)
	if got := selection.Text(len(selectionTexts), "|", selectionText); got != "alpha beta|gamma|delta epsilon" {
		t.Fatalf("select all copied %q", got)
	}
	selection.SelectAll(0, selectionLength)
	if !selection.Empty() {
		t.Fatal("select all of nothing left a selection")
	}
}

func TestTextSelectionClampFollowsTextThatChanged(t *testing.T) {
	var selection TextSelection
	selection.Select(TextPoint{Surface: 0, Offset: 6}, TextPoint{Surface: 9, Offset: 40})
	selection.Clamp(len(selectionTexts), selectionLength)
	if selection.Focus != (TextPoint{Surface: 3, Offset: len("delta epsilon")}) || selection.Anchor != (TextPoint{Surface: 0, Offset: 6}) {
		t.Fatalf("clamped to %+v", selection)
	}
	selection.Clamp(0, selectionLength)
	if !selection.Empty() {
		t.Fatal("a selection of texts that are gone stayed")
	}
}

func TestTextSelectionPointerPathDoesNotAllocate(t *testing.T) {
	var selection TextSelection
	allocs := testing.AllocsPerRun(100, func() {
		selection.Pointer(press(2, 1), selectionAt)
		selection.Pointer(drag(7, 31), selectionAt)
		selection.Pointer(release(7, 31), selectionAt)
		_ = selection.In(1)
	})
	if allocs != 0 {
		t.Fatalf("the pointer path allocates %v times", allocs)
	}
}

func BenchmarkTextSelectionDrag(b *testing.B) {
	var selection TextSelection
	selection.Pointer(press(2, 1), selectionAt)
	b.ReportAllocs()
	for b.Loop() {
		selection.Pointer(drag(7, 31), selectionAt)
		selection.Pointer(drag(3, 11), selectionAt)
	}
}

func BenchmarkTextSelectionIn(b *testing.B) {
	var selection TextSelection
	selection.Select(TextPoint{Surface: 0, Offset: 6}, TextPoint{Surface: 3, Offset: 5})
	b.ReportAllocs()
	for b.Loop() {
		for surface := range 4 {
			_ = selection.In(surface)
		}
	}
}

func TestTextSelectionEndDragKeepsTheSelection(t *testing.T) {
	var selection TextSelection
	selection.Pointer(press(2, 1), selectionAt)
	selection.Pointer(drag(7, 1), selectionAt)
	selection.EndDrag()
	if selection.Dragging() || selection.Empty() {
		t.Fatalf("EndDrag left %+v", selection)
	}
	if result := selection.Pointer(drag(1, 1), selectionAt); result.Handled || selection.Focus.Offset != 7 {
		t.Fatalf("a move after EndDrag = %+v focus=%+v", result, selection.Focus)
	}
}
