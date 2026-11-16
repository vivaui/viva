package text

import "testing"

func TestSelectionsStartAsThePrimaryAndAreOrdered(t *testing.T) {
	editor := NewEditor("abcdef")
	editor.SetSelection(Range{Start: 4, End: 5})
	if got := editor.SelectionCount(); got != 1 {
		t.Fatalf("plain editor has %d selections, want 1", got)
	}
	if got := editor.Selections(); len(got) != 1 || got[0] != (Range{Start: 4, End: 5}) {
		t.Fatalf("selections = %+v", got)
	}
	// Added ranges are ordered by position, wherever the primary is.
	editor.AddSelection(Range{Start: 0, End: 1})
	editor.AddSelection(Range{Start: 3, End: 3})
	got := editor.Selections()
	want := []Range{{Start: 0, End: 1}, {Start: 3, End: 3}, {Start: 4, End: 5}}
	if len(got) != len(want) {
		t.Fatalf("selections = %+v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("selections = %+v, want %+v", got, want)
		}
	}
	// The last one added is the primary: the caret and the next movement are on it.
	if selection := editor.Selection(); selection != (Range{Start: 3, End: 3}) || editor.Caret() != 3 {
		t.Fatalf("primary = %+v caret %d", selection, editor.Caret())
	}
}

func TestAddSelectionMergesOverlapsAndIsIdempotent(t *testing.T) {
	editor := NewEditor("abcdefgh")
	editor.SetSelection(Range{Start: 0, End: 2})
	editor.AddSelection(Range{Start: 3, End: 5})
	// Overlapping the middle of nothing new: merges both into one union.
	if !editor.AddSelection(Range{Start: 1, End: 4}) {
		t.Fatal("an overlapping selection reported no change")
	}
	if got := editor.Selections(); len(got) != 1 || got[0] != (Range{Start: 0, End: 5}) {
		t.Fatalf("merged selections = %+v", got)
	}
	if selection := editor.Selection(); selection != (Range{Start: 0, End: 5}) {
		t.Fatalf("merged primary = %+v", selection)
	}
	if editor.AddSelection(Range{Start: 0, End: 5}) {
		t.Fatal("adding the selection the editor has reported a change")
	}
	// Identical ranges on different boundaries stay separate.
	editor.AddSelection(Range{Start: 7, End: 8})
	if got := editor.SelectionCount(); got != 2 {
		t.Fatalf("selections = %+v", editor.Selections())
	}
}

func TestSetSelectionsPicksThePrimaryAndClamps(t *testing.T) {
	editor := NewEditor("abcdef")
	if !editor.SetSelections([]Range{{Start: 5, End: 6}, {Start: 0, End: 1}, {Start: 2, End: 3}}, 0) {
		t.Fatal("SetSelections reported no change")
	}
	if got := editor.Selections(); len(got) != 3 || got[0] != (Range{Start: 0, End: 1}) || got[2] != (Range{Start: 5, End: 6}) {
		t.Fatalf("selections = %+v", got)
	}
	if selection := editor.Selection(); selection != (Range{Start: 5, End: 6}) {
		t.Fatalf("primary = %+v, want the one main indexed", selection)
	}
	// Out of the text and out of order: clamped and ordered.
	editor.SetSelections([]Range{{Start: 9, End: 99}, {Start: 3, End: 4}, {Start: 4, End: 9}}, 2)
	if got := editor.Selections(); len(got) != 3 || got[0] != (Range{Start: 3, End: 4}) || got[2] != (Range{Start: 6, End: 6}) {
		t.Fatalf("clamped selections = %+v", got)
	}
	// An overlap merges into the union, and the primary is that union.
	editor.SetSelections([]Range{{Start: 2, End: 5}, {Start: 4, End: 6}, {Start: 0, End: 1}}, 1)
	if got := editor.Selections(); len(got) != 2 || got[0] != (Range{Start: 0, End: 1}) || got[1] != (Range{Start: 2, End: 6}) {
		t.Fatalf("merged selections = %+v", got)
	}
	if selection := editor.Selection(); selection != (Range{Start: 2, End: 6}) {
		t.Fatalf("merged primary = %+v", selection)
	}
	// A bad main or an empty set changes nothing about the text and reports false.
	if editor.SetSelections([]Range{{Start: 0, End: 1}}, 3) {
		t.Fatal("an out-of-range main was accepted")
	}
	if editor.SetSelections(nil, 0) {
		t.Fatal("an empty set reported a change")
	}
	if got := editor.SelectionCount(); got != 1 {
		t.Fatalf("empty set left %d selections", got)
	}
}

func TestInsertAppliesToEverySelectionBackToFront(t *testing.T) {
	// Three selections, deliberately given out of order, all get the text and the
	// text stays consistent: the shifts of the later ones never move the earlier.
	editor := NewEditor("0123456789")
	editor.SetSelections([]Range{{Start: 7, End: 8}, {Start: 0, End: 1}, {Start: 3, End: 4}}, 2)
	editor.Insert("X")
	if got := editor.String(); got != "X12X456X89" {
		t.Fatalf("text = %q", got)
	}
	if got := editor.Selections(); len(got) != 3 || got[0] != (Range{Start: 1, End: 1}) || got[1] != (Range{Start: 4, End: 4}) || got[2] != (Range{Start: 8, End: 8}) {
		t.Fatalf("carets after insert = %+v", got)
	}
	// The primary was given as the third range, 3..4: its caret is the one at 4.
	if editor.Caret() != 4 {
		t.Fatalf("primary caret = %d, want 4", editor.Caret())
	}
	// One undo unit restores all of it and leaves one selection.
	if !editor.Undo() || editor.String() != "0123456789" || editor.SelectionCount() != 1 {
		t.Fatalf("undo = %q, %d selections", editor.String(), editor.SelectionCount())
	}
}

func TestInsertAtCollapsedCaretsAndRelativePositions(t *testing.T) {
	editor := NewEditor("ab")
	editor.SetSelections([]Range{{Start: 0, End: 0}, {Start: 1, End: 1}, {Start: 2, End: 2}}, 2)
	editor.Insert("|")
	if got := editor.String(); got != "|a|b|" {
		t.Fatalf("text = %q", got)
	}
	if got := editor.Selections(); len(got) != 3 || got[0].Start != 1 || got[1].Start != 3 || got[2].Start != 5 {
		t.Fatalf("carets = %+v", got)
	}
}

func TestMultiDeleteRemovesAtEverySelection(t *testing.T) {
	editor := NewEditor("0123456789")
	editor.SetSelections([]Range{{Start: 0, End: 1}, {Start: 4, End: 5}, {Start: 8, End: 9}}, 0)
	if !editor.DeleteForward() {
		t.Fatal("DeleteForward reported nothing to delete")
	}
	if got := editor.String(); got != "1235679" {
		t.Fatalf("text = %q", got)
	}
	// The carets are where each deletion began, then in order.
	if got := editor.Selections(); len(got) != 3 || got[0].Start != 0 || got[1].Start != 3 || got[2].Start != 6 {
		t.Fatalf("carets = %+v", got)
	}
	// Backspace at carets removes the cluster before each, capped by its neighbour.
	editor = NewEditor("abc")
	editor.SetSelections([]Range{{Start: 1, End: 1}, {Start: 2, End: 2}, {Start: 3, End: 3}}, 2)
	if !editor.DeleteBackward() || editor.String() != "" {
		t.Fatalf("backspace at carets = %q", editor.String())
	}
	if got := editor.SelectionCount(); got != 1 {
		t.Fatalf("carets all collapsed to one: %d selections", got)
	}
	if !editor.Undo() || editor.String() != "abc" {
		t.Fatalf("undo = %q", editor.String())
	}
}

func TestMultiDeleteDeletesOncePerCaretWithoutCrossing(t *testing.T) {
	// Two carets one apart: the first deletes only the rune between them.
	editor := NewEditor("abc")
	editor.SetSelections([]Range{{Start: 0, End: 0}, {Start: 1, End: 1}}, 0)
	if !editor.DeleteForward() || editor.String() != "c" {
		t.Fatalf("text = %q", editor.String())
	}
	// Backspace stops at the previous caret rather than deleting through it.
	editor = NewEditor("abc")
	editor.SetSelections([]Range{{Start: 1, End: 1}, {Start: 2, End: 2}}, 0)
	if !editor.DeleteBackward() || editor.String() != "c" {
		t.Fatalf("backspace text = %q", editor.String())
	}
	// Two carets a rune apart at either end delete the two outer runes, once each.
	editor = NewEditor("abc")
	editor.SetSelections([]Range{{Start: 1, End: 1}, {Start: 3, End: 3}}, 1)
	if !editor.DeleteBackward() || editor.String() != "b" {
		t.Fatalf("spaced backspace text = %q", editor.String())
	}
}

func TestMovementsAndClearSecondaryReturnToOneSelection(t *testing.T) {
	editor := NewEditor("abcdef")
	editor.SetSelections([]Range{{Start: 0, End: 1}, {Start: 3, End: 2}}, 1)
	if !editor.ClearSecondarySelections() {
		t.Fatal("ClearSecondarySelections reported nothing to clear")
	}
	if editor.SelectionCount() != 1 || editor.Selection() != (Range{Start: 2, End: 3}) {
		t.Fatalf("after clear: %+v", editor.Selections())
	}
	editor.SetSelections([]Range{{Start: 0, End: 1}, {Start: 4, End: 4}}, 1)
	editor.MoveLeft(false)
	if editor.SelectionCount() != 1 {
		t.Fatalf("a movement kept %d selections", editor.SelectionCount())
	}
	editor.SetSelections([]Range{{Start: 0, End: 1}, {Start: 4, End: 4}}, 1)
	editor.SetSelection(Range{Start: 2, End: 2})
	if editor.SelectionCount() != 1 {
		t.Fatalf("SetSelection kept %d selections", editor.SelectionCount())
	}
}

func TestSelectionsIntoReusesItsScratch(t *testing.T) {
	editor := NewEditor("abcd")
	editor.SetSelections([]Range{{Start: 0, End: 1}, {Start: 3, End: 4}}, 1)
	dst := make([]Range, 0, 4)
	if allocations := testing.AllocsPerRun(100, func() { dst = editor.SelectionsInto(dst) }); allocations != 0 {
		t.Fatalf("SelectionsInto allocated %v times with a reused scratch", allocations)
	}
	if len(dst) != 2 {
		t.Fatalf("selections = %+v", dst)
	}
}
