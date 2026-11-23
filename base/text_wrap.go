package base

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// wrapCache remembers one wrapped Text's lines between frames. Wrapping is
// deterministic for a text, size, weight, face, width and line limit, so a page
// that did not change can append the saved lines instead of measuring every
// prefix again.
type wrapCache struct {
	Text     string
	Size     float32
	Weight   float32
	mono     bool
	italic   bool
	Face     gfx.MonoFace
	width    float32
	MaxLines int
	Lines    []string
	filled   bool
}

// wrapText appends the lines of a wrapped Text, breaking them as gfx.Canvas.WrapLines
// does (spaces, East Asian text with its line-start and line-end rules, separators
// inside a long path or URL, and as a last resort between whole characters), each
// line measured as it is drawn: a Mono text on its cells, an Italic one in the
// italic face. Both the full line set and the ellipsized last line are kept for the
// life of the view, so a page that does not change does no wrapping or string
// building.
func (f *Frame) wrapText(dst []string, n *Node) []string {
	if f.wrapCursor == len(f.Ellipses) {
		f.Ellipses = append(f.Ellipses, gfx.EllipsisCache{})
	}
	if f.wrapCursor == len(f.Wraps) {
		f.Wraps = append(f.Wraps, wrapCache{})
	}
	cache := &f.Ellipses[f.wrapCursor]
	wrapped := &f.Wraps[f.wrapCursor]
	f.wrapCursor++
	width := max(n.Rect.Width-n.PadX(), 1)
	maxLines := int(n.MaxLines)
	mono := n.Flags&flagMono != 0
	italic := n.TextDecor&textItalic != 0
	face := gfx.MonoFace(0)
	if mono {
		face = n.Face
	}
	if wrapped.filled && wrapped.Text == n.Text && wrapped.Size == n.FontSize && wrapped.Weight == n.Weight &&
		wrapped.mono == mono && wrapped.italic == italic && wrapped.Face == face &&
		wrapped.width == width && wrapped.MaxLines == maxLines {
		return append(dst, wrapped.Lines...)
	}
	start := len(dst)
	dst = f.Surface.WrapLinesStyle(dst, n.Text, n.textStyle(), width, maxLines, cache)
	wrapped.Text, wrapped.Size, wrapped.Weight, wrapped.mono, wrapped.italic, wrapped.Face = n.Text, n.FontSize, n.Weight, mono, italic, face
	wrapped.width, wrapped.MaxLines = width, maxLines
	wrapped.Lines = append(wrapped.Lines[:0], dst[start:]...)
	wrapped.filled = true
	return dst
}

// textStyle is the style a Text's width depends on: its size, weight, slant and,
// when it is Mono, its face.
func (n *Node) textStyle() gfx.TextStyle {
	return gfx.TextStyle{Size: n.FontSize, Weight: n.Weight, Italic: n.TextDecor&textItalic != 0, Mono: n.Flags&flagMono != 0, MonoFace: n.Face}
}

// Wrap adds text that breaks into as many lines as the width it is given needs,
// so its height follows its width: Paragraph without a limit.
func (f *Frame) Wrap(Value string) Box { return f.Paragraph(Value, math.MaxInt32) }

// MaxLines makes a Text a paragraph of at most n lines, the last ending with an
// ellipsis when the text needs more (n of one is Ellipsis); n of zero or less
// takes as many lines as the width needs.
func (b Box) MaxLines(n int) Box {
	node := b.N()
	node.Flags |= flagWrap
	if n <= 0 {
		n = math.MaxInt32
	}
	node.MaxLines = int32(n)
	return b
}

// OverflowWrap is how a wrapped Text breaks a word that is too wide for its line,
// as CSS's overflow-wrap property says.
type OverflowWrap uint8

const (
	// OverflowWrapBreakWord breaks a line inside a word too wide for it, which is what
	// a wrapped Text already does and what CSS calls overflow-wrap: break-word: the
	// width a Text asks for is its widest line, so a word wider than the room the Text
	// is given is broken rather than overflow it. It is the zero value and the answer
	// of every Text that is not given OverflowWrap. CSS overflow-wrap: normal, where
	// the word overflows instead, is not expressible.
	OverflowWrapBreakWord OverflowWrap = iota
	// OverflowWrapAnywhere may break inside a word, at any character boundary, when the
	// word does not fit its line. As with CSS overflow-wrap: anywhere, the width the
	// Text asks for is then the room it is given rather than its widest line, because
	// its narrowest line is one character: a paragraph centred in a Column stays inside
	// the Column and breaks its long URL instead of running out over both edges. A break
	// never falls inside a character as a reader sees it (UAX #29: a base with its
	// marks, an emoji sequence, a flag, an Indic conjunct), and while the text fits the
	// room it is given nothing about it changes.
	OverflowWrapAnywhere
)

// OverflowWrap sets how a wrapped Text breaks a word too wide for its line (see Wrap,
// Paragraph and MaxLines); it has no effect on a Text that does not wrap, which is one
// line and is clipped.
//
//	f.Column().Items(Center).Children(func() {
//		f.Wrap(url).OverflowWrap(OverflowWrapAnywhere)
//	})
func (b Box) OverflowWrap(v OverflowWrap) Box {
	n := b.N()
	if v == OverflowWrapBreakWord && n.Ext == 0 {
		return b // the zero value says it already; no Extra is made to say it again
	}
	b.f.Extra(b.i).overflowWrap = v
	return b
}
