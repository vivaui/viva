package text

import (
	"strings"
	"unicode"
)

// SingleLine folds pasted lines and tabs into spaces and drops other control
// characters. Printable Unicode and spaces within each line are preserved. Text that
// already fits one line is returned as it is, so typing costs no allocation.
func SingleLine(value string) string {
	for _, r := range value {
		if r == '\t' || r == '\u2028' || r == '\u2029' || unicode.IsControl(r) {
			return singleLineFold(value)
		}
	}
	return value
}

func singleLineFold(value string) string {
	lines := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\r' || r == '\n' || r == '\u0085' || r == '\u2028' || r == '\u2029'
	})
	value = strings.Join(lines, " ")
	return strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\u0085', '\u2028', '\u2029':
			return -1
		case '\t':
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

// wordClass groups runs for word navigation. Letters, numbers and underscores are
// one class, so a word is a whole run of them. That includes a run of Han
// characters: Control+Backspace in 主机地址 removes all four, where a native word
// breaker removes one. Host, path and URL fields are the target here, and their runs
// are separated by punctuation, not by script.
func wordClass(r rune) int {
	if unicode.IsSpace(r) {
		return 0
	}
	if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
		return 1
	}
	return 2
}

// wordBoundary walks whole grapheme clusters, keeping combining characters and
// emoji together. Punctuation separates components of addresses and paths.
func (editor *Editor) wordBoundary(forward bool) int {
	at := editor.caret
	if forward {
		if at < len(editor.runes) {
			kind := wordClass(editor.runes[at])
			for at < len(editor.runes) && wordClass(editor.runes[at]) == kind {
				at = nextBoundary(editor.runes, at)
			}
		}
		for at < len(editor.runes) && wordClass(editor.runes[at]) == 0 {
			at = nextBoundary(editor.runes, at)
		}
	} else {
		for at > 0 && wordClass(editor.runes[previousBoundary(editor.runes, at)]) == 0 {
			at = previousBoundary(editor.runes, at)
		}
		if at > 0 {
			kind := wordClass(editor.runes[previousBoundary(editor.runes, at)])
			for at > 0 {
				previous := previousBoundary(editor.runes, at)
				if wordClass(editor.runes[previous]) != kind {
					break
				}
				at = previous
			}
		}
	}
	return at
}

// MoveWord moves to the next/previous word boundary, optionally extending the selection.
func (editor *Editor) MoveWord(forward, extend bool) {
	editor.MoveTo(editor.wordBoundary(forward), extend)
}

// WordRange returns the run around a pointer position, on grapheme boundaries.
func WordRange(value string, index int) Range {
	runes := []rune(value)
	if len(runes) == 0 {
		return Range{}
	}
	index = max(0, min(index, len(runes)-1))
	start := 0
	for next := nextBoundary(runes, start); next <= index; next = nextBoundary(runes, start) {
		start = next
	}
	kind := wordClass(runes[start])
	end := nextBoundary(runes, start)
	for start > 0 {
		previous := previousBoundary(runes, start)
		if wordClass(runes[previous]) != kind {
			break
		}
		start = previous
	}
	for end < len(runes) && wordClass(runes[end]) == kind {
		end = nextBoundary(runes, end)
	}
	return Range{Start: start, End: end}
}

// DeleteWord removes a selection or a word as one undo unit. Undo restores the
// original caret, rather than an artificial selection used to delete the word.
func (editor *Editor) DeleteWord(forward bool) {
	editor.BreakUndoGroup()
	if editor.Selection().Start != editor.Selection().End || editor.composing {
		editor.DeleteBackward()
		editor.BreakUndoGroup()
		return
	}
	before := editor.selectionState()
	end := editor.wordBoundary(forward)
	selection := ordered(Range{Start: editor.caret, End: end})
	change := editor.replace(selection, "", false)
	editor.collapse(selection.Start)
	editor.record(change, before, editor.selectionState(), editSeparate)
}

// DeleteLineBoundary removes text from the caret to the start or end of its logical line.
// It keeps the operation as one undo unit and preserves the caret at the surviving boundary.
func (editor *Editor) DeleteLineBoundary(end bool) bool {
	editor.BreakUndoGroup()
	if editor.Selection().Start != editor.Selection().End || editor.composing {
		changed := editor.DeleteBackward()
		// Keep the semantic boundary explicit even if the fallback deletion
		// implementation changes its history kind in the future.
		editor.BreakUndoGroup()
		return changed
	}
	boundary := editor.caret
	if end {
		for boundary < len(editor.runes) && editor.runes[boundary] != '\n' {
			boundary++
		}
	} else {
		for boundary > 0 && editor.runes[boundary-1] != '\n' {
			boundary--
		}
	}
	if boundary == editor.caret {
		return false
	}
	before := editor.selectionState()
	selection := ordered(Range{Start: editor.caret, End: boundary})
	change := editor.replace(selection, "", false)
	editor.collapse(selection.Start)
	editor.record(change, before, editor.selectionState(), editSeparate)
	return true
}

// MoveLineBoundary implements Home/End for an unwrapped logical line.
func (editor *Editor) MoveLineBoundary(end, extend bool) {
	at := editor.caret
	if end {
		for at < len(editor.runes) && editor.runes[at] != '\n' {
			at++
		}
	} else {
		for at > 0 && editor.runes[at-1] != '\n' {
			at--
		}
	}
	editor.MoveTo(at, extend)
}
