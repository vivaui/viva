package text

import "unicode/utf8"

// The conversions between a Range, which counts runes, and the offsets a platform
// text service counts in: bytes of UTF-8, or UTF-16 code units. They read the
// string in place and allocate nothing, so a platform that answers every text
// event with the selection in its own units does not copy the field's text.

// ByteOffsetOf is the byte offset in value of the rune at index: the start of that
// rune, or len(value) for an index at or past the end.
func ByteOffsetOf(value string, index int) int {
	if index <= 0 {
		return 0
	}
	n := 0
	for offset := range value {
		if n == index {
			return offset
		}
		n++
	}
	return len(value)
}

// ByteRange is r in byte offsets of value, each end clamped to it.
func ByteRange(value string, r Range) (start, end int) {
	return ByteOffsetOf(value, r.Start), ByteOffsetOf(value, r.End)
}

// UTF16Offset is the number of UTF-16 code units before the rune at index in
// value: two for every rune outside the Basic Multilingual Plane, one for the
// others. An index past the end counts the whole of value.
func UTF16Offset(value string, index int) int {
	units, n := 0, 0
	for _, r := range value {
		if n >= index {
			break
		}
		units += utf16Len(r)
		n++
	}
	return units
}

// RuneIndexAtUTF16 is the index of the rune that starts units UTF-16 code units
// into value, or of the rune whose pair those units fall inside: the nearest rune
// boundary at or before them.
func RuneIndexAtUTF16(value string, units int) int {
	if units <= 0 {
		return 0
	}
	consumed, n := 0, 0
	for _, r := range value {
		width := utf16Len(r)
		if consumed+width > units {
			return n
		}
		consumed += width
		n++
		if consumed == units {
			return n
		}
	}
	return n
}

func utf16Len(r rune) int {
	if r > 0xffff && r <= utf8.MaxRune {
		return 2
	}
	return 1
}
