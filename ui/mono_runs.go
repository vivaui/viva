package ui

import (
	"unicode/utf8"

	"github.com/vivaui/viva/internal/textmetrics"
	"github.com/vivaui/viva/internal/uniprops"
)

// monoCells is how many cells r takes on a fixed-pitch grid: none for a combining mark,
// which shares the cell of the character it follows, two for a wide character (East
// Asian Width F or W), and one for any other, a control character included. It is the
// rule of text.CellWidth, which code editors and terminals lay their grids out by.
func monoCells(r rune) int {
	if r < utf8.RuneSelf {
		return 1
	}
	switch {
	case uniprops.Mark(r):
		return 0
	case uniprops.Wide(r):
		return 2
	}
	return 1
}

// monoTextCells is the cells value covers on a fixed-pitch grid, monoCells for each of
// its characters.
func monoTextCells(value string) int {
	cells := 0
	for offset := 0; offset < len(value); {
		if end := asciiEnd(value, offset); end > offset {
			cells += end - offset
			offset = end
			continue
		}
		r, size := utf8.DecodeRuneInString(value[offset:])
		cells += monoCells(r)
		offset += size
	}
	return cells
}

// appendMonoRuns appends to dst the runs that draw value in style on a fixed-pitch grid,
// the runs LabelRuns would be given for a line of one color: one for each stretch of
// printable ASCII, and one for every other character, with the combining marks that
// follow it, on the cells monoCells counts for it. With ligatures, a stretch of two or
// more characters is shaped as one line fitted to its cells (TextRun.Ligatures). It
// reports false, and returns dst as it was, when value is not valid UTF-8, which no run
// may split, or is laid out in more than one direction or from right to left, which a
// grid in logical order cannot show.
func appendMonoRuns(dst []TextRun, value string, style TextStyle, ligatures bool) ([]TextRun, bool) {
	run := TextRun{Color: style.Color, Weight: style.Weight, Italic: style.Italic}
	end := printableASCIIEnd(value, 0)
	if end == len(value) {
		if end > 0 { // the common line, one stretch of printable ASCII
			run.Bytes, run.Cells, run.Ligatures = int32(end), int32(end), ligatures && end > 1
			dst = append(dst, run)
		}
		return dst, true
	}
	// ASCII has no character that needs bidi, so only what follows it is asked.
	if other := asciiEnd(value, end); other < len(value) && textmetrics.Bidirectional(value[other:]) {
		return dst, false
	}
	first := len(dst)
	for offset, column := 0, int32(0); ; {
		// end is where the printable ASCII from offset ends.
		if end > offset && markAt(value, end) {
			end-- // the last character goes with the marks that follow it
		}
		if end > offset {
			cells := int32(end - offset)
			run.Column, run.Bytes, run.Cells, run.Ligatures = column, cells, cells, ligatures && cells > 1
			dst = append(dst, run)
			offset, column = end, column+cells
		} else {
			r, size := utf8.DecodeRuneInString(value[offset:])
			if r == utf8.RuneError && size <= 1 {
				return dst[:first], false
			}
			end = offset + size
			for markAt(value, end) {
				_, size := utf8.DecodeRuneInString(value[end:])
				end += size
			}
			cells := int32(monoCells(r))
			run.Column, run.Bytes, run.Cells, run.Ligatures = column, int32(end-offset), cells, false
			dst = append(dst, run)
			offset, column = end, column+cells
		}
		if offset == len(value) {
			return dst, true
		}
		end = printableASCIIEnd(value, offset)
	}
}

// markAt reports whether a combining mark starts at offset of value.
func markAt(value string, offset int) bool {
	if offset >= len(value) || value[offset] < utf8.RuneSelf {
		return false
	}
	r, _ := utf8.DecodeRuneInString(value[offset:])
	return uniprops.Mark(r)
}

const (
	everyByte = 0x0101010101010101
	highBits  = 0x8080808080808080
)

// word is the eight bytes of value from offset, the first in the lowest byte.
func word(value string, offset int) uint64 {
	value = value[offset : offset+8]
	return uint64(value[0]) | uint64(value[1])<<8 | uint64(value[2])<<16 | uint64(value[3])<<24 |
		uint64(value[4])<<32 | uint64(value[5])<<40 | uint64(value[6])<<48 | uint64(value[7])<<56
}

// asciiEnd is the end of the ASCII bytes of value from offset.
func asciiEnd(value string, offset int) int {
	for offset+8 <= len(value) && word(value, offset)&highBits == 0 {
		offset += 8
	}
	for offset < len(value) && value[offset] < utf8.RuneSelf {
		offset++
	}
	return offset
}

// printableASCIIEnd is the end of the printable ASCII bytes (0x20 to 0x7E) of value from
// offset.
func printableASCIIEnd(value string, offset int) int {
	// A byte of 0x80 or more has its high bit set; with none, a byte of 0x7F sets it once 1
	// is added, and one below 0x20 sets it, borrowing, once 0x20 is taken.
	outside := func(w uint64) uint64 { return (w | (w + everyByte) | (w-0x20*everyByte)&^w) & highBits }
	for offset+16 <= len(value) && outside(word(value, offset))|outside(word(value, offset+8)) == 0 {
		offset += 16
	}
	for offset+8 <= len(value) && outside(word(value, offset)) == 0 {
		offset += 8
	}
	for offset < len(value) && value[offset]-0x20 < 0x7f-0x20 {
		offset++
	}
	return offset
}
