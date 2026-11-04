package text

import "github.com/vivaui/viva/internal/uniprops"

// CellWidth is how many character cells r takes on a fixed-pitch grid, as a
// terminal or a code editor sets text: none for a combining mark, which shares
// the cell of the character it follows, two for a wide character (East Asian
// Width F or W: CJK, fullwidth forms, most emoji), and one for any other. A tab
// or another control character is the caller's to place; it counts as one.
func CellWidth(r rune) int {
	if r >= 0x20 && r < 0x7f {
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
