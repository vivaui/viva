package text

import "unicode/utf8"

// offsetMemo is one resolved pair of a rune index and the byte offset of that
// rune in String. The zero value is the pair (0, 0), which is always true.
type offsetMemo struct {
	index, offset int
}

// trim drops the pair when an edit at rune index start changed what it counts.
// A pair at or before the edit still holds: nothing before an edit moves.
func (memo *offsetMemo) trim(start int) {
	if memo.index > start {
		*memo = offsetMemo{}
	}
}

// ByteOffset converts a rune index, as Caret and Selection count, to the byte
// offset of that rune in String: what a component that works on the UTF-8 text
// wants. An index outside the text is clamped. The walk starts from whichever
// of the start of the text, the end and the last answer is nearest, so asking
// for a caret that moves or types a little costs a few runes however long the
// text is.
func (editor *Editor) ByteOffset(index int) int {
	index = clamp(index, 0, len(editor.runes))
	value := editor.String()
	at, offset := editor.offsets.index, editor.offsets.offset
	if index < at {
		if index <= at-index {
			at, offset = 0, 0
		}
	} else if len(editor.runes)-index < index-at {
		at, offset = len(editor.runes), len(value)
	}
	for at < index {
		_, size := utf8.DecodeRuneInString(value[offset:])
		offset += size
		at++
	}
	for at > index {
		_, size := utf8.DecodeLastRuneInString(value[:offset])
		offset -= size
		at--
	}
	editor.offsets = offsetMemo{index: at, offset: offset}
	return offset
}

// RuneIndex converts a byte offset in String to the index of the rune it falls
// in: ByteOffset the other way round. An offset inside a multi-byte character
// gives the character's index, and one outside the text is clamped.
func (editor *Editor) RuneIndex(offset int) int {
	value := editor.String()
	offset = clamp(offset, 0, len(value))
	at, from := editor.offsets.index, editor.offsets.offset
	if offset < from {
		if offset <= from-offset {
			at, from = 0, 0
		}
	} else if len(value)-offset < offset-from {
		at, from = len(editor.runes), len(value)
	}
	for from < offset {
		_, size := utf8.DecodeRuneInString(value[from:])
		if from+size > offset {
			break // offset is inside this rune
		}
		from += size
		at++
	}
	for from > offset {
		_, size := utf8.DecodeLastRuneInString(value[:from])
		from -= size
		at--
	}
	editor.offsets = offsetMemo{index: at, offset: from}
	return at
}
