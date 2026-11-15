package text

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestLFLabelCompositionCancelRestoresOriginalIdentity(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	calls := 0
	mapper := func(change LFLabelReplacement) []byte {
		calls++
		return lfLabelPreserveReplacement(change)
	}
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', mapper) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{Start: 4, End: 1})
	selection, caret := editor.Selection(), editor.Caret()
	editor.SetMarkedText("p\n", Range{Start: 2, End: 2})
	assertLFLabels(t, editor, []byte{'L'})
	editor.SetMarkedText("q\nr\n", Range{Start: 4, End: 4})
	assertLFLabels(t, editor, []byte{'M', 'L'})
	_ = editor.CommittedString()
	if !editor.CancelComposition() || editor.String() != "a\nb\nc" || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() || editor.CanRedo() || calls != 2 {
		t.Fatalf("cancel changed text/selection/history or reran mapping: text=%q calls=%d", editor.String(), calls)
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if editor.historyRunes != 0 || len(editor.lfLabelState().history) != 0 || len(editor.lfLabelState().pending) != 0 {
		t.Fatal("cancel retained provisional label history")
	}
	if editor.CancelComposition() {
		t.Fatal("second cancel reported a composition")
	}
}

func TestLFLabelCompositionCancelPreservesUndoAndRedo(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'M', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.MoveTo(editor.RuneCount(), false)
	editor.Paste("\n")
	editor.SetSavePoint()
	editor.Paste("tail\n")
	if !editor.Undo() || !editor.CanUndo() || !editor.CanRedo() {
		t.Fatal("fixture did not establish both history directions")
	}
	beforeRunes := editor.historyRunes
	editor.SetSelection(Range{Start: 2, End: 0})
	editor.SetMarkedText("x\n", Range{Start: 2, End: 2})
	editor.SetMarkedText("\n\n", Range{Start: 2, End: 2})
	if !editor.CancelComposition() || editor.String() != "a\nb\n" || !editor.AtSavePoint() || editor.historyRunes != beforeRunes {
		t.Fatal("cancel did not restore the original saved history")
	}
	assertLFLabels(t, editor, []byte{'W', 'M'})
	if !editor.Redo() || editor.String() != "a\nb\ntail\n" {
		t.Fatal("cancel lost the old Redo branch")
	}
	assertLFLabels(t, editor, []byte{'W', 'M', 'M'})
	if !editor.Undo() || !editor.Undo() || editor.String() != "a\nb" {
		t.Fatal("cancel lost the original Undo unit")
	}
	assertLFLabels(t, editor, []byte{'W'})
}

func TestLFLabelCompositionCommitReplaysOneUnit(t *testing.T) {
	for _, insertResult := range []bool{false, true} {
		t.Run(fmt.Sprintf("insert-result-%t", insertResult), func(t *testing.T) {
			editor := NewEditor("a\nb\nc")
			calls := 0
			if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', func(change LFLabelReplacement) []byte {
				calls++
				return lfLabelPreserveReplacement(change)
			}) {
				t.Fatal("label setup failed")
			}
			editor.SetSelection(Range{Start: 1, End: 4})
			editor.SetMarkedText("p\n", Range{Start: 2, End: 2})
			editor.SetMarkedText("q\nr\n", Range{Start: 4, End: 4})
			want := []byte{'M', 'L'}
			value := "aq\nr\nc"
			if insertResult {
				editor.Insert("z\n")
				want, value = []byte{'L'}, "az\nc"
			} else {
				editor.Unmark()
			}
			beforeReplay := calls
			assertLFLabels(t, editor, want)
			if editor.String() != value || editor.CancelComposition() || !editor.Undo() || editor.String() != "a\nb\nc" || editor.CanUndo() {
				t.Fatal("composition commit did not retain one matching history unit")
			}
			assertLFLabels(t, editor, []byte{'W', 'L'})
			if !editor.Redo() || editor.String() != value || calls != beforeReplay {
				t.Fatal("composition replay reran policy or lost text")
			}
			assertLFLabels(t, editor, want)
		})
	}
}

func TestLFLabelCompositionClearHistoryKeepsOnlyCancellationData(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-%t", cancel), func(t *testing.T) {
			editor := NewEditor("a\nb\nc")
			if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', lfLabelPreserveReplacement) {
				t.Fatal("label setup failed")
			}
			editor.MoveTo(editor.RuneCount(), false)
			editor.Paste("X\n")
			editor.SetSelection(Range{Start: 1, End: 4})
			editor.SetMarkedText("p\n", Range{Start: 2, End: 2})
			editor.ClearHistory()
			if editor.CanUndo() || editor.CanRedo() {
				t.Fatal("ClearHistory retained source history")
			}
			editor.SetMarkedText("q\nr\n", Range{Start: 4, End: 4})
			if cancel {
				if !editor.CancelComposition() || editor.String() != "a\nb\ncX\n" || editor.CanUndo() || editor.CanRedo() {
					t.Fatal("cancel after ClearHistory restored forgotten history or lost text")
				}
				assertLFLabels(t, editor, []byte{'W', 'L', 'M'})
				if editor.historyRunes != 0 || len(editor.lfLabelState().history) != 0 {
					t.Fatal("cancel after ClearHistory retained old label ownership")
				}
			} else {
				editor.Unmark()
				assertLFLabels(t, editor, []byte{'M', 'L', 'M'})
				if !editor.Undo() || editor.String() != "ap\ncX\n" || editor.CanUndo() {
					t.Fatal("post-clear commit did not start its history at the cleared text")
				}
				assertLFLabels(t, editor, []byte{'L', 'M'})
				if !editor.Redo() {
					t.Fatal("post-clear commit lost Redo")
				}
				assertLFLabels(t, editor, []byte{'M', 'L', 'M'})
			}
		})
	}
}

func TestLFLabelCompositionRetainedCostDoesNotGrowPerUpdate(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'M', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{End: editor.RuneCount()})
	for index := range 10 {
		editor.SetMarkedText(fmt.Sprintf("v%04d\n", index), Range{Start: 6, End: 6})
	}
	before := editor.historyRunes
	for index := 10; index < 1000; index++ {
		editor.SetMarkedText(fmt.Sprintf("v%04d\n", index), Range{Start: 6, End: 6})
	}
	if editor.historyRunes != before || !editor.CanUndo() {
		t.Fatalf("composition retained per-update metadata: before=%d after=%d", before, editor.historyRunes)
	}
	if !editor.CancelComposition() {
		t.Fatal("cancel failed")
	}
	assertLFLabels(t, editor, []byte{'W'})
}

func TestLFLabelCompositionRestoresTrimmedAndClearedHistory(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(fmt.Sprintf("automatic-clear-%t", clear), func(t *testing.T) {
			editor := NewEditor("a\n")
			if !editor.TrackLFLabels([]byte{'L'}, 'M', nil) {
				t.Fatal("label setup failed")
			}
			editor.MoveTo(editor.RuneCount(), false)
			for range 8 {
				editor.Paste("x\n")
			}
			before, beforeRunes := editor.String(), editor.historyRunes
			count := maxHistoryRunes/2 + 1024
			if clear {
				count = maxSingleEditRunes/8 + 1024
			}
			editor.SetMarkedText(strings.Repeat("p\n", count), Range{Start: count * 2, End: count * 2})
			if clear && editor.CanUndo() {
				t.Fatal("oversized label history was not included in automatic-clear budget")
			}
			if !editor.CancelComposition() || editor.String() != before || editor.historyRunes != beforeRunes {
				t.Fatal("cancel did not restore held history after budget pruning")
			}
			assertLFLabels(t, editor, append([]byte{'L'}, bytesOfLabel('M', 8)...))
			for remaining := 7; remaining >= 0; remaining-- {
				if !editor.Undo() {
					t.Fatalf("cancel lost original unit %d", remaining)
				}
				assertLFLabels(t, editor, append([]byte{'L'}, bytesOfLabel('M', remaining)...))
			}
		})
	}
}

func TestLFLabelHistoryBudgetCountsMetadataAndReleasesIt(t *testing.T) {
	editor := NewEditor("a\n")
	if !editor.TrackLFLabels([]byte{'W'}, 'M', nil) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, "b\n")
	textCost := editor.history[0].retainedRunes()
	if editor.historyRunes <= textCost {
		t.Fatalf("label metadata is uncharged: history=%d text=%d", editor.historyRunes, textCost)
	}
	editor.ClearHistory()
	if editor.historyRunes != 0 || len(editor.lfLabelState().history) != 0 || len(editor.lfLabelState().pending) != 0 {
		t.Fatal("ClearHistory retained label metadata")
	}
	assertLFLabels(t, editor, []byte{'M'})
}

func TestLFLabelMetadataCanPruneALargeUnit(t *testing.T) {
	const count = 30000
	editor := NewEditor(strings.Repeat("x\n", count))
	if !editor.TrackLFLabels(bytesOfLabel('L', count), 'M', nil) {
		t.Fatal("label setup failed")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, strings.Repeat("y\n", count))
	if !editor.CanUndo() {
		t.Fatal("a single affordable large unit was not retained")
	}
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("!")
	if !editor.Undo() || editor.CanUndo() {
		t.Fatal("label cost did not participate in pruning the large older unit")
	}
	assertLFLabels(t, editor, bytesOfLabel('M', count))
}

func TestLFLabelLargeShrinkReleasesCurrentBacking(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(fmt.Sprintf("keep-one-%t", keep), func(t *testing.T) {
			editor := NewEditor(strings.Repeat("x\n", 4096))
			if !editor.TrackLFLabels(bytesOfLabel('L', 4096), 'M', nil) {
				t.Fatal("label setup failed")
			}
			state := editor.lfLabelState()
			previousBacking := &state.current[0]
			start := 0
			if keep {
				start = 2
			}
			editor.ReplaceRange(Range{Start: start, End: editor.RuneCount()}, "flat")
			editor.ClearHistory()
			if keep {
				assertLFLabels(t, editor, []byte{'L'})
				if cap(state.current) > 64 || &state.current[0] == previousBacking {
					t.Fatal("large shrink retained the previous pointer backing")
				}
			} else if len(state.current) != 0 || cap(state.current) != 0 {
				t.Fatal("empty current labels retained the old pointer backing")
			}
		})
	}
}

func TestLFLabelWindowExpiryAndPrefixTrimDoNotLoseCurrentData(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'M', nil) {
		t.Fatal("label setup failed")
	}
	cursor := editor.TrackLineEdits()
	editor.MoveTo(editor.RuneCount(), false)
	for range 2000 {
		editor.Paste("x\n")
	}
	if _, _, ok := editor.LineEditsSince(cursor, nil); ok {
		t.Fatal("fixture did not expire the delivery window")
	}
	assertLFLabels(t, editor, append([]byte{'W'}, bytesOfLabel('M', 2000)...))
	undone := 0
	for editor.CanUndo() {
		if !editor.Undo() {
			t.Fatal("Undo failed")
		}
		undone++
	}
	if undone == 0 || undone >= 2000 {
		t.Fatal("fixture did not prune the history prefix")
	}
	assertLFLabels(t, editor, append([]byte{'W'}, bytesOfLabel('M', 2000-undone)...))
	editor.MoveTo(editor.RuneCount(), false)
	editor.Paste("\n")
	if editor.CanRedo() {
		t.Fatal("branch retained the discarded Redo tail")
	}
	editor.ClearHistory()
	if editor.historyRunes != 0 || len(editor.lfLabelState().history) != 0 {
		t.Fatal("branch/Clear retained unreachable label ownership")
	}
	assertLFLabels(t, editor, append([]byte{'W'}, bytesOfLabel('M', 2001-undone)...))
}

func TestLFLabelCompositionRejectedUpdateKeepsCancellationAndBudget(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', func(change LFLabelReplacement) []byte {
		if strings.HasPrefix(change.Inserted, "bad") {
			return nil
		}
		return lfLabelPreserveReplacement(change)
	}) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{Start: 1, End: 4})
	editor.SetMarkedText("p\n", Range{Start: 2, End: 2})
	before, selection, cost := editor.String(), editor.Selection(), editor.historyRunes
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		editor.SetMarkedText("bad\n\n", Range{Start: 5, End: 5})
	}()
	if !panicked || editor.String() != before || editor.Selection() != selection || editor.historyRunes != cost {
		t.Fatal("rejected marked update changed text, selection or retained budget")
	}
	assertLFLabels(t, editor, []byte{'L'})
	if !editor.CancelComposition() || editor.String() != "a\nb\nc" || editor.historyRunes != 0 || editor.CanUndo() {
		t.Fatal("rejected update damaged the cancellation snapshot")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelReplaceRangeCommitsCompositionBeforeIndependentHistory(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.SetMarkedText("x\n", Range{Start: 2, End: 2})
	editor.ReplaceRange(Range{Start: 4, End: 5}, "")
	if editor.String() != "ax\nbc" || editor.CancelComposition() {
		t.Fatal("ReplaceRange did not end composition")
	}
	assertLFLabels(t, editor, []byte{'W'})
	if !editor.Undo() || editor.String() != "ax\nb\nc" {
		t.Fatal("independent replacement lost its source text")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Undo() || editor.String() != "a\nb\nc" || editor.CanUndo() {
		t.Fatal("composition did not retain its independent Undo unit")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Redo() || !editor.Redo() || editor.String() != "ax\nbc" {
		t.Fatal("composition and independent edit lost Redo")
	}
	assertLFLabels(t, editor, []byte{'W'})
	editor.ClearHistory()
	if editor.historyRunes != 0 {
		t.Fatal("ReplaceRange retained abandoned composition history")
	}
}

func TestLFLabelTransactionCompositionLifecycle(t *testing.T) {
	for _, ending := range []string{"cancel", "commit", "error", "unfinished"} {
		t.Run(ending, func(t *testing.T) {
			editor := NewEditor("a\nb\nc")
			if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', lfLabelPreserveReplacement) {
				t.Fatal("label setup failed")
			}
			failure := errors.New("reject composition transaction")
			err := editor.Transact(func(draft *Editor) error {
				draft.SetSelection(Range{Start: 1, End: 4})
				draft.SetMarkedText("p\n", Range{Start: 2, End: 2})
				draft.SetMarkedText("q\nr\n", Range{Start: 4, End: 4})
				assertLFLabels(t, draft, []byte{'M', 'L'})
				assertLFLabels(t, editor, []byte{'W', 'L'})
				switch ending {
				case "cancel":
					if !draft.CancelComposition() {
						t.Fatal("draft cancellation failed")
					}
					assertLFLabels(t, draft, []byte{'W', 'L'})
					draft.MoveTo(draft.RuneCount(), false)
					draft.Insert("\n")
				case "commit":
					draft.Unmark()
				case "error":
					return failure
				}
				return nil
			})
			if ending == "error" || ending == "unfinished" {
				if err == nil || ending == "error" && !errors.Is(err, failure) || editor.String() != "a\nb\nc" || editor.CanUndo() || editor.historyRunes != 0 {
					t.Fatal("rejected private composition changed source ownership")
				}
				assertLFLabels(t, editor, []byte{'W', 'L'})
				return
			}
			wantText, wantLabels := "a\nb\nc\n", []byte{'W', 'L', 'M'}
			if ending == "commit" {
				wantText, wantLabels = "aq\nr\nc", []byte{'M', 'L'}
			}
			if err != nil || editor.String() != wantText {
				t.Fatalf("composition transaction: %q, %v", editor.String(), err)
			}
			assertLFLabels(t, editor, wantLabels)
			if !editor.Undo() || editor.String() != "a\nb\nc" || editor.CanUndo() {
				t.Fatal("composition transaction did not retain one source Undo unit")
			}
			assertLFLabels(t, editor, []byte{'W', 'L'})
			if !editor.Redo() || editor.String() != wantText {
				t.Fatal("composition transaction lost Redo")
			}
			assertLFLabels(t, editor, wantLabels)
			editor.ClearHistory()
			if editor.historyRunes != 0 {
				t.Fatal("composition transaction retained forgotten label history")
			}
		})
	}
}

func TestLFLabelLFFreeTypingDoesNotAllocateFragments(t *testing.T) {
	makeEditor := func(tracked bool) *Editor {
		editor := NewEditor("a\nb")
		editor.TrackLineEdits()
		if tracked && !editor.TrackLFLabels([]byte{'W'}, 'M', nil) {
			t.Fatal("label setup failed")
		}
		editor.MoveTo(editor.RuneCount(), false)
		return editor
	}
	measure := func(editor *Editor) float64 {
		return testing.AllocsPerRun(100, func() {
			editor.Insert("x")
			editor.DeleteBackward()
			editor.ClearHistory()
		})
	}
	baseline, tracked := makeEditor(false), makeEditor(true)
	baselineAllocs, trackedAllocs := measure(baseline), measure(tracked)
	if trackedAllocs != baselineAllocs {
		t.Fatalf("LF-free typing allocated label fragments: baseline=%g tracked=%g", baselineAllocs, trackedAllocs)
	}
	assertLFLabels(t, tracked, []byte{'W'})
	if len(tracked.lfLabelState().history) != 0 || len(tracked.lfLabelState().pending) != 0 || tracked.historyRunes != 0 {
		t.Fatal("LF-free typing retained label fragments")
	}
}
