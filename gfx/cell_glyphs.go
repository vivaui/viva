package gfx

import "unicode/utf8"

// IsCellGlyph reports whether r is a character a fixed-pitch line draws as geometry in its cell
// instead of from the font: the Block Elements, Box Drawing, Braille and the corner shapes of
// Geometric Shapes. The line's text still holds the character, for accessibility and for
// copying; see ReplaceCellGlyphs.
func IsCellGlyph(r rune) bool { return cellGlyphDrawer(r) != nil }

// cellGlyphDrawer is what draws r as geometry in a cell, or nil when the font should draw it. The
// families are named here and nowhere else, so adding one is a method on cellGlyph and a case
// here.
func cellGlyphDrawer(r rune) func(*cellGlyph, rune) {
	switch {
	case IsBoxDrawing(r):
		return (*cellGlyph).boxDrawing
	case IsBlockElement(r):
		return (*cellGlyph).blockElement
	case IsBraille(r):
		return (*cellGlyph).braille
	case IsGeometricShape(r):
		return (*cellGlyph).geometricShape
	}
	return nil
}

// CellGlyphShapes is the geometry of a fixed-pitch line's cell glyphs: rectangles for what is
// axis-aligned and snapped to whole device pixels, triangles for what fills, bends or slants,
// whose edges a renderer feathers over the device pixel they end in.
type CellGlyphShapes struct {
	Rectangles []Rectangle
	Triangles  []Triangle
}

// cellMetrics is the shape of one cell in whole device pixels: its size, the thickness a rule has
// on it, and how far a shape may reach past it.
type cellMetrics struct {
	width, height int32
	rule          int32
	padX, padY    int32
}

// cellGlyph draws one cell glyph in device pixels from the cell's own corner. What it draws is the
// character and these metrics alone, which is what makes the same character the same geometry in
// every cell of its size.
type cellGlyph struct {
	canvas *Canvas // only a stroke needs it: a path is tessellated through the canvas
	cellMetrics
	shapes CellGlyphShapes
}

// cellLine is a text seen as cells: the scale back to logical pixels, and what every cell of the
// line has in common — its top edge on the device pixel grid, its height, the rule its font size
// gives it, and the layer and clip its shapes are drawn in. The width is not here: it is the
// distance to the next cell's snapped edge, so it belongs to the column.
type cellLine struct {
	text   Text
	scale  float64
	top    float32 // the line's top edge, logical pixels, on the device pixel grid
	height int32   // device pixels
	padY   int32
	// rule is one rule's thickness, in device pixels.
	//
	// ponytail: it is 5% of the font size, the underline a fixed-pitch face draws, because no
	// text engine reports its underline thickness yet; an engine that does can hand it over the
	// way MonoCell hands the cell, and only the line that computes it changes.
	rule int32
}

// rect fills the rectangle between two points of the cell, both in device pixels from its corner.
// A shape may reach a quarter of a cell past the cell, the room a sprite has in ghostty, so a
// diagonal can meet the line in the cell it points at; an empty rectangle adds nothing, so a
// shape too small for the cell disappears instead of inverting.
func (g *cellGlyph) rect(x0, y0, x1, y1 int32) {
	x0, x1 = max(x0, -g.padX), min(x1, g.width+g.padX)
	y0, y1 = max(y0, -g.padY), min(y1, g.height+g.padY)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	g.shapes.Rectangles = append(g.shapes.Rectangles, Rectangle{
		Bounds: Rect{
			X:      float32(x0),
			Y:      float32(y0),
			Width:  float32(x1 - x0),
			Height: float32(y1 - y0),
		},
		Color: Color{Alpha: 1},
	})
}

// fill fills the triangle through three points of the cell, in device pixels from its corner.
func (g *cellGlyph) fill(a, b, c Point) {
	g.shapes.Triangles = append(g.shapes.Triangles, Triangle{
		Points:   [3]Point{a, b, c},
		Color:    Color{Alpha: 1},
		EdgeMask: triangleAllEdges,
	})
}

// bounds is the cell's own rectangle, the box a path is laid out in.
func (g cellGlyph) bounds() Rect { return Rect{Width: float32(g.width), Height: float32(g.height)} }

// fillPath fills a closed path, laid out in the cell's device pixels, under the given rule.
func (g *cellGlyph) fillPath(path Path, rule PathFillRule) {
	g.shapes.Triangles = g.canvas.PathTriangles(g.shapes.Triangles, path, g.bounds(),
		PathStyle{Fill: Color{Alpha: 1}, FillRule: rule})
}

// stroke strokes path, laid out in the cell's device pixels, one rule wide, with butt ends. A cell
// is always built at scale 1, where the rule's width in device pixels is also its width in the
// path's own units.
func (g *cellGlyph) stroke(path Path) {
	g.shapes.Triangles = g.canvas.PathTriangles(g.shapes.Triangles, path, g.bounds(),
		PathStyle{Stroke: Color{Alpha: 1}, StrokeWidth: float32(g.rule), Cap: CapButt})
}

// cellAt is where the cell at column starts, in logical pixels, and its metrics in device
// pixels. Both edges are snapped to the device pixel grid and the width is the distance between
// them, so two cells that share an edge share the pixel too.
func (l cellLine) cellAt(column, cells int32) (Point, cellMetrics) {
	left := float64(l.text.Bounds.X)
	from := round32((left + float64(column)*float64(l.text.CellWidth)) * l.scale)
	to := round32((left + float64(column+cells)*float64(l.text.CellWidth)) * l.scale)
	width := max(to-from, 1)
	return Point{X: float32(float64(from) / l.scale), Y: l.top},
		cellMetrics{width: width, height: l.height, rule: l.rule, padX: width / 4, padY: l.padY}
}

// place appends shapes to dst at the cell's corner, scaled back to logical pixels, in the run's
// colour and in the line's layer and clip.
func (l cellLine) place(dst, shapes CellGlyphShapes, at Point, color Color) CellGlyphShapes {
	shaded := func(from Color) Color {
		return Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Alpha: color.Alpha * from.Alpha}
	}
	scale := float32(l.scale)
	for _, shape := range shapes.Rectangles {
		bounds := shape.Bounds
		dst.Rectangles = append(dst.Rectangles, Rectangle{
			Bounds: Rect{
				X:      at.X + bounds.X/scale,
				Y:      at.Y + bounds.Y/scale,
				Width:  bounds.Width / scale,
				Height: bounds.Height / scale,
			},
			Color: shaded(shape.Color), Layer: l.text.Layer, ClipOf: l.text.ClipOf, HasClip: l.text.HasClip,
		})
	}
	for _, shape := range shapes.Triangles {
		for index, point := range shape.Points {
			shape.Points[index] = Point{X: at.X + point.X/scale, Y: at.Y + point.Y/scale}
		}
		shape.Color = shaded(shape.Color)
		shape.Layer, shape.ClipOf, shape.HasClip = l.text.Layer, l.text.ClipOf, l.text.HasClip
		dst.Triangles = append(dst.Triangles, shape)
	}
	return dst
}

// deviceScale is how many device pixels one logical pixel is, 1 at the reference scale.
func (canvas *Canvas) deviceScale() float64 {
	if canvas.pixelScale <= 0 {
		return 1
	}
	return float64(canvas.pixelScale)
}

// cellLine is text as cells, for CellGlyphGeometry.
func (canvas *Canvas) cellLine(text Text) cellLine {
	scale := canvas.deviceScale()
	top := round32(float64(text.Bounds.Y) * scale)
	height := max(round32(float64(text.Bounds.Y+text.Bounds.Height)*scale)-top, 1)
	return cellLine{
		text: text, scale: scale, top: float32(float64(top) / scale),
		height: height, padY: height / 4,
		rule: max(round32(0.05*float64(text.Style.Size)*scale), 1),
	}
}

// glyphShapes is what r draws in a cell of these metrics, in device pixels from the cell's corner:
// the same for every cell of that size, whatever its place or colour. It draws into the canvas's
// scratch cell, which a line of hundreds of cells reuses without allocating.
func (canvas *Canvas) glyphShapes(metrics cellMetrics, r rune) CellGlyphShapes {
	cell := &canvas.cell
	cell.canvas, cell.cellMetrics = canvas, metrics
	cell.shapes.Rectangles, cell.shapes.Triangles = cell.shapes.Rectangles[:0], cell.shapes.Triangles[:0]
	if draw := cellGlyphDrawer(r); draw != nil {
		draw(cell, r)
	}
	return cell.shapes
}

// CellGlyphGeometry appends to dst the geometry that draws the cell glyphs of a text made by
// LabelRuns: rectangles for the strokes that are axis-aligned and snapped to whole device pixels,
// triangles for the fills, curves and slants a renderer feathers. The shapes belong to the layer of
// the text and are drawn before it: the line's value still holds the characters, and a renderer
// must not also draw them from the font; see ReplaceCellGlyphs. A text without cell glyphs adds
// nothing and does not allocate.
//
// ponytail: a cell glyph is built for every cell of every paint, though all but its place, colour
// and layer are the same for every cell of its size: an 80 cell line costs 5µs of rules, 17µs of
// corner triangles and blocks and 63µs of rounded corners and diagonals, and a whole 200x50 screen
// of `┼` 0.8ms a frame. If a profile shows that, cache the built shapes by character, cell size and
// rule — nothing else goes into them.
func (canvas *Canvas) CellGlyphGeometry(dst CellGlyphShapes, text Text) CellGlyphShapes {
	if text.CellWidth <= 0 || text.Bounds.Width <= 0 || text.Bounds.Height <= 0 {
		return dst
	}
	line := canvas.cellLine(text)
	canvas.eachCellGlyph(text, func(column, cells int32, r rune, color Color) {
		at, metrics := line.cellAt(column, cells)
		dst = line.place(dst, canvas.glyphShapes(metrics, r), at, color)
	})
	return dst
}

// eachCellGlyph calls visit with the column, the cells it takes, the character and the color of
// every cell glyph of text, in the order the line draws them.
func (canvas *Canvas) eachCellGlyph(text Text, visit func(column, cells int32, r rune, color Color)) {
	runs := canvas.TextRuns(text)
	if len(runs) == 0 {
		return
	}
	eachRun(text.Value, runs, func(index int, segment string) {
		run := runs[index]
		column := run.Column
		// A run that holds one character knows how many cells it takes; a terminal sends every
		// character that is not plain ASCII in a run of its own, so a wide block or braille
		// character is drawn across the cells it fills. In a longer run each character is one
		// cell, which is what every family drawn here needs but a wide one.
		cells := int32(1)
		if utf8.RuneCountInString(segment) == 1 {
			cells = max(run.Cells, 1)
		}
		for _, r := range segment {
			if IsCellGlyph(r) && run.Color.Alpha > 0 {
				visit(column, cells, r, run.Color)
			}
			column += cells
		}
	})
}

// eachRun calls visit with the index of each run and the bytes of value it covers, and returns the
// offset in value the runs stopped covering: a run that does not fit the value ends the walk.
func eachRun(value string, runs []TextRun, visit func(index int, segment string)) int {
	offset := 0
	for index, run := range runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(value) {
			break
		}
		visit(index, value[offset:end])
		offset = end
	}
	return offset
}

// ReplaceCellGlyphs is the value and runs of a LabelRuns line as a font should draw them:
// every cell glyph becomes a space, which holds its cell and draws nothing, since
// CellGlyphGeometry draws it. The runs are adjusted to the shorter text; the rest of the line
// keeps its columns. A line without cell glyphs is returned as it is, without allocating;
// otherwise the result is new and the arguments are left alone.
func ReplaceCellGlyphs(value string, runs []TextRun) (string, []TextRun) {
	if !hasCellGlyph(value) {
		return value, runs
	}
	text := make([]byte, 0, len(value))
	replaced := make([]TextRun, len(runs))
	copy(replaced, runs)
	rest := eachRun(value, replaced, func(index int, segment string) {
		before := len(text)
		for at := 0; at < len(segment); {
			if isCellGlyphAt(segment, at) {
				text = append(text, ' ')
				at += 3
				continue
			}
			text = append(text, segment[at])
			at++
		}
		replaced[index].Bytes = int32(len(text) - before)
	})
	return string(append(text, value[rest:]...)), replaced
}

// hasCellGlyph reports whether value holds a character CellGlyphGeometry draws.
func hasCellGlyph(value string) bool {
	for index := 0; index+2 < len(value); index++ {
		if isCellGlyphAt(value, index) {
			return true
		}
	}
	return false
}

// isCellGlyphAt reports whether value holds a cell glyph at index. Every family drawn here is a
// three-byte character that starts with E2.
func isCellGlyphAt(value string, index int) bool {
	if index+2 >= len(value) || value[index] != 0xE2 {
		return false
	}
	r, size := utf8.DecodeRuneInString(value[index:])
	return size == 3 && IsCellGlyph(r)
}
