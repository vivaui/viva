package text

import "testing"

func TestLFLabelRevisionTracksReplayAndMetadata(t *testing.T) {
	editor := NewEditor("a\nb")
	source, ok := any(editor).(interface{ LFLabelRevision() (uint64, bool) })
	if !ok {
		t.Fatal("LF labels have no source revision query")
	}
	if _, enabled := source.LFLabelRevision(); enabled {
		t.Fatal("disabled tracker has a revision")
	}
	if !editor.TrackLFLabels([]byte{'M'}, 'M', nil) {
		t.Fatal("tracker setup failed")
	}
	previous, enabled := source.LFLabelRevision()
	if !enabled {
		t.Fatal("enabled tracker has no revision")
	}
	checkChange := func() {
		t.Helper()
		next, enabled := source.LFLabelRevision()
		if !enabled || next == previous {
			t.Fatal("accepted LF state change did not advance its revision")
		}
		previous = next
	}
	editor.SetSavePoint()
	editor.ReplaceRange(Range{End: editor.RuneCount()}, editor.String())
	checkChange()
	if !editor.SetLFLabels([]byte{'W'}, 'M') {
		t.Fatal("metadata update refused")
	}
	checkChange()
	if !editor.Undo() {
		t.Fatal("equal replacement lost Undo")
	}
	checkChange()
	assertLFLabels(t, editor, []byte{'M'})
	if !editor.Redo() {
		t.Fatal("equal replacement lost Redo")
	}
	checkChange()
	assertLFLabels(t, editor, []byte{'W'})
	if !editor.UnifyLFLabels('L') {
		t.Fatal("conversion refused")
	}
	checkChange()
	editor.SetSavePoint()
	editor.LFLabels(nil)
	if after, _ := source.LFLabelRevision(); after != previous {
		t.Fatal("Save or read changed the LF revision")
	}
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{Start: 1, End: 2}, Text: "\n"}}); err != nil {
		t.Fatal(err)
	}
	checkChange()
	if err := editor.Transact(func(draft *Editor) error {
		draft.ReplaceRange(Range{Start: 1, End: 2}, "")
		draft.ReplaceRange(Range{Start: 1, End: 1}, "\n")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	checkChange()
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.SetMarkedText("x\n", Range{})
	checkChange()
	if !editor.CancelComposition() {
		t.Fatal("legacy Cancel refused")
	}
	checkChange()
	assertLFLabels(t, editor, []byte{'L'})
}
