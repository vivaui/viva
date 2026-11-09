package gfx

import "math"

// IsBlockElement reports whether r is a Unicode Block Element (U+2580 to U+259F): the
// halves, eighths, quadrants and shades that terminals draw as exact fractions of a
// character cell. A fixed-pitch line from LabelRuns draws them as geometry, never from a
// font, so neighbouring cells meet without a seam whatever the font's side bearings, the
// ClearType filter or the display scale. Canvas.BlockRectangles is that geometry.
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

// BlockRectangles appends to dst the rectangles that draw the Block Elements of a text made
// by LabelRuns, and returns the extended slice. Each is a cell fraction snapped to whole
// device pixels at the canvas's pixel scale, with the text's layer and clip. Edges are
// computed from the cell column, not from the neighbour's width, so two cells that share an
// edge share the snapped pixel too; a run of the same character in one color becomes one
// rectangle where its parts span the whole cell width. The rectangles belong to the layer
// of the text, drawn before it: the line's value still holds the characters (for
// accessibility and copying), and a renderer must not also draw them from the font; see
// ReplaceBlockElements. A text without Block Elements adds nothing and does not allocate.
func (canvas *Canvas) BlockRectangles(dst []Rectangle, text Text) []Rectangle {
	runs := canvas.TextRuns(text)
	if len(runs) == 0 || text.CellWidth <= 0 || text.Bounds.Width <= 0 || text.Bounds.Height <= 0 {
		return dst
	}
	var (
		pending      rune
		pendingColor Color
		first, count int32
	)
	flush := func() {
		if count > 0 {
			dst = canvas.appendBlock(dst, text, pending, pendingColor, first, count)
			count = 0
		}
	}
	offset := 0
	for _, run := range runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(text.Value) {
			break
		}
		segment := text.Value[offset:end]
		offset = end
		column := run.Column
		for _, r := range segment {
			if IsBlockElement(r) && run.Color.Alpha > 0 {
				if count > 0 && r == pending && run.Color == pendingColor && column == first+count {
					count++
				} else {
					flush()
					pending, pendingColor, first, count = r, run.Color, column, 1
				}
			}
			column++
		}
	}
	flush()
	return dst
}

// appendBlock adds the rectangles of count adjacent cells, from column first, that show r.
func (canvas *Canvas) appendBlock(dst []Rectangle, text Text, r rune, color Color, first, count int32) []Rectangle {
	shape := blockShapeOf(r)
	color.Alpha *= float32(shape.shade) / 4
	cell := float64(text.CellWidth)
	for _, part := range shape.parts[:shape.count] {
		top, bottom := canvas.blockEdges(float64(text.Bounds.Y), float64(text.Bounds.Height), float64(part.top)/8, float64(part.bottom)/8)
		if part.left == 0 && part.right == 8 {
			// A whole cell wide: the cells run together as one rectangle.
			left, right := canvas.blockEdges(float64(text.Bounds.X), cell, float64(first), float64(first+count))
			dst = appendBlockRectangle(dst, text, color, left, top, right, bottom)
			continue
		}
		for index := first; index < first+count; index++ {
			left, right := canvas.blockEdges(float64(text.Bounds.X), cell, float64(index)+float64(part.left)/8, float64(index)+float64(part.right)/8)
			dst = appendBlockRectangle(dst, text, color, left, top, right, bottom)
		}
	}
	return dst
}

// blockEdges is the two edges of a span that starts at origin and runs from fraction low to
// high of unit, on whole device pixels. A span that would vanish keeps one device pixel.
func (canvas *Canvas) blockEdges(origin, unit, low, high float64) (float32, float32) {
	start, end := origin+low*unit, origin+high*unit
	if canvas.pixelScale <= 0 {
		return float32(start), float32(end)
	}
	scale := float64(canvas.pixelScale)
	from, to := math.Floor(start*scale+0.5), math.Floor(end*scale+0.5)
	if to <= from {
		to = from + 1
	}
	return float32(from / scale), float32(to / scale)
}

func appendBlockRectangle(dst []Rectangle, text Text, color Color, left, top, right, bottom float32) []Rectangle {
	return append(dst, Rectangle{
		Bounds: Rect{X: left, Y: top, Width: right - left, Height: bottom - top},
		Color:  color, Layer: text.Layer, ClipOf: text.ClipOf, HasClip: text.HasClip,
	})
}

// ReplaceBlockElements is the value and runs of a LabelRuns line as a font should draw
// them: every Block Element becomes a space, which holds its cell and draws nothing, since
// BlockRectangles draws it. The runs are adjusted to the shorter text; the rest of the line
// keeps its columns. A line without Block Elements is returned as it is, without
// allocating; otherwise the result is new and the arguments are left alone.
func ReplaceBlockElements(value string, runs []TextRun) (string, []TextRun) {
	if !hasBlockElement(value) {
		return value, runs
	}
	text := make([]byte, 0, len(value))
	replaced := make([]TextRun, len(runs))
	copy(replaced, runs)
	offset := 0
	for index := range replaced {
		end := offset + int(replaced[index].Bytes)
		if replaced[index].Bytes < 0 || end > len(value) {
			// Not a cover of the value: keep the rest as it is.
			text = append(text, value[offset:]...)
			offset = len(value)
			break
		}
		before := len(text)
		for index := offset; index < end; {
			if isBlockElementAt(value[:end], index) {
				text = append(text, ' ')
				index += 3
				continue
			}
			text = append(text, value[index])
			index++
		}
		replaced[index].Bytes = int32(len(text) - before)
		offset = end
	}
	text = append(text, value[offset:]...)
	return string(text), replaced
}

// hasBlockElement reports whether value holds a character of U+2580 to U+259F.
func hasBlockElement(value string) bool {
	for index := 0; index+2 < len(value); index++ {
		if isBlockElementAt(value, index) {
			return true
		}
	}
	return false
}

// isBlockElementAt reports whether value holds a character of U+2580 to U+259F at index,
// which UTF-8 spells E2 96 80 to E2 96 9F.
func isBlockElementAt(value string, index int) bool {
	return index+2 < len(value) && value[index] == 0xE2 && value[index+1] == 0x96 && value[index+2] >= 0x80 && value[index+2] <= 0x9F
}
