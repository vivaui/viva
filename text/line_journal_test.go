package text

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unsafe"
)

func requireLineEdits(t *testing.T, editor *Editor, cursor uint64, want []LineEdit) uint64 {
	t.Helper()
	got, next, complete := editor.LineEditsSince(cursor, nil)
	legacy := slices.Clone(got)
	for index := range legacy {
		legacy[index].Line, legacy[index].RemovedLines, legacy[index].InsertedLines = 0, 0, 0
	}
	if !complete || !slices.Equal(legacy, want) || next != cursor+uint64(len(want)) {
		t.Fatalf("line edits = %+v, next=%d complete=%v; want %+v after %d", got, next, complete, want, cursor)
	}
	byteEdits, byteNext, byteComplete := editor.EditsSince(cursor, nil)
	if !byteComplete || byteNext != next || len(byteEdits) != len(got) {
		t.Fatal("line and byte cursors diverged")
	}
	for index, edit := range got {
		if edit.Edit != byteEdits[index] {
			t.Fatal("line metadata changed the byte stream")
		}
	}
	return next
}

func retainedLineBatchCount(editor *Editor) int {
	return len(editor.journal.lines.batches)
}

func TestLineJournalActualUTF8AndInverseContext(t *testing.T) {
	editor := NewEditor("\u732bab")
	cursor := editor.TrackLineEdits()
	editor.MoveTo(1, false)
	editor.Insert("\n")
	editor.Undo()
	editor.Redo()
	requireLineEdits(t, editor, cursor, []LineEdit{
		{Edit: Edit{Start: 3, Inserted: 1}, InsertedLF: true},
		{Edit: Edit{Start: 3, Removed: 1}},
		{Edit: Edit{Start: 3, Inserted: 1}, InsertedLF: true},
	})
	if editor.String() != "\u732b\nab" {
		t.Fatal(editor.String())
	}
	cursor = editor.TrackLineEdits()
	editor.MoveTo(2, false)
	editor.Insert("x\n")
	requireLineEdits(t, editor, cursor, []LineEdit{{Edit: Edit{Start: 4, Inserted: 2}, AtLineStart: true, InsertedLF: true}})
}

func TestLineJournalBatchesKeepIntermediateAndInverseContext(t *testing.T) {
	for _, forward := range []bool{false, true} {
		editor := NewEditor("ab\ncd")
		cursor := editor.TrackLineEdits()
		if err := editor.ReplaceRangesWith([]Replacement{
			{Range: Range{Start: 0, End: 1}, Text: "\n"},
			{Range: Range{Start: 1, End: 2}, Text: "x"},
		}, ReplaceRangesOptions{Forward: forward}); err != nil {
			t.Fatal(err)
		}
		first := LineEdit{Edit: Edit{Start: 0, Removed: 1, Inserted: 1}, AtLineStart: true, InsertedLF: true}
		second := LineEdit{Edit: Edit{Start: 1, Removed: 1, Inserted: 1}, AtLineStart: forward}
		want := []LineEdit{first, second}
		if !forward {
			slices.Reverse(want)
		}
		cursor = requireLineEdits(t, editor, cursor, want)
		if !editor.Undo() || editor.String() != "ab\ncd" {
			t.Fatal("batch Undo failed")
		}
		first.InsertedLF = false
		want = []LineEdit{second, first}
		if !forward {
			slices.Reverse(want)
		}
		cursor = requireLineEdits(t, editor, cursor, want)
		if !editor.Redo() || editor.String() != "\nx\ncd" {
			t.Fatal("batch Redo failed")
		}
		first.InsertedLF = true
		want = []LineEdit{first, second}
		if !forward {
			slices.Reverse(want)
		}
		requireLineEdits(t, editor, cursor, want)
	}
}

func TestLineJournalTransactionKeepsTransientTextFacts(t *testing.T) {
	for _, transient := range []string{"x", "\n"} {
		editor := NewEditor("ab")
		cursor := editor.TrackLineEdits()
		if err := editor.Transact(func(draft *Editor) error {
			draft.ReplaceRange(Range{}, transient)
			draft.ReplaceRange(Range{Start: 1, End: 1}, "\n")
			draft.ReplaceRange(Range{End: 1}, "")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		firstLF := transient == "\n"
		cursor = requireLineEdits(t, editor, cursor, []LineEdit{
			{Edit: Edit{Inserted: 1}, AtLineStart: true, InsertedLF: firstLF},
			{Edit: Edit{Start: 1, Inserted: 1}, AtLineStart: firstLF, InsertedLF: true},
			{Edit: Edit{Removed: 1}, AtLineStart: true},
		})
		if !editor.Undo() || editor.String() != "ab" || editor.CanUndo() {
			t.Fatal("transaction Undo failed")
		}
		cursor = requireLineEdits(t, editor, cursor, []LineEdit{
			{Edit: Edit{Inserted: 1}, AtLineStart: true, InsertedLF: firstLF},
			{Edit: Edit{Start: 1, Removed: 1}, AtLineStart: firstLF},
			{Edit: Edit{Removed: 1}, AtLineStart: true},
		})
		if !editor.Redo() || editor.String() != "\nab" {
			t.Fatal("transaction Redo failed")
		}
		requireLineEdits(t, editor, cursor, []LineEdit{
			{Edit: Edit{Inserted: 1}, AtLineStart: true, InsertedLF: firstLF},
			{Edit: Edit{Start: 1, Inserted: 1}, AtLineStart: firstLF, InsertedLF: true},
			{Edit: Edit{Removed: 1}, AtLineStart: true},
		})
	}
}

func TestLineJournalRefusesUnavailableHistoricalContext(t *testing.T) {
	editor := NewEditor("ab")
	oldCursor := editor.TrackEdits()
	editor.Insert("x")
	if err := editor.Transact(func(draft *Editor) error {
		draft.ReplaceRange(Range{}, "\n")
		draft.ReplaceRange(Range{End: 1}, "")
		draft.ReplaceRange(Range{Start: 1, End: 2}, "B")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cursor := editor.TrackLineEdits()
	if got, _, complete := editor.LineEditsSince(oldCursor, []LineEdit{{AtLineStart: true}}); complete || len(got) != 0 {
		t.Fatal("late enable invented earlier context")
	}
	if !editor.Undo() {
		t.Fatal("expected old batch history")
	}
	if got, next, complete := editor.LineEditsSince(cursor, nil); complete || len(got) != 0 || next <= cursor {
		t.Fatal("old batch inverse claimed complete context")
	}
	cursor = editor.TrackLineEdits()
	editor.MoveTo(0, false)
	editor.Insert("\n")
	requireLineEdits(t, editor, cursor, []LineEdit{{Edit: Edit{Inserted: 1}, AtLineStart: true, InsertedLF: true}})
}

func TestLineJournalGapRejectsPartialSequenceAndReusesDestination(t *testing.T) {
	editor := NewEditor("")
	cursor := editor.TrackLineEdits()
	for range EditJournalLimit + 2 {
		editor.Insert("x")
	}
	buffer := make([]LineEdit, 0, EditJournalLimit)
	if got, _, complete := editor.LineEditsSince(cursor, buffer); complete || len(got) != 0 {
		t.Fatal("expired context returned a partial sequence")
	}
	got, next, complete := editor.LineEditsSince(cursor+2, buffer)
	if !complete || len(got) != EditJournalLimit || got[0].Start != 2 || got[0].AtLineStart || got[len(got)-1].Start != EditJournalLimit+1 {
		t.Fatal("ring contexts lost byte alignment")
	}
	if empty, same, ok := editor.LineEditsSince(next, got); !ok || len(empty) != 0 || same != next {
		t.Fatal("empty query did not reset its destination")
	}
	if got, _, complete := editor.LineEditsSince(next+1, buffer); complete || len(got) != 0 {
		t.Fatal("future cursor accepted")
	}
}

func TestLineJournalOptInRefusalRestrictionAndNonmutation(t *testing.T) {
	editor := NewEditor("12")
	editor.TrackEdits()
	if editor.journal.lines != nil {
		t.Fatal("byte-only tracking allocated line metadata")
	}
	if _, _, complete := editor.LineEditsSince(0, nil); complete {
		t.Fatal("byte-only tracking claimed line context")
	}
	cursor := editor.TrackLineEdits()
	editor.Restrict(unicode.IsDigit, 3)
	editor.Insert("x\n34")
	requireLineEdits(t, editor, cursor, []LineEdit{{Edit: Edit{Start: 2, Inserted: 1}}})
	cursor = editor.TrackLineEdits()
	want := errors.New("abort")
	if err := editor.Transact(func(*Editor) error { return want }); err == nil {
		t.Fatal("restricted transaction accepted")
	}
	editor.MoveTo(0, false)
	editor.ClearHistory()
	requireLineEdits(t, editor, cursor, nil)
	editor.Restrict(nil, 0)
	if err := editor.Transact(func(draft *Editor) error {
		draft.Insert("\n")
		return want
	}); !errors.Is(err, want) {
		t.Fatal(err)
	}
	requireLineEdits(t, editor, cursor, nil)
}

func TestLineJournalForgetsDiscardedBatchHistory(t *testing.T) {
	for _, action := range []string{"clear", "branch", "trim"} {
		t.Run(action, func(t *testing.T) {
			editor := NewEditor("ab")
			editor.TrackLineEdits()
			if err := editor.ReplaceRanges([]Replacement{{Range: Range{End: 1}, Text: "\n"}}); err != nil {
				t.Fatal(err)
			}
			if retainedLineBatchCount(editor) != 1 {
				t.Fatal("batch context not retained for history")
			}
			switch action {
			case "clear":
				editor.ClearHistory()
			case "branch":
				editor.Undo()
				editor.Insert("x")
			case "trim":
				for range maxHistoryEntries + 1 {
					editor.BreakUndoGroup()
					editor.Insert("x")
				}
			}
			if retainedLineBatchCount(editor) != 0 {
				t.Fatal("discarded history retained context")
			}
		})
	}
}

func TestLineJournalOversizedBatchStillReportsActualContext(t *testing.T) {
	editor := NewEditor("a")
	cursor := editor.TrackLineEdits()
	value := strings.Repeat("x", maxSingleEditRunes+1) + "\n"
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{End: 1}, Text: value}}); err != nil {
		t.Fatal(err)
	}
	requireLineEdits(t, editor, cursor, []LineEdit{{Edit: Edit{Removed: 1, Inserted: len(value)}, AtLineStart: true, InsertedLF: true}})
	if editor.CanUndo() || retainedLineBatchCount(editor) != 0 {
		t.Fatal("discarded oversized batch retained history metadata")
	}
}

func TestLineJournalNestedAndEqualTransactions(t *testing.T) {
	editor := NewEditor("ab")
	cursor := editor.TrackLineEdits()
	if err := editor.Transact(func(draft *Editor) error {
		draft.ReplaceRange(Range{}, "x")
		return draft.Transact(func(nested *Editor) error {
			nested.ReplaceRange(Range{Start: 1, End: 1}, "\n")
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
	cursor = requireLineEdits(t, editor, cursor, []LineEdit{
		{Edit: Edit{Inserted: 1}, AtLineStart: true},
		{Edit: Edit{Start: 1, Inserted: 1}, InsertedLF: true},
	})
	if !editor.Undo() || editor.String() != "ab" || editor.CanUndo() {
		t.Fatal("nested transaction history changed")
	}
	requireLineEdits(t, editor, cursor, []LineEdit{
		{Edit: Edit{Start: 1, Removed: 1}},
		{Edit: Edit{Removed: 1}, AtLineStart: true},
	})
	editor = NewEditor("")
	editor.SetSavePoint()
	cursor = editor.TrackLineEdits()
	if err := editor.Transact(func(draft *Editor) error {
		draft.Insert("\n")
		draft.ReplaceRange(Range{End: 1}, "")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []LineEdit{
		{Edit: Edit{Inserted: 1}, AtLineStart: true, InsertedLF: true},
		{Edit: Edit{Removed: 1}, AtLineStart: true},
	}
	cursor = requireLineEdits(t, editor, cursor, want)
	if !editor.Undo() || !editor.AtSavePoint() || editor.String() != "" {
		t.Fatal("equal-byte transaction lost its real save-point history")
	}
	requireLineEdits(t, editor, cursor, want)
}

func TestLineJournalManyBatchesStayBoundedAcrossTrimAndBranch(t *testing.T) {
	editor := NewEditor("ab")
	editor.TrackLineEdits()
	for index := range maxHistoryEntries + 12 {
		value := "A"
		if index%2 != 0 {
			value = "B"
		}
		if err := editor.ReplaceRanges([]Replacement{{Range: Range{End: 1}, Text: value}}); err != nil {
			t.Fatal(err)
		}
	}
	if retainedLineBatchCount(editor) != maxHistoryEntries {
		t.Fatal("pruned batches remained in the sidecar")
	}
	for range maxHistoryEntries / 2 {
		cursor := editor.TrackLineEdits()
		if !editor.Undo() {
			t.Fatal("missing retained Undo")
		}
		requireLineEdits(t, editor, cursor, []LineEdit{{Edit: Edit{Removed: 1, Inserted: 1}, AtLineStart: true}})
	}
	editor.Insert("x")
	if editor.CanRedo() || retainedLineBatchCount(editor) != maxHistoryEntries/2 {
		t.Fatal("branch retained discarded batch contexts")
	}
	editor.ClearHistory()
	if retainedLineBatchCount(editor) != 0 {
		t.Fatal("history clear retained sidecar nodes")
	}
}

func TestLineJournalCompositionAndReusableQuery(t *testing.T) {
	editor := NewEditor("ab")
	cursor := editor.TrackLineEdits()
	editor.MoveTo(1, false)
	editor.SetMarkedText("\n", Range{Start: 1, End: 1})
	editor.SetMarkedText("X", Range{Start: 1, End: 1})
	editor.Insert("X")
	editor.Undo()
	editor.Redo()
	want := []LineEdit{
		{Edit: Edit{Start: 1, Inserted: 1}, InsertedLF: true},
		{Edit: Edit{Start: 1, Removed: 1, Inserted: 1}},
		{Edit: Edit{Start: 1, Removed: 1, Inserted: 1}},
		{Edit: Edit{Start: 1, Removed: 1}},
		{Edit: Edit{Start: 1, Inserted: 1}},
	}
	requireLineEdits(t, editor, cursor, want)
	buffer := make([]LineEdit, 0, len(want))
	allocations := testing.AllocsPerRun(20, func() {
		if _, _, complete := editor.LineEditsSince(cursor, buffer); !complete {
			panic("lost query context")
		}
	})
	if allocations != 0 {
		t.Fatalf("reused query allocated %.0f times", allocations)
	}
}

var lineJournalBenchmarkEditor *Editor

func BenchmarkLineJournalMetadata(b *testing.B) {
	for _, mode := range []string{"default", "bytes", "lines"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(unsafe.Sizeof(Editor{})), "editor-B")
			b.ReportMetric(float64(unsafe.Sizeof(lineContext{})), "context-B")
			for range b.N {
				editor := NewEditor("base")
				if mode == "bytes" {
					editor.TrackEdits()
				} else if mode == "lines" {
					editor.TrackLineEdits()
				}
				lineJournalBenchmarkEditor = editor
			}
		})
	}
}

func BenchmarkLineJournalTyping(b *testing.B) {
	for _, mode := range []string{"default", "bytes", "lines"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				editor := NewEditor("")
				if mode == "bytes" {
					editor.TrackEdits()
				} else if mode == "lines" {
					editor.TrackLineEdits()
				}
				for _, value := range []string{"h", "e", "\n", "l", "l", "o"} {
					editor.Insert(value)
				}
				editor.Undo()
				editor.Redo()
			}
		})
	}
}
