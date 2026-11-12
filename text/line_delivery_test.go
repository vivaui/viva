package text

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func largeLineTargets(count int) (string, []Replacement) {
	before := strings.Repeat("a\n", count)
	edits := make([]Replacement, count)
	for index := range edits {
		edits[index] = Replacement{Range: Range{Start: index * 2, End: index*2 + 1}, Text: "A\nx"}
	}
	return before, edits
}

func TestLargeLineDeliveryIncludesForwardUndoRedoAndOwnedQueries(t *testing.T) {
	before, targets := largeLineTargets(EditJournalLimit + 37)
	editor := NewEditor(before)
	cursor := editor.TrackLineEdits()
	if err := editor.ReplaceRangesWith(targets, ReplaceRangesOptions{Forward: true}); err != nil {
		t.Fatal(err)
	}
	forward, next, complete := editor.LineEditsSince(cursor, nil)
	if !complete || len(forward) != len(targets) {
		t.Fatalf("large atomic action expired: edits=%d complete=%v", len(forward), complete)
	}
	for index, edit := range forward {
		if edit.Line != index*2 || edit.RemovedLines != 0 || edit.InsertedLines != 1 {
			t.Fatalf("large forward context %d: %+v", index, edit)
		}
	}
	if partial, _, complete := editor.EditsSince(cursor, nil); complete || len(partial) != 0 {
		t.Fatal("byte journal retention was silently expanded")
	}
	expected := slices.Clone(forward)
	forward[0].Line = -10
	if repeated, same, complete := editor.LineEditsSince(cursor, nil); !complete || same != next || !slices.Equal(repeated, expected) {
		t.Fatal("large query mutated retained context")
	}
	buffer := make([]LineEdit, 0, len(targets))
	if allocations := testing.AllocsPerRun(100, func() {
		got, _, complete := editor.LineEditsSince(cursor, buffer)
		if !complete || len(got) != len(targets) {
			t.Fatal("reused large query became incomplete")
		}
	}); allocations != 0 {
		t.Fatalf("reused large query allocated %.0f times", allocations)
	}
	if !editor.Undo() || editor.String() != before {
		t.Fatal("missing large Undo")
	}
	inverse, redoCursor, complete := editor.LineEditsSince(next, nil)
	if !complete || len(inverse) != len(targets) {
		t.Fatal("large inverse context was truncated")
	}
	for index, edit := range inverse {
		if edit.Line != (len(targets)-index-1)*2 || edit.RemovedLines != 1 || edit.InsertedLines != 0 {
			t.Fatalf("large inverse context %d: %+v", index, edit)
		}
	}
	if !editor.Redo() {
		t.Fatal("missing large Redo")
	}
	if replay, _, complete := editor.LineEditsSince(redoCursor, nil); !complete || !slices.Equal(replay, expected) {
		t.Fatal("large Redo lost its exact ordered contexts")
	}
}

func TestLargeLineDeliveryDoesNotRepairOlderGapsAndExpiresOnMutation(t *testing.T) {
	before, targets := largeLineTargets(EditJournalLimit + 3)
	editor := NewEditor(before)
	old := editor.TrackLineEdits()
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("!")
	start := editor.TrackLineEdits()
	if err := editor.ReplaceRanges(targets); err != nil {
		t.Fatal(err)
	}
	if edits, _, complete := editor.LineEditsSince(old, nil); complete || len(edits) != 0 {
		t.Fatal("latest action fabricated an older missing prefix")
	}
	if edits, _, complete := editor.LineEditsSince(start, nil); !complete || len(edits) != len(targets) {
		t.Fatal("latest action itself was unavailable")
	}
	editor.MoveTo(0, false)
	if edits, _, complete := editor.LineEditsSince(start, nil); !complete || len(edits) != len(targets) {
		t.Fatal("selection-only work invalidated delivery")
	}
	editor.Insert("z")
	if edits, _, complete := editor.LineEditsSince(start, nil); complete || len(edits) != 0 || editor.journal.lines.delivery != nil {
		t.Fatal("later mutation retained stale large delivery")
	}
}

func TestLargeLineDeliveryClearAndLateEnableLifetime(t *testing.T) {
	before, targets := largeLineTargets(EditJournalLimit + 2)
	editor := NewEditor(before)
	start := editor.TrackLineEdits()
	if err := editor.ReplaceRanges(targets); err != nil {
		t.Fatal(err)
	}
	if editor.journal.lines.delivery == nil {
		t.Fatal("large delivery missing")
	}
	editor.ClearHistory()
	if edits, _, complete := editor.LineEditsSince(start, nil); complete || len(edits) != 0 || editor.journal.lines.delivery != nil {
		t.Fatal("history clear retained large references")
	}
	editor = NewEditor(before)
	editor.TrackEdits()
	if err := editor.ReplaceRanges(targets); err != nil {
		t.Fatal(err)
	}
	start = editor.TrackLineEdits()
	editor.Undo()
	if edits, _, complete := editor.LineEditsSince(start, nil); complete || len(edits) != 0 {
		t.Fatal("late-enabled large inverse invented context")
	}
}

func TestLargeLineDeliverySurvivesRefusalAndNoOp(t *testing.T) {
	before, targets := largeLineTargets(EditJournalLimit + 1)
	editor := NewEditor(before)
	start := editor.TrackLineEdits()
	if err := editor.ReplaceRanges(targets); err != nil {
		t.Fatal(err)
	}
	want, revision, complete := editor.LineEditsSince(start, nil)
	if !complete {
		t.Fatal("missing delivery fixture")
	}
	if err := editor.Transact(func(*Editor) error { return errors.New("abort") }); err == nil {
		t.Fatal("refused transaction unexpectedly committed")
	}
	editor.MoveTo(0, false)
	editor.Insert("")
	editor.BreakUndoGroup()
	got, same, complete := editor.LineEditsSince(start, nil)
	if !complete || same != revision || !slices.Equal(got, want) {
		t.Fatal("refusal/no-op invalidated unread large action")
	}
}

func TestLineHistoryBudgetIncludesOptionalContexts(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		editor := NewEditor("a")
		editor.SetSavePoint()
		if tracked {
			editor.TrackLineEdits()
		} else {
			editor.TrackEdits()
		}
		for range 2 {
			if err := editor.Transact(func(draft *Editor) error {
				for index := 0; index < 12000; index++ {
					value := "A"
					if index%2 != 0 {
						value = "B"
					}
					draft.ReplaceRange(Range{End: 1}, value)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		want := 2
		if tracked {
			want = 1
		}
		if len(editor.history) != want {
			t.Fatalf("line-tracked=%v retained %d units, want %d", tracked, len(editor.history), want)
		}
		expected := 0
		for index := range editor.history {
			expected += editor.history[index].retainedRunes()
			if tracked {
				expected += lineContextRunes * len(editor.lineBatchContexts(editor.history[index].primitives))
			}
		}
		if editor.historyRunes != expected {
			t.Fatalf("history context accounting=%d want %d", editor.historyRunes, expected)
		}
		if tracked {
			editor.Undo()
			editor.Insert("z")
			if len(editor.journal.lines.batches) != 0 || editor.journal.lines.delivery != nil {
				t.Fatal("branch retained pruned metadata")
			}
		}
	}
}
