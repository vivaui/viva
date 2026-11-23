// Package bidi implements the Unicode Bidirectional Algorithm (UAX #9, Unicode 18.0.0)
// for one paragraph of UTF-8 text.
//
// The implementation covers P2-P3 (paragraph level from the first strong character,
// skipping isolates), X1-X10 (explicit embeddings, overrides and isolates with the
// directional status stack of depth 125, overflow counters and isolating run
// sequences), W1-W7, N0 (paired brackets, with U+2329/U+232A canonically equivalent to
// U+3008/U+3009), N1-N2, I1-I2, L1 (separators and trailing white space reset to the
// paragraph level) and L2 (reordering). Character properties come from
// github.com/vivaui/viva/internal/uniprops.
//
// The API works on a UTF-8 string and reports byte offsets. Resolved levels are kept
// per rune; an invalid byte counts as one U+FFFD rune of one byte. A Resolver owns
// reusable scratch memory, so resolving and reordering allocate nothing once it is warm
// (scratch is released when a short text follows a very long one). A Resolver is not
// safe for concurrent use and a Paragraph or run list is only valid until the next call
// on the same Resolver.
//
// NeedsBidi is a cheap pre-check: when it reports false the text has no right-to-left
// letter, Arabic number or explicit formatting character and every level is the
// left-to-right paragraph level 0, so no resolving or reordering is needed.
//
// Limits: text is treated as one paragraph (callers split at paragraph separators; a
// separator inside the text gets the paragraph level and does not reset the explicit
// state), L3 (combining marks) and L4 (mirroring, see Mirrored) are left to the caller,
// and at most 1<<30 bytes of text are examined. Characters removed by X9 (embedding,
// override and PDF controls, BN) get the level of the character before them, which
// keeps them out of the way of reordering.
package bidi

import (
	"unicode/utf8"

	"github.com/vivaui/viva/internal/uniprops"
)

// Direction is the base direction of a paragraph.
type Direction uint8

const (
	// Auto takes the direction of the first strong character (P2, P3), LTR if none.
	Auto Direction = iota
	// LTR forces paragraph level 0.
	LTR
	// RTL forces paragraph level 1.
	RTL
)

// MaxDepth is the deepest explicit embedding level (BD2).
const MaxDepth = 125

// maxText bounds the examined text so that offsets fit in int32.
const maxText = 1 << 30

type class = uniprops.BidiClass

const (
	cL   = uniprops.BidiL
	cR   = uniprops.BidiR
	cAL  = uniprops.BidiAL
	cEN  = uniprops.BidiEN
	cES  = uniprops.BidiES
	cET  = uniprops.BidiET
	cAN  = uniprops.BidiAN
	cCS  = uniprops.BidiCS
	cNSM = uniprops.BidiNSM
	cBN  = uniprops.BidiBN
	cB   = uniprops.BidiB
	cS   = uniprops.BidiS
	cWS  = uniprops.BidiWS
	cON  = uniprops.BidiON
	cLRE = uniprops.BidiLRE
	cLRO = uniprops.BidiLRO
	cRLE = uniprops.BidiRLE
	cRLO = uniprops.BidiRLO
	cPDF = uniprops.BidiPDF
	cLRI = uniprops.BidiLRI
	cRLI = uniprops.BidiRLI
	cFSI = uniprops.BidiFSI
	cPDI = uniprops.BidiPDI
)

// NeedsBidi reports whether text can need bidirectional processing with an LTR or Auto
// base direction: it holds an R, AL or AN character or an explicit embedding,
// override or isolate control. When it returns false every level is 0 and nothing has
// to be reordered. It allocates nothing and skips Latin text a byte at a time.
func NeedsBidi(text string) bool {
	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c < utf8.RuneSelf:
			i++
		case c >= 0xC2 && c <= 0xD5 && i+1 < len(text) && text[i+1]&0xC0 == 0x80:
			i += 2 // U+0080..U+057F holds no relevant class
		case c >= 0xE3 && c <= 0xEA && i+2 < len(text) && text[i+1]&0xC0 == 0x80 && text[i+2]&0xC0 == 0x80:
			i += 3 // U+3000..U+AFFF likewise
		default:
			r, w := utf8.DecodeRuneInString(text[i:])
			switch uniprops.Bidi(r) {
			case cR, cAL, cAN, cLRE, cLRO, cRLE, cRLO, cPDF, cLRI, cRLI, cFSI, cPDI:
				return true
			}
			i += w
		}
	}
	return false
}

// Mirrored returns the character whose glyph is the mirror image of r (rule L4), or r
// itself. Apply it to characters at odd levels.
func Mirrored(r rune) rune { return uniprops.Mirror(r) }

// Run is a maximal range [Start, End) of text in byte offsets with one level. An odd
// level is right-to-left.
type Run struct {
	Start, End int
	Level      uint8
}

// Paragraph is the result of Resolve. Its slices belong to the Resolver.
type Paragraph struct {
	// Level is the paragraph embedding level, 0 or 1.
	Level  uint8
	levels []uint8
	offs   []int32 // byte offset of every rune plus the text length
	orig   []class // original bidi classes for L1
}

// Len is the number of runes.
func (p Paragraph) Len() int { return len(p.levels) }

// Levels returns the resolved embedding level of every rune before L1 (rule L1 depends
// on the line, see LineLevels). The slice is valid until the next Resolve.
func (p Paragraph) Levels() []uint8 { return p.levels }

// RuneIndex returns the index of the rune that contains byteOffset, clamped to
// [0, Len()-1] (-1 for empty text).
func (p Paragraph) RuneIndex(byteOffset int) int {
	n := len(p.levels)
	lo, hi := 0, n
	for lo < hi { // first rune starting after byteOffset
		mid := int(uint(lo+hi) >> 1)
		if int(p.offs[mid]) <= byteOffset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo > 0 {
		lo--
	}
	if n == 0 {
		return -1
	}
	return lo
}

// LevelAt returns the resolved level (before L1) of the rune containing byteOffset, or
// the paragraph level when the offset is outside the text.
func (p Paragraph) LevelAt(byteOffset int) uint8 {
	if byteOffset < 0 || len(p.levels) == 0 || byteOffset >= int(p.offs[len(p.levels)]) {
		return p.Level
	}
	return p.levels[p.RuneIndex(byteOffset)]
}
