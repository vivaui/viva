package text

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode"
)

func transact(t *testing.T, editor *Editor, apply func(*Editor) error) error {
	t.Helper()
	transactional, ok := any(editor).(interface {
		Transact(func(*Editor) error) error
	})
	if !ok {
		t.Fatal("editor has no isolated transaction capability")
	}
	return transactional.Transact(apply)
}

func TestTransactionCommitsActualUTF8EditsAndDirectionalHistory(t *testing.T) {
	editor := NewEditor("猫ab")
	editor.SetSelection(Range{Start: 2, End: 1})
	cursor := editor.TrackEdits()
	var retained *Editor
	err := transact(t, editor, func(draft *Editor) error {
		retained = draft
		if draft == editor || draft.Caret() != 1 || draft.Selection() != (Range{Start: 1, End: 2}) {
			t.Fatal("transaction did not isolate the original directional selection")
		}
		draft.Insert("é")
		draft.MoveTo(2, false)
		draft.Insert("!")
		draft.SetSelection(Range{Start: 3, End: 1})
		if editor.String() != "猫ab" || editor.CanUndo() {
			t.Fatal("draft mutation escaped before commit")
		}
		return nil
	})
	if err != nil || editor.String() != "猫é!b" || editor.Selection() != (Range{Start: 1, End: 3}) || editor.Caret() != 1 {
		t.Fatalf("transaction result = %q selection=%v caret=%d: %v", editor.String(), editor.Selection(), editor.Caret(), err)
	}
	retained.Insert("later")
	if editor.String() != "猫é!b" {
		t.Fatal("retained draft shares committed text")
	}
	if !editor.Undo() || editor.String() != "猫ab" || editor.Selection() != (Range{Start: 1, End: 2}) || editor.Caret() != 1 || editor.CanUndo() {
		t.Fatal("transaction Undo did not restore one original directional history unit")
	}
	if !editor.Redo() || editor.String() != "猫é!b" || editor.Caret() != 1 || editor.Selection() != (Range{Start: 1, End: 3}) {
		t.Fatal("transaction Redo lost its final selection")
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	want := []Edit{
		{Start: 3, Removed: 1, Inserted: 2}, {Start: 5, Inserted: 1},
		{Start: 5, Removed: 1}, {Start: 3, Removed: 2, Inserted: 1},
		{Start: 3, Removed: 1, Inserted: 2}, {Start: 5, Inserted: 1},
	}
	if !complete || !slices.Equal(edits, want) {
		t.Fatalf("transaction journal = %+v complete=%v, want %+v", edits, complete, want)
	}
}

func TestTransactionFailurePreservesSourceAndRedo(t *testing.T) {
	want := errors.New("refused draft")
	editor := NewEditor("base")
	editor.Insert("!")
	editor.Undo()
	cursor := editor.TrackEdits()
	err := transact(t, editor, func(draft *Editor) error {
		draft.Insert("uncommitted")
		return want
	})
	if !errors.Is(err, want) || editor.String() != "base" || editor.Caret() != 4 || editor.CanUndo() || !editor.CanRedo() {
		t.Fatal("failed transaction changed source or history")
	}
	if edits, _, complete := editor.EditsSince(cursor, nil); !complete || len(edits) != 0 {
		t.Fatal("failed transaction changed the source journal")
	}
	if err := transact(t, editor, nil); err == nil {
		t.Fatal("nil transaction callback was accepted")
	}
	if !editor.Redo() || editor.String() != "base!" {
		t.Fatal("failed transaction discarded an existing Redo unit")
	}
}

func TestTransactionRefusesComposingRestrictedAndReplacedDrafts(t *testing.T) {
	for _, scenario := range []struct {
		name string
		set  func(*Editor)
	}{
		{"composition", func(editor *Editor) { editor.SetMarkedText("ni", Range{Start: 2, End: 2}) }},
		{"restricted", func(editor *Editor) { editor.Restrict(unicode.IsDigit, 8) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := NewEditor("abc")
			scenario.set(editor)
			before, selection, caret := editor.String(), editor.Selection(), editor.Caret()
			composition, composing := editor.Composition()
			canUndo, canRedo := editor.CanUndo(), editor.CanRedo()
			called := false
			if err := transact(t, editor, func(*Editor) error { called = true; return nil }); err == nil || called {
				t.Fatal("ineligible source ran a transaction callback")
			}
			if editor.String() != before || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() != canUndo || editor.CanRedo() != canRedo {
				t.Fatal("ineligible source changed during refusal")
			}
			if got, active := editor.Composition(); got != composition || active != composing {
				t.Fatal("refused transaction ended source composition")
			}
		})
	}
	for _, apply := range []func(*Editor) error{
		func(draft *Editor) error { draft.SetMarkedText("ni", Range{Start: 2, End: 2}); return nil },
		func(draft *Editor) error { *draft = *NewEditor("unrecorded"); return nil },
	} {
		editor := NewEditor("abc")
		if err := transact(t, editor, apply); err == nil || editor.String() != "abc" || editor.CanUndo() {
			t.Fatal("uncommitted composition or replaced recorder escaped the draft")
		}
	}
}

func TestTransactionKeepsEqualNetMutationsWithoutInventingHistory(t *testing.T) {
	editor := NewEditor("aaaa")
	editor.Insert("!")
	editor.Undo()
	cursor := editor.TrackEdits()
	if err := transact(t, editor, func(draft *Editor) error {
		draft.SetSelection(Range{Start: 3, End: 4})
		draft.Insert("a")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(edits, []Edit{{Start: 3, Removed: 1, Inserted: 1}}) || editor.String() != "aaaa" || editor.CanUndo() || !editor.CanRedo() {
		t.Fatal("equal-net transaction skipped primitives or rewrote prior history")
	}
	if err := transact(t, editor, func(draft *Editor) error { draft.MoveTo(1, false); return nil }); err != nil || editor.Caret() != 1 || !editor.CanRedo() {
		t.Fatal("cursor-only transaction changed history")
	}
}

func TestTransactionCopiesModesAndCapturesNestedBatchEdits(t *testing.T) {
	editor := NewEditor("a-b")
	editor.SetCode(CodeOptions{TabWidth: 2, AutoClose: true})
	editor.SetOverwrite(true)
	editor.MoveTo(0, false)
	if err := transact(t, editor, func(draft *Editor) error {
		if options, enabled := draft.Code(); !enabled || options.TabWidth != 2 || !options.AutoClose {
			t.Fatal("transaction lost code options")
		}
		draft.Insert("A")
		if draft.String() != "A-b" {
			t.Fatal("transaction lost overwrite mode")
		}
		if err := draft.ReplaceRangesWith([]Replacement{{Range: Range{Start: 2, End: 3}, Text: "B"}}, ReplaceRangesOptions{Forward: true}); err != nil {
			return err
		}
		return transact(t, draft, func(inner *Editor) error {
			inner.SetOverwrite(false)
			inner.MoveTo(1, false)
			inner.Insert("!")
			return nil
		})
	}); err != nil || editor.String() != "A!-B" {
		t.Fatalf("nested transaction = %q: %v", editor.String(), err)
	}
	cursor := editor.TrackEdits()
	if !editor.Undo() || editor.String() != "a-b" || editor.CanUndo() {
		t.Fatal("nested transaction was not one history unit")
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	want := []Edit{{Start: 1, Removed: 1}, {Start: 2, Removed: 1, Inserted: 1}, {Start: 0, Removed: 1, Inserted: 1}}
	if !complete || !slices.Equal(edits, want) {
		t.Fatalf("late tracking lost nested primitives: %+v, want %+v", edits, want)
	}
}

func TestTransactionRetainsMoreThanTheOrdinaryJournalLimit(t *testing.T) {
	editor := NewEditor("x")
	count := EditJournalLimit + 17
	if err := transact(t, editor, func(draft *Editor) error {
		for range count {
			draft.MoveTo(0, false)
			draft.Insert("!")
		}
		return nil
	}); err != nil || editor.String() != strings.Repeat("!", count)+"x" {
		t.Fatal("long transaction refused or lost staged text")
	}
	if editor.journal != nil || len(editor.history) != 1 || len(editor.history[0].primitives) != count {
		t.Fatal("long transaction truncated temporary primitives or enabled source tracking")
	}
	if !editor.Undo() || editor.String() != "x" || !editor.Redo() || editor.String() != strings.Repeat("!", count)+"x" {
		t.Fatal("long transaction history was incomplete")
	}
}

func TestTransactionPrimitiveMetadataRespectsTheHistoryBudget(t *testing.T) {
	editor := NewEditor("a")
	const transactions, changes = 80, 701
	for range transactions {
		if err := transact(t, editor, func(draft *Editor) error {
			for range changes {
				value := "b"
				if draft.String() == "b" {
					value = "a"
				}
				draft.ReplaceRange(Range{End: 1}, value)
				draft.ClearHistory()
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	undos := 0
	for editor.Undo() {
		undos++
	}
	if undos == 0 || undos >= transactions {
		t.Fatalf("retained %d small-text transactions: their primitive metadata exceeds the ordinary history budget", undos)
	}
	editor.Insert("x")
	if editor.historyRunes != 1 || editor.CanRedo() {
		t.Fatal("dropping transaction Redo did not release its metadata accounting")
	}
}

func TestTransactionRejectsStaleSourceAndReleasesDraftAfterPanic(t *testing.T) {
	editor := NewEditor("base")
	if err := transact(t, editor, func(draft *Editor) error {
		draft.Insert("draft")
		editor.MoveTo(0, false)
		return nil
	}); err == nil || editor.String() != "base" || editor.Caret() != 0 || editor.CanUndo() {
		t.Fatal("transaction overwrote a source changed by its callback")
	}
	var retained *Editor
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("transaction swallowed a callback panic")
			}
		}()
		_ = transact(t, editor, func(draft *Editor) error {
			retained = draft
			draft.Insert("private")
			panic("callback panic")
		})
	}()
	if editor.String() != "base" || editor.Caret() != 0 || editor.CanUndo() || retained == nil || retained.journal != nil {
		t.Fatal("panicking callback changed the source or retained its temporary recorder")
	}
}
