package gfx

import (
	"math"
	"unicode/utf8"
)

// IsCellGlyph reports whether r is a character a fixed-pitch line draws as geometry in its cell
// instead of from the font: the Block Elements, Box Drawing, Braille, the corner shapes of
// Geometric Shapes, Powerline and the Symbols for Legacy Computing a cell fills. The line's text
// still holds the character, for accessibility and for copying; see ReplaceCellGlyphs.
func IsCellGlyph(r rune) bool { return cellGlyphDrawer(r) != nil }

// cellGlyphDrawer is what draws r as geometry in a cell, or nil when the font should draw it. The
// families small enough for one line are named here; the blocks that carry a table of their own
// name themselves in their own drawer, beside it. Adding a character is a method on cellGlyph and
// a case in one of the two.
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
	if draw := powerlineDrawer(r); draw != nil {
		return draw
	}
	return legacyDrawer(r)
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
	shapes     CellGlyphShapes
	clipSource []Triangle // scratch of the glyphs that cut their shapes to the cell
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
	// It is the face's underline thickness (MonoCellMetrics.Underline) at the line's scale,
	// rounded up to the whole device pixel a stroke covers: the weight the font itself draws
	// an underline with, so a box drawing or block character drawn as geometry weighs what
	// the font's own glyphs around it weigh, as in Ghostty (src/font/Metrics.zig). Rounding
	// down would make a stroke thinner, relative to the glyphs, at every scale that leaves
	// it a fraction of a pixel; the reference thickness stands in when no engine measured
	// the face.
	rule int32
}

// rect fills the rectangle between two points of the cell, both in device pixels from its corner,
// at the text's own opacity.
func (g *cellGlyph) rect(x0, y0, x1, y1 int32) {
	g.shadeRect(x0, y0, x1, y1, 1)
}

// shadeRect fills a rectangle of the cell at a shade of the text's opacity. A shape may reach a
// quarter of a cell past the cell, the room a sprite has in ghostty, so a diagonal can meet the
// line in the cell it points at; an empty rectangle adds nothing, so a shape too small for the cell
// disappears instead of inverting.
func (g *cellGlyph) shadeRect(x0, y0, x1, y1 int32, alpha float32) {
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
		Color: Color{Alpha: alpha},
	})
}

// fill fills the triangle through three points of the cell, in device pixels from its corner.
func (g *cellGlyph) fill(a, b, c Point) {
	g.fillShade(a, b, c, 1)
}

// fillShade fills a triangle at a shade of the text's opacity, the way a dither-free shading
// character fills its cell.
func (g *cellGlyph) fillShade(a, b, c Point, alpha float32) {
	g.shapes.Triangles = append(g.shapes.Triangles, Triangle{
		Points:   [3]Point{a, b, c},
		Color:    Color{Alpha: alpha},
		EdgeMask: triangleAllEdges,
	})
}

// fillPolygon fills a closed path through points of the cell, in device pixels from its corner.
// A shape with fewer than three points adds nothing.
func (g *cellGlyph) fillPolygon(points []Point) {
	if len(points) < 3 {
		return
	}
	var fixed [12]PathCommand
	commands := fixed[:0]
	commands = append(commands, MoveTo(points[0].X, points[0].Y))
	for _, point := range points[1:] {
		commands = append(commands, LineTo(point.X, point.Y))
	}
	commands = append(commands, ClosePath())
	g.fillPath(NewPath(float32(g.width), float32(g.height), commands...), PathFillNonZero)
}

// cellAlign is where a block sits along one axis of its cell.
type cellAlign int8

const (
	alignStart cellAlign = iota
	alignCenter
	alignEnd
)

// fracMax is where a fraction of the way along extent lands, in whole device pixels, rounded to
// the nearest. It is Ghostty's Fraction.max.
func fracMax(extent, numerator, denominator int32) int32 {
	return int32(math.Round(float64(extent) * float64(numerator) / float64(denominator)))
}

// fracMin is where the fraction one step before a fraction lands, measured from the far end the
// way ghostty measures it, so the two pieces a line splits a cell into keep the odd pixel between
// them rather than one of them taking it.
func fracMin(extent, numerator, denominator int32) int32 {
	return extent - fracMax(extent, denominator-numerator, denominator)
}

// cellBlock is a block of the cell: a fraction of its width and height, flush with an edge or
// centered. The zero block is the whole cell.
type cellBlock struct {
	horizontal, vertical cellAlign
	width, height        cellFraction
}

// blockRect is the rectangle of the cell a block fills, with the odd pixel kept on the end side
// the way ghostty places a block.
func (g *cellGlyph) blockRect(block cellBlock) (x0, y0, x1, y1 int32) {
	width, height := block.width.extent(g.width), block.height.extent(g.height)
	x, y := int32(0), int32(0)
	switch block.horizontal {
	case alignCenter:
		x = (g.width - width) / 2
	case alignEnd:
		x = g.width - width
	}
	switch block.vertical {
	case alignCenter:
		y = (g.height - height) / 2
	case alignEnd:
		y = g.height - height
	}
	return x, y, x + width, y + height
}

// extent is where a fraction of an axis lands, measured from its near edge; the zero fraction is
// the whole axis, so a block that names only one side fills the other.
func (f cellFraction) extent(size int32) int32 {
	if f.denominator == 0 {
		return size
	}
	return fracMax(size, f.numerator, f.denominator)
}

// block fills a block of the cell at a shade: the eighths, quarters and halves of a legacy
// character that are not block elements of their own.
func (g *cellGlyph) block(block cellBlock, alpha float32) {
	x0, y0, x1, y1 := g.blockRect(block)
	g.shadeRect(x0, y0, x1, y1, alpha)
}

// strokeLine strokes the line between two points of the cell, one rule wide, with butt ends.
func (g *cellGlyph) strokeLine(x0, y0, x1, y1 float32) {
	g.stroke(NewPath(float32(g.width), float32(g.height), MoveTo(x0, y0), LineTo(x1, y1)))
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
			// The geometry is part of the line's own paint: it shares the
			// label's order and is drawn just beneath its glyphs.
			Order: l.text.Order,
		})
	}
	for _, shape := range shapes.Triangles {
		for index, point := range shape.Points {
			shape.Points[index] = Point{X: at.X + point.X/scale, Y: at.Y + point.Y/scale}
		}
		shape.Color = shaded(shape.Color)
		shape.Layer, shape.ClipOf, shape.HasClip = l.text.Layer, l.text.ClipOf, l.text.HasClip
		shape.Order = l.text.Order
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
	_, _, _, underline := canvas.monoCell(text.Style.Size, text.Style.MonoFace)
	return cellLine{
		text: text, scale: scale, top: float32(float64(top) / scale),
		height: height, padY: height / 4,
		rule: max(ceil32(float64(underline)*scale), 1),
	}
}

// cellGlyphCacheEntry is one built cell glyph, kept while the same character keeps being
// drawn at the same metrics.
type cellGlyphCacheEntry struct {
	r       rune
	metrics cellMetrics
	shapes  CellGlyphShapes
}

// cellCacheLimit bounds the built cell glyphs a canvas keeps: box drawing, blocks,
// braille and powerline together are a few hundred characters, and a screen shows
// a few dozen of them at one or two metrics.
const cellCacheLimit = 64

// glyphShapes is what r draws in a cell of these metrics, in device pixels from the cell's corner:
// the same for every cell of that size, whatever its place or colour. It draws into the canvas's
// scratch cell, which a line of hundreds of cells reuses without allocating. The built shapes are
// cached by character and metrics — a cell glyph is the character and the metrics alone, so a
// whole screen of the same table glyph rebuilds one of them — and the cache's entries are only
// read: place appends copies.
func (canvas *Canvas) glyphShapes(metrics cellMetrics, r rune) CellGlyphShapes {
	for i := range canvas.cellCache {
		if entry := &canvas.cellCache[i]; entry.r == r && entry.metrics == metrics {
			return entry.shapes
		}
	}
	cell := &canvas.cell
	cell.canvas, cell.cellMetrics = canvas, metrics
	cell.shapes.Rectangles, cell.shapes.Triangles = cell.shapes.Rectangles[:0], cell.shapes.Triangles[:0]
	if draw := cellGlyphDrawer(r); draw != nil {
		draw(cell, r)
	}
	var entry cellGlyphCacheEntry
	entry.r, entry.metrics = r, metrics
	entry.shapes.Rectangles = append(entry.shapes.Rectangles, cell.shapes.Rectangles...)
	entry.shapes.Triangles = append(entry.shapes.Triangles, cell.shapes.Triangles...)
	if len(canvas.cellCache) >= cellCacheLimit {
		// The working set of a screen is a few dozen characters at one or two
		// metrics; past the limit the whole cache starts over rather than
		// growing with every character a document ever showed.
		canvas.cellCache = canvas.cellCache[:0]
	}
	canvas.cellCache = append(canvas.cellCache, entry)
	return canvas.cellCache[len(canvas.cellCache)-1].shapes
}

// CellGlyphGeometry appends to dst the geometry that draws the cell glyphs of a text made by
// LabelRuns: rectangles for the strokes that are axis-aligned and snapped to whole device pixels,
// triangles for the fills, curves and slants a renderer feathers. The shapes belong to the layer of
// the text and are drawn before it: the line's value still holds the characters, and a renderer
// must not also draw them from the font; see ReplaceCellGlyphs. A text without cell glyphs adds
// nothing and does not allocate.
//
// The built shapes are cached by character and metrics (see glyphShapes), so a paint that shows
// the same glyphs as the last pays a lookup per cell, not a rebuild: an 80 cell line of rounded
// corners and diagonals was 63µs to rebuild and a whole 200x50 screen of `┼` 0.8ms a frame.
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
				_, size := utf8.DecodeRuneInString(segment[at:])
				text = append(text, ' ')
				at += size
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
	for index := 0; index < len(value); index++ {
		if isCellGlyphAt(value, index) {
			return true
		}
	}
	return false
}

// isCellGlyphAt reports whether value holds a cell glyph at index. The lowest lead byte any cell
// glyph starts with is 0xE2, of Box Drawing, so anything below it is skipped without decoding.
func isCellGlyphAt(value string, index int) bool {
	if value[index] < 0xE2 {
		return false
	}
	r, size := utf8.DecodeRuneInString(value[index:])
	return size > 1 && IsCellGlyph(r)
}
