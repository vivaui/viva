package ui

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// wrapCache remembers one wrapped Text's lines between frames. Wrapping is
// deterministic for a text, size, weight, face, width and line limit, so a page
// that did not change can append the saved lines instead of measuring every
// prefix again.
type wrapCache struct {
	text     string
	size     float32
	weight   float32
	mono     bool
	face     gfx.MonoFace
	width    float32
	maxLines int
	lines    []string
	filled   bool
}

// wrapText appends the lines of a wrapped Text, breaking them as gfx.Canvas.WrapLines
// does (spaces, East Asian text with its line-start and line-end rules, separators
// inside a long path or URL, and as a last resort between whole characters), each
// line measured as it is drawn: a Mono text on its cells. Both
// the full line set and the ellipsized last line are kept for the life of the View,
// so a page that does not change does no wrapping or string building.
func (f *Frame) wrapText(dst []string, n *node) []string {
	if f.wrapCursor == len(f.ellipses) {
		f.ellipses = append(f.ellipses, gfx.EllipsisCache{})
	}
	if f.wrapCursor == len(f.wraps) {
		f.wraps = append(f.wraps, wrapCache{})
	}
	cache := &f.ellipses[f.wrapCursor]
	wrapped := &f.wraps[f.wrapCursor]
	f.wrapCursor++
	width := max(n.rect.Width-n.padX(), 1)
	maxLines := int(n.maxLines)
	mono := n.flags&flagMono != 0
	face := gfx.MonoFace(0)
	if mono {
		face = n.face
	}
	if wrapped.filled && wrapped.text == n.text && wrapped.size == n.fontSize && wrapped.weight == n.weight && wrapped.mono == mono && wrapped.face == face && wrapped.width == width && wrapped.maxLines == maxLines {
		return append(dst, wrapped.lines...)
	}
	start := len(dst)
	dst = f.canvas.WrapLinesStyle(dst, n.text, n.textStyle(), width, maxLines, cache)
	wrapped.text, wrapped.size, wrapped.weight, wrapped.mono, wrapped.face, wrapped.width, wrapped.maxLines = n.text, n.fontSize, n.weight, mono, face, width, maxLines
	wrapped.lines = append(wrapped.lines[:0], dst[start:]...)
	wrapped.filled = true
	return dst
}

// textStyle is the style a Text's width depends on: its size, weight and, when it is
// Mono, its face.
func (n *node) textStyle() gfx.TextStyle {
	return gfx.TextStyle{Size: n.fontSize, Weight: n.weight, Mono: n.flags&flagMono != 0, MonoFace: n.face}
}

// Wrap adds text that breaks into as many lines as the width it is given needs,
// so its height follows its width: Paragraph without a limit.
func (f *Frame) Wrap(value string) Box { return f.Paragraph(value, math.MaxInt32) }

// MaxLines makes a Text a paragraph of at most n lines, the last ending with an
// ellipsis when the text needs more (n of one is Ellipsis); n of zero or less
// takes as many lines as the width needs.
func (b Box) MaxLines(n int) Box {
	node := b.n()
	node.flags |= flagWrap
	if n <= 0 {
		n = math.MaxInt32
	}
	node.maxLines = int32(n)
	return b
}
