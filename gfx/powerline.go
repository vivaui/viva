package gfx

import "math"

// IsPowerline reports whether r is one of the Powerline separators and extra symbols a fixed-pitch
// line draws as geometry: the geometric U+E0B0 to U+E0D4. The stylised ones of the block, which
// are a glyph rather than an edge, stay with the font, as in ghostty (sprite/draw/powerline.zig).
func IsPowerline(r rune) bool { return powerlineDrawer(r) != nil }

// powerlineDrawer is what draws r as geometry in a cell, or nil when the font should draw it.
func powerlineDrawer(r rune) func(*cellGlyph, rune) {
	switch r {
	case 0xE0B0, 0xE0B1, 0xE0B2, 0xE0B3, 0xE0B4, 0xE0B5, 0xE0B6, 0xE0B7,
		0xE0B8, 0xE0B9, 0xE0BA, 0xE0BB, 0xE0BC, 0xE0BD, 0xE0BE, 0xE0BF,
		0xE0D2, 0xE0D4:
		return (*cellGlyph).powerline
	}
	return nil
}

// powerline draws r in the cell. Its filled shapes are triangles and paths whose edges reach the
// cell's own edges, so a run of them in one colour joins seamlessly, and its rounded bars are
// scaled to the cell height rather than to the font's line box.
func (g *cellGlyph) powerline(r rune) {
	width, height := float32(g.width), float32(g.height)
	middle := height / 2
	switch r {
	// A right-pointing triangle in the whole cell.
	case 0xE0B0:
		g.fill(Point{X: 0, Y: 0}, Point{X: width, Y: middle}, Point{X: 0, Y: height})
	case 0xE0B2:
		g.fill(Point{X: width, Y: 0}, Point{X: 0, Y: middle}, Point{X: width, Y: height})
	// Half-cell diagonal triangles.
	case 0xE0B8:
		g.fill(Point{X: 0, Y: 0}, Point{X: width, Y: height}, Point{X: 0, Y: height})
	case 0xE0BA:
		g.fill(Point{X: width, Y: 0}, Point{X: width, Y: height}, Point{X: 0, Y: height})
	case 0xE0BC:
		g.fill(Point{X: 0, Y: 0}, Point{X: width, Y: 0}, Point{X: 0, Y: height})
	case 0xE0BE:
		g.fill(Point{X: 0, Y: 0}, Point{X: width, Y: 0}, Point{X: width, Y: height})
	// Cell diagonals.
	case 0xE0B9, 0xE0BF:
		g.strokeDiagonal(false)
	case 0xE0BB, 0xE0BD:
		g.strokeDiagonal(true)
	// Outlined triangles.
	case 0xE0B1:
		g.strokeTriangle(false)
	case 0xE0B3:
		g.strokeTriangle(true)
	// Rounded bars, filled and outlined, in both directions.
	case 0xE0B4:
		g.roundedBar(false, false)
	case 0xE0B5:
		g.roundedBar(false, true)
	case 0xE0B6:
		g.roundedBar(true, false)
	case 0xE0B7:
		g.roundedBar(true, true)
	// Arrows pointing at a line between two rows.
	case 0xE0D2:
		g.powerlineArrow(false)
	case 0xE0D4:
		g.powerlineArrow(true)
	}
}

// strokeTriangle strokes the outline of the triangle that fills the cell from its left edge to its
// middle. The outline is the triangle's own path, so its hypotenuse is one stroke and its corners
// stay inside the cell.
func (g *cellGlyph) strokeTriangle(mirrored bool) {
	width, height := float32(g.width), float32(g.height)
	middle := height / 2
	from, to := float32(0), width
	if mirrored {
		from, to = to, from
	}
	g.stroke(NewPath(width, height,
		MoveTo(from, 0),
		LineTo(to, middle),
		LineTo(from, height)))
}

// roundedBar fills or strokes the rounded bar ghostty draws for U+E0B4 to U+E0B7: a vertical band
// whose outer edge is a quarter circle at the top and the bottom, as wide as the cell's height
// allows. Stroked, the path is moved in by half a rule so the ink lies inside the bar, as ghostty
// strokes its inner side.
func (g *cellGlyph) roundedBar(mirrored, outline bool) {
	width, height := float64(g.width), float64(g.height)
	radius := math.Min(width, height/2)
	// Filled, the curves run to the bar's own corner; stroked, they leave a pixel of straight edge
	// at each end, so the butt caps of the stroke lie square across it.
	top, bottom, edge := 0.0, height, 0.0
	if outline {
		half := float64(g.rule) / 2
		radius -= half
		if radius <= 0 {
			return
		}
		top, bottom, edge = half, height-half, 1
	}
	// The coefficient that approximates a quarter circle with one cubic.
	const bend = (math.Sqrt2 - 1) * 4 / 3
	arc := radius * bend
	// The path runs from the top edge, around the bar's outer side, to the bottom edge.
	x := func(value float64) float64 {
		if mirrored {
			return width - value
		}
		return value
	}
	commands := []PathCommand{
		MoveTo(float32(x(0)), float32(top)),
		LineTo(float32(x(edge)), float32(top)),
		CubicTo(
			float32(x(arc)), float32(top),
			float32(x(radius)), float32(radius-arc),
			float32(x(radius)), float32(radius)),
		LineTo(float32(x(radius)), float32(height-radius)),
		CubicTo(
			float32(x(radius)), float32(height-radius+arc),
			float32(x(arc)), float32(bottom),
			float32(x(edge)), float32(bottom)),
		LineTo(float32(x(0)), float32(bottom)),
	}
	if outline {
		g.stroke(NewPath(float32(width), float32(height), commands...))
		return
	}
	commands = append(commands, ClosePath())
	g.fillPath(NewPath(float32(width), float32(height), commands...), PathFillNonZero)
}

// powerlineArrow fills the two triangles of U+E0D2 and U+E0D4 that meet in the middle of the cell,
// one from the top edge and one from the bottom, leaving a rule of background between them.
func (g *cellGlyph) powerlineArrow(mirrored bool) {
	width, height := float32(g.width), float32(g.height)
	middle, half := height/2, float32(g.rule)/2
	x := func(value float32) float32 { return value }
	if mirrored {
		x = func(value float32) float32 { return width - value }
	}
	g.fillPolygon([]Point{
		{X: x(0), Y: 0}, {X: x(width), Y: 0},
		{X: x(width / 2), Y: middle - half}, {X: x(0), Y: middle - half},
	})
	g.fillPolygon([]Point{
		{X: x(0), Y: height}, {X: x(width), Y: height},
		{X: x(width / 2), Y: middle + half}, {X: x(0), Y: middle + half},
	})
}
