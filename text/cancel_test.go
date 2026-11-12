package text

import (
	"strings"
	"testing"
)

func TestCancelCompositionWithoutCompositionDoesNothing(t *testing.T) {
	editor := NewEditor("abc")
	editor.SetSelection(Range{Start: 2, End: 1})
	if editor.CancelComposition() {
		t.Fatal("an idle cancel reported a composition")
	}
	if editor.String() != "abc" || editor.Selection() != (Range{Start: 1, End: 2}) || editor.Caret() != 1 {
		t.Fatalf("idle cancel changed the editor: %q %+v at %d", editor.String(), editor.Selection(), editor.Caret())
	}
	if editor.CanUndo() || editor.CanRedo() {
		t.Fatal("idle cancel changed history")
	}
}

func TestCancelCompositionRestoresOriginalTextAndSelection(t *testing.T) {
	for _, test := range []struct {
		name      string
		initial   string
		selection Range
		marked    []string
	}{
		{"caret", "A中B", Range{Start: 1, End: 1}, []string{"ni", "你好"}},
		{"selection", "A中B", Range{Start: 1, End: 2}, []string{"pin", "拼音"}},
		{"emoji cluster", "A👩‍💻B", Range{Start: 1, End: 4}, []string{"x", "家族"}},
		{"empty mark", "search: ", Range{Start: 8, End: 8}, []string{"ni", ""}},
		{"empty first mark", "abc", Range{Start: 1, End: 1}, []string{""}},
		{"multiple updates", "before bold after", Range{Start: 7, End: 11}, []string{"n", "ni", "你", "你好", ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := NewEditor(test.initial)
			editor.SetSelection(test.selection)
			wantSelection := editor.Selection()
			wantCaret := editor.Caret()
			wantCommitted := editor.CommittedString()
			for _, value := range test.marked {
				editor.SetMarkedText(value, Range{Start: len([]rune(value)), End: len([]rune(value))})
			}
			if _, composing := editor.Composition(); !composing {
				t.Fatal("setup did not leave a composition")
			}
			// Reading both caches during composition must not change what cancel restores.
			_ = editor.String()
			_ = editor.CommittedString()
			if !editor.CancelComposition() {
				t.Fatal("cancel did not report the composition")
			}
			if editor.String() != test.initial {
				t.Fatalf("cancel left %q, want %q", editor.String(), test.initial)
			}
			if editor.Selection() != wantSelection || editor.Caret() != wantCaret {
				t.Fatalf("cancel left selection %+v at %d, want %+v at %d", editor.Selection(), editor.Caret(), wantSelection, wantCaret)
			}
			if composition, composing := editor.Composition(); composing || composition != (Range{}) {
				t.Fatalf("cancel left composition %+v, composing=%v", composition, composing)
			}
			if editor.CommittedString() != wantCommitted {
				t.Fatalf("cancel changed committed text to %q, want %q", editor.CommittedString(), wantCommitted)
			}
			if editor.CanUndo() || editor.CanRedo() {
				t.Fatal("cancel created a history unit")
			}
			if editor.CancelComposition() {
				t.Fatal("a second cancel reported the composition")
			}
			if editor.String() != test.initial || editor.Selection() != wantSelection || editor.Caret() != wantCaret {
				t.Fatal("a second cancel changed the editor")
			}
		})
	}
}

func TestCancelCompositionRestoresAReverseUnicodeSelection(t *testing.T) {
	editor := NewEditor("A👩‍💻B")
	editor.SetSelection(Range{Start: 4, End: 1})
	if editor.Caret() != 1 {
		t.Fatalf("setup caret = %d", editor.Caret())
	}
	editor.SetMarkedText("x", Range{Start: 1, End: 1})
	if !editor.CancelComposition() {
		t.Fatal("cancel did not report the composition")
	}
	if editor.String() != "A👩‍💻B" || editor.Selection() != (Range{Start: 1, End: 4}) || editor.Caret() != 1 {
		t.Fatalf("cancel left %q with selection %+v at %d", editor.String(), editor.Selection(), editor.Caret())
	}
}

func TestCancelCompositionPreservesEarlierEditAndRedoBranch(t *testing.T) {
	editor := NewEditor("")
	editor.Insert("base")
	editor.BreakUndoGroup()
	editor.Insert(" tail")
	if !editor.Undo() || editor.String() != "base" || !editor.CanRedo() {
		t.Fatalf("setup undo left %q, redo=%v", editor.String(), editor.CanRedo())
	}
	caret := editor.Caret()
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	_ = editor.String()
	_ = editor.CommittedString()
	editor.SetMarkedText("你好", Range{Start: 2, End: 2})
	_ = editor.String()
	if !editor.CancelComposition() {
		t.Fatal("cancel did not report the composition")
	}
	if editor.String() != "base" || editor.Caret() != caret || !editor.CanRedo() {
		t.Fatalf("cancel lost the redo branch: %q at %d, redo=%v", editor.String(), editor.Caret(), editor.CanRedo())
	}
	if !editor.Redo() || editor.String() != "base tail" {
		t.Fatalf("redo after cancel = %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "base" {
		t.Fatalf("undo after redo = %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "" {
		t.Fatalf("cancel consumed the earlier edit: %q", editor.String())
	}
}

func TestCancelCompositionDoesNotResurrectClearedHistory(t *testing.T) {
	editor := NewEditor("A")
	editor.Paste("Q")
	editor.Paste("future")
	if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
		t.Fatal("setup did not leave undo and redo history")
	}
	editor.SetSelection(Range{Start: 0, End: 1})
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	editor.ClearHistory()
	if editor.CanUndo() || editor.CanRedo() {
		t.Fatal("ClearHistory did not clear both histories during composition")
	}
	editor.SetMarkedText("你", Range{Start: 1, End: 1})
	if !editor.CancelComposition() {
		t.Fatal("cancel did not report the composition")
	}
	if editor.String() != "AQ" || editor.Selection() != (Range{Start: 0, End: 1}) {
		t.Fatalf("cancel after ClearHistory left %q with %+v", editor.String(), editor.Selection())
	}
	if editor.CanUndo() || editor.CanRedo() {
		t.Fatalf("cancel resurrected forgotten history: undo=%v redo=%v", editor.CanUndo(), editor.CanRedo())
	}
}

func TestCancelCompositionRestoresHistoryTrimmedByALargeReplacement(t *testing.T) {
	editor := NewEditor("")
	const edits = 8
	for index := 0; index < edits; index++ {
		editor.BreakUndoGroup()
		editor.Insert("x")
	}
	before := editor.String()
	large := strings.Repeat("大", maxHistoryRunes+1024)
	editor.SetMarkedText(large, Range{Start: len([]rune(large)), End: len([]rune(large))})
	if !editor.CancelComposition() {
		t.Fatal("cancel did not report the composition")
	}
	if editor.String() != before {
		t.Fatalf("cancel left %d runes, want %d", editor.RuneCount(), len(before))
	}
	for index := 0; index < edits; index++ {
		if !editor.Undo() {
			t.Fatalf("cancel lost undo unit %d", index)
		}
	}
	if editor.String() != "" {
		t.Fatalf("undo after cancel left %q", editor.String())
	}
	if editor.CanUndo() || !editor.CanRedo() {
		t.Fatalf("cancel changed the trimmed history: undo=%v redo=%v", editor.CanUndo(), editor.CanRedo())
	}
}

func TestCancelCompositionLeavesCommitAndOrdinaryEmptyMarkAlone(t *testing.T) {
	editor := NewEditor("ab")
	editor.SetSelection(Range{Start: 0, End: 1})
	editor.SetMarkedText("x", Range{Start: 1, End: 1})
	editor.Unmark()
	if editor.CancelComposition() {
		t.Fatal("cancel reported a committed composition")
	}
	if editor.String() != "xb" {
		t.Fatalf("committed text = %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "ab" {
		t.Fatalf("undo after commit = %q", editor.String())
	}

	editor = NewEditor("x")
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	editor.SetMarkedText("", Range{})
	if composition, composing := editor.Composition(); !composing || composition.Start != 1 || composition.End != 1 {
		t.Fatalf("ordinary empty mark left %+v, composing=%v", composition, composing)
	}
	editor.SetMarkedText("你", Range{Start: 1, End: 1})
	if !editor.CancelComposition() {
		t.Fatal("cancel did not report the composition after an empty mark")
	}
	if editor.String() != "x" {
		t.Fatalf("cancel after an empty mark left %q", editor.String())
	}

	editor = NewEditor("")
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	editor.Insert("你")
	if editor.CancelComposition() {
		t.Fatal("cancel reported a composition committed by typing")
	}
	if editor.String() != "你" {
		t.Fatalf("typed commit left %q", editor.String())
	}
}

func TestCompositionAfterCancelUsesTheRestoredText(t *testing.T) {
	editor := NewEditor("A中B")
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.SetMarkedText("pin", Range{Start: 3, End: 3})
	if !editor.CancelComposition() {
		t.Fatal("cancel did not report the composition")
	}
	editor.SetMarkedText("新", Range{Start: 1, End: 1})
	editor.Insert("新")
	if editor.String() != "A新B" {
		t.Fatalf("composition after cancel = %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "A中B" {
		t.Fatalf("undo after cancel and commit = %q", editor.String())
	}
}

func TestCancelCompositionRestoresHistoryAfterOversizedPreedit(t *testing.T) {
	editor := NewEditor("base")
	editor.Paste("!")
	marked := strings.Repeat("x", maxSingleEditRunes+1)
	editor.SetMarkedText(marked, Range{Start: len(marked), End: len(marked)})
	if editor.CanUndo() {
		t.Fatal("oversized preedit did not exercise history clearing")
	}
	if !editor.CancelComposition() || editor.String() != "base!" {
		t.Fatalf("cancel after oversized preedit left %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "base" {
		t.Fatalf("cancel lost the original undo history: %q", editor.String())
	}
	if !editor.Redo() || editor.String() != "base!" {
		t.Fatalf("redo after oversized preedit cancel left %q", editor.String())
	}
}

func TestClearHistoryDuringCompositionClearsTrackedHistory(t *testing.T) {
	editor := NewEditor("ab\ncd")
	editor.TrackLineEdits()
	if err := editor.ReplaceRangesWith([]Replacement{
		{Range: Range{Start: 0, End: 1}, Text: "A"},
		{Range: Range{Start: 3, End: 4}, Text: "C"},
	}, ReplaceRangesOptions{Forward: true}); err != nil {
		t.Fatal(err)
	}
	if len(editor.journal.lines.batches) == 0 {
		t.Fatal("setup did not retain line batch metadata")
	}
	editor.SetSavePoint()
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	editor.ClearHistory()
	if len(editor.journal.lines.batches) != 0 || editor.journal.lines.delivery != nil {
		t.Fatal("ClearHistory retained line history metadata")
	}
	if !editor.AtSavePoint() || editor.CanUndo() || editor.CanRedo() {
		t.Fatal("ClearHistory did not reset the save point and forget history")
	}
	if !editor.CancelComposition() || editor.String() != "Ab\nCd" {
		t.Fatalf("cancel after tracked ClearHistory left %q", editor.String())
	}
	if !editor.AtSavePoint() || editor.CanUndo() || editor.CanRedo() {
		t.Fatal("cancel resurrected cleared history or changed the reset save point")
	}
}
