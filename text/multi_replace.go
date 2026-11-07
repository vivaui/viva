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
	primitives := make([]Edit, 0, len(edits))
	position := start
	runePosition, bytePosition := 0, 0
	for _, edit := range edits {
		for runePosition < edit.Range.Start {
			bytePosition += runeBytes(editor.runes[runePosition])
			runePosition++
		}
		primitive := Edit{Start: bytePosition}
		for runePosition < edit.Range.End {
			width := runeBytes(editor.runes[runePosition])
			primitive.Removed += width
			bytePosition += width
			runePosition++
		}
		inserted := []rune(edit.Text)
		for _, character := range inserted {
			primitive.Inserted += runeBytes(character)
		}
		if primitive.Removed != 0 || primitive.Inserted != 0 {
			primitives = append(primitives, primitive)
		}
		replacement = append(replacement, editor.runes[position:edit.Range.Start]...)
		replacement = append(replacement, inserted...)
		position = edit.Range.End
	}
	if slices.Equal(replacement, editor.runes[start:end]) {
		return nil
	}
	journal := editor.journal
	editor.journal = nil
	editor.ReplaceRange(Range{Start: start, End: end}, string(replacement))
	editor.journal = journal
	if editor.historyIndex > 0 {
		editor.history[editor.historyIndex-1].primitives = primitives
	}
	editor.recordBatchEdits(primitives, false)
	return nil
}
