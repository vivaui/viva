package platform

// A platform text engine measures one line: the shaped line that is drawn, with
// its caret stops (gfx.TextEngine). The native editor behind a field, though, is
// handed the whole value of the field, and a multi-line field's value is a
// document. TextLine finds the one line of it the platform has to lay out, so
// that the cost of a caret, a selection or an input-method window follows the
// length of a line and not the size of the file, and so that a value of many
// lines never reaches an engine that limits the lines it will shape.

// isLineBreak reports whether r ends a line the way a text layout does: the
// mandatory breaks of Unicode line breaking (LF, VT, FF, CR, NEL, LS and PS).
// Text up to the next such rune is one line to DirectWrite and to Pango.
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\r', '\u0085', '\u2028', '\u2029':
		return true
	}
	return false
}

// TextLine locates the line of value that the caret before the rune at
// runeIndex is on. start and end are the byte offsets of the line, without its
// break, and first is the rune index of its first rune. A caret just after a
// break is on the line that break begins; an index past the last rune is on the
// last line, and one below zero is on the first. A value without a break is one
// line, whole.
func TextLine(value string, runeIndex int) (start, end, first int) {
	index := 0
	for offset, r := range value {
		if isLineBreak(r) {
			if index >= runeIndex {
				return start, offset, first
			}
			// The break is before the caret: the line it begins is the caret's, unless
			// another break follows before the caret.
			start, first = offset+runeLen(r), index+1
		}
		index++
	}
	return start, len(value), first
}

// runeLen is the encoded size of a line break rune, all of which are valid.
func runeLen(r rune) int {
	switch {
	case r < 0x80:
		return 1
	case r < 0x800:
		return 2
	}
	return 3
}
