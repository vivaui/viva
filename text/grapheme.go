package text

import "github.com/vivaui/viva/internal/grapheme"

// NextGraphemeBoundary returns the byte offset after the next extended
// grapheme cluster in value, starting at from.
func NextGraphemeBoundary(value string, from int) int {
	return grapheme.Next(value, from)
}

// PreviousGraphemeBoundary returns the byte offset before the extended
// grapheme cluster ending at end in value.
func PreviousGraphemeBoundary(value string, end int) int {
	return grapheme.Prev(value, end)
}
