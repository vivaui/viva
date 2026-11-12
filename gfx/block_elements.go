package gfx

// IsBlockElement reports whether r is a Unicode Block Element (U+2580 to U+259F): the
// halves, eighths, quadrants and shades that terminals draw as exact fractions of a
// character cell. A fixed-pitch line from LabelRuns draws them as geometry, never from a
// font, so neighbouring cells meet without a seam whatever the font's side bearings, the
// ClearType filter or the display scale. Canvas.CellGlyphRectangles is that geometry.
func IsBlockElement(r rune) bool { return r >= 0x2580 && r <= 0x259F }

// blockPart is one filled rectangle of a cell, in eighths of its width and height.
type blockPart struct{ left, top, right, bottom uint8 }

// blockShape is what a Block Element fills: up to three rectangles of the cell, drawn at
// shade quarters of the text color's opacity (4 is solid).
type blockShape struct {
	parts [3]blockPart
	count uint8
	shade uint8
}

const (
	upperLeft = 1 << iota
	upperRight
	lowerLeft
	lowerRight
)

// quadrants are U+2596 to U+259F as the set of quadrants each fills.
var quadrants = [...]uint8{
	lowerLeft,                           // ▖
	lowerRight,                          // ▗
	upperLeft,                           // ▘
	upperLeft | lowerLeft | lowerRight,  // ▙
	upperLeft | lowerRight,              // ▚
	upperLeft | upperRight | lowerLeft,  // ▛
	upperLeft | upperRight | lowerRight, // ▜
	upperRight,                          // ▝
	upperRight | lowerLeft,              // ▞
	upperRight | lowerLeft | lowerRight, // ▟
}

func blockShapeOf(r rune) blockShape {
	one := func(left, top, right, bottom uint8) blockShape {
		return blockShape{parts: [3]blockPart{{left, top, right, bottom}}, count: 1, shade: 4}
	}
	switch {
	case r == 0x2580:
		return one(0, 0, 8, 4)
	case r >= 0x2581 && r <= 0x2588: // lower one eighth to the full block
		return one(0, 8-uint8(r-0x2580), 8, 8)
	case r >= 0x2589 && r <= 0x258F: // left seven eighths to one eighth
		return one(0, 0, uint8(0x2590-r), 8)
	case r == 0x2590:
		return one(4, 0, 8, 8)
	case r >= 0x2591 && r <= 0x2593: // light, medium and dark shade
		shape := one(0, 0, 8, 8)
		shape.shade = uint8(r - 0x2590)
		return shape
	case r == 0x2594:
		return one(0, 0, 8, 1)
	case r == 0x2595:
		return one(7, 0, 8, 8)
	}
	shape := blockShape{shade: 4}
	set := quadrants[r-0x2596]
	for quadrant, part := range [4]blockPart{{0, 0, 4, 4}, {4, 0, 8, 4}, {0, 4, 4, 8}, {4, 4, 8, 8}} {
		if set&(1<<quadrant) != 0 {
			shape.parts[shape.count] = part
			shape.count++
		}
	}
	return shape
}

// blockElement fills the parts of the block element r in the cell. A part of the cell counts in
// the cell's whole device pixels, so two neighbouring cells meet at the pixel they share.
func (g *cellGlyph) blockElement(r rune) {
	shape := blockShapeOf(r)
	first := len(g.shapes.Rectangles)
	for _, part := range shape.parts[:shape.count] {
		left, right := g.nearEdge(g.width, part.left), g.farEdge(g.width, part.right)
		top, bottom := g.nearEdge(g.height, part.top), g.farEdge(g.height, part.bottom)
		if right <= left {
			right = left + 1
		}
		if bottom <= top {
			bottom = top + 1
		}
		g.rect(left, top, right, bottom)
	}
	// A shade fills its part of the cell at a quarter of the text's opacity per shade step.
	for index := first; index < len(g.shapes.Rectangles); index++ {
		g.shapes.Rectangles[index].Color.Alpha = float32(shape.shade) / 4
	}
}

// farEdge is where eighths of the way along extent lands, in whole device pixels.
func (g cellGlyph) farEdge(extent int32, eighths uint8) int32 {
	return fracMax(extent, int32(eighths), 8)
}

// nearEdge is where the edge one step before eighths lands. It is measured from the far end, the
// way ghostty measures it: a cell of seven pixels split in half gives three and four, so neither
// half loses the odd pixel, and `▕` in a twelve pixel cell is the two pixels `round(12/8)` of it,
// flush against the edge.
func (g cellGlyph) nearEdge(extent int32, eighths uint8) int32 {
	return fracMin(extent, int32(eighths), 8)
}
