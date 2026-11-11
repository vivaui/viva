package gfx

import "strings"

// TextRow is one row of a wrapped text: the bytes value[Start:End], where Start
// is a place a row can begin and End is where it stops, before the line break
// that ends it or, where the text wrapped, at the start of the next row. The
// spaces a row ends with are part of it, left hanging past the edge as a browser
// does, so every byte of the text but its line breaks belongs to exactly one row
// and a text field can put its caret on any of them.
type TextRow struct{ Start, End int }

// WrapRow returns the row of value that begins at from, and where the next one
// begins (-1 when this is the last). from is 0 or the next of a previous call.
// Rows break where WrapLines breaks lines (see there), at font size size and
// weight, and at every newline in the text, which belongs to no row: a text
// ending in a newline ends with an empty row. A width of zero or less never
// wraps, so each paragraph is a row. Nothing is allocated, so a caller that
// needs only the rows in view can ask for them one at a time.
func (canvas *Canvas) WrapRow(value string, from int, size, weight, width float32) (row TextRow, next int) {
	from = max(0, min(from, len(value)))
	end := len(value)
	if newline := strings.IndexByte(value[from:], '\n'); newline >= 0 {
		end = from + newline
	}
	next = -1
	if end < len(value) {
		next = end + 1
	}
	if end > from && value[end-1] == '\r' {
		end--
	}
	if width <= 0 {
		return TextRow{Start: from, End: end}, next
	}
	line := value[from:end]
	if canvas.TextWidthWeight(trimSpaceRight(line), size, weight) <= width {
		return TextRow{Start: from, End: end}, next
	}
	cut := canvas.breakPoint(line, lineFont{size: size, weight: weight}, width)
	cut += len(line[cut:]) - len(trimSpaceLeft(line[cut:])) // the spaces after the break hang on this row
	if from+cut >= end {
		return TextRow{Start: from, End: end}, next
	}
	return TextRow{Start: from, End: from + cut}, from + cut
}

// WrapRows appends the rows of value to dst (see WrapRow). Painting a text one
// row to a Label, with the rows in hand, is how an editor maps between a byte
// offset and a place on the screen.
func (canvas *Canvas) WrapRows(dst []TextRow, value string, size, weight, width float32) []TextRow {
	for from := 0; from >= 0; {
		var row TextRow
		row, from = canvas.WrapRow(value, from, size, weight, width)
		dst = append(dst, row)
	}
	return dst
}
