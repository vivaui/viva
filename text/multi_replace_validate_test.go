package text

import (
	"testing"
	"unicode"
)

func TestValidateReplaceRangesDoesNotMutateEditorOrJournal(t *testing.T) {
	editor := NewEditor("abc")
	editor.Insert("!")
	editor.Undo()
	before, selection, caret := editor.String(), editor.Selection(), editor.Caret()
	cursor := editor.TrackEdits()
	after := Range{Start: 2, End: 1}
	edits := []Replacement{{Range: Range{Start: 0, End: 1}, Text: "猫"}}
	if err := editor.ValidateReplaceRanges(edits, ReplaceRangesOptions{Forward: true, AfterSelection: &after}); err != nil {
		t.Fatal(err)
	}
	if editor.String() != before || editor.Selection() != selection || editor.Caret() != caret || editor.CanUndo() || !editor.CanRedo() {
		t.Fatal("successful preflight changed editor state")
	}
	if got, next, complete := editor.EditsSince(cursor, nil); !complete || len(got) != 0 || next != cursor {
		t.Fatalf("preflight entered edit journal: %+v cursor %d, %v", got, next, complete)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		if err := editor.ValidateReplaceRanges(edits, ReplaceRangesOptions{AfterSelection: &after}); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatalf("valid preflight allocated a replacement buffer: %.0f allocations", allocations)
	}
}

func TestValidateReplaceRangesSharesCommitRefusals(t *testing.T) {
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
			before, selection := editor.String(), editor.Selection()
			composition, active := editor.Composition()
			cursor := editor.TrackEdits()
			edits := []Replacement{{Range: Range{Start: 0, End: 1}, Text: "X"}}
			preflight := editor.ValidateReplaceRanges(edits, ReplaceRangesOptions{})
			commit := editor.ReplaceRangesWith(edits, ReplaceRangesOptions{})
			if preflight == nil || commit == nil || preflight.Error() != commit.Error() || editor.String() != before || editor.Selection() != selection {
				t.Fatalf("preflight and commit diverged: %v / %v", preflight, commit)
			}
			if got, stillActive := editor.Composition(); got != composition || stillActive != active {
				t.Fatal("refused preflight changed composition")
			}
			if got, _, complete := editor.EditsSince(cursor, nil); !complete || len(got) != 0 {
				t.Fatal("refused preflight changed journal")
			}
		})
	}
}

func TestValidateReplaceRangesChecksRangesAndFinalRuneCount(t *testing.T) {
	editor := NewEditor("abc")
	before := editor.Selection()
	invalid := Range{Start: 0, End: 4}
	for _, edits := range [][]Replacement{
		{{Range: Range{Start: -1, End: 0}, Text: "X"}},
		{{Range: Range{Start: 0, End: 2}, Text: "X"}, {Range: Range{Start: 1, End: 3}, Text: "Y"}},
		{{Range: Range{Start: 0, End: 1}, Text: "猫"}},
	} {
		if err := editor.ValidateReplaceRanges(edits, ReplaceRangesOptions{AfterSelection: &invalid}); err == nil || editor.Selection() != before || editor.CanUndo() || editor.journal != nil {
			t.Fatalf("invalid preflight mutated or enabled tracking: %+v, %v", edits, err)
		}
	}
	if err := editor.ValidateReplaceRanges(nil, ReplaceRangesOptions{AfterSelection: &invalid}); err != nil {
		t.Fatalf("empty batch no longer a no-op: %v", err)
	}
}
