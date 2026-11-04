package text

import (
	"strings"
	"unicode/utf8"
)

// bracketScanLimit bounds how far MatchBracket looks for a partner, so a stray
// bracket in a megabyte of text costs a bounded scan on every caret move.
const bracketScanLimit = 256 << 10

// MatchBracket finds the partner of the bracket next to offset, a byte offset
// into value: the bracket just before it, or if that is not one the bracket at
// it, as an editor highlights when the caret touches a bracket. It returns the
// byte offsets of the opening and closing bracket of the pair. Only the three
// ASCII pairs () [] {} count, and only brackets of the same kind nest: it knows
// nothing of strings or comments, so a bracket inside one counts like any
// other. ok is false when nothing is next to the caret or the partner is
// missing or farther than 256 KiB away.
func MatchBracket(value string, offset int) (open, close int, ok bool) {
	offset = clamp(offset, 0, len(value))
	for _, at := range [2]int{offset - 1, offset} {
		if at < 0 || at >= len(value) {
			continue
		}
		switch value[at] {
		case '(', '[', '{':
			if partner, found := scanForward(value, at); found {
				return at, partner, true
			}
		case ')', ']', '}':
			if partner, found := scanBackward(value, at); found {
				return partner, at, true
			}
		}
	}
	return 0, 0, false
}

func partnerByte(b byte) byte {
	switch b {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case ')':
		return '('
	case ']':
		return '['
	}
	return '{'
}

func scanForward(value string, at int) (int, bool) {
	opening, closing := value[at], partnerByte(value[at])
	depth := 1
	end := min(len(value), at+bracketScanLimit)
	for index := at + 1; index < end; index++ {
		switch value[index] {
		case opening:
			depth++
		case closing:
			if depth--; depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func scanBackward(value string, at int) (int, bool) {
	closing, opening := value[at], partnerByte(value[at])
	depth := 1
	end := max(0, at-bracketScanLimit)
	for index := at - 1; index >= end; index-- {
		switch value[index] {
		case closing:
			depth++
		case opening:
			if depth--; depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

// Position is a place in a text by line and column, both counted from zero.
// Column counts runes from the start of the line, not grapheme clusters or
// display cells: it is the number a status bar shows after adding one.
type Position struct {
	Line, Column int
}

// LineCount is the number of lines in value: one more than its newlines, so an
// empty text and a text ending in a newline both end with an empty line.
func LineCount(value string) int {
	return strings.Count(value, "\n") + 1
}

// PositionAt is the line and column of byte offset in value. An offset inside a
// multi-byte character counts as the start of the character.
func PositionAt(value string, offset int) Position {
	offset = clamp(offset, 0, len(value))
	for offset > 0 && offset < len(value) && !utf8.RuneStart(value[offset]) {
		offset--
	}
	head := value[:offset]
	start := strings.LastIndexByte(head, '\n') + 1
	return Position{Line: strings.Count(head, "\n"), Column: utf8.RuneCountInString(head[start:])}
}

// OffsetAt is the byte offset of position in value, the inverse of PositionAt.
// A line past the last is the last, and a column past the end of its line is
// the end of the line.
func OffsetAt(value string, position Position) int {
	offset := 0
	for line := max(position.Line, 0); line > 0; line-- {
		next := strings.IndexByte(value[offset:], '\n')
		if next < 0 {
			break
		}
		offset += next + 1
	}
	end := len(value)
	if next := strings.IndexByte(value[offset:], '\n'); next >= 0 {
		end = offset + next
	}
	for column := max(position.Column, 0); column > 0 && offset < end; column-- {
		_, size := utf8.DecodeRuneInString(value[offset:])
		offset += size
	}
	return offset
}

const (
	detectIndentLines = 4000
	detectIndentBytes = 256 << 10
)

// DetectIndent guesses how value is indented from its first lines: hard says
// tabs are used rather than spaces, and width is the number of spaces of one
// level, 2, 3, 4 or 8, or 4 when the text shows nothing to go by (and for a
// tab-indented text, the width to display a tab at). It lets an editor follow
// the style of the file it opened instead of imposing its own.
func DetectIndent(value string) (hard bool, width int) {
	value = value[:min(len(value), detectIndentBytes)]
	tabs, spaced, previous := 0, 0, 0
	var steps [17]int
	for lines := 0; value != "" && lines < detectIndentLines; lines++ {
		line := value
		if end := strings.IndexByte(value, '\n'); end >= 0 {
			line, value = value[:end], value[end+1:]
		} else {
			value = ""
		}
		switch {
		case strings.TrimLeft(line, " \t") == "":
			continue // blank lines say nothing about the style
		case line[0] == '\t':
			tabs++
			previous = 0
		case line[0] == ' ':
			indent := len(line) - len(strings.TrimLeft(line, " "))
			spaced++
			if step := indent - previous; step > 0 && step < len(steps) {
				steps[step]++
			}
			previous = indent
		default:
			previous = 0
		}
	}
	if tabs > spaced {
		return true, 4
	}
	width, best := 4, 0
	for _, candidate := range [...]int{2, 4, 3, 8} {
		if steps[candidate] > best {
			width, best = candidate, steps[candidate]
		}
	}
	return false, width
}
