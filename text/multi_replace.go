package text

import (
	"fmt"
	"slices"
)

// Replacement changes one half-open rune range in the editor's original text.
type Replacement struct {
	Range Range
	Text  string
}

// ReplaceRanges applies sorted, disjoint replacements as one undo unit, subject
// to the editor's existing history size limit.
// A rejected batch leaves both the text and undo/redo history untouched.
func (editor *Editor) ReplaceRanges(edits []Replacement) error {
	if len(edits) == 0 {
		return nil
	}
	if editor.composing {
		return fmt.Errorf("cannot replace ranges during composition")
	}
	if editor.accept != nil || editor.maxRunes > 0 {
		return fmt.Errorf("cannot replace ranges in a restricted editor")
	}
	previousStart, previousEnd := -1, -1
	for index, edit := range edits {
		if edit.Range.Start < 0 || edit.Range.End < edit.Range.Start || edit.Range.End > len(editor.runes) ||
			index > 0 && (edit.Range.Start <= previousStart || edit.Range.Start < previousEnd) {
			return fmt.Errorf("invalid or overlapping replacement at index %d", index)
		}
		previousStart, previousEnd = edit.Range.Start, edit.Range.End
	}
	start, end := edits[0].Range.Start, edits[len(edits)-1].Range.End
	replacement := make([]rune, 0, end-start)
	position := start
	for _, edit := range edits {
		replacement = append(replacement, editor.runes[position:edit.Range.Start]...)
		replacement = append(replacement, []rune(edit.Text)...)
		position = edit.Range.End
	}
	if slices.Equal(replacement, editor.runes[start:end]) {
		return nil
	}
	editor.ReplaceRange(Range{Start: start, End: end}, string(replacement))
	return nil
}
