package text

import (
	"slices"
	"testing"
)

func TestReplaceRangesWithForwardOrderAndAtomicSelection(t *testing.T) {
	editor := NewEditor("aaXbbYcc")
	editor.SetSelection(Range{Start: 1, End: 4})
	cursor := editor.TrackEdits()
	after := Range{Start: 7, End: 3}
	err := editor.ReplaceRangesWith([]Replacement{
		{Range: Range{Start: 2, End: 3}, Text: "猫"},
		{Range: Range{Start: 5, End: 6}, Text: "é"},
	}, ReplaceRangesOptions{Forward: true, AfterSelection: &after})
	if err != nil || editor.String() != "aa猫bbécc" || editor.Selection() != (Range{Start: 3, End: 7}) || editor.Caret() != 3 {
		t.Fatalf("forward batch = %q, %v caret %d, %v", editor.String(), editor.Selection(), editor.Caret(), err)
	}
	after = Range{}
	if !editor.Undo() || editor.String() != "aaXbbYcc" || editor.Selection() != (Range{Start: 1, End: 4}) || editor.Caret() != 4 {
		t.Fatalf("batch Undo selection = %q %v caret %d", editor.String(), editor.Selection(), editor.Caret())
	}
	if !editor.Redo() || editor.Selection() != (Range{Start: 3, End: 7}) || editor.Caret() != 3 {
		t.Fatalf("batch Redo lost copied final selection: %v caret %d", editor.Selection(), editor.Caret())
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	want := []Edit{
		{Start: 2, Removed: 1, Inserted: 3}, {Start: 7, Removed: 1, Inserted: 2},
		{Start: 7, Removed: 2, Inserted: 1}, {Start: 2, Removed: 3, Inserted: 1},
		{Start: 2, Removed: 1, Inserted: 3}, {Start: 7, Removed: 1, Inserted: 2},
	}
	if !complete || !slices.Equal(edits, want) {
		t.Fatalf("forward/inverse primitive order = %+v, %v", edits, complete)
	}
}

func TestReplaceRangesWithRejectsFinalSelectionBeforeMutation(t *testing.T) {
	editor := NewEditor("abc")
	editor.Insert("!")
	editor.Undo()
	cursor := editor.TrackEdits()
	before := editor.Selection()
	for _, after := range []Range{{Start: -1, End: 0}, {Start: 0, End: 5}} {
		if err := editor.ReplaceRangesWith([]Replacement{{Range: Range{Start: 1, End: 2}, Text: "X"}}, ReplaceRangesOptions{AfterSelection: &after}); err == nil {
			t.Fatalf("invalid final selection accepted: %+v", after)
		}
		if editor.String() != "abc" || editor.Selection() != before || !editor.CanRedo() {
			t.Fatal("invalid final selection partially mutated the batch")
		}
	}
	if edits, _, complete := editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatalf("rejected options entered journal: %+v", edits)
	}
}

func TestReplaceRangesWithCanApplyEqualTextWithoutChangingUndoHistory(t *testing.T) {
	editor := NewEditor("aaa")
	editor.Insert("!")
	editor.Undo()
	cursor := editor.TrackEdits()
	after := Range{}
	err := editor.ReplaceRangesWith([]Replacement{{Range: Range{Start: 1, End: 2}, Text: "a"}}, ReplaceRangesOptions{
		Forward: true, AfterSelection: &after, ApplyUnchanged: true,
	})
	if err != nil || editor.String() != "aaa" || editor.Caret() != 0 || editor.CanUndo() || !editor.CanRedo() {
		t.Fatalf("equal batch altered history: %q caret %d, %v", editor.String(), editor.Caret(), err)
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(edits, []Edit{{Start: 1, Removed: 1, Inserted: 1}}) {
		t.Fatalf("equal target primitive = %+v, %v", edits, complete)
	}
	if !editor.Redo() || editor.String() != "aaa!" {
		t.Fatalf("equal target corrupted existing Redo: %q", editor.String())
	}
}

func TestReplaceRangesWithEqualTextDoesNotRewriteAnOlderUndoUnit(t *testing.T) {
	editor := NewEditor("aaa")
	editor.Insert("!")
	cursor := editor.TrackEdits()
	after := Range{}
	if err := editor.ReplaceRangesWith([]Replacement{{Range: Range{Start: 1, End: 2}, Text: "a"}}, ReplaceRangesOptions{ApplyUnchanged: true, AfterSelection: &after}); err != nil {
		t.Fatal(err)
	}
	if !editor.Undo() || editor.String() != "aaa" || editor.Caret() != 3 {
		t.Fatalf("equal batch corrupted older Undo: %q at %d", editor.String(), editor.Caret())
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(edits, []Edit{{Start: 1, Removed: 1, Inserted: 1}, {Start: 3, Removed: 1}}) {
		t.Fatalf("older Undo primitives were overwritten: %+v, %v", edits, complete)
	}
}

func TestReplaceRangesWithEmptyBatchDoesNotApplySelectionOptions(t *testing.T) {
	editor := NewEditor("abc")
	before := editor.Selection()
	after := Range{}
	if err := editor.ReplaceRangesWith(nil, ReplaceRangesOptions{ApplyUnchanged: true, AfterSelection: &after}); err != nil || editor.Selection() != before {
		t.Fatalf("empty batch changed selection: %+v, %v", editor.Selection(), err)
	}
}
