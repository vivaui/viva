package text

import "testing"

func TestLineJournalCapturesSourceLineAndLFCounts(t *testing.T) {
	editor := NewEditor("aa\nbb")
	cursor := editor.TrackLineEdits()
	if err := editor.Transact(func(draft *Editor) error {
		draft.SetSelection(Range{Start: 2, End: 3})
		draft.Insert("")
		draft.MoveTo(1, false)
		draft.Insert("\n")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	edits, next, complete := editor.LineEditsSince(cursor, nil)
	if !complete || len(edits) != 2 || edits[0].Line != 0 || edits[0].RemovedLines != 1 || edits[0].InsertedLines != 0 || edits[1].Line != 0 || edits[1].RemovedLines != 0 || edits[1].InsertedLines != 1 {
		t.Fatalf("source line facts missing: %+v complete=%v", edits, complete)
	}
	if !editor.Undo() {
		t.Fatal("missing transaction Undo")
	}
	inverse, _, complete := editor.LineEditsSince(next, nil)
	if !complete || len(inverse) != 2 || inverse[0].Line != 0 || inverse[0].RemovedLines != 1 || inverse[1].InsertedLines != 1 {
		t.Fatalf("inverse line facts missing: %+v complete=%v", inverse, complete)
	}
}

func TestLineJournalBatchLineNumbersIncludeEarlierLFChanges(t *testing.T) {
	for _, forward := range []bool{false, true} {
		editor := NewEditor("a\nb\nc")
		cursor := editor.TrackLineEdits()
		if err := editor.ReplaceRangesWith([]Replacement{
			{Range: Range{End: 1}, Text: "a\nx"},
			{Range: Range{Start: 2, End: 3}, Text: "b\nz"},
			{Range: Range{Start: 4, End: 5}, Text: "c"},
		}, ReplaceRangesOptions{Forward: forward}); err != nil {
			t.Fatal(err)
		}
		edits, _, complete := editor.LineEditsSince(cursor, nil)
		want := []int{2, 1, 0}
		if forward {
			want = []int{0, 2, 4}
		}
		if !complete || len(edits) != len(want) {
			t.Fatalf("batch line context unavailable: %+v", edits)
		}
		for index, edit := range edits {
			if edit.Line != want[index] {
				t.Fatalf("forward=%v step=%d line=%d want %d", forward, index, edit.Line, want[index])
			}
		}
	}
}

func TestLineJournalMemoResetsAfterBatchAndHistory(t *testing.T) {
	editor := NewEditor("a\nb\nc")
	editor.TrackLineEdits()
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("!")
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{End: editor.RuneCount()}, Text: "abcd\nx"}}); err != nil {
		t.Fatal(err)
	}
	cursor := editor.TrackLineEdits()
	editor.MoveTo(editor.RuneCount(), false)
	editor.Insert("?")
	edits, _, complete := editor.LineEditsSince(cursor, nil)
	if !complete || len(edits) != 1 || edits[0].Line != 1 {
		t.Fatalf("stale source line memo after batch: %+v", edits)
	}
}
