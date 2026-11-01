package platform

import (
	"unicode/utf8"

	"github.com/vivaui/viva/internal/textmetrics"
	"github.com/vivaui/viva/internal/uniprops"
	"github.com/vivaui/viva/ui"
)

// A terminal preedit is drawn on the terminal's own grid: every character on
// a cell of the cursor's row, a wide one on two, a combining mark on the cell
// of the character it belongs to. These helpers give a backend that draws the
// preedit itself the same columns the terminal uses, so the composition text,
// its underline and the candidate window all follow the cells.

// TerminalPreeditCellWidth is the width of one terminal cell at a font size: the pitch
// every platform's fixed-pitch text is placed on (see ui.Canvas.MonoCellSize).
func TerminalPreeditCellWidth(size float32) float32 {
	return size * textmetrics.MonoAdvance
}

// TerminalPreeditCells is how many cells r occupies: none for a combining mark, which
// shares the cell of the character it follows, two for a wide character, else one. A
// backend that draws the preedit in native code asks it for every character, so the
// answer is the terminal's own and not a second table in another language.
func TerminalPreeditCells(r rune) int {
	switch {
	case uniprops.Mark(r):
		return 0
	case uniprops.Wide(r):
		return 2
	}
	return 1
}

// TerminalPreeditColumn is the cell column where the runes-th rune of value
// starts, counted from the first cell of value.
func TerminalPreeditColumn(value string, runes int) int {
	column := 0
	for _, r := range value {
		if runes <= 0 {
			break
		}
		runes--
		column += TerminalPreeditCells(r)
	}
	return column
}

// TerminalPreeditRuns appends to dst the grid runs that draw value in color:
// consecutive single-cell ASCII characters share a run, every other character
// (with its combining marks) is a run of its own that spans the cells it
// occupies, as a terminal line is drawn.
func TerminalPreeditRuns(dst []ui.TextRun, value string, color ui.Color) []ui.TextRun {
	column := int32(0)
	ascii := false // whether the last run is a group of ASCII characters
	for offset := 0; offset < len(value); {
		r, size := utf8.DecodeRuneInString(value[offset:])
		if r >= 0x20 && r < 0x7f && !startsWithMark(value[offset+1:]) {
			if ascii {
				last := len(dst) - 1
				dst[last].Bytes++
				dst[last].Cells++
			} else {
				dst = append(dst, ui.TextRun{Column: column, Bytes: 1, Cells: 1, Color: color})
				ascii = true
			}
			offset++
			column++
			continue
		}
		ascii = false
		end := offset + size
		for end < len(value) {
			next, nextSize := utf8.DecodeRuneInString(value[end:])
			if !uniprops.Mark(next) {
				break
			}
			end += nextSize
		}
		cells := int32(TerminalPreeditCells(r))
		dst = append(dst, ui.TextRun{Column: column, Bytes: int32(end - offset), Cells: cells, Color: color})
		offset = end
		column += cells
	}
	return dst
}

func startsWithMark(value string) bool {
	r, _ := utf8.DecodeRuneInString(value)
	return value != "" && uniprops.Mark(r)
}
