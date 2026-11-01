package platform

import "unicode/utf8"

// BlockRect is a rectangle of a character cell in eighths of the cell, from (X0, Y0) to
// (X1, Y1) with the origin at the cell's top left corner.
type BlockRect struct{ X0, Y0, X1, Y1 uint8 }

// BlockElement is the shape of a Block Elements character (U+2580 to U+259F): up to two
// rectangles of its cell, filled to Shade percent. A terminal draws these itself instead of
// taking them from a font: antialiasing the edges of two neighbouring glyphs leaves a seam in
// the pixel they share, and a font's side bearings leave a gap, where the character is meant
// to fill its cell edge to edge.
type BlockElement struct {
	Rects [2]BlockRect
	Count uint8
	Shade uint8
}

// BlockElementOf reports the shape of text when it is exactly one block element.
func BlockElementOf(text string) (BlockElement, bool) {
	r, size := utf8.DecodeRuneInString(text)
	if size == 0 || size != len(text) || r < 0x2580 || r > 0x259F {
		return BlockElement{}, false
	}
	solid := func(rects ...BlockRect) BlockElement {
		element := BlockElement{Count: uint8(len(rects)), Shade: 100}
		copy(element.Rects[:], rects)
		return element
	}
	switch {
	case r == 0x2580:
		return solid(BlockRect{0, 0, 8, 4}), true
	case r <= 0x2587: // lower one eighth to seven eighths
		return solid(BlockRect{0, 8 - uint8(r-0x2580), 8, 8}), true
	case r == 0x2588:
		return solid(BlockRect{0, 0, 8, 8}), true
	case r <= 0x258F: // left seven eighths to one eighth
		return solid(BlockRect{0, 0, 8 - uint8(r-0x2588), 8}), true
	case r == 0x2590:
		return solid(BlockRect{4, 0, 8, 8}), true
	case r <= 0x2593: // light, medium and dark shade
		element := solid(BlockRect{0, 0, 8, 8})
		element.Shade = uint8(25 * (r - 0x2590))
		return element, true
	case r == 0x2594:
		return solid(BlockRect{0, 0, 8, 1}), true
	case r == 0x2595:
		return solid(BlockRect{7, 0, 8, 8}), true
	}
	upperLeft, upperRight := BlockRect{0, 0, 4, 4}, BlockRect{4, 0, 8, 4}
	lowerLeft, lowerRight := BlockRect{0, 4, 4, 8}, BlockRect{4, 4, 8, 8}
	switch r {
	case 0x2596:
		return solid(lowerLeft), true
	case 0x2597:
		return solid(lowerRight), true
	case 0x2598:
		return solid(upperLeft), true
	case 0x2599: // all but the upper right
		return solid(BlockRect{0, 0, 4, 8}, lowerRight), true
	case 0x259A:
		return solid(upperLeft, lowerRight), true
	case 0x259B: // all but the lower right
		return solid(BlockRect{0, 0, 8, 4}, lowerLeft), true
	case 0x259C: // all but the lower left
		return solid(BlockRect{0, 0, 8, 4}, lowerRight), true
	case 0x259D:
		return solid(upperRight), true
	case 0x259E:
		return solid(upperRight, lowerLeft), true
	}
	return solid(BlockRect{4, 0, 8, 4}, BlockRect{0, 4, 8, 8}), true // 0x259F: all but the upper left
}

// DeviceRect maps a rectangle of the cell to whole device pixels, for a cell that starts at
// left device pixels and is span wide, in a line height tall. Each edge is rounded on its own,
// so the right edge of one cell and the left edge of the next, which are the same number, land
// on the same pixel and the cells meet without a seam or an overlap.
func (rect BlockRect) DeviceRect(left, span float32, height int) (x0, y0, x1, y1 int) {
	edge := func(origin, length float32, eighths uint8) int {
		return int(origin + length*float32(eighths)/8 + 0.5)
	}
	return edge(left, span, rect.X0), edge(0, float32(height), rect.Y0), edge(left, span, rect.X1), edge(0, float32(height), rect.Y1)
}
