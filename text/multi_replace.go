package text

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// Replacement changes one half-open rune range in the editor's original text.
type Replacement struct {
	Range Range
	Text  string
}

// ReplaceRangesOptions opts into target-replacement behavior without changing
// ReplaceRanges defaults. The batch still performs one buffer replacement and undo unit.
type ReplaceRangesOptions struct {
	// Forward reports primitives from first target to last, adjusting positions
	// after each replacement. The default reports last to first.
	Forward bool
	// AfterSelection is copied, validated against the resulting rune count, and
	// retained by Undo/Redo. Nil keeps the default caret after the last target.
	AfterSelection *Range
	// ApplyUnchanged reports target primitives even when final text is equal.
	// Equal text does not add an Undo unit or discard an existing Redo unit.
	ApplyUnchanged bool
}

// ReplaceRanges applies sorted, disjoint replacements as one undo unit, subject
// to the editor's existing history size limit.
// A rejected batch leaves both the text and undo/redo history untouched.
func (editor *Editor) ReplaceRanges(edits []Replacement) error {
	return editor.ReplaceRangesWith(edits, ReplaceRangesOptions{})
}

// ValidateReplaceRanges checks the same constraints as ReplaceRangesWith without
// changing text, selection, composition, history or the edit journal. It does not
// allocate a replacement buffer. Editors remain single-threaded; callers must
// not mutate a source between validating it and committing a coordinated batch.
func (editor *Editor) ValidateReplaceRanges(edits []Replacement, options ReplaceRangesOptions) error {
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
	resultingRunes := len(editor.runes)
	for index, edit := range edits {
		if edit.Range.Start < 0 || edit.Range.End < edit.Range.Start || edit.Range.End > len(editor.runes) ||
			index > 0 && (edit.Range.Start <= previousStart || edit.Range.Start < previousEnd) {
			return fmt.Errorf("invalid or overlapping replacement at index %d", index)
		}
		remaining := resultingRunes - (edit.Range.End - edit.Range.Start)
		inserted := utf8.RuneCountInString(edit.Text)
		if inserted > int(^uint(0)>>1)-remaining {
			return fmt.Errorf("replacement result is too large")
		}
		resultingRunes = remaining + inserted
		previousStart, previousEnd = edit.Range.Start, edit.Range.End
	}
	if after := options.AfterSelection; after != nil && (after.Start < 0 || after.End < 0 || after.Start > resultingRunes || after.End > resultingRunes) {
		return fmt.Errorf("final selection is outside the replacement result")
	}
	return nil
}

// ReplaceRangesWith validates and applies a batch with optional primitive order
// and final selection. An empty batch is a no-op. Unchanged text is also a no-op
// unless ApplyUnchanged is set; otherwise final selection is not changed.
func (editor *Editor) ReplaceRangesWith(edits []Replacement, options ReplaceRangesOptions) error {
	if err := editor.ValidateReplaceRanges(edits, options); err != nil {
		return err
	}
	if len(edits) == 0 {
		return nil
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
	changed := !slices.Equal(replacement, editor.runes[start:end])
	if !changed && !options.ApplyUnchanged {
		return nil
	}
	if options.Forward {
		shift := 0
		for index := range primitives {
			primitives[index].Start += shift
			shift += primitives[index].Inserted - primitives[index].Removed
		}
	} else {
		slices.Reverse(primitives)
	}
	journal := editor.journal
	editor.journal = nil
	editor.ReplaceRange(Range{Start: start, End: end}, string(replacement))
	editor.journal = journal
	if options.AfterSelection != nil {
		editor.SetSelection(*options.AfterSelection)
	}
	if changed && editor.historyIndex > 0 {
		editor.history[editor.historyIndex-1].primitives = primitives
		editor.history[editor.historyIndex-1].after = editor.selectionState()
	}
	editor.recordBatchEdits(primitives, false)
	return nil
}
