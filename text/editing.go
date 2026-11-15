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

// LineRange returns the logical line around a pointer position, including its
// trailing newline when it has one. It is the selection a triple click takes.
func LineRange(value string, index int) Range {
	runes := []rune(value)
	if len(runes) == 0 {
		return Range{}
	}
	index = max(0, min(index, len(runes)-1))
	start := index
	for start > 0 && runes[start-1] != '\n' {
		start--
	}
	end := index
	for end < len(runes) && runes[end] != '\n' {
		end++
	}
	if end < len(runes) {
		end++ // the newline belongs to the line it ends
	}
	return Range{Start: start, End: end}
}

// DeleteWord removes a selection or a word as one undo unit. Undo restores the
// original caret, rather than an artificial selection used to delete the word.
func (editor *Editor) DeleteWord(forward bool) {
	editor.BreakUndoGroup()
	editor.clearSecondary() // a word deletion is a single-selection edit
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

// lineBoundary is the start or end of the logical line the caret is on: the
// nearest newline, or the end of the text.
func (editor *Editor) lineBoundary(end bool) int {
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
	return at
}

// MoveLineBoundary implements Home/End for an unwrapped logical line.
func (editor *Editor) MoveLineBoundary(end, extend bool) {
	editor.MoveTo(editor.lineBoundary(end), extend)
}

// DeleteToLineBoundary removes a selection, or the text from the caret to the
// start (end false) or end (end true) of its logical line, as one undo unit. A
// caret already on the boundary removes the one grapheme beyond it, which joins
// the line to its neighbour, as DeleteBackward and DeleteForward do. Rows a
// field wraps to are not lines here; a field that lays its own rows out moves
// by them with MoveTo.
func (editor *Editor) DeleteToLineBoundary(end bool) {
	editor.BreakUndoGroup()
	editor.clearSecondary() // a line deletion is a single-selection edit
	selection := editor.Selection()
	if selection.Start != selection.End || editor.composing {
		editor.DeleteBackward()
		editor.BreakUndoGroup()
		return
	}
	boundary := editor.lineBoundary(end)
	if boundary == editor.caret {
		if end {
			editor.DeleteForward()
		} else {
			editor.DeleteBackward()
		}
		editor.BreakUndoGroup()
		return
	}
	before := editor.selectionState()
	target := ordered(Range{Start: editor.caret, End: boundary})
	change := editor.replace(target, "", false)
	editor.collapse(target.Start)
	editor.record(change, before, editor.selectionState(), editSeparate)
}
