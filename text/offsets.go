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

// spliceText records an edit that replaces runes [start, end) with replacement,
// made before the runes change, against the cached String: the runes before start
// and after end are still the cached text's, so the next String keeps them and
// encodes only what changed. Keeping the bytes is a copy, where building the
// text from every rune encodes the whole document; and an input method's many
// small edits between two reads cost one rebuild, not one copy each.
func (editor *Editor) spliceText(start, end int, replacement []rune) {
	tail := len(editor.runes) - end
	switch {
	case editor.textValid:
		editor.keepPrefix, editor.keepSuffix = start, tail
		editor.keepSuffixBytes = len(editor.text) - editor.byteAt(start) - runesBytes(editor.runes[start:end])
		editor.textValid, editor.textBase = false, true
	case editor.textBase:
		editor.keepPrefix = min(editor.keepPrefix, start)
		if tail < editor.keepSuffix {
			// The edit reaches into the kept tail: what it covers of it is no
			// longer the cached text's.
			editor.keepSuffixBytes -= runesBytes(editor.runes[len(editor.runes)-editor.keepSuffix : end])
			editor.keepSuffix = tail
		}
	}
}

// rebuildText makes text the value again: from the cached text's kept ends and the
// runes between them when it has them, else from every rune.
func (editor *Editor) rebuildText() {
	if editor.textBase {
		middle := editor.runes[editor.keepPrefix : len(editor.runes)-editor.keepSuffix]
		head := editor.prefixBytes(editor.keepPrefix)
		editor.text = editor.text[:head] + string(middle) + editor.text[len(editor.text)-editor.keepSuffixBytes:]
	} else {
		editor.text = string(editor.runes)
	}
	editor.textValid, editor.textBase = true, false
}

// prefixBytes is how many bytes the first index runes take, walked from the start
// or from the last offset asked for, which stays valid for the runes before an
// edit. Unlike byteAt it does not read the cached text, which may be stale.
func (editor *Editor) prefixBytes(index int) int {
	at, offset := 0, 0
	if editor.offsets.index <= index {
		at, offset = editor.offsets.index, editor.offsets.offset
	}
	for ; at < index; at++ {
		offset += runeBytes(editor.runes[at])
	}
	editor.offsets = offsetMemo{index: index, offset: offset}
	return offset
}

func runesBytes(runes []rune) int {
	total := 0
	for _, character := range runes {
		total += runeBytes(character)
	}
	return total
}

// byteAt is the byte offset of rune index in the cached text, walked from the
// nearest of its start, its end and the last offset asked for, which it then
// remembers. The cached text must be valid.
func (editor *Editor) byteAt(index int) int {
	at, offset := editor.offsets.index, editor.offsets.offset
	if index < at {
		if index <= at-index {
			at, offset = 0, 0
		}
	} else if len(editor.runes)-index < index-at {
		at, offset = len(editor.runes), len(editor.text)
	}
	for at < index {
		offset += runeBytes(editor.runes[at])
		at++
	}
	for at > index {
		at--
		offset -= runeBytes(editor.runes[at])
	}
	editor.offsets = offsetMemo{index: at, offset: offset}
	return offset
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
