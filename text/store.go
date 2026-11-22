package text

import "github.com/vivaui/viva/internal/grapheme"

// runeStore is the editor's text as runes: what Range, Caret and Selection count.
// The editor reads and changes it only through these methods, so how the text is
// held stays its own business (ADR 0092).
type runeStore struct{ runes []rune }

// Len is the number of runes.
func (store *runeStore) Len() int { return len(store.runes) }

// At is the rune at index.
func (store *runeStore) At(index int) rune { return store.runes[index] }

// Slice is runes [start, end). It is valid until the next change of the text and
// must not be changed.
func (store *runeStore) Slice(start, end int) []rune { return store.runes[start:end] }

// View is every rune, under Slice's rules: for code that walks the whole text.
func (store *runeStore) View() []rune { return store.runes }

// Next is the grapheme cluster boundary after start; Prev the one before end.
func (store *runeStore) Next(start int) int { return grapheme.NextRunes(store.runes, start) }
func (store *runeStore) Prev(end int) int   { return grapheme.PrevRunes(store.runes, end) }

// Replace makes runes [start, end) replacement.
func (store *runeStore) Replace(start, end int, replacement []rune) {
	delta := len(replacement) - (end - start)
	if delta > 0 {
		store.runes = append(store.runes, make([]rune, delta)...)
		copy(store.runes[end+delta:], store.runes[end:len(store.runes)-delta])
	} else if delta < 0 {
		copy(store.runes[start+len(replacement):], store.runes[end:])
		store.runes = store.runes[:len(store.runes)+delta]
	}
	copy(store.runes[start:], replacement)
}

// String is the text as UTF-8.
func (store *runeStore) String() string { return string(store.runes) }

// Reset makes the text value.
func (store *runeStore) Reset(value []rune) { store.runes = value }
