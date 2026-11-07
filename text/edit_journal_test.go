package text

import (
	"slices"
	"strings"
	"testing"
	"unicode"
)

func TestEditJournalReportsActualUTF8Replacements(t *testing.T) {
	editor := NewEditor("猫ab")
	cursor := editor.TrackEdits()
	editor.MoveTo(1, false)
	editor.Insert("é")
	editor.SetSelection(Range{Start: 2, End: 4})
	editor.Insert("中")
	edits, next, complete := editor.EditsSince(cursor, nil)
	want := []Edit{{Start: 3, Inserted: 2}, {Start: 5, Removed: 2, Inserted: 3}}
	if !complete || next != cursor+2 || !slices.Equal(edits, want) || editor.String() != "猫é中" {
		t.Fatalf("byte edits = %+v next=%d complete=%v text=%q", edits, next, complete, editor.String())
	}
	if empty, same, ok := editor.EditsSince(next, edits); !ok || len(empty) != 0 || same != next {
		t.Fatalf("unchanged cursor = %+v, %d, %v", empty, same, ok)
	}
}

func TestEditJournalDoesNotInferOffsetsFromRepeatedOrEqualText(t *testing.T) {
	editor := NewEditor("aaaa")
	cursor := editor.TrackEdits()
	editor.MoveTo(3, false)
	editor.Insert("a")
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.Insert("a")
	edits, _, complete := editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(edits, []Edit{{Start: 3, Inserted: 1}, {Start: 1, Removed: 1, Inserted: 1}}) {
		t.Fatalf("actual replacements = %+v, complete=%v", edits, complete)
	}
}

func TestEditJournalUsesAcceptedAndOverwriteAdjustedText(t *testing.T) {
	editor := NewEditor("12")
	editor.Restrict(unicode.IsDigit, 3)
	cursor := editor.TrackEdits()
	editor.Insert("x34")
	editor.SetOverwrite(true)
	editor.MoveTo(3, false)
	editor.Insert("9")
	edits, _, complete := editor.EditsSince(cursor, nil)
	if !complete || !slices.Equal(edits, []Edit{{Start: 2, Inserted: 1}, {Start: 2, Removed: 1, Inserted: 1}}) || editor.String() != "129" {
		t.Fatalf("accepted edits = %+v complete=%v text=%q", edits, complete, editor.String())
	}
}

func TestEditJournalIncludesIMEAndUndoRedoPrimitives(t *testing.T) {
	editor := NewEditor("ab")
	cursor := editor.TrackEdits()
	editor.MoveTo(1, false)
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	editor.SetMarkedText("你", Range{Start: 1, End: 1})
	editor.Insert("你")
	if !editor.Undo() || editor.String() != "ab" || !editor.Redo() || editor.String() != "a你b" {
		t.Fatalf("journal changed IME history: %q", editor.String())
	}
	edits, _, complete := editor.EditsSince(cursor, nil)
	want := []Edit{
		{Start: 1, Inserted: 2}, {Start: 1, Removed: 2, Inserted: 3},
		{Start: 1, Removed: 3, Inserted: 3}, {Start: 1, Removed: 3}, {Start: 1, Inserted: 3},
	}
	if !complete || !slices.Equal(edits, want) {
		t.Fatalf("IME primitives = %+v complete=%v", edits, complete)
	}
}

func TestEditJournalIsOptInAndIgnoresNonMutatingActions(t *testing.T) {
	editor := NewEditor("ab")
	editor.Insert("c")
	if _, _, complete := editor.EditsSince(0, nil); complete {
		t.Fatal("an untracked editor claimed retained mutations")
	}
	cursor := editor.TrackEdits()
	editor.MoveTo(0, false)
	editor.DeleteBackward()
	editor.Insert("")
	editor.SetSelection(Range{Start: 0, End: 1})
	editor.ClearHistory()
	if next := editor.TrackEdits(); next != cursor {
		t.Fatal("selection or history operation entered the mutation journal")
	}
	edits, next, complete := editor.EditsSince(cursor, nil)
	if !complete || len(edits) != 0 || next != cursor || editor.String() != "abc" {
		t.Fatalf("nonmutation journal = %+v next=%d complete=%v", edits, next, complete)
	}
}

func TestEditJournalReportsGapsAndRetainsLatestPrimitives(t *testing.T) {
	editor := NewEditor("")
	initial := editor.TrackEdits()
	for index := 0; index < EditJournalLimit+2; index++ {
		editor.Insert("a")
	}
	if _, _, complete := editor.EditsSince(initial, nil); complete {
		t.Fatal("expired cursor did not report a gap")
	}
	buffer := make([]Edit, 0, EditJournalLimit)
	edits, next, complete := editor.EditsSince(initial+2, buffer)
	if !complete || len(edits) != EditJournalLimit || next != initial+EditJournalLimit+2 || edits[0].Start != 2 || edits[len(edits)-1].Start != EditJournalLimit+1 {
		t.Fatalf("retained edits = count %d next=%d complete=%v", len(edits), next, complete)
	}
	if _, _, complete := editor.EditsSince(next+1, nil); complete {
		t.Fatal("future cursor accepted")
	}
}

func TestEditJournalDoesNotAllocateFullTextForEachBatchMutation(t *testing.T) {
	value := strings.Repeat("x", 10000)
	measure := func(tracked bool) float64 {
		return testing.AllocsPerRun(20, func() {
			editor := NewEditor(value)
			editor.MoveTo(5000, false)
			if tracked {
				editor.TrackEdits()
			}
			for index := 0; index < 32; index++ {
				editor.Insert("é")
			}
		})
	}
	plain, tracked := measure(false), measure(true)
	if tracked > plain+12 {
		t.Fatalf("tracked batch allocations = %.0f, untracked %.0f: journal rebuilt text per edit", tracked, plain)
	}
}
