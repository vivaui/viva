package app

import (
	"testing"

	"github.com/vivaui/viva/text"
)

func TestPlaceUnitSelectionKeepsWholeWordsAndLines(t *testing.T) {
	const value = "alpha beta gamma"
	word := text.WordRange(value, 7) // beta
	if word != (text.Range{Start: 6, End: 10}) {
		t.Fatalf("word = %+v", word)
	}
	if got := placeUnitSelection(value, 12, 2, word); got != (TextRange{Start: 6, End: 16}) {
		t.Fatalf("drag right = %+v", got)
	}
	if got := placeUnitSelection(value, 1, 2, word); got != (TextRange{Start: 0, End: 10}) {
		t.Fatalf("drag left = %+v", got)
	}
	if got := placeUnitSelection(value, 8, 2, word); got != (TextRange{Start: 6, End: 10}) {
		t.Fatalf("drag inside the word = %+v", got)
	}

	const lines = "one\ntwo\nthree"
	line := text.LineRange(lines, 5) // two, with its newline
	if line != (text.Range{Start: 4, End: 8}) {
		t.Fatalf("line = %+v", line)
	}
	if got := placeUnitSelection(lines, 10, 3, line); got != (TextRange{Start: 4, End: 13}) {
		t.Fatalf("line drag down = %+v", got)
	}
	if got := placeUnitSelection(lines, 1, 3, line); got != (TextRange{Start: 0, End: 8}) {
		t.Fatalf("line drag up = %+v", got)
	}
}

func TestRangeIsSpaceFindsBlankRuns(t *testing.T) {
	const value = "alpha\n\ngamma"
	blank := text.WordRange(value, 6) // the two newlines of the empty line
	if !rangeIsSpace(value, blank) {
		t.Fatalf("blank run %+v is not whitespace", blank)
	}
	if rangeIsSpace(value, text.Range{Start: 0, End: 5}) {
		t.Fatal("a word was taken for whitespace")
	}
}
