package gfx

import (
	"math"
	"strings"
)

// legacyOctants is legacyOctantSpec as one bit per eighth of the cell.
var legacyOctants = parseOctants()

func parseOctants() [0xE6]uint8 {
	var octants [0xE6]uint8
	index := 0
	for _, entry := range strings.Fields(legacyOctantSpec) {
		for _, eighth := range entry {
			octants[index] |= 1 << (eighth - '1')
		}
		index++
	}
	return octants
}

// octantRows are the quarters of the cell an octant fills vertically.
var octantRows = [4][2]cellFraction{
	{fracZero, fracQuarter}, {fracQuarter, fracHalf}, {fracHalf, fracThreeQuarters}, {fracThreeQuarters, fracOne},
}

// legacyOctant fills the eighths of the cell U+1CD00 to U+1CDE5 name, two columns by four rows.
func (g *cellGlyph) legacyOctant(r rune) {
	eighths := legacyOctants[r-0x1CD00]
	for index := 0; index < 8; index++ {
		if eighths&(1<<uint(index)) != 0 {
			column, row := cellColumns[index%2], octantRows[index/2]
			g.fracRect(column[0], column[1], row[0], row[1], 1)
		}
	}
}

// separatedColumns are where the two columns of the separated quadrant and sextant glyphs are: two
// blocks with a gap of a twelfth of the cell's width around and between them, the odd pixel of the
// width kept in the gap.
func (g *cellGlyph) separatedColumns() (gap, width, right int32) {
	gap = max(g.width/12, 1)
	midGap := gap*2 + g.width%2
	width = (g.width - gap*2 - midGap) / 2
	return gap, width, gap + width + midGap
}

// legacySeparatedQuadrant fills the separated quadrants U+1CC21 to U+1CC2F: four blocks of the
// cell, each with a gap of a twelfth of its width around it.
func (g *cellGlyph) legacySeparatedQuadrant(r rune) {
	bits := uint8(r - 0x1CC20)
	gap, width, right := g.separatedColumns()
	midGap := gap*2 + g.height%2
	height := (g.height - gap*2 - midGap) / 2
	bottom := gap + height + midGap
	for index := 0; index < 4; index++ {
		if bits&(1<<uint(index)) == 0 {
			continue
		}
		x, y := gap, gap
		if index&1 != 0 {
			x = right
		}
		if index&2 != 0 {
			y = bottom
		}
		g.rect(x, y, x+width, y+height)
	}
}

// circlePieceSpec is the ellipse a twelfth or quarter circle is an arc of: where its bounding box
// starts in cells, its size in cells, and the corner of the arc that lies in this cell.
type circlePieceSpec struct {
	x, y, width, height float64
	corner              shapeCorner
}

// legacyCirclePieces are the arcs of U+1CC30 to U+1CC3F.
var legacyCirclePieces = [16]circlePieceSpec{
	{0, 0, 2, 2, cornerTopLeft}, {1, 0, 2, 2, cornerTopLeft},
	{2, 0, 2, 2, cornerTopRight}, {3, 0, 2, 2, cornerTopRight},
	{0, 1, 2, 2, cornerTopLeft}, {0, 0, 1, 1, cornerTopLeft},
	{1, 0, 1, 1, cornerTopRight}, {3, 1, 2, 2, cornerTopRight},
	{0, 2, 2, 2, cornerBottomLeft}, {0, 1, 1, 1, cornerBottomLeft},
	{1, 1, 1, 1, cornerBottomRight}, {3, 2, 2, 2, cornerBottomRight},
	{0, 3, 2, 2, cornerBottomLeft}, {1, 3, 2, 2, cornerBottomLeft},
	{2, 3, 2, 2, cornerBottomRight}, {3, 3, 2, 2, cornerBottomRight},
}

// legacyCirclePiece draws one twelfth or quarter of a circle, U+1CC30 to U+1CC3F: a quarter of an
// ellipse whose centre lies in a neighbouring cell or further away, so only the part inside this
// cell is kept.
func (g *cellGlyph) legacyCirclePiece(r rune) {
	piece := legacyCirclePieces[r-0x1CC30]
	mark := len(g.shapes.Triangles)
	g.circlePiece(piece.x, piece.y, piece.width, piece.height, piece.corner)
	g.clipToCell(mark)
}

// circlePiece strokes the quarter of the ellipse of a box width by height cells at x and y cells
// from the cell's corner, the quarter the corner names. The arc is moved in by half a rule so its
// ink lies inside the box, and the part of it outside the cell is cut away.
func (g *cellGlyph) circlePiece(x, y, width, height float64, corner shapeCorner) {
	cellWidth, cellHeight := float64(g.width), float64(g.height)
	// The ellipse is inscribed in its box, around the box's far corner.
	centerX, centerY := cellWidth*(width-x), cellHeight*(height-y)
	inside := float64(g.rule) / 2
	radiusX, radiusY := cellWidth*width-inside, cellHeight*height-inside
	if radiusX <= 0 || radiusY <= 0 {
		return
	}
	// The quarter runs between two axes of the ellipse, the top and left one first.
	start, end := -math.Pi/2, -math.Pi
	switch corner {
	case cornerTopRight:
		end = 0
	case cornerBottomLeft:
		start, end = math.Pi, math.Pi/2
	case cornerBottomRight:
		start, end = 0, math.Pi/2
	}
	var scratch [9]PathCommand
	g.stroke(NewPath(float32(cellWidth), float32(cellHeight),
		arcCommands(scratch[:0], arc{
			x: centerX, y: centerY,
			radiusX: radiusX, radiusY: radiusY,
			controlX: cellWidth * width, controlY: cellHeight * height,
		}, start, end)...))
}

// clipToCell keeps only the part of the triangles added since mark that lies inside the cell's
// rectangle: a glyph that draws a piece of a shape centred in another cell, or a line drawn past
// the cell and cut to it. The source and the clipped points live on the canvas's scratch cell, so a
// line of these glyphs does not allocate.
func (g *cellGlyph) clipToCell(mark int) {
	source := append(g.clipSource[:0], g.shapes.Triangles[mark:]...)
	g.clipSource = source
	kept := g.shapes.Triangles[:mark]
	for _, triangle := range source {
		if triangleInsideCell(triangle.Points, g.width, g.height) {
			kept = append(kept, triangle)
			continue
		}
		points, count := clipTriangle(triangle.Points, g.width, g.height)
		for index := 2; index < count; index++ {
			clipped := triangle
			clipped.Points = [3]Point{points[0], points[index-1], points[index]}
			kept = append(kept, clipped)
		}
	}
	g.shapes.Triangles = kept
}

// triangleInsideCell reports whether a triangle lies entirely inside the cell's rectangle.
func triangleInsideCell(points [3]Point, width, height int32) bool {
	for _, point := range points {
		if point.X < 0 || point.Y < 0 || point.X > float32(width) || point.Y > float32(height) {
			return false
		}
	}
	return true
}

// cellSide is one side of a cell as the direction that points into it, and where the side lies
// along that direction: a point is inside the side when its depth is not negative.
type cellSide struct{ x, y, offset float64 }

// depth is how far inside the side a point is.
func (side cellSide) depth(at Point) float64 {
	return side.x*float64(at.X) + side.y*float64(at.Y) - side.offset
}

// cross is where the segment from one point to another leaves the side, one of them on each side.
func (side cellSide) cross(from, to Point) Point {
	ratio := side.depth(from) / (side.depth(from) - side.depth(to))
	return Point{
		X: float32(float64(from.X) + ratio*(float64(to.X)-float64(from.X))),
		Y: float32(float64(from.Y) + ratio*(float64(to.Y)-float64(from.Y))),
	}
}

// clipTriangle cuts a triangle to the cell's rectangle and returns the polygon inside, in order. A
// clipped convex polygon has at most seven corners.
func clipTriangle(points [3]Point, width, height int32) ([7]Point, int) {
	sides := [4]cellSide{
		{x: 1, offset: 0}, {y: 1, offset: 0},
		{x: -1, offset: -float64(width)}, {y: -1, offset: -float64(height)},
	}
	current := [7]Point{points[0], points[1], points[2]}
	next := [7]Point{}
	count := 3
	for _, side := range sides {
		nextCount := 0
		for index := 0; index < count; index++ {
			previous, point := current[(index+count-1)%count], current[index]
			previousDepth, depth := side.depth(previous), side.depth(point)
			switch {
			case previousDepth >= 0 && depth >= 0:
				next[nextCount] = point
				nextCount++
			case depth >= 0:
				next[nextCount] = side.cross(previous, point)
				nextCount++
				next[nextCount] = point
				nextCount++
			case previousDepth >= 0:
				next[nextCount] = side.cross(previous, point)
				nextCount++
			}
		}
		current, next = next, current
		count = nextCount
	}
	return current, count
}

// legacyBoxPiece draws the four box-drawing pieces of U+1CC1B to U+1CC1E, which add a half line to
// a box drawing junction.
func (g *cellGlyph) legacyBoxPiece(r rune) {
	switch r {
	case 0x1CC1B: // light horizontal and upper right
		g.drawLines([4]boxStyle{boxNone, boxLight, boxNone, boxLight})
		g.rect(g.width-g.rule, 0, g.width, g.height/2)
	case 0x1CC1C: // light horizontal and lower right
		g.drawLines([4]boxStyle{boxNone, boxLight, boxNone, boxLight})
		g.rect(g.width-g.rule, g.height/2, g.width, g.height)
	case 0x1CC1D: // light top and upper left
		g.rect(0, 0, g.width, g.rule)
		g.rect(0, 0, g.rule, g.height/2)
	case 0x1CC1E: // light bottom and lower left
		g.rect(0, g.height-g.rule, g.width, g.height)
		g.rect(0, g.height/2, g.rule, g.height)
	}
}

// legacyTwoCircles draws the paired white circles of U+1CE00 and U+1CE01: one on each of two
// opposite edges.
func (g *cellGlyph) legacyTwoCircles(r rune) {
	width, height := float64(g.width), float64(g.height)
	radius := 0.5 * math.Min(width, height)
	if r == 0x1CE00 { // left and right
		g.circleSector(0, height/2, radius, -math.Pi/2, math.Pi/2, false)
		g.circleSector(width, height/2, radius, math.Pi/2, 3*math.Pi/2, false)
		return
	}
	g.circleSector(width/2, 0, radius, 0, math.Pi, false)
	g.circleSector(width/2, height, radius, math.Pi, 2*math.Pi, false)
}

// legacyHalfEllipse draws the left or right half of a white ellipse, U+1CE0B and U+1CE0C.
func (g *cellGlyph) legacyHalfEllipse(r rune) {
	mark := len(g.shapes.Triangles)
	if r == 0x1CE0B {
		g.circlePiece(0, 0, 1, 0.5, cornerTopLeft)
		g.circlePiece(0, 0, 1, 0.5, cornerBottomLeft)
	} else {
		g.circlePiece(1, 0, 1, 0.5, cornerTopRight)
		g.circlePiece(1, 0, 1, 0.5, cornerBottomRight)
	}
	g.clipToCell(mark)
}

// legacyVerticalPiece draws the four vertical box pieces of U+1CE16 to U+1CE19.
func (g *cellGlyph) legacyVerticalPiece(r rune) {
	g.drawLines([4]boxStyle{boxLight, boxNone, boxLight, boxNone})
	switch r {
	case 0x1CE16: // vertical and top right
		g.rect(g.width/2, 0, g.width, g.rule)
	case 0x1CE17: // vertical and bottom right
		g.rect(g.width/2, g.height-g.rule, g.width, g.height)
	case 0x1CE18: // vertical and top left
		g.rect(0, 0, g.width/2, g.rule)
	case 0x1CE19: // vertical and bottom left
		g.rect(0, g.height-g.rule, g.width/2, g.height)
	}
}

// legacySeparatedSextant fills the separated sextants U+1CE51 to U+1CE8F: six blocks of the cell,
// each with a gap of a twelfth of its width around it, the leftover height in the middle row.
func (g *cellGlyph) legacySeparatedSextant(r rune) {
	bits := uint8(r - 0x1CE50)
	gap, width, right := g.separatedColumns()
	midGap := gap*2 + (g.height%3)/2
	height := (g.height - gap*2 - midGap*2) / 3
	middleHeight := g.height - gap*2 - midGap*2 - height*2
	rows := [3]int32{gap, gap + height + midGap, gap + height + midGap*2 + middleHeight}
	heights := [3]int32{height, middleHeight, height}
	for index := 0; index < 6; index++ {
		if bits&(1<<uint(index)) == 0 {
			continue
		}
		x, row := gap, index/2
		if index&1 != 0 {
			x = right
		}
		g.rect(x, rows[row], x+width, rows[row]+heights[row])
	}
}

// legacySixteenths are the quarter-by-quarter rectangles U+1CE90 to U+1CEAF fill, each as the
// fraction of the cell's width and height its corners sit at.
var legacySixteenths = [32][4]int32{
	{0, 1, 0, 1}, {1, 2, 0, 1}, {2, 3, 0, 1}, {3, 4, 0, 1},
	{0, 1, 1, 2}, {1, 2, 1, 2}, {2, 3, 1, 2}, {3, 4, 1, 2},
	{0, 1, 2, 3}, {1, 2, 2, 3}, {2, 3, 2, 3}, {3, 4, 2, 3},
	{0, 1, 3, 4}, {1, 2, 3, 4}, {2, 3, 3, 4}, {3, 4, 3, 4},
	{2, 4, 3, 4}, {1, 4, 3, 4}, {0, 3, 3, 4}, {0, 2, 3, 4},
	{0, 1, 2, 4}, {0, 1, 1, 4}, {0, 1, 0, 3}, {0, 1, 0, 2},
	{0, 2, 0, 1}, {0, 3, 0, 1}, {1, 4, 0, 1}, {2, 4, 0, 1},
	{3, 4, 0, 2}, {3, 4, 0, 3}, {3, 4, 1, 4}, {3, 4, 2, 4},
}

// legacySixteenth fills the sixteenth block U+1CE90 to U+1CEAF names.
func (g *cellGlyph) legacySixteenth(r rune) {
	fill := legacySixteenths[r-0x1CE90]
	g.fracRect(
		cellFraction{fill[0], 4}, cellFraction{fill[1], 4},
		cellFraction{fill[2], 4}, cellFraction{fill[3], 4}, 1)
}
