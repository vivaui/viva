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
	// Without save-point mode equal text adds no Undo unit and preserves Redo.
	// SetSavePoint opts into retaining those actual replacements in history.
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
// unless ApplyUnchanged or save-point mode is enabled; otherwise final selection is unchanged.
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
	var contexts []lineContext
	if editor.journal != nil && editor.journal.lines != nil {
		contexts = make([]lineContext, 0, len(edits))
	}
	position := start
	runePosition, bytePosition := 0, 0
	linePosition, lineShift := 0, 0
	for _, edit := range edits {
		for runePosition < edit.Range.Start {
			if contexts != nil && editor.runes[runePosition] == '\n' {
				linePosition++
			}
			bytePosition += runeBytes(editor.runes[runePosition])
			runePosition++
		}
		primitive := Edit{Start: bytePosition}
		sourceLine := linePosition
		for runePosition < edit.Range.End {
			if contexts != nil && editor.runes[runePosition] == '\n' {
				linePosition++
			}
			width := runeBytes(editor.runes[runePosition])
			primitive.Removed += width
			bytePosition += width
			runePosition++
		}
		inserted := []rune(edit.Text)
		for _, character := range inserted {
			primitive.Inserted += runeBytes(character)
		}
		replacement = append(replacement, editor.runes[position:edit.Range.Start]...)
		if primitive.Removed != 0 || primitive.Inserted != 0 {
			primitives = append(primitives, primitive)
			if contexts != nil {
				atLineStart := edit.Range.Start == 0 || editor.runes[edit.Range.Start-1] == '\n'
				if options.Forward {
					atLineStart = start == 0 || editor.runes[start-1] == '\n'
					if len(replacement) > 0 {
						atLineStart = replacement[len(replacement)-1] == '\n'
					}
				}
				line := sourceLine
				if options.Forward {
					line += lineShift
				}
				context := makeLineContext(line, atLineStart, editor.runes[edit.Range.Start:edit.Range.End], inserted)
				contexts = append(contexts, context)
				lineShift += context.inserted - context.removed
			}
		}
		replacement = append(replacement, inserted...)
		position = edit.Range.End
	}
	changed := !slices.Equal(replacement, editor.runes[start:end])
	if !changed && !options.ApplyUnchanged && editor.savePointIndex() == nil {
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
		slices.Reverse(contexts)
	}
	editor.commitBatch(Range{Start: start, End: end}, string(replacement), primitives, contexts, options.AfterSelection)
	return nil
}

func (editor *Editor) commitBatch(target Range, replacement string, primitives []Edit, contexts []lineContext, after *Range) {
	target = normalizeRange(target, len(editor.runes))
	before := editor.selectionState()
	journal := editor.journal
	editor.journal = nil
	change := editor.replace(target, replacement, false)
	editor.journal = journal
	editor.collapse(target.Start + len(change.inserted))
	editor.composing = false
	if after != nil {
		editor.SetSelection(*after)
	}
	if len(primitives) != 0 {
		editor.recordWithPrimitives(change, before, editor.selectionState(), editSeparate, primitives)
	}
	editor.rememberLineBatch(primitives, contexts)
	editor.recordBatchEdits(primitives, contexts, false)
}
