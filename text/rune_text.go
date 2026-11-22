package text

import "sort"

// runeText is text read by rune index without a slice of every rune: the editor's
// store, or the text before an input method's tentative suffix.
type runeText interface {
	Len() int
	At(index int) rune
	// UTF16Offset is the number of UTF-16 units before rune index.
	UTF16Offset(index int) int
	// RuneAtUTF16 is the rune units UTF-16 units in, and whether units falls on
	// a rune boundary.
	RuneAtUTF16(units int) (int, bool)
}

// rolledBackText is the store with the runes of some edits read as the runes they
// replaced: pieces of the store and of history, in order.
type rolledBackText struct {
	store  *runeStore
	pieces []rolledBackPiece
	length int
}

type rolledBackPiece struct {
	// at is the piece's first rune in the rolled-back text; start its first rune
	// in the store, unless the piece is history's removed runes.
	at, start, length int
	history           bool
	removed           []rune
}

func (text *rolledBackText) add(piece rolledBackPiece) {
	if piece.length == 0 {
		return
	}
	piece.at = text.length
	text.pieces = append(text.pieces, piece)
	text.length += piece.length
}

func (text *rolledBackText) Len() int { return text.length }

// piece is the index of the piece holding rune index, which is inside the text.
func (text *rolledBackText) piece(index int) int {
	return sort.Search(len(text.pieces), func(i int) bool {
		return text.pieces[i].at+text.pieces[i].length > index
	})
}

func (text *rolledBackText) At(index int) rune {
	piece := &text.pieces[text.piece(index)]
	if piece.history {
		return piece.removed[index-piece.at]
	}
	return text.store.At(piece.start + index - piece.at)
}

// units is how many UTF-16 units the first count runes of piece take.
func (text *rolledBackText) units(piece *rolledBackPiece, count int) int {
	if !piece.history {
		return text.store.UTF16Offset(piece.start+count) - text.store.UTF16Offset(piece.start)
	}
	units := 0
	for _, r := range piece.removed[:count] {
		units += utf16Len(r)
	}
	return units
}

func (text *rolledBackText) UTF16Offset(index int) int {
	units := 0
	for i := range text.pieces {
		piece := &text.pieces[i]
		if index <= piece.at {
			break
		}
		units += text.units(piece, min(index-piece.at, piece.length))
	}
	return units
}

func (text *rolledBackText) RuneAtUTF16(units int) (int, bool) {
	consumed := 0
	for i := range text.pieces {
		piece := &text.pieces[i]
		width := text.units(piece, piece.length)
		if consumed+width <= units {
			consumed += width
			continue
		}
		remaining := units - consumed
		if !piece.history {
			from := text.store.UTF16Offset(piece.start)
			index, exact := text.store.RuneAtUTF16(from + remaining)
			return piece.at + index - piece.start, exact
		}
		for offset, r := range piece.removed {
			if remaining == 0 {
				return piece.at + offset, true
			}
			if remaining < utf16Len(r) {
				return piece.at + offset, false
			}
			remaining -= utf16Len(r)
		}
	}
	return text.length, consumed == units
}

// runeTextString is text as a string.
func runeTextString(text runeText) string {
	switch text := text.(type) {
	case *runeStore:
		return text.String()
	case *rolledBackText:
		var runes []rune
		for i := range text.pieces {
			piece := &text.pieces[i]
			if piece.history {
				runes = append(runes, piece.removed...)
			} else {
				runes = text.store.AppendSlice(runes, piece.start, piece.start+piece.length)
			}
		}
		return string(runes)
	}
	panic("text: unknown rune text")
}
