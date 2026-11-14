package text

import "testing"

// CodeMirror's add-next-occurrence and select-all-occurrences are built from
// SetSelections and the word range: this is the shape DBX's two commands take.

// occurrenceWord is the word around the primary selection, or the selection
// itself when it already reaches beyond a word.
func occurrenceWord(editor *Editor) (Range, bool) {
	selection := editor.Selection()
	if selection.Start != selection.End {
		return selection, true
	}
	word := WordRange(editor.String(), selection.Start)
	if editor.ByteOffset(word.Start) == editor.ByteOffset(word.End) {
		return Range{}, false
	}
	return word, true
}

// selectNextOccurrence adds the next occurrence after the last selection,
// wrapping to the first, without overlapping an existing one.
func selectNextOccurrence(editor *Editor) bool {
	word, ok := occurrenceWord(editor)
	if !ok {
		return false
	}
	value := editor.String()
	start := editor.ByteOffset(word.End)
	seen := make(map[int]bool)
	for _, selection := range editor.Selections() {
		seen[editor.ByteOffset(selection.Start)] = true
	}
	for attempts := 0; attempts <= len(value)+1; attempts++ {
		from := start
		if from >= len(value) {
			from = 0
		}
		index := indexOf(value, editor.String()[editor.ByteOffset(word.Start):editor.ByteOffset(word.End)], from)
		if index < 0 {
			if from == 0 {
				return false
			}
			start = 0
			continue
		}
		candidate := Range{
			Start: editor.RuneIndex(index),
			End:   editor.RuneIndex(index + editor.ByteOffset(word.End) - editor.ByteOffset(word.Start)),
		}
		start = index + 1
		if seen[index] {
			continue
		}
		selections := editor.Selections()
		return editor.SetSelections(append(selections, candidate), len(selections))
	}
	return false
}

func indexOf(value, needle string, from int) int {
	if from > len(value) {
		return -1
	}
	for at := from; at+len(needle) <= len(value); at++ {
		if value[at:at+len(needle)] == needle {
			return at
		}
	}
	return -1
}

func TestSelectNextOccurrenceAddsEveryCopyOfTheWord(t *testing.T) {
	editor := NewEditor("name name other name")
	editor.SetSelection(Range{Start: 0, End: 4})
	if !selectNextOccurrence(editor) {
		t.Fatal("the second occurrence was not found")
	}
	if got := editor.Selections(); len(got) != 2 || got[1] != (Range{Start: 5, End: 9}) {
		t.Fatalf("selections = %+v", got)
	}
	if editor.Selection() != (Range{Start: 5, End: 9}) {
		t.Fatalf("the new occurrence is not primary: %+v", editor.Selection())
	}
	if !selectNextOccurrence(editor) {
		t.Fatal("the third occurrence was not found")
	}
	if got := editor.Selections(); len(got) != 3 || got[2] != (Range{Start: 16, End: 20}) {
		t.Fatalf("selections = %+v", got)
	}
	// Typing replaces all three and leaves three carets, as CodeMirror does.
	editor.Insert("X")
	if got := editor.String(); got != "X X other X" {
		t.Fatalf("text = %q", got)
	}
	if got := editor.SelectionCount(); got != 3 {
		t.Fatalf("carets after typing = %d", got)
	}
	if !editor.Undo() || editor.String() != "name name other name" {
		t.Fatalf("undo = %q", editor.String())
	}
}

func TestOccurrenceWordFallsBackToTheCaretWord(t *testing.T) {
	editor := NewEditor("host.example.com")
	editor.SetSelection(Range{Start: 5, End: 5})
	word, ok := occurrenceWord(editor)
	if !ok || word != (Range{Start: 5, End: 12}) {
		t.Fatalf("word = %+v, %v", word, ok)
	}
	editor.SetSelection(Range{Start: 0, End: 4})
	if word, _ := occurrenceWord(editor); word != (Range{Start: 0, End: 4}) {
		t.Fatalf("an existing selection was not used: %+v", word)
	}
}
