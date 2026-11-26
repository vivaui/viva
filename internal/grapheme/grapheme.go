// Package grapheme finds extended grapheme cluster boundaries as UAX #29
// defines them (rules GB1 to GB999, including the Indic conjunct rule GB9c as
// Unicode 18 words it (a Linker, no Consonant before it needed),
// emoji ZWJ sequences and regional indicator pairs). Properties come from
// internal/uniprops. Every function is allocation free.
//
// Bytes that are not valid UTF-8 are one-byte clusters, and so are runes that
// are not Unicode scalar values; both behave like a Control character.
package grapheme

import (
	"unicode/utf8"

	"github.com/vivaui/viva/internal/uniprops"
)

// prop is the part of a rune's Unicode properties the boundary rules read.
type prop struct {
	gb   uniprops.GraphemeClass
	incb uniprops.InCB
	pict bool
}

// invalid is the stand-in rune for a byte or value that is not a scalar value.
const invalid rune = -1

func propOf(r rune) prop {
	// ASCII is the common case in every text path, and a letter there takes
	// three Unicode table lookups for three known answers. Answer it here.
	if r < utf8.RuneSelf {
		switch {
		case r < 0:
			return prop{gb: uniprops.GBControl}
		case r == '\r':
			return prop{gb: uniprops.GBCR}
		case r == '\n':
			return prop{gb: uniprops.GBLF}
		case r < 0x20 || r == 0x7f:
			return prop{gb: uniprops.GBControl}
		}
		return prop{}
	}
	if r > utf8.MaxRune || (r >= 0xd800 && r < 0xe000) {
		return prop{gb: uniprops.GBControl}
	}
	gb, incb, pict := uniprops.GraphemeProps(r)
	return prop{gb: gb, incb: incb, pict: pict}
}

// Source is text the rules read a rune at a time: a string, a rune slice, or
// text held in pieces that NextIn and PrevIn read in place. Offsets are whatever
// the source counts (bytes, runes) from 0 to Len. At is the rune starting at
// offset i and its width, Before the rune ending at i; a rune that is not a
// scalar value, such as a byte that is not UTF-8 reported as -1, is a cluster of
// its own and behaves like a Control character.
type Source interface {
	Len() int
	At(i int) (rune, int)
	Before(i int) (rune, int)
}

type strText string

func (s strText) Len() int { return len(s) }

func (s strText) At(i int) (rune, int) {
	if c := s[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	r, n := utf8.DecodeRuneInString(string(s[i:]))
	if r == utf8.RuneError && n <= 1 {
		return invalid, 1
	}
	return r, n
}

func (s strText) Before(i int) (rune, int) {
	if c := s[i-1]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	r, n := utf8.DecodeLastRuneInString(string(s[:i]))
	if r == utf8.RuneError && n <= 1 {
		return invalid, 1
	}
	return r, n
}

type runeText []rune

func (s runeText) Len() int { return len(s) }

func (s runeText) At(i int) (rune, int) { return s[i], 1 }

func (s runeText) Before(i int) (rune, int) { return s[i-1], 1 }

// Context states of the forward scan.
const (
	pictNone = iota
	pictSeen // ExtPict Extend*
	pictZWJ  // ExtPict Extend* ZWJ
)

const (
	conjNone   = iota
	conjCons   // Consonant [Extend]*
	conjLinked // Linker [Extend Linker]*; since Unicode 18 no Consonant has to lead
)

// scan is the forward state: the previous character and the rule context
// that cannot be read from the pair alone.
type scan struct {
	prev prop
	ri   int
	pict int
	conj int
}

func (s *scan) start(p prop) {
	s.prev = p
	s.ri, s.pict, s.conj = 0, pictNone, conjNone
	s.advance(p)
}

// advance folds p into the context.
func (s *scan) advance(p prop) {
	if p.gb == uniprops.GBRegionalIndicator {
		s.ri++
	} else {
		s.ri = 0
	}
	switch {
	case p.pict:
		s.pict = pictSeen
	case s.pict == pictSeen && p.gb == uniprops.GBExtend:
	case s.pict == pictSeen && p.gb == uniprops.GBZWJ:
		s.pict = pictZWJ
	default:
		s.pict = pictNone
	}
	switch p.incb {
	case uniprops.InCBConsonant:
		s.conj = conjCons
	case uniprops.InCBLinker:
		s.conj = conjLinked
	case uniprops.InCBExtend:
	default:
		s.conj = conjNone
	}
	s.prev = p
}

// joins reports whether there is no boundary between the previous character
// and p (rules GB3 to GB999).
func (s *scan) joins(p prop) bool {
	a := s.prev.gb
	b := p.gb
	switch {
	case a == uniprops.GBCR && b == uniprops.GBLF:
		return true
	case a == uniprops.GBControl || a == uniprops.GBCR || a == uniprops.GBLF:
		return false
	case b == uniprops.GBControl || b == uniprops.GBCR || b == uniprops.GBLF:
		return false
	case a == uniprops.GBL && (b == uniprops.GBL || b == uniprops.GBV || b == uniprops.GBLV || b == uniprops.GBLVT):
		return true
	case (a == uniprops.GBLV || a == uniprops.GBV) && (b == uniprops.GBV || b == uniprops.GBT):
		return true
	case (a == uniprops.GBLVT || a == uniprops.GBT) && b == uniprops.GBT:
		return true
	case b == uniprops.GBExtend || b == uniprops.GBZWJ || b == uniprops.GBSpacingMark:
		return true
	case a == uniprops.GBPrepend:
		return true
	case p.incb == uniprops.InCBConsonant && s.conj == conjLinked:
		return true
	case p.pict && s.pict == pictZWJ:
		return true
	case a == uniprops.GBRegionalIndicator && b == uniprops.GBRegionalIndicator:
		return s.ri%2 == 1
	}
	return false
}

func next[T Source](t T, from int) int {
	n := t.Len()
	if from >= n {
		return n
	}
	if from < 0 {
		from = 0
	}
	r, w := t.At(from)
	i := from + w
	if i >= n {
		return n
	}
	var s scan
	s.start(propOf(r))
	for i < n {
		r, w = t.At(i)
		p := propOf(r)
		if !s.joins(p) {
			return i
		}
		s.advance(p)
		i += w
	}
	return n
}

// breakBefore reports whether there is a boundary at i, 0 < i < size, which
// must be on a character boundary; it reads only what precedes and follows i.
func breakBefore[T Source](t T, i int) bool {
	ar, aw := t.Before(i)
	br, _ := t.At(i)
	a, b := propOf(ar), propOf(br)
	var s scan
	// Reconstruct the context the forward scan would hold after a.
	s.prev = a
	switch {
	case a.gb == uniprops.GBRegionalIndicator && b.gb == uniprops.GBRegionalIndicator:
		s.ri = 1
		for j := i - aw; j > 0; {
			r, w := t.Before(j)
			if propOf(r).gb != uniprops.GBRegionalIndicator {
				break
			}
			s.ri++
			j -= w
		}
	case a.gb == uniprops.GBZWJ && b.pict:
		s.pict = pictNone
		for j := i - aw; j > 0; {
			r, w := t.Before(j)
			q := propOf(r)
			if q.pict {
				s.pict = pictZWJ
				break
			}
			if q.gb != uniprops.GBExtend {
				break
			}
			j -= w
		}
	case b.incb == uniprops.InCBConsonant && a.incb != uniprops.InCBNone:
		linked := a.incb == uniprops.InCBLinker
		j := i - aw
		if a.incb != uniprops.InCBConsonant {
			for j > 0 {
				r, w := t.Before(j)
				q := propOf(r)
				if q.incb == uniprops.InCBLinker {
					linked = true
				} else if q.incb != uniprops.InCBExtend {
					break
				}
				j -= w
			}
		}
		if linked {
			s.conj = conjLinked
		}
	}
	return !s.joins(b)
}

func boundary[T Source](t T, i int) bool {
	n := t.Len()
	if i <= 0 || i >= n {
		return i == 0 || i == n
	}
	return breakBefore(t, i)
}

func prev[T Source](t T, end int) int {
	n := t.Len()
	if end > n {
		end = n
	}
	if end <= 0 {
		return 0
	}
	_, w := t.Before(end)
	for i := end - w; i > 0; i -= w {
		if breakBefore(t, i) {
			return i
		}
		_, w = t.Before(i)
	}
	return 0
}

// Next returns the end offset of the cluster of s that starts at from, or
// len(s) when it is the last. from must be a cluster start.
func Next(s string, from int) int {
	if from >= 0 && from < len(s) {
		// ASCII fast path: an ASCII character is a whole cluster unless it
		// is CR before LF; nothing ASCII can follow and extend it.
		if c := s[from]; c < utf8.RuneSelf && (from+1 == len(s) || s[from+1] < utf8.RuneSelf) {
			if c == '\r' && from+1 < len(s) && s[from+1] == '\n' {
				return from + 2
			}
			return from + 1
		}
	}
	return next(strText(s), from)
}

// Prev returns the start offset of the cluster of s that ends at end, or 0.
// It agrees with Next walked from the start of s.
func Prev(s string, end int) int {
	if end > 1 && end <= len(s) && s[end-1] < utf8.RuneSelf && s[end-2] < utf8.RuneSelf && !(s[end-1] == '\n' && s[end-2] == '\r') {
		if end == len(s) || s[end] < utf8.RuneSelf {
			return end - 1
		}
	}
	return prev(strText(s), end)
}

// Boundary reports whether i is a cluster boundary of s; 0 and len(s) are.
// An offset inside a UTF-8 sequence is not.
func Boundary(s string, i int) bool {
	if i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		k := i
		for k > 0 && i-k < utf8.UTFMax && !utf8.RuneStart(s[k]) {
			k--
		}
		if _, n := utf8.DecodeRuneInString(s[k:]); utf8.RuneStart(s[k]) && k+n > i {
			return false
		}
	}
	return boundary(strText(s), i)
}

// NextRunes is Next for a rune slice; offsets count runes.
func NextRunes(r []rune, from int) int {
	if from >= 0 && from < len(r) {
		if c := r[from]; c >= 0 && c < utf8.RuneSelf && (from+1 == len(r) || (r[from+1] >= 0 && r[from+1] < utf8.RuneSelf)) {
			if c == '\r' && from+1 < len(r) && r[from+1] == '\n' {
				return from + 2
			}
			return from + 1
		}
	}
	return next(runeText(r), from)
}

// PrevRunes is Prev for a rune slice.
func PrevRunes(r []rune, end int) int { return prev(runeText(r), end) }

// NextIn is Next for a Source, for text that is not one string or rune slice.
// It reads only the runes the rules need around from, so a source held in
// pieces is read where it is, with every rune of context the rules ask for.
func NextIn[S Source](s S, from int) int { return next(s, from) }

// PrevIn is Prev for a Source (see NextIn).
func PrevIn[S Source](s S, end int) int { return prev(s, end) }

// BoundaryRunes is Boundary for a rune slice.
func BoundaryRunes(r []rune, i int) bool { return boundary(runeText(r), i) }
