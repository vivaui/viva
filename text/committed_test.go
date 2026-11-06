package text

import "testing"

func TestCommittedStringExcludesActiveCompositionAndPreservesSelection(t *testing.T) {
	editor := NewEditor("A中B")
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.SetMarkedText("pin", Range{Start: 3, End: 3})
	if editor.String() != "ApinB" || editor.CommittedString() != "A中B" {
		t.Fatalf("preedit visible=%q committed=%q", editor.String(), editor.CommittedString())
	}
	editor.SetMarkedText("输入", Range{Start: 2, End: 2})
	if editor.String() != "A输入B" || editor.CommittedString() != "A中B" {
		t.Fatalf("updated preedit visible=%q committed=%q", editor.String(), editor.CommittedString())
	}
	editor.Insert("新")
	if editor.String() != "A新B" || editor.CommittedString() != "A新B" {
		t.Fatalf("committed visible=%q committed=%q", editor.String(), editor.CommittedString())
	}
	if !editor.Undo() || editor.CommittedString() != "A中B" {
		t.Fatalf("undo after commit=%q", editor.CommittedString())
	}
}

func TestCommittedStringFollowsUnmarkAndNavigation(t *testing.T) {
	editor := NewEditor("old")
	editor.SetMarkedText("中", Range{Start: 1, End: 1})
	if editor.CommittedString() != "old" {
		t.Fatalf("marked text was committed: %q", editor.CommittedString())
	}
	editor.Unmark()
	if editor.CommittedString() != "old中" {
		t.Fatalf("unmark did not commit: %q", editor.CommittedString())
	}
	editor.SetMarkedText("文", Range{Start: 1, End: 1})
	if editor.CommittedString() != "old中" {
		t.Fatalf("new preedit base=%q", editor.CommittedString())
	}
	editor.MoveTo(0, false)
	if editor.CommittedString() != "old中文" {
		t.Fatalf("movement did not end composition: %q", editor.CommittedString())
	}
}

func TestCompositionDoesNotEagerlyAllocateCommittedString(t *testing.T) {
	editor := NewEditor("搜索")
	allocations := testing.AllocsPerRun(100, func() {
		editor.SetSelection(Range{Start: 2, End: editor.RuneCount()})
		editor.SetMarkedText("输入框", Range{Start: 3, End: 3})
		editor.Unmark()
	})
	if allocations > 2 {
		t.Fatalf("composition allocated %v times, want at most 2", allocations)
	}
}
