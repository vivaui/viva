package text

import (
	"slices"
	"strings"
	"testing"
)

func lfLabelPreserveReplacement(change LFLabelReplacement) []byte {
	count := strings.Count(change.Inserted, "\n")
	labels := make([]byte, count)
	for index := range labels {
		previous := len(change.Labels) - count + index
		if previous >= 0 {
			labels[index] = change.Labels[previous]
		} else {
			labels[index] = change.Default
		}
	}
	return labels
}

func assertLFLabels(t *testing.T, editor *Editor, want []byte) {
	t.Helper()
	got, ok := editor.LFLabels(nil)
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("LF labels = %q, %t; want %q", got, ok, want)
	}
}

func TestLFLabelSeedOwnershipAndValidation(t *testing.T) {
	editor := NewEditor("a\nb\n")
	if labels, ok := editor.LFLabels(nil); ok || len(labels) != 0 {
		t.Fatal("untracked editor reported LF labels")
	}
	if editor.TrackLFLabels([]byte{'W'}, 'W', nil) {
		t.Fatal("wrong seed cardinality accepted")
	}
	seed := []byte{'W', 'L'}
	if !editor.TrackLFLabels(seed, 'W', lfLabelPreserveReplacement) {
		t.Fatal("valid LF labels refused")
	}
	seed[0] = 'M'
	assertLFLabels(t, editor, []byte{'W', 'L'})
	got, _ := editor.LFLabels(nil)
	got[0] = 'M'
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if editor.TrackLFLabels([]byte{'M', 'M'}, 'M', nil) {
		t.Fatal("tracking replaced an existing label source")
	}
	if editor.SetLFLabels([]byte{'L'}, 'L') {
		t.Fatal("wrong relabel cardinality accepted")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if editor.String() != "a\nb\n" || editor.CanUndo() || editor.CanRedo() {
		t.Fatal("label setup changed text or history")
	}
	late := NewEditor("a\n")
	late.Insert("!")
	if late.TrackLFLabels([]byte{'L'}, 'L', nil) {
		t.Fatal("tracking attached after untracked history already existed")
	}
	var absent *Editor
	if _, ok := absent.LFLabels(nil); ok || absent.TrackLFLabels(nil, 0, nil) || absent.SetLFLabels(nil, 0) || absent.UnifyLFLabels(0) {
		t.Fatal("nil editor accepted LF label state")
	}
}

func TestLFLabelSaveUndoAndIndependentInsertion(t *testing.T) {
	editor := NewEditor("a\nb\nc\nd")
	editor.SetSavePoint()
	if !editor.TrackLFLabels([]byte{'W', 'L', 'M'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{Start: 3, End: 4}, "")
	assertLFLabels(t, editor, []byte{'W', 'M'})
	editor.SetSavePoint()
	editor.ReplaceRange(Range{Start: 2, End: 2}, "new\n")
	assertLFLabels(t, editor, []byte{'W', 'W', 'M'})
	editor.SetSavePoint()
	if !editor.Undo() || editor.String() != "a\nbc\nd" {
		t.Fatal("Undo insertion failed")
	}
	assertLFLabels(t, editor, []byte{'W', 'M'})
	if !editor.Undo() || editor.String() != "a\nb\nc\nd" {
		t.Fatal("Undo deletion failed")
	}
	assertLFLabels(t, editor, []byte{'W', 'L', 'M'})
	if !editor.Redo() || !editor.Redo() || !editor.AtSavePoint() {
		t.Fatal("Redo failed to recover the latest saved boundary")
	}
	assertLFLabels(t, editor, []byte{'W', 'W', 'M'})
}

func TestLFLabelEqualTextKeepsDistinctNewlineIdentity(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	editor.SetSavePoint()
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, "a\nb\nc")
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Undo() || !editor.Redo() {
		t.Fatal("equal-text replacement lost history")
	}
	editor.ReplaceRange(Range{Start: 3, End: 4}, "")
	editor.ReplaceRange(Range{Start: 3, End: 3}, "\n")
	if editor.String() != "a\nb\nc" {
		t.Fatal("fixture no longer has equal visible text")
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
	if !editor.Undo() || !editor.Undo() {
		t.Fatal("Undo separate deletion/insertion failed")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelMapperRunsOnlyOnActualForwardReplacement(t *testing.T) {
	editor := NewEditor("a\nkeep\nc")
	calls := 0
	mapper := func(change LFLabelReplacement) []byte {
		calls++
		if change.Removed != "a\nkeep\nc" || change.Inserted != "a\nkeep\nz" || !slices.Equal(change.Labels, []byte{'W', 'L'}) || change.Default != 'W' {
			t.Fatalf("mapper received wrong actual span: %+v", change)
		}
		return []byte{'W', 'L'}
	}
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', mapper) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, "a\nkeep\nz")
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Undo() || !editor.Redo() || calls != 1 {
		t.Fatalf("history replay reran policy or lost the edit: mapper calls=%d", calls)
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelCurrentRelabelFollowsExistingRedoIdentity(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'L'}, 'L', nil) {
		t.Fatal("label setup failed")
	}
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("\n")
	if !editor.SetLFLabels([]byte{'W', 'M'}, 'W') {
		t.Fatal("valid relabel refused")
	}
	assertLFLabels(t, editor, []byte{'W', 'M'})
	if !editor.Undo() || editor.CanUndo() {
		t.Fatal("metadata relabel created another text Undo action")
	}
	assertLFLabels(t, editor, []byte{'W'})
	if !editor.Redo() {
		t.Fatal("Redo failed")
	}
	assertLFLabels(t, editor, []byte{'W', 'M'})
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("\n")
	assertLFLabels(t, editor, []byte{'W', 'M', 'W'})
}

func TestLFLabelUnifyIncludesReachableHistory(t *testing.T) {
	editor := NewEditor("a\nb\nc\nd")
	if !editor.TrackLFLabels([]byte{'W', 'L', 'M'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{Start: 3, End: 4}, "")
	if !editor.UnifyLFLabels('L') {
		t.Fatal("uniform label request refused")
	}
	assertLFLabels(t, editor, []byte{'L', 'L'})
	if !editor.Undo() || editor.CanUndo() {
		t.Fatal("uniform labels changed the text history boundary")
	}
	assertLFLabels(t, editor, []byte{'L', 'L', 'L'})
	if !editor.Redo() {
		t.Fatal("Redo failed")
	}
	assertLFLabels(t, editor, []byte{'L', 'L'})
}

func TestLFLabelCoalescedDeletionKeepsMovedHistoryBacking(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', nil) {
		t.Fatal("label setup failed")
	}
	editor.MoveTo(editor.RuneCount(), false)
	for range 4 {
		if !editor.DeleteBackward() {
			t.Fatal("coalesced deletion failed")
		}
	}
	assertLFLabels(t, editor, nil)
	if !editor.Undo() || editor.String() != "a\nb\nc" || editor.CanUndo() {
		t.Fatal("coalesced Undo did not restore its original unit")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Redo() || editor.String() != "a" {
		t.Fatal("coalesced Redo failed")
	}
	assertLFLabels(t, editor, nil)
}

func TestLFLabelPhysicalChangeRetainsEqualTextHistory(t *testing.T) {
	editor := NewEditor("a\nb")
	mapper := func(change LFLabelReplacement) []byte { return []byte{'L'} }
	if !editor.TrackLFLabels([]byte{'W'}, 'W', mapper) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, "a\nb")
	assertLFLabels(t, editor, []byte{'L'})
	if !editor.Undo() || editor.String() != "a\nb" {
		t.Fatal("physical change with equal text was not undoable")
	}
	assertLFLabels(t, editor, []byte{'W'})
	if !editor.Redo() {
		t.Fatal("equal-text physical change lost Redo")
	}
	assertLFLabels(t, editor, []byte{'L'})
}

func TestLFLabelInvalidMapperPanicsBeforeMutation(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'W', func(LFLabelReplacement) []byte { return nil }) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{Start: 2, End: 0})
	selection, caret := editor.Selection(), editor.Caret()
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		editor.ReplaceRange(Range{Start: 2, End: 2}, "new\n")
	}()
	if !panicked || editor.String() != "a\nb" || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() || editor.CanRedo() {
		t.Fatal("invalid mapper changed the source before its contract was checked")
	}
	assertLFLabels(t, editor, []byte{'W'})
}

func TestLFLabelEqualReplacementRelabelKeepsPriorState(t *testing.T) {
	editor := NewEditor("a\nb")
	editor.SetSavePoint()
	if !editor.TrackLFLabels([]byte{'W'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, "a\nb")
	if !editor.SetLFLabels([]byte{'L'}, 'L') {
		t.Fatal("current relabel failed")
	}
	if !editor.Undo() {
		t.Fatal("equal-text replacement lost Undo")
	}
	assertLFLabels(t, editor, []byte{'W'})
	if !editor.Redo() {
		t.Fatal("equal-text replacement lost Redo")
	}
	assertLFLabels(t, editor, []byte{'L'})
}
