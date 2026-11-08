// Package textmetrics is the text measure that layout uses on every platform.
//
// The platforms' own engines give the same string different widths (Segoe UI is 0.92
// to 1.06 times as wide as San Francisco, Noto Sans up to 1.11, and Windows CJK up to
// 1.32), so a layout sized by them differs per platform. Viva lays out with this
// package instead: the advance widths of one reference font (Inter, SIL OFL 1.1),
// in font units per weight, plus rules for what the table does not hold. It does not
// ask the platform and does not depend on an installed font, so a string has the same
// width everywhere. The platforms still draw the glyphs, natively and unscaled, each
// word anchored at the x this package gives it.
//
// Widths are exact sums of integer advances scaled by the size: no kerning, no
// shaping, no hinting. Beyond Latin, Greek and Cyrillic the answer is a class width,
// not a measurement (see Width).
package textmetrics

import (
	"unicode"
	"unicode/utf8"
)

// missing is the table entry of a code point the reference font has no glyph for.
const missing = 0xFFFF

// span is a run of consecutive code points whose advances are in the tables.
type span struct {
	from, to rune
	first    int
}

// Class widths, in font units, for what the tables do not hold.
const (
	// wideAdvance is a full-width character: CJK ideographs, kana, Hangul, full-width
	// forms. Every platform's CJK font sets these at one em.
	wideAdvance = unitsPerEm
	// halfAdvance is a half-width form (half-width katakana and Hangul).
	halfAdvance = unitsPerEm / 2
	// emojiAdvance is a pictographic character. The color emoji fonts of macOS and
	// Linux set it a little wider than the em.
	emojiAdvance = unitsPerEm * 5 / 4
	// symbolAdvance is a dingbat or miscellaneous symbol (U+2300 to U+27BF).
	symbolAdvance = unitsPerEm
	// otherAdvance stands in for every other script (Arabic, Hebrew, Thai, Indic and
	// so on) and for code points the reference font lacks: about the mean of Latin
	// lowercase. Text in these is not laid out as the platform will draw it; the
	// rasterizer's fit absorbs the difference within its limits.
	otherAdvance = unitsPerEm * 6 / 10
)

// The reference fixed-pitch cell, in units of the font size: the advance of one
// character and the height of one line. A canvas whose text engine cannot measure a
// font (ui.MonoCellEngine) lays fixed-pitch text on this grid.
const (
	MonoAdvance = 0.6
	MonoLine    = 1.18
)

// weightIndex selects the table for a Viva weight (ui.TextStyle.Weight: 0 regular,
// about 0.15 medium, 0.23 semibold, 0.4 bold); the thresholds are the ones every
// backend uses to pick a face.
func weightIndex(weight float32) int {
	switch {
	case weight >= 0.35:
		return 3
	case weight >= 0.20:
		return 2
	case weight >= 0.10:
		return 1
	default:
		return 0
	}
}

// Width is the advance width, in logical pixels, of one line of text at a font size
// and weight. Characters in the reference tables (Basic Latin, Latin-1, Latin
// Extended-A, B and Additional, Greek, Cyrillic, general punctuation, currency,
// arrows and mathematical operators) have the reference font's advance. Others
// follow a rule: CJK, kana, Hangul and full-width forms are one em, half-width forms
// half an em, pictographs 1.25 em (and what follows one in a zero width joiner
// sequence adds nothing), combining marks, variation selectors, skin tone modifiers
// and format characters have no width, and every other character is 0.6 em. Control
// characters have no width.
func Width(value string, size, weight float32) float32 {
	return float32(units(value, weightIndex(weight))) * size / unitsPerEm
}

func units(value string, weight int) int {
	table := &advances[weight]
	total := 0
	joined := false
	jamo := uint8(0) // the Hangul jamo kind of the previous rune
	for i := 0; i < len(value); {
		if c := value[i]; c < utf8.RuneSelf {
			if c >= 0x20 && c < 0x7f {
				total += int(table[c-0x20])
			}
			joined, jamo = false, 0
			i++
			continue
		}
		r, w := utf8.DecodeRuneInString(value[i:])
		if r >= 0x530 {
			if r < 0x1100 || r >= 0x1780 && r <= 0x17FF || r >= 0xFE70 && r <= 0xFEFF {
				if scriptOf(r) != nil {
					advance, end := complexRun(value, i, weight)
					total += advance
					joined, jamo = false, 0
					i = end
					continue
				}
			}
			if r < 0x1200 || r >= 0xA960 && r <= 0xA97F || r >= 0xAC00 && r <= 0xD7FF {
				if advance, kind, ok := hangul(jamo, r); ok {
					total += advance
					joined, jamo = false, kind
					i += w
					continue
				}
			}
		}
		jamo = 0
		advance, join := classify(r, table)
		if joined && advance == emojiAdvance {
			advance = 0 // the next part of a zero width joiner sequence
		}
		joined = join
		total += advance
		i += w
	}
	return total
}

// classify returns the advance of r and whether a pictograph that follows should
// join it (r is a zero width joiner).
func classify(r rune, table *[len(advances[0])]uint16) (advance int, join bool) {
	if r == 0x200D {
		return 0, true
	}
	if r <= spans[len(spans)-1].to {
		for i := range spans {
			s := &spans[i]
			if r < s.from {
				break
			}
			if r <= s.to {
				if a := table[s.first+int(r-s.from)]; a != missing {
					return int(a), false
				}
				break
			}
		}
	}
	switch {
	case isFormat(r):
		return 0, false
	case isWide(r):
		return wideAdvance, false
	case r >= 0xFF61 && r <= 0xFFDC:
		return halfAdvance, false
	case r >= 0x1F000 && r <= 0x1FAFF:
		return emojiAdvance, false
	case r >= 0x2300 && r <= 0x27BF, r >= 0x2B00 && r <= 0x2BFF:
		return symbolAdvance, false
	case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r):
		return 0, false
	}
	return otherAdvance, false
}

// isFormat is a character with no advance of its own: controls, skin tone modifiers,
// variation selectors, tags and format characters.
func isFormat(r rune) bool {
	return r == 0x7f || r >= 0x80 && r < 0xA0 ||
		r >= 0x1F3FB && r <= 0x1F3FF || // skin tone modifiers
		r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0100 && r <= 0xE01EF || // variation selectors
		r >= 0xE0020 && r <= 0xE007F || // tags
		r >= 0x200B && r <= 0x200F || r >= 0x202A && r <= 0x202E || r >= 0x2060 && r <= 0x2064 || r == 0xFEFF
}

func isWide(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF,
		r >= 0x1100 && r <= 0x11FF,
		r >= 0x2E80 && r <= 0x303E,
		r >= 0x3041 && r <= 0x33FF,
		r >= 0x3400 && r <= 0x4DBF,
		r >= 0xA000 && r <= 0xA4CF,
		r >= 0xA960 && r <= 0xA97F,
		r >= 0xD7B0 && r <= 0xD7FF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE10 && r <= 0xFE19,
		r >= 0xFE30 && r <= 0xFE6F,
		r >= 0xFF01 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x3FFFD:
		return true
	}
	return false
}

// FullWidth reports whether run is a full-width character with its marks, as NextRun cuts
// them: drawn in its em square at its layout x, whatever advance the platform's font gives
// it.
func FullWidth(run string) bool {
	for _, r := range run {
		return isFullWidth(r)
	}
	return false
}

func isFullWidth(r rune) bool { return isWide(r) || r >= 0xFF61 && r <= 0xFFDC }

// isMark is a character that belongs to the one before it.
func isMark(r rune) bool {
	return isFormat(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) ||
		r >= 0x1160 && r <= 0x11FF || r >= 0xD7B0 && r <= 0xD7FF // Hangul vowels and final consonants
}

// NextRun is the first piece of value at or after byte offset from that Runs would draw:
// value[start:end], with ok false when there is none.
func NextRun(value string, from int) (start, end int, ok bool) {
	start = -1
	wide := false
	for at, r := range value[from:] {
		at += from
		switch {
		case start < 0:
			if r == ' ' {
				continue
			}
			start, wide = at, isFullWidth(r)
		case r == ' ':
			return start, at, true
		case wide && !isMark(r), !wide && isFullWidth(r):
			return start, at, true
		}
	}
	if start >= 0 {
		return start, len(value), true
	}
	return 0, 0, false
}
