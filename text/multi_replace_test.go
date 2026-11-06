package text

import "testing"

func TestReplaceRangesDisjointEditsUndoAndRedoTogether(t *testing.T) {
	editor := NewEditor("ab\n猫c\nz")
	editor.ClearHistory()
	editor.SetSelection(Range{Start: 1, End: 1})
	err := editor.ReplaceRanges([]Replacement{
		{Range: Range{Start: 1, End: 1}, Text: "X"},
		{Range: Range{Start: 4, End: 5}, Text: "D"},
		{Range: Range{Start: 6, End: 6}, Text: "!"},
	})
	if err != nil || editor.String() != "aXb\n猫D\n!z" {
		t.Fatalf("multi-replace = %q, %v", editor.String(), err)
	}
	if !editor.Undo() || editor.String() != "ab\n猫c\nz" || editor.Caret() != 1 || editor.CanUndo() {
		t.Fatalf("multi-replace undo = %q at %d", editor.String(), editor.Caret())
	}
	if !editor.Redo() || editor.String() != "aXb\n猫D\n!z" {
		t.Fatalf("multi-replace redo = %q", editor.String())
	}
}

func TestReplaceRangesRejectsInvalidOrAmbiguousEditsAtomically(t *testing.T) {
	editor := NewEditor("abc\ndef")
	editor.Insert("!")
	editor.Undo()
	for _, edits := range [][]Replacement{
		{{Range: Range{Start: 1, End: 3}, Text: "X"}, {Range: Range{Start: 2, End: 4}, Text: "Y"}},
		{{Range: Range{Start: 5, End: 5}, Text: "X"}, {Range: Range{Start: 1, End: 1}, Text: "Y"}},
		{{Range: Range{Start: 1, End: 1}, Text: "X"}, {Range: Range{Start: 1, End: 1}, Text: "Y"}},
		{{Range: Range{Start: -1, End: 0}, Text: "X"}},
		{{Range: Range{Start: 0, End: 99}, Text: "X"}},
	} {
		if err := editor.ReplaceRanges(edits); err == nil || editor.String() != "abc\ndef" || !editor.CanRedo() {
			t.Fatalf("invalid edits %+v changed editor: text=%q err=%v", edits, editor.String(), err)
		}
	}
	if err := editor.ReplaceRanges(nil); err != nil || editor.String() != "abc\ndef" || !editor.CanRedo() {
		t.Fatalf("empty edits changed editor: %q, %v", editor.String(), err)
	}
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{Start: 0, End: 1}, Text: "a"}}); err != nil || editor.String() != "abc\ndef" || !editor.CanRedo() {
		t.Fatalf("unchanged replacement consumed redo: %q, %v", editor.String(), err)
	}
}

func TestReplaceRangesDoesNotFilterUntouchedTextInRestrictedField(t *testing.T) {
	editor := NewEditor("a1b2")
	editor.Restrict(func(character rune) bool { return character >= '0' && character <= '9' }, 6)
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{Start: 1, End: 1}, Text: "3"}, {Range: Range{Start: 3, End: 3}, Text: "4"}}); err == nil || editor.String() != "a1b2" {
		t.Fatalf("restricted multi-replace changed existing text: %q, %v", editor.String(), err)
	}
	composing := NewEditor("abc")
	composing.SetMarkedText("中", Range{Start: 1, End: 1})
	before := composing.String()
	if err := composing.ReplaceRanges([]Replacement{{Range: Range{Start: 0, End: 0}, Text: "1"}}); err == nil || composing.String() != before {
		t.Fatalf("multi-replace interrupted composition: %q, %v", composing.String(), err)
	}
}
