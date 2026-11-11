package gfx

import "math"

// IsLegacyComputing reports whether r is one of the Symbols for Legacy Computing a fixed-pitch
// line draws as geometry: the sextants, smooth mosaics, eighth blocks, shades, diagonals and
// circles of U+1FB00 to U+1FBFF, and the octants and separated blocks of the supplement, U+1CC00
// to U+1CEBF. The block's unassigned holes and its two inverted diagonals stay with the font, as
// do the characters of the supplement ghostty does not draw.
func IsLegacyComputing(r rune) bool { return legacyDrawer(r) != nil }

// legacyDrawer is what draws r as geometry in a cell, or nil when the font should draw it. The
// ranges are ghostty's sprite ranges (src/font/sprite/draw/symbols_for_legacy_computing*.zig).
func legacyDrawer(r rune) func(*cellGlyph, rune) {
	switch {
	case r >= 0x1FB00 && r <= 0x1FB3B:
		return (*cellGlyph).legacySextant
	case r >= 0x1FB3C && r <= 0x1FB67:
		return (*cellGlyph).legacyMosaic
	case r >= 0x1FB68 && r <= 0x1FB6F:
		return (*cellGlyph).legacyEdgeTriangle
	case r >= 0x1FB70 && r <= 0x1FB75:
		return (*cellGlyph).legacyVerticalEighth
	case r >= 0x1FB76 && r <= 0x1FB7B:
		return (*cellGlyph).legacyHorizontalEighth
	case r >= 0x1FB7C && r <= 0x1FB97 && r != 0x1FB93:
		return (*cellGlyph).legacyBlock
	case r >= 0x1FB98 && r <= 0x1FB99:
		return (*cellGlyph).legacyHatch
	case r >= 0x1FB9A && r <= 0x1FB9F:
		return (*cellGlyph).legacyEdgeOrCorner
	case r >= 0x1FBA0 && r <= 0x1FBAE:
		return (*cellGlyph).legacyCornerDiagonals
	case r == 0x1FBAF:
		return (*cellGlyph).legacyBoxLines
	case r >= 0x1FBCE && r <= 0x1FBCF:
		return (*cellGlyph).legacyLeftBlock
	case r >= 0x1FBD0 && r <= 0x1FBDF:
		return (*cellGlyph).legacyCellDiagonal
	case r >= 0x1FBE0 && r <= 0x1FBEF:
		return (*cellGlyph).legacyCircle
	case r >= 0x1CC1B && r <= 0x1CC1E:
		return (*cellGlyph).legacyBoxPiece
	case r >= 0x1CC21 && r <= 0x1CC2F:
		return (*cellGlyph).legacySeparatedQuadrant
	case r >= 0x1CC30 && r <= 0x1CC3F:
		return (*cellGlyph).legacyCirclePiece
	case r >= 0x1CD00 && r <= 0x1CDE5:
		return (*cellGlyph).legacyOctant
	case r == 0x1CE00 || r == 0x1CE01:
		return (*cellGlyph).legacyTwoCircles
	case r == 0x1CE0B || r == 0x1CE0C:
		return (*cellGlyph).legacyHalfEllipse
	case r >= 0x1CE16 && r <= 0x1CE19:
		return (*cellGlyph).legacyVerticalPiece
	case r >= 0x1CE51 && r <= 0x1CE8F:
		return (*cellGlyph).legacySeparatedSextant
	case r >= 0x1CE90 && r <= 0x1CEAF:
		return (*cellGlyph).legacySixteenth
	}
	return nil
}

// cellFraction is a fraction along one axis of the cell.
type cellFraction struct{ numerator, denominator int32 }

// The fractions the legacy glyphs are built from, named the way ghostty names them.
var (
	fracZero          = cellFraction{0, 1}
	fracQuarter       = cellFraction{1, 4}
	fracThird         = cellFraction{1, 3}
	fracHalf          = cellFraction{1, 2}
	fracTwoThirds     = cellFraction{2, 3}
	fracThreeQuarters = cellFraction{3, 4}
	fracOne           = cellFraction{1, 1}
	eighths           = [9]cellFraction{{0, 1}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}, {6, 8}, {7, 8}, {1, 1}}
)

// fracRect fills the rectangle between two fractions of the cell at a shade, snapped to whole
// device pixels the way ghostty snaps a fill: each inner edge is measured from the far end, each
// outer edge from the near one.
func (g *cellGlyph) fracRect(x0, x1, y0, y1 cellFraction, alpha float32) {
	g.shadeRect(
		fracMin(g.width, x0.numerator, x0.denominator),
		fracMin(g.height, y0.numerator, y0.denominator),
		fracMax(g.width, x1.numerator, x1.denominator),
		fracMax(g.height, y1.numerator, y1.denominator),
		alpha)
}

// cellColumns and cellRows are the halves and thirds the sextant glyphs fill.
var (
	cellColumns = [2][2]cellFraction{{fracZero, fracHalf}, {fracHalf, fracOne}}
	cellRows    = [3][2]cellFraction{{fracZero, fracThird}, {fracThird, fracTwoThirds}, {fracTwoThirds, fracOne}}
)

// legacySextant fills the two-by-three sextants a U+1FB00 to U+1FB3B character names. The
// codepoints encode the pattern as a number with one bit per sextant, skipping the full block.
func (g *cellGlyph) legacySextant(r rune) {
	n := int32(r - 0x1FB00)
	n += n / 20
	n++
	for column, x := range cellColumns {
		for row, y := range cellRows {
			if n&(1<<uint(column+2*row)) != 0 {
				g.fracRect(x[0], x[1], y[0], y[1], 1)
			}
		}
	}
}

// legacyMosaics are the smooth mosaics U+1FB3C to U+1FB67 as the four rows of three cells
// ghostty writes them in: a '#' fills a square, a '\' or '/' marks the square a cut corner runs
// through, which the fill of the squares already gives.
var legacyMosaics = [44]string{
	`......#..##.`, `......#\.###`, `...#..#\.##.`, `...#..##.###`, `#..#..##.##.`,
	`/###########`, `./##########`, `.##.########`, `..#.########`, `.##.##.#####`,
	`..../#######`, `........#.##`, `......./####`, `.....#./#.##`, `.....#.#####`,
	`..#..#.##.##`, `##\#########`, `#\.#########`, `##.##.######`, `#..##.######`,
	`##.##.##.###`, `...#\.######`, `#########\##`, `#########.\#`, `######.##.##`,
	`######.##..#`, `###.##.##.##`, `##.#........`, `####/.......`, `##.#/.#.....`,
	`#####.#.....`, `##.##.#..#..`, `#######/....`, `###########/`, `##########/.`,
	`########.##.`, `########.#..`, `#####.##.##.`, `.##..#......`, `###.\#......`,
	`.##.\#..#...`, `###.##..#...`, `.##.##..#..#`, `######.\#...`,
}

// legacyMosaic fills the polygon a smooth mosaic draws through the nine points of its four rows
// and three columns, at the thirds of the cell's height.
func (g *cellGlyph) legacyMosaic(r rune) {
	pattern := legacyMosaics[r-0x1FB3C]
	filled := func(row, column int) bool { return pattern[row*3+column] == '#' }
	left, right := 0.0, float64(g.width)
	top, bottom := 0.0, float64(g.height)
	center := float64(g.width) / 2
	upper, lower := bottom/3, 2*bottom/3
	var points [10]Point
	count := 0
	add := func(x, y float64) {
		points[count] = Point{X: float32(x), Y: float32(y)}
		count++
	}
	if filled(0, 0) {
		add(left, top)
	}
	if filled(1, 0) && (!filled(0, 0) || !filled(2, 0)) {
		add(left, upper)
	}
	if filled(2, 0) && (!filled(1, 0) || !filled(3, 0)) {
		add(left, lower)
	}
	if filled(3, 0) {
		add(left, bottom)
	}
	if filled(3, 1) && (!filled(3, 0) || !filled(3, 2)) {
		add(center, bottom)
	}
	if filled(3, 2) {
		add(right, bottom)
	}
	if filled(2, 2) && (!filled(3, 2) || !filled(1, 2)) {
		add(right, lower)
	}
	if filled(1, 2) && (!filled(2, 2) || !filled(0, 2)) {
		add(right, upper)
	}
	if filled(0, 2) {
		add(right, top)
	}
	if filled(0, 1) && (!filled(0, 2) || !filled(0, 0)) {
		add(center, top)
	}
	g.fillPolygon(points[:count])
}

// cellCorners are the cell's corners, in turn from its top left, clockwise on screen. The edges the
// legacy glyphs name follow the same turn: the left edge starts at the last corner and the top edge
// at the first, so edge n runs from corner n+3 to corner n, all four modulo four.
func (g cellGlyph) cellCorners() [4]Point {
	width, height := float32(g.width), float32(g.height)
	return [4]Point{{X: 0, Y: 0}, {X: width, Y: 0}, {X: width, Y: height}, {X: 0, Y: height}}
}

// legacyEdgeTriangle draws the triangles and their negatives of U+1FB68 to U+1FB6F: the four
// halves of the cell a line from its middle to one of its edges cuts off, filled or left as the
// rest of the cell.
func (g *cellGlyph) legacyEdgeTriangle(r rune) {
	corners := g.cellCorners()
	edge := int((r - 0x1FB68) % 4)
	at := Point{
		X: float32(math.Round(float64(g.width) / 2)),
		Y: float32(math.Round(float64(g.height) / 2)),
	}
	if r < 0x1FB6C {
		// The rest of the cell is the pentagon the triangle's edge leaves: the edge's own corners,
		// then the two the turn reaches before it comes back.
		g.fillPolygon([]Point{at,
			corners[edge], corners[(edge+1)%4], corners[(edge+2)%4], corners[(edge+3)%4]})
		return
	}
	g.fill(at, corners[(edge+3)%4], corners[edge])
}

// legacyVerticalEighth fills one eighth of the cell's width, full height, growing from the left.
func (g *cellGlyph) legacyVerticalEighth(r rune) {
	n := r - 0x1FB70 + 1
	g.fracRect(eighths[n], eighths[n+1], fracZero, fracOne, 1)
}

// legacyHorizontalEighth fills one eighth of the cell's height, full width, growing from the top.
func (g *cellGlyph) legacyHorizontalEighth(r rune) {
	n := r - 0x1FB76 + 1
	g.fracRect(fracZero, fracOne, eighths[n], eighths[n+1], 1)
}

// quarterSteps are the fractions the quarter and eighth blocks of U+1FB82 to U+1FB8B step by.
var quarterSteps = [5]cellFraction{{1, 4}, {3, 8}, {5, 8}, fracThreeQuarters, {7, 8}}

// legacyBlock draws the blocks, shades and checkerboards of U+1FB7C to U+1FB97.
func (g *cellGlyph) legacyBlock(r rune) {
	switch r {
	case 0x1FB7C: // left and lower one eighth
		g.block(cellBlock{width: eighths[1]}, 1)
		g.block(cellBlock{vertical: alignEnd, height: eighths[1]}, 1)
	case 0x1FB7D: // left and upper one eighth
		g.block(cellBlock{width: eighths[1]}, 1)
		g.block(cellBlock{height: eighths[1]}, 1)
	case 0x1FB7E: // right and upper one eighth
		g.block(cellBlock{horizontal: alignEnd, width: eighths[1]}, 1)
		g.block(cellBlock{height: eighths[1]}, 1)
	case 0x1FB7F: // right and lower one eighth
		g.block(cellBlock{horizontal: alignEnd, width: eighths[1]}, 1)
		g.block(cellBlock{vertical: alignEnd, height: eighths[1]}, 1)
	case 0x1FB80: // upper and lower one eighth
		g.block(cellBlock{height: eighths[1]}, 1)
		g.block(cellBlock{vertical: alignEnd, height: eighths[1]}, 1)
	case 0x1FB81: // the four horizontal eighths that are not adjacent
		for _, n := range [4]int32{0, 2, 4, 7} {
			g.fracRect(fracZero, fracOne, eighths[n], eighths[n+1], 1)
		}
	case 0x1FB82, 0x1FB83, 0x1FB84, 0x1FB85, 0x1FB86: // upper quarter to seven eighths
		g.block(cellBlock{height: quarterSteps[r-0x1FB82]}, 1)
	case 0x1FB87, 0x1FB88, 0x1FB89, 0x1FB8A, 0x1FB8B: // right quarter to seven eighths
		g.block(cellBlock{horizontal: alignEnd, width: quarterSteps[r-0x1FB87]}, 1)
	case 0x1FB8C: // left half, medium shade
		g.block(cellBlock{width: fracHalf}, 0.5)
	case 0x1FB8D: // right half, medium shade
		g.block(cellBlock{horizontal: alignEnd, width: fracHalf}, 0.5)
	case 0x1FB8E: // upper half, medium shade
		g.block(cellBlock{height: fracHalf}, 0.5)
	case 0x1FB8F: // lower half, medium shade
		g.block(cellBlock{vertical: alignEnd, height: fracHalf}, 0.5)
	case 0x1FB90: // medium shade
		g.block(cellBlock{}, 0.5)
	case 0x1FB91: // medium shade and the upper half
		g.block(cellBlock{}, 0.5)
		g.block(cellBlock{height: fracHalf}, 1)
	case 0x1FB92: // medium shade and the lower half
		g.block(cellBlock{}, 0.5)
		g.block(cellBlock{vertical: alignEnd, height: fracHalf}, 1)
	case 0x1FB94: // medium shade and the right half
		g.block(cellBlock{}, 0.5)
		g.block(cellBlock{horizontal: alignEnd, width: fracHalf}, 1)
	case 0x1FB95, 0x1FB96: // checkerboard, either parity
		g.checkerboard(r - 0x1FB95)
	case 0x1FB97: // two horizontal quarters
		g.rect(0, g.height/4, g.width, g.height/2)
		g.rect(0, g.height*3/4, g.width, g.height)
	}
}

// checkerboard fills every other square of a four-column board whose rows are as tall as the cell
// makes them square, starting with the square at parity.
func (g *cellGlyph) checkerboard(parity rune) {
	columns := int32(4)
	rows := int32(math.Round(4 * float64(g.height) / float64(g.width)))
	if rows < 1 {
		rows = 1
	}
	for x := int32(0); x < columns; x++ {
		for y := int32(0); y < rows; y++ {
			if (x+y)%2 != parity {
				continue
			}
			g.rect(
				g.width*x/columns, g.height*y/rows,
				g.width*(x+1)/columns, g.height*(y+1)/rows)
		}
	}
}

// legacyHatch fills the cell with diagonal lines a rule wide: falling when r is U+1FB98, rising
// when it is U+1FB99.
func (g *cellGlyph) legacyHatch(r rune) {
	width, height := float64(g.width), float64(g.height)
	count := max(int32(width/(2*math.Max(float64(g.rule), 1))), 1)
	stride := math.Round(width / float64(count))
	// A rising line is a falling one mirrored across the cell.
	rising := r == 0x1FB99
	mark := len(g.shapes.Triangles)
	for index := -count; index <= count; index++ {
		shift := float64(index) * stride
		if rising {
			shift = -shift
		}
		// Each line runs the height of the cell and is cut to it, rather than being drawn only
		// between the points it crosses the cell's sides at: a butt cap there would leave the
		// corner the line enters by empty.
		x0, x1 := shift, shift+width
		if rising {
			x0, x1 = width-x0, width-x1
		}
		g.strokeLine(float32(x0), 0, float32(x1), float32(height))
	}
	g.clipToCell(mark)
}

// shadedCorners are the corners U+1FB9C to U+1FB9F shade, in order.
var shadedCorners = [4]shapeCorner{cornerTopLeft, cornerTopRight, cornerBottomRight, cornerBottomLeft}

// legacyEdgeOrCorner draws the two doubled edge triangles and the four shaded corner triangles of
// U+1FB9A to U+1FB9F.
func (g *cellGlyph) legacyEdgeOrCorner(r rune) {
	width, height := float64(g.width), float64(g.height)
	center := Point{X: float32(math.Round(width / 2)), Y: float32(math.Round(height / 2))}
	corners := g.cellCorners()
	switch r {
	case 0x1FB9A: // top and bottom
		g.fill(center, corners[0], corners[1])
		g.fill(center, corners[2], corners[3])
	case 0x1FB9B: // left and right
		g.fill(center, corners[3], corners[0])
		g.fill(center, corners[1], corners[2])
	case 0x1FB9C, 0x1FB9D, 0x1FB9E, 0x1FB9F: // shaded corner triangles
		points := g.triangleCorners(shadedCorners[r-0x1FB9C])
		g.fillShade(points[0], points[1], points[2], 0.5)
	}
}

// legacyCornerDiagonalLines are the four corner diagonals of U+1FBA0 to U+1FBAE, as the corners
// they meet: top left, top right, bottom left, bottom right.
var legacyCornerDiagonalLines = [15]uint8{
	1, 2, 4, 8, 1 | 4, 2 | 8, 4 | 8, 1 | 2, 1 | 8, 2 | 4,
	2 | 4 | 8, 1 | 4 | 8, 1 | 2 | 8, 1 | 2 | 4, 1 | 2 | 4 | 8,
}

// legacyCornerDiagonals draws lines from the middle of the cell's top or bottom edge to the middle
// of its left or right edge, one for each corner the character names.
func (g *cellGlyph) legacyCornerDiagonals(r rune) {
	width, height := float32(g.width), float32(g.height)
	centerX := float32(g.width/2 + g.width%2)
	centerY := float32(g.height/2 + g.height%2)
	corners := legacyCornerDiagonalLines[r-0x1FBA0]
	if corners&1 != 0 {
		g.strokeLine(centerX, 0, 0, centerY)
	}
	if corners&2 != 0 {
		g.strokeLine(centerX, 0, width, centerY)
	}
	if corners&4 != 0 {
		g.strokeLine(centerX, height, 0, centerY)
	}
	if corners&8 != 0 {
		g.strokeLine(centerX, height, width, centerY)
	}
}

// legacyBoxLines is the heavy vertical and light horizontal cross of U+1FBAF, drawn by the same
// rules as a Box Drawing junction so it meets its neighbours.
func (g *cellGlyph) legacyBoxLines(rune) {
	g.drawLines([4]boxStyle{boxHeavy, boxLight, boxHeavy, boxLight})
}

// legacyLeftBlock fills two thirds or one third of the cell's width from the left: U+1FBCE and
// U+1FBCF.
func (g *cellGlyph) legacyLeftBlock(r rune) {
	right := fracTwoThirds
	if r == 0x1FBCF {
		right = fracThird
	}
	g.fracRect(fracZero, right, fracZero, fracOne, 1)
}

// cellPoint is a point of the cell the legacy diagonals run between: a third of the way along an
// axis, at its ends.
type cellPoint uint8

const (
	pointTopLeft cellPoint = iota
	pointTopCenter
	pointTopRight
	pointMiddleLeft
	pointMiddleCenter
	pointMiddleRight
	pointBottomLeft
	pointBottomCenter
	pointBottomRight
)

// point is where one of the nine points lies in the cell.
func (g cellGlyph) point(at cellPoint) Point {
	x, y := float32(0), float32(0)
	switch at % 3 {
	case 1:
		x = float32(g.width) / 2
	case 2:
		x = float32(g.width)
	}
	switch at / 3 {
	case 1:
		y = float32(g.height) / 2
	case 2:
		y = float32(g.height)
	}
	return Point{X: x, Y: y}
}

// legacyDiagonals are the lines U+1FBD0 to U+1FBDF draw: one, or a bent pair through the middle.
var legacyDiagonals = [16][2][2]cellPoint{
	{{pointMiddleRight, pointBottomLeft}}, {{pointTopRight, pointMiddleLeft}},
	{{pointTopLeft, pointMiddleRight}}, {{pointMiddleLeft, pointBottomRight}},
	{{pointTopLeft, pointBottomCenter}}, {{pointTopCenter, pointBottomRight}},
	{{pointTopRight, pointBottomCenter}}, {{pointTopCenter, pointBottomLeft}},
	{{pointTopLeft, pointMiddleCenter}, {pointMiddleCenter, pointTopRight}},
	{{pointTopRight, pointMiddleCenter}, {pointMiddleCenter, pointBottomRight}},
	{{pointBottomLeft, pointMiddleCenter}, {pointMiddleCenter, pointBottomRight}},
	{{pointTopLeft, pointMiddleCenter}, {pointMiddleCenter, pointBottomLeft}},
	{{pointTopLeft, pointBottomCenter}, {pointBottomCenter, pointTopRight}},
	{{pointTopRight, pointMiddleLeft}, {pointMiddleLeft, pointBottomRight}},
	{{pointBottomLeft, pointTopCenter}, {pointTopCenter, pointBottomRight}},
	{{pointTopLeft, pointMiddleRight}, {pointMiddleRight, pointBottomLeft}},
}

// legacyCellDiagonal strokes one or two lines between the points of the cell U+1FBD0 to U+1FBDF
// name.
func (g *cellGlyph) legacyCellDiagonal(r rune) {
	for _, segment := range legacyDiagonals[r-0x1FBD0] {
		from, to := g.point(segment[0]), g.point(segment[1])
		g.strokeLine(from.X, from.Y, to.X, to.Y)
	}
}

// legacyCircle draws the circles, half circles and quarter blocks of U+1FBE0 to U+1FBEF.
func (g *cellGlyph) legacyCircle(r rune) {
	width, height := float64(g.width), float64(g.height)
	radius := 0.5 * math.Min(width, height)
	switch r {
	case 0x1FBE0: // white circle on the top edge
		g.circleSector(width/2, 0, radius, 0, math.Pi, false)
	case 0x1FBE1: // right edge
		g.circleSector(width, height/2, radius, math.Pi/2, 3*math.Pi/2, false)
	case 0x1FBE2: // bottom edge
		g.circleSector(width/2, height, radius, math.Pi, 2*math.Pi, false)
	case 0x1FBE3: // left edge
		g.circleSector(0, height/2, radius, -math.Pi/2, math.Pi/2, false)
	case 0x1FBE4: // upper middle quarter
		g.block(cellBlock{horizontal: alignCenter, width: fracHalf, height: fracHalf}, 1)
	case 0x1FBE5: // lower middle quarter
		g.block(cellBlock{horizontal: alignCenter, vertical: alignEnd, width: fracHalf, height: fracHalf}, 1)
	case 0x1FBE6: // middle left quarter
		g.block(cellBlock{vertical: alignCenter, width: fracHalf, height: fracHalf}, 1)
	case 0x1FBE7: // middle right quarter
		g.block(cellBlock{horizontal: alignEnd, vertical: alignCenter, width: fracHalf, height: fracHalf}, 1)
	case 0x1FBE8: // filled circle on the top edge
		g.circleSector(width/2, 0, radius, 0, math.Pi, true)
	case 0x1FBE9: // right edge
		g.circleSector(width, height/2, radius, math.Pi/2, 3*math.Pi/2, true)
	case 0x1FBEA: // bottom edge
		g.circleSector(width/2, height, radius, math.Pi, 2*math.Pi, true)
	case 0x1FBEB: // left edge
		g.circleSector(0, height/2, radius, -math.Pi/2, math.Pi/2, true)
	case 0x1FBEC: // top right quarter
		g.circleSector(width, 0, radius, math.Pi/2, math.Pi, true)
	case 0x1FBED: // bottom left quarter
		g.circleSector(0, height, radius, -math.Pi/2, 0, true)
	case 0x1FBEE: // bottom right quarter
		g.circleSector(width, height, radius, math.Pi, 3*math.Pi/2, true)
	case 0x1FBEF: // top left quarter
		g.circleSector(0, 0, radius, 0, math.Pi/2, true)
	}
}

// circleSector draws the part of the circle of radius r around (cx, cy) between two angles, which
// turn clockwise on screen. Filled, it is the sector from the centre; outlined, it is the arc
// stroked one rule wide, moved in by half the rule so the ink lies inside the circle's edge.
func (g *cellGlyph) circleSector(cx, cy, r, start, end float64, filled bool) {
	var scratch [9]PathCommand
	curve := arcCommands(scratch[:0], circle(cx, cy, r), start, end)
	if filled {
		var commands [10]PathCommand
		commands[0] = MoveTo(float32(cx), float32(cy))
		commands[1] = LineTo(curve[0].Point.X, curve[0].Point.Y)
		count := 2 + copy(commands[2:], curve[1:])
		commands[count] = ClosePath()
		g.fillPath(NewPath(float32(g.width), float32(g.height), commands[:count+1]...), PathFillNonZero)
		return
	}
	half := float64(g.rule) / 2
	if r <= half {
		return
	}
	var outline [9]PathCommand
	g.stroke(NewPath(float32(g.width), float32(g.height),
		arcCommands(outline[:0], circle(cx, cy, r-half), start, end)...))
}

// arc is the ellipse an arc is a piece of: where it turns, how far its curve reaches, and how far
// the controls of the cubics that approximate it reach. The two are the same radius for a circle; a
// sprite's arc is drawn a rule's half inside its box while its controls keep the box's own radii,
// which is how ghostty approximates one.
type arc struct {
	x, y               float64
	radiusX, radiusY   float64
	controlX, controlY float64
}

// circle is the arc of a circle of radius r around (cx, cy).
func circle(cx, cy, r float64) arc {
	return arc{x: cx, y: cy, radiusX: r, radiusY: r, controlX: r, controlY: r}
}

// arcCommands appends the arc of curve between two angles as cubic segments, each no wider than a
// quarter turn. The angles turn clockwise on screen.
func arcCommands(dst []PathCommand, curve arc, start, end float64) []PathCommand {
	segments := int(math.Ceil((end - start) / (math.Pi / 2)))
	if segments < 1 {
		segments = 1
	}
	step := (end - start) / float64(segments)
	// The distance from an arc's ends to its control points, as a share of the radius.
	bend := 4.0 / 3.0 * math.Tan(step/4)
	dst = append(dst, MoveTo(
		float32(curve.x+curve.radiusX*math.Cos(start)),
		float32(curve.y+curve.radiusY*math.Sin(start))))
	for index := 0; index < segments; index++ {
		from, to := start+float64(index)*step, start+float64(index+1)*step
		fromX, fromY := curve.x+curve.radiusX*math.Cos(from), curve.y+curve.radiusY*math.Sin(from)
		toX, toY := curve.x+curve.radiusX*math.Cos(to), curve.y+curve.radiusY*math.Sin(to)
		dst = append(dst, CubicTo(
			float32(fromX-curve.controlX*bend*math.Sin(from)),
			float32(fromY+curve.controlY*bend*math.Cos(from)),
			float32(toX+curve.controlX*bend*math.Sin(to)),
			float32(toY-curve.controlY*bend*math.Cos(to)),
			float32(toX), float32(toY)))
	}
	return dst
}
