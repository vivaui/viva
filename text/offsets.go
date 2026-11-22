package text

import "strings"

// spliceText records an edit that replaces runes [start, end) with replacement,
// made before the runes change, against the cached String: the bytes before start
// and after end are still the cached text's, so the next String keeps them and
// copies only what changed from the store. An input method's many small edits
// between two reads cost one rebuild, not one copy each.
func (editor *Editor) spliceText(start, end int) {
	head := editor.runes.ByteOffset(start)
	tail := editor.runes.Bytes() - editor.runes.ByteOffset(end)
	switch {
	case editor.textValid:
		editor.keepPrefixBytes, editor.keepSuffixBytes = head, tail
		editor.textValid, editor.textBase = false, true
	case editor.textBase:
		editor.keepPrefixBytes = min(editor.keepPrefixBytes, head)
		editor.keepSuffixBytes = min(editor.keepSuffixBytes, tail)
	}
}

// rebuildText makes text the value again: from the cached text's kept ends and the
// store's bytes between them when it has them, else from the whole store.
func (editor *Editor) rebuildText() {
	if editor.textBase {
		head, tail := editor.keepPrefixBytes, editor.keepSuffixBytes
		var builder strings.Builder
		builder.Grow(editor.runes.Bytes())
		builder.WriteString(editor.text[:head])
		editor.runes.WriteBytes(&builder, head, editor.runes.Bytes()-tail)
		builder.WriteString(editor.text[len(editor.text)-tail:])
		editor.text = builder.String()
	} else {
		editor.text = editor.runes.String()
	}
	editor.textValid, editor.textBase = true, false
}

// ByteOffset converts a rune index, as Caret and Selection count, to the byte
// offset of that rune in String: what a component that works on the UTF-8 text
// wants. An index outside the text is clamped.
func (editor *Editor) ByteOffset(index int) int {
	return editor.runes.ByteOffset(index)
}

// RuneIndex converts a byte offset in String to the index of the rune it falls
// in: ByteOffset the other way round. An offset inside a multi-byte character
// gives the character's index, and one outside the text is clamped.
func (editor *Editor) RuneIndex(offset int) int {
	return editor.runes.RuneIndex(offset)
}
