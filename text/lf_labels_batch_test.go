package text

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestLFLabelMultipleSelectionsUseActualSpans(t *testing.T) {
	for _, operation := range []string{"replace", "insert", "delete-backward", "delete-forward"} {
		t.Run(operation, func(t *testing.T) {
			editor := NewEditor("a\nb\nc\nd")
			if !editor.TrackLFLabels([]byte{'W', 'L', 'M'}, 'D', lfLabelPreserveReplacement) {
				t.Fatal("label setup failed")
			}
			selections := []Range{{Start: 1, End: 2}, {Start: 5, End: 6}}
			wantText, wantLabels := "ax\nb\ncx\nd", []byte{'W', 'L', 'M'}
			switch operation {
			case "insert":
				selections = []Range{{}, {Start: 4, End: 4}}
				wantText, wantLabels = "\na\nb\n\nc\nd", []byte{'D', 'W', 'L', 'D', 'M'}
			case "delete-backward":
				selections = []Range{{Start: 2, End: 2}, {Start: 6, End: 6}}
				wantText, wantLabels = "ab\ncd", []byte{'L'}
			case "delete-forward":
				selections = []Range{{Start: 1, End: 1}, {Start: 5, End: 5}}
				wantText, wantLabels = "ab\ncd", []byte{'L'}
			}
			if !editor.SetSelections(selections, 1) {
				t.Fatal("multiple-selection setup failed")
			}
			switch operation {
			case "replace":
				editor.Insert("x\n")
			case "insert":
				editor.Insert("\n")
			case "delete-backward":
				if !editor.DeleteBackward() {
					t.Fatal("multiple deletion failed")
				}
			case "delete-forward":
				if !editor.DeleteForward() {
					t.Fatal("multiple deletion failed")
				}
			}
			if editor.String() != wantText || editor.SelectionCount() != 2 {
				t.Fatalf("multiple-selection text/count: %q, %d", editor.String(), editor.SelectionCount())
			}
			assertLFLabels(t, editor, wantLabels)
			if !editor.Undo() || editor.String() != "a\nb\nc\nd" || editor.CanUndo() {
				t.Fatal("multiple selections did not retain one matching Undo unit")
			}
			assertLFLabels(t, editor, []byte{'W', 'L', 'M'})
			if !editor.Redo() || editor.String() != wantText {
				t.Fatal("multiple selections lost matching Redo")
			}
			assertLFLabels(t, editor, wantLabels)
		})
	}
}

func TestLFLabelInvalidMapperPreservesMultipleSelections(t *testing.T) {
	for _, command := range []string{"insert", "marked", "range", "block"} {
		t.Run(command, func(t *testing.T) {
			editor := NewEditor("a\nb\nc")
			if !editor.TrackLFLabels([]byte{'W', 'L'}, 'D', func(LFLabelReplacement) []byte { return nil }) {
				t.Fatal("label setup failed")
			}
			if !editor.SetSelections([]Range{{Start: 1, End: 1}, {Start: 3, End: 3}}, 1) {
				t.Fatal("multiple-selection setup failed")
			}
			selections := editor.Selections()
			panicked := false
			func() {
				defer func() { panicked = recover() != nil }()
				switch command {
				case "marked":
					editor.SetMarkedText("new\n", Range{Start: 4, End: 4})
				case "range":
					editor.ReplaceRange(Range{}, "new\n")
				case "block":
					editor.DuplicateLines(false)
				default:
					editor.Insert("new\n")
				}
			}()
			if !panicked || editor.String() != "a\nb\nc" || !slices.Equal(editor.Selections(), selections) || editor.CanUndo() || editor.CanRedo() || editor.historyRunes != 0 {
				t.Fatal("invalid mapper changed the source selection set or history")
			}
			if _, composing := editor.Composition(); composing {
				t.Fatal("invalid mapper started composition")
			}
			assertLFLabels(t, editor, []byte{'W', 'L'})
		})
	}
}

func TestLFLabelTransactionRejectsSourceSecondarySelectionChanges(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', nil) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{})
	selections := []Range{{}, {Start: 3, End: 3}}
	err := editor.Transact(func(draft *Editor) error {
		editor.SetSelections(selections, 0)
		draft.Insert("x\n")
		return nil
	})
	if err == nil || editor.String() != "a\nb\nc" || !slices.Equal(editor.Selections(), selections) || editor.CanUndo() || editor.historyRunes != 0 {
		t.Fatal("transaction committed over a source secondary selection change")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelBatchRejectsSourceSecondarySelectionChanges(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	selections := []Range{{}, {Start: 3, End: 3}}
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'M', func(change LFLabelReplacement) []byte {
		editor.SetSelections(selections, 0)
		return lfLabelPreserveReplacement(change)
	}) {
		t.Fatal("label setup failed")
	}
	editor.SetSelection(Range{})
	err := editor.ReplaceRanges([]Replacement{{Range: Range{}, Text: "x\n"}})
	if err == nil || editor.String() != "a\nb\nc" || !slices.Equal(editor.Selections(), selections) || editor.CanUndo() || editor.historyRunes != 0 {
		t.Fatal("batch committed over a source secondary selection change")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelBatchOrderAndReplay(t *testing.T) {
	for _, forward := range []bool{false, true} {
		t.Run(fmt.Sprintf("forward-%t", forward), func(t *testing.T) {
			editor := NewEditor("a\nb\nc\nd\ne")
			editor.SetSavePoint()
			if !editor.TrackLFLabels([]byte{'W', 'L', 'M', 'W'}, 'W', lfLabelPreserveReplacement) {
				t.Fatal("label setup failed")
			}
			editor.SetSelection(Range{Start: 3, End: 0})
			selection, caret := editor.Selection(), editor.Caret()
			after := Range{Start: 5, End: 1}
			err := editor.ReplaceRangesWith([]Replacement{
				{Range: Range{Start: 3, End: 4}},
				{Range: Range{Start: 6, End: 6}, Text: "new\n"},
			}, ReplaceRangesOptions{Forward: forward, AfterSelection: &after})
			if err != nil || editor.String() != "a\nbc\nnew\nd\ne" || editor.Caret() != 1 {
				t.Fatalf("batch text/selection = %q, %d, %v", editor.String(), editor.Caret(), err)
			}
			assertLFLabels(t, editor, []byte{'W', 'M', 'W', 'W'})
			for range 2 {
				if !editor.Undo() || editor.String() != "a\nb\nc\nd\ne" || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() || !editor.AtSavePoint() {
					t.Fatal("batch Undo lost its source or history boundary")
				}
				assertLFLabels(t, editor, []byte{'W', 'L', 'M', 'W'})
				if !editor.Redo() || editor.String() != "a\nbc\nnew\nd\ne" || editor.Caret() != 1 {
					t.Fatal("batch Redo failed")
				}
				assertLFLabels(t, editor, []byte{'W', 'M', 'W', 'W'})
			}
		})
	}
}

func TestLFLabelBatchMapperReceivesActualDisjointSpans(t *testing.T) {
	for _, forward := range []bool{false, true} {
		t.Run(fmt.Sprintf("forward-%t", forward), func(t *testing.T) {
			editor := NewEditor("a\nkeep\nz\n")
			calls := 0
			mapper := func(change LFLabelReplacement) []byte {
				calls++
				want := byte('W')
				switch change.Removed {
				case "a\n":
					if change.Inserted != "A\n" {
						t.Fatalf("first replacement inserted %q", change.Inserted)
					}
				case "z\n":
					want = 'M'
					if change.Inserted != "Z\n" {
						t.Fatalf("last replacement inserted %q", change.Inserted)
					}
				default:
					t.Fatalf("mapper received aggregate or unchanged gap %q", change.Removed)
				}
				if !slices.Equal(change.Labels, []byte{want}) {
					t.Fatalf("mapper received labels %q for %q", change.Labels, change.Removed)
				}
				return change.Labels
			}
			if !editor.TrackLFLabels([]byte{'W', 'L', 'M'}, 'W', mapper) {
				t.Fatal("label setup failed")
			}
			if err := editor.ReplaceRangesWith([]Replacement{
				{Range: Range{Start: 0, End: 2}, Text: "A\n"},
				{Range: Range{Start: 7, End: 9}, Text: "Z\n"},
			}, ReplaceRangesOptions{Forward: forward}); err != nil {
				t.Fatal(err)
			}
			assertLFLabels(t, editor, []byte{'W', 'L', 'M'})
			if !editor.Undo() || !editor.Redo() || editor.String() != "A\nkeep\nZ\n" || calls != 2 {
				t.Fatalf("batch replay changed policy calls or text: calls=%d text=%q", calls, editor.String())
			}
			assertLFLabels(t, editor, []byte{'W', 'L', 'M'})
		})
	}
}

func TestLFLabelBatchEqualTextPhysicalChange(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'W', func(LFLabelReplacement) []byte { return []byte{'M'} }) {
		t.Fatal("label setup failed")
	}
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{End: editor.RuneCount()}, Text: "a\nb"}}); err != nil {
		t.Fatal(err)
	}
	assertLFLabels(t, editor, []byte{'M'})
	if !editor.Undo() || editor.String() != "a\nb" {
		t.Fatal("equal-text label change was not undoable")
	}
	assertLFLabels(t, editor, []byte{'W'})
	if !editor.Redo() {
		t.Fatal("equal-text label change lost Redo")
	}
	assertLFLabels(t, editor, []byte{'M'})
}

func TestLFLabelBatchInvalidMapperIsAtomic(t *testing.T) {
	editor := NewEditor("a\nb\n")
	calls := 0
	mapper := func(change LFLabelReplacement) []byte {
		calls++
		if calls == 2 {
			return nil
		}
		return []byte{'W'}
	}
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', mapper) {
		t.Fatal("label setup failed")
	}
	cursor := editor.TrackEdits()
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		_ = editor.ReplaceRangesWith([]Replacement{
			{Range: Range{Start: 0, End: 0}, Text: "x\n"},
			{Range: Range{Start: 4, End: 4}, Text: "y\n"},
		}, ReplaceRangesOptions{Forward: true})
	}()
	if !panicked || calls != 2 || editor.String() != "a\nb\n" || editor.CanUndo() || editor.CanRedo() {
		t.Fatalf("late invalid mapper partially committed: panic=%t calls=%d text=%q", panicked, calls, editor.String())
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if edits, _, ok := editor.EditsSince(cursor, nil); !ok || len(edits) != 0 {
		t.Fatal("late invalid mapper published source edits")
	}
}

func TestLFLabelBatchRejectedTargetDoesNotRunPolicy(t *testing.T) {
	editor := NewEditor("a\nb\n")
	calls := 0
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', func(change LFLabelReplacement) []byte {
		calls++
		return lfLabelPreserveReplacement(change)
	}) {
		t.Fatal("label setup failed")
	}
	if err := editor.ReplaceRanges([]Replacement{
		{Range: Range{Start: 0, End: 0}, Text: "new\n"},
		{Range: Range{Start: 99, End: 99}, Text: "bad\n"},
	}); err == nil || calls != 0 || editor.String() != "a\nb\n" || editor.CanUndo() {
		t.Fatal("invalid target reached policy or changed the source")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelTransactionCommitAndDraftReplay(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	editor.SetSavePoint()
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	err := editor.Transact(func(draft *Editor) error {
		draft.ReplaceRange(Range{Start: 2, End: 2}, "new\n")
		assertLFLabels(t, draft, []byte{'W', 'W', 'L'})
		if !draft.Undo() {
			t.Fatal("draft Undo failed")
		}
		assertLFLabels(t, draft, []byte{'W', 'L'})
		if !draft.Redo() {
			t.Fatal("draft Redo failed")
		}
		draft.ReplaceRange(Range{Start: 7, End: 8}, "")
		assertLFLabels(t, draft, []byte{'W', 'W'})
		if editor.String() != "a\nb\nc" {
			t.Fatal("draft changed source text before commit")
		}
		assertLFLabels(t, editor, []byte{'W', 'L'})
		return nil
	})
	if err != nil || editor.String() != "a\nnew\nbc" {
		t.Fatalf("transaction commit = %q, %v", editor.String(), err)
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
	if !editor.Undo() || editor.String() != "a\nb\nc" || editor.CanUndo() || !editor.AtSavePoint() {
		t.Fatal("transaction did not retain one source Undo action")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Redo() {
		t.Fatal("transaction Redo failed")
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
}

func TestLFLabelTransactionFailureAndRetry(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	wantErr := errors.New("stop staged edits")
	err := editor.Transact(func(draft *Editor) error {
		draft.ReplaceRange(Range{Start: 3, End: 4}, "")
		draft.MoveTo(draft.RuneCount(), false)
		draft.Insert("new\n")
		return wantErr
	})
	if !errors.Is(err, wantErr) || editor.String() != "a\nb\nc" || editor.CanUndo() {
		t.Fatal("failed transaction changed the source")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if err := editor.Transact(func(draft *Editor) error {
		draft.MoveTo(draft.RuneCount(), false)
		draft.Insert("\n")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	assertLFLabels(t, editor, []byte{'W', 'L', 'W'})
	if !editor.Undo() {
		t.Fatal("retry lost Undo")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
}

func TestLFLabelTransactionPrivateConfigurationRefused(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		t.Run(fmt.Sprintf("tracked-%t", tracked), func(t *testing.T) {
			editor := NewEditor("a\nb")
			if tracked && !editor.TrackLFLabels([]byte{'W'}, 'W', nil) {
				t.Fatal("label setup failed")
			}
			if err := editor.Transact(func(draft *Editor) error {
				if draft.TrackLFLabels([]byte{'M'}, 'M', nil) || draft.SetLFLabels([]byte{'M'}, 'M') || draft.UnifyLFLabels('M') {
					t.Fatal("private draft changed label configuration")
				}
				if tracked {
					assertLFLabels(t, draft, []byte{'W'})
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if tracked {
				assertLFLabels(t, editor, []byte{'W'})
			} else if _, ok := editor.LFLabels(nil); ok {
				t.Fatal("private tracking leaked into source")
			}
		})
	}
}

func TestLFLabelTransactionRejectsSourceRelabel(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'W', nil) {
		t.Fatal("label setup failed")
	}
	err := editor.Transact(func(draft *Editor) error {
		if !editor.SetLFLabels([]byte{'M'}, 'M') {
			t.Fatal("fixture source relabel failed")
		}
		draft.MoveTo(draft.RuneCount(), false)
		draft.Insert("\n")
		return nil
	})
	if err == nil || editor.String() != "a\nb" || editor.CanUndo() {
		t.Fatal("transaction committed over a source metadata change")
	}
	assertLFLabels(t, editor, []byte{'M'})
}

func TestLFLabelTransactionEqualTextPhysicalChange(t *testing.T) {
	for _, saved := range []bool{false, true} {
		t.Run(fmt.Sprintf("saved-%t", saved), func(t *testing.T) {
			editor := NewEditor("a\nb\nc")
			if saved {
				editor.SetSavePoint()
			}
			if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', lfLabelPreserveReplacement) {
				t.Fatal("label setup failed")
			}
			if err := editor.Transact(func(draft *Editor) error {
				draft.ReplaceRange(Range{Start: 3, End: 4}, "")
				draft.ReplaceRange(Range{Start: 3, End: 3}, "\n")
				return nil
			}); err != nil || editor.String() != "a\nb\nc" {
				t.Fatalf("zero-net transaction = %q, %v", editor.String(), err)
			}
			assertLFLabels(t, editor, []byte{'W', 'W'})
			if !editor.Undo() {
				t.Fatal("equal-text physical transaction lost Undo")
			}
			assertLFLabels(t, editor, []byte{'W', 'L'})
			if !editor.Redo() {
				t.Fatal("equal-text physical transaction lost Redo")
			}
			assertLFLabels(t, editor, []byte{'W', 'W'})
		})
	}
}

func TestLFLabelLargeBatchAndTransaction(t *testing.T) {
	t.Run("batch", func(t *testing.T) {
		const count = 2050
		editor := NewEditor(strings.Repeat("x\n", count))
		seed := make([]byte, count)
		for index := range seed {
			seed[index] = []byte{'W', 'L', 'M'}[index%3]
		}
		if !editor.TrackLFLabels(seed, 'W', lfLabelPreserveReplacement) {
			t.Fatal("label setup failed")
		}
		edits := make([]Replacement, count-1)
		for index := range edits {
			edits[index] = Replacement{Range: Range{Start: index*2 + 1, End: index*2 + 2}}
		}
		if err := editor.ReplaceRanges(edits); err != nil {
			t.Fatal(err)
		}
		assertLFLabels(t, editor, []byte{'W'})
		if !editor.Undo() {
			t.Fatal("large batch lost Undo")
		}
		assertLFLabels(t, editor, seed)
		if !editor.Redo() {
			t.Fatal("large batch lost Redo")
		}
		assertLFLabels(t, editor, []byte{'W'})
	})
	t.Run("transaction", func(t *testing.T) {
		editor := NewEditor("a\n")
		if !editor.TrackLFLabels([]byte{'L'}, 'W', nil) {
			t.Fatal("label setup failed")
		}
		if err := editor.Transact(func(draft *Editor) error {
			draft.MoveTo(draft.RuneCount(), false)
			for range 2050 {
				draft.Insert("n\n")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		want := append([]byte{'L'}, bytesOfLabel('W', 2050)...)
		assertLFLabels(t, editor, want)
		if !editor.Undo() || editor.String() != "a\n" || editor.CanUndo() {
			t.Fatal("large transaction lost its source Undo boundary")
		}
		assertLFLabels(t, editor, []byte{'L'})
		if !editor.Redo() {
			t.Fatal("large transaction lost Redo")
		}
		assertLFLabels(t, editor, want)
	})
}

func TestLFLabelBatchMapperCannotConsumeSourceUndo(t *testing.T) {
	editor := NewEditor("a\nb")
	reenter := false
	mapper := func(change LFLabelReplacement) []byte {
		if reenter {
			editor.Undo()
		}
		return lfLabelPreserveReplacement(change)
	}
	if !editor.TrackLFLabels([]byte{'W'}, 'W', mapper) {
		t.Fatal("label setup failed")
	}
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("\n")
	editor.SetSavePoint()
	reenter = true
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		_ = editor.ReplaceRanges([]Replacement{{Range: Range{}, Text: "new\n"}})
	}()
	if !panicked || editor.String() != "a\nb\n" || !editor.CanUndo() || editor.CanRedo() || !editor.AtSavePoint() {
		t.Fatal("reentrant mapper consumed source history before being refused")
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
	reenter = false
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{}, Text: "new\n"}}); err != nil {
		t.Fatal(err)
	}
	assertLFLabels(t, editor, []byte{'W', 'W', 'W'})
	if !editor.Undo() || !editor.AtSavePoint() || !editor.CanUndo() {
		t.Fatal("safe retry lost the preexisting history")
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
}

func TestLFLabelBatchNoOpKeepsExistingRedo(t *testing.T) {
	for _, applyUnchanged := range []bool{false, true} {
		t.Run(fmt.Sprintf("apply-unchanged-%t", applyUnchanged), func(t *testing.T) {
			editor := NewEditor("a\nb")
			if !editor.TrackLFLabels([]byte{'W'}, 'W', lfLabelPreserveReplacement) {
				t.Fatal("label setup failed")
			}
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("!")
			if !editor.Undo() {
				t.Fatal("fixture Undo failed")
			}
			if err := editor.ReplaceRangesWith([]Replacement{{Range: Range{End: editor.RuneCount()}, Text: "a\nb"}}, ReplaceRangesOptions{ApplyUnchanged: applyUnchanged}); err != nil {
				t.Fatal(err)
			}
			if editor.CanUndo() || !editor.CanRedo() {
				t.Fatal("equal-text label-preserving batch changed existing history")
			}
			if !editor.SetLFLabels([]byte{'L'}, 'L') || !editor.Redo() || editor.String() != "a\nb!" {
				t.Fatal("no-op batch lost the original LF identity or Redo")
			}
			assertLFLabels(t, editor, []byte{'L'})
		})
	}
}

func TestLFLabelNestedTransactionAndBatch(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	if !editor.TrackLFLabels([]byte{'W', 'L'}, 'W', lfLabelPreserveReplacement) {
		t.Fatal("label setup failed")
	}
	err := editor.Transact(func(draft *Editor) error {
		if err := draft.Transact(func(inner *Editor) error {
			return inner.ReplaceRangesWith([]Replacement{
				{Range: Range{Start: 3, End: 4}},
				{Range: Range{Start: 5, End: 5}, Text: "new\n"},
			}, ReplaceRangesOptions{Forward: true})
		}); err != nil {
			return err
		}
		assertLFLabels(t, draft, []byte{'W', 'W'})
		return nil
	})
	if err != nil || editor.String() != "a\nbcnew\n" {
		t.Fatalf("nested transaction = %q, %v", editor.String(), err)
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
	if !editor.Undo() || editor.String() != "a\nb\nc" || editor.CanUndo() {
		t.Fatal("nested transaction did not commit as one source action")
	}
	assertLFLabels(t, editor, []byte{'W', 'L'})
	if !editor.Redo() {
		t.Fatal("nested transaction lost Redo")
	}
	assertLFLabels(t, editor, []byte{'W', 'W'})
}

func bytesOfLabel(label byte, count int) []byte {
	labels := make([]byte, count)
	for index := range labels {
		labels[index] = label
	}
	return labels
}
