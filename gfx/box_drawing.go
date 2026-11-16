package gfx

import "math"

// IsBoxDrawing reports whether r is a Unicode Box Drawing character, U+2500 to U+257F.
// A fixed-pitch line from LabelRuns draws them as geometry in their cell, never from a
// font, so neighbouring cells and rows meet without a seam whatever the font's side
// bearings, the cell's size, or the terminal's line spacing.
func IsBoxDrawing(r rune) bool { return r >= 0x2500 && r <= 0x257F }

// boxStyle is the stroke one edge of a Box Drawing character joins with.
type boxStyle uint8

const (
	boxNone boxStyle = iota
	boxLight
	boxHeavy
	boxDouble
)

// boxEdges is what each Box Drawing character joins, U+2500 to U+257F and four characters to a
// line: up, right, down and left, as n (none), l (light), h (heavy) or d (double). A character
// drawn as a dash, an arc or a diagonal joins nothing and is all n.
const boxEdges = "nlnlnhnhlnlnhnhn" + // U+2500 ─ … U+2503 ┃
	"nnnnnnnnnnnnnnnn" + // U+2504 ┄ … U+2507 ┇
	"nnnnnnnnnnnnnnnn" + // U+2508 ┈ … U+250B ┋
	"nllnnhlnnlhnnhhn" + // U+250C ┌ … U+250F ┏
	"nnllnnlhnnhlnnhh" + // U+2510 ┐ … U+2513 ┓
	"llnnlhnnhlnnhhnn" + // U+2514 └ … U+2517 ┗
	"lnnllnnhhnnlhnnh" + // U+2518 ┘ … U+251B ┛
	"lllnlhlnhllnllhn" + // U+251C ├ … U+251F ┟
	"hlhnhhlnlhhnhhhn" + // U+2520 ┠ … U+2523 ┣
	"lnlllnlhhnlllnhl" + // U+2524 ┤ … U+2527 ┧
	"hnhlhnlhlnhhhnhh" + // U+2528 ┨ … U+252B ┫
	"nlllnllhnhllnhlh" + // U+252C ┬ … U+252F ┯
	"nlhlnlhhnhhlnhhh" + // U+2530 ┰ … U+2533 ┳
	"llnlllnhlhnllhnh" + // U+2534 ┴ … U+2537 ┷
	"hlnlhlnhhhnlhhnh" + // U+2538 ┸ … U+253B ┻
	"lllllllhlhlllhlh" + // U+253C ┼ … U+253F ┿
	"hlllllhlhlhlhllh" + // U+2540 ╀ … U+2543 ╃
	"hhllllhhlhhlhhlh" + // U+2544 ╄ … U+2547 ╇
	"lhhhhlhhhhhlhhhh" + // U+2548 ╈ … U+254B ╋
	"nnnnnnnnnnnnnnnn" + // U+254C ╌ … U+254F ╏
	"ndnddndnndlnnldn" + // U+2550 ═ … U+2553 ╓
	"nddnnnldnndlnndd" + // U+2554 ╔ … U+2557 ╗
	"ldnndlnnddnnlnnd" + // U+2558 ╘ … U+255B ╛
	"dnnldnndldlndldn" + // U+255C ╜ … U+255F ╟
	"dddnlnlddndldndd" + // U+2560 ╠ … U+2563 ╣
	"ndldnldlndddldnd" + // U+2564 ╤ … U+2567 ╧
	"dlnlddndldlddldl" + // U+2568 ╨ … U+256B ╫
	"ddddnnnnnnnnnnnn" + // U+256C ╬ … U+256F ╯
	"nnnnnnnnnnnnnnnn" + // U+2570 ╰ … U+2573 ╳
	"nnnllnnnnlnnnnln" + // U+2574 ╴ … U+2577 ╷
	"nnnhhnnnnhnnnnhn" + // U+2578 ╸ … U+257B ╻
	"nhnllnhnnlnhhnln" // U+257C ╼ … U+257F ╿

// boxLines is those edges as a style each, up, right, down and left, by codepoint.
var boxLines = parseBoxEdges()

func parseBoxEdges() [0x80][4]boxStyle {
	var lines [0x80][4]boxStyle
	styles := [256]boxStyle{'l': boxLight, 'h': boxHeavy, 'd': boxDouble}
	for index := range lines {
		for side := range lines[index] {
			lines[index][side] = styles[boxEdges[index*4+side]]
		}
	}
	return lines
}

// The edges of the boxLines table, in the order they are stored.
const (
	boxUp = iota
	boxRight
	boxDown
	boxLeft
)

// boxDashes are the dashed characters: count dashes of style across the cell, separated by
// gaps at least minGap and at most the gapStyle stroke, in whole device pixels.
type boxDash struct {
	vertical bool
	count    int32
	style    boxStyle
	gapStyle boxStyle
	minGap   int32
}

var boxDashes = map[rune]boxDash{
	0x2504: {false, 3, boxLight, boxLight, 4},
	0x2505: {false, 3, boxHeavy, boxLight, 4},
	0x2506: {true, 3, boxLight, boxLight, 4},
	0x2507: {true, 3, boxHeavy, boxLight, 4},
	0x2508: {false, 4, boxLight, boxLight, 4},
	0x2509: {false, 4, boxHeavy, boxLight, 4},
	0x250A: {true, 4, boxLight, boxLight, 4},
	0x250B: {true, 4, boxHeavy, boxLight, 4},
	0x254C: {false, 2, boxLight, boxLight, 0},
	0x254D: {false, 2, boxHeavy, boxHeavy, 0},
	0x254E: {true, 2, boxLight, boxHeavy, 0},
	0x254F: {true, 2, boxHeavy, boxHeavy, 0},
}

// boxArc is a rounded corner: the direction each of its two stems leaves the cell.
type boxArc struct{ down, right bool }

var boxArcs = map[rune]boxArc{
	0x256D: {true, true},   // ╭
	0x256E: {true, false},  // ╮
	0x256F: {false, false}, // ╯
	0x2570: {false, true},  // ╰
}

// boxDiagonal is one of the three diagonal lines: the corners it runs between.
type boxDiagonal struct{ fromTopRight, fromTopLeft bool }

var boxDiagonals = map[rune]boxDiagonal{
	0x2571: {true, false}, // ╱
	0x2572: {false, true}, // ╲
	0x2573: {true, true},  // ╳
}

// boxDrawing draws r in the cell, in device pixels laid out in cell space. It is only called for a
// Box Drawing character, which is what makes the table's index safe; a character the rest of the
// tables do not hold adds nothing.
func (g *cellGlyph) boxDrawing(r rune) {
	if lines := boxLines[r-0x2500]; lines != [4]boxStyle{} {
		g.drawLines(lines)
		return
	}
	if dash, ok := boxDashes[r]; ok {
		g.drawDash(dash)
		return
	}
	if arc, ok := boxArcs[r]; ok {
		g.drawArc(arc)
		return
	}
	if diagonal, ok := boxDiagonals[r]; ok {
		g.drawDiagonal(diagonal)
	}
}

// appendBoxLines draws the character whose edges join with the given strokes: straight lines,
// corners, junctions, half lines and the doubled and mixed families all come out of here. The
// rule that a stroke which meets a heavier one, a doubled one or the middle of a straight line
// must stop where that one begins is ghostty's, and it is what makes a junction look joined
// instead of overdrawn.
func (g *cellGlyph) drawLines(lines [4]boxStyle) {
	cellWidth, cellHeight := g.width, g.height
	light, heavy := g.rule, g.heavy
	hLightTop := (cellHeight - light) / 2
	hLightBottom := hLightTop + light
	hHeavyTop := (cellHeight - heavy) / 2
	hHeavyBottom := hHeavyTop + heavy
	hDoubleTop := hLightTop - light
	hDoubleBottom := hLightBottom + light
	vLightLeft := (cellWidth - light) / 2
	vLightRight := vLightLeft + light
	vHeavyLeft := (cellWidth - heavy) / 2
	vHeavyRight := vHeavyLeft + heavy
	vDoubleLeft := vLightLeft - light
	vDoubleRight := vLightRight + light
	up, right, down, left := lines[boxUp], lines[boxRight], lines[boxDown], lines[boxLeft]

	// Where each stroke stops.
	var upBottom, downTop, leftRight, rightLeft int32
	switch {
	case left == boxHeavy || right == boxHeavy:
		upBottom, downTop = hHeavyBottom, hHeavyTop
	case left != right || down == up:
		if left == boxDouble || right == boxDouble {
			upBottom, downTop = hDoubleBottom, hDoubleTop
		} else {
			upBottom, downTop = hLightBottom, hLightTop
		}
	case left == boxNone && right == boxNone:
		upBottom, downTop = hLightBottom, hLightTop
	default:
		upBottom, downTop = hLightTop, hLightBottom
	}
	switch {
	case up == boxHeavy || down == boxHeavy:
		leftRight, rightLeft = vHeavyRight, vHeavyLeft
	case up != down || left == right:
		if up == boxDouble || down == boxDouble {
			leftRight, rightLeft = vDoubleRight, vDoubleLeft
		} else {
			leftRight, rightLeft = vLightRight, vLightLeft
		}
	case up == boxNone && down == boxNone:
		leftRight, rightLeft = vLightRight, vLightLeft
	default:
		leftRight, rightLeft = vLightLeft, vLightRight
	}

	switch up {
	case boxNone:
	case boxLight:
		g.rect(vLightLeft, 0, vLightRight, upBottom)
	case boxHeavy:
		g.rect(vHeavyLeft, 0, vHeavyRight, upBottom)
	case boxDouble:
		leftBottom, rightBottom := upBottom, upBottom
		if left == boxDouble {
			leftBottom = hLightTop
		}
		if right == boxDouble {
			rightBottom = hLightTop
		}
		g.rect(vDoubleLeft, 0, vLightLeft, leftBottom)
		g.rect(vLightRight, 0, vDoubleRight, rightBottom)
	}

	switch right {
	case boxNone:
	case boxLight:
		g.rect(rightLeft, hLightTop, cellWidth, hLightBottom)
	case boxHeavy:
		g.rect(rightLeft, hHeavyTop, cellWidth, hHeavyBottom)
	case boxDouble:
		topLeft, bottomLeft := rightLeft, rightLeft
		if up == boxDouble {
			topLeft = vLightRight
		}
		if down == boxDouble {
			bottomLeft = vLightRight
		}
		g.rect(topLeft, hDoubleTop, cellWidth, hLightTop)
		g.rect(bottomLeft, hLightBottom, cellWidth, hDoubleBottom)
	}

	switch down {
	case boxNone:
	case boxLight:
		g.rect(vLightLeft, downTop, vLightRight, cellHeight)
	case boxHeavy:
		g.rect(vHeavyLeft, downTop, vHeavyRight, cellHeight)
	case boxDouble:
		leftTop, rightTop := downTop, downTop
		if left == boxDouble {
			leftTop = hLightBottom
		}
		if right == boxDouble {
			rightTop = hLightBottom
		}
		g.rect(vDoubleLeft, leftTop, vLightLeft, cellHeight)
		g.rect(vLightRight, rightTop, vDoubleRight, cellHeight)
	}

	switch left {
	case boxNone:
	case boxLight:
		g.rect(0, hLightTop, leftRight, hLightBottom)
	case boxHeavy:
		g.rect(0, hHeavyTop, leftRight, hHeavyBottom)
	case boxDouble:
		topRight, bottomRight := leftRight, leftRight
		if up == boxDouble {
			topRight = vLightLeft
		}
		if down == boxDouble {
			bottomRight = vLightLeft
		}
		g.rect(0, hDoubleTop, topRight, hLightTop)
		g.rect(0, hLightBottom, bottomRight, hDoubleBottom)
	}
}

// drawDash draws a dashed line so that tiled cells read as one line: the gaps are equal
// and the leftover pixels go to the dashes, where a longer dash is less obvious than a wider gap.
func (g *cellGlyph) drawDash(dash boxDash) {
	length, span := g.width, g.height
	thick := g.rule
	if dash.style == boxHeavy {
		thick = g.heavy
	}
	if dash.vertical {
		length, span = g.height, g.width
	}
	gap := g.rule
	if dash.gapStyle == boxHeavy {
		gap = g.heavy
	}
	if gap < dash.minGap {
		gap = dash.minGap
	}
	if limit := length / (2 * dash.count); gap > limit {
		gap = limit
	}
	short := length - gap*dash.count
	dashLength := short / dash.count
	extra := short % dash.count
	start := gap / 2
	if dash.vertical {
		start = 0
	}
	at := start
	for index := int32(0); index < dash.count; index++ {
		end := at + dashLength
		if extra > 0 {
			extra--
			end++
		}
		if dash.vertical {
			g.rect((span-thick)/2, at, (span-thick)/2+thick, end)
		} else {
			g.rect(at, (span-thick)/2, end, (span-thick)/2+thick)
		}
		at = end + gap
	}
}

// drawArc draws a rounded corner as the one stroked path ghostty's sprite strokes: from the cell's
// edge along one stem, around the corner, and along the other stem to the cell's edge. The bend is
// a cubic whose middle control points sit a quarter of the radius in — shallower than a circle,
// and the one the sprite has. The stems are part of the stroke, so they carry the same
// antialiasing as the bend and no seam can fall between them.
func (g *cellGlyph) drawArc(arc boxArc) {
	thick := float64(g.rule)
	middleX := float64((g.width-g.rule)/2) + thick/2
	middleY := float64((g.height-g.rule)/2) + thick/2
	radius := float64(min(g.width, g.height)) / 2
	bend := 0.25 * radius
	width, height := float64(g.width), float64(g.height)
	across, down, startY, endX := -1.0, -1.0, 0.0, 0.0
	if arc.right {
		across, endX = 1, width
	}
	if arc.down {
		down, startY = 1, height
	}
	g.stroke(NewPath(float32(g.width), float32(g.height),
		MoveTo(float32(middleX), float32(startY)),
		LineTo(float32(middleX), float32(middleY+down*radius)),
		CubicTo(
			float32(middleX), float32(middleY+down*bend),
			float32(middleX+across*bend), float32(middleY),
			float32(middleX+across*radius), float32(middleY)),
		LineTo(float32(endX), float32(middleY))))
}

// drawDiagonal strokes a diagonal across the cell. Ghostty runs it half a pixel past both
// corners while keeping its slope, so it meets the diagonal of the cell it points at instead of
// stopping short of the corner.
func (g *cellGlyph) drawDiagonal(diagonal boxDiagonal) {
	if diagonal.fromTopRight {
		g.strokeDiagonal(true)
	}
	if diagonal.fromTopLeft {
		g.strokeDiagonal(false)
	}
}

// strokeDiagonal strokes the diagonal that runs from the cell's bottom left to its top right
// when rising is set.
func (g *cellGlyph) strokeDiagonal(rising bool) {
	width, height := float64(g.width), float64(g.height)
	slopeX, slopeY := math.Min(1, width/height), math.Min(1, height/width)
	fromX, fromY := -0.5*slopeX, -0.5*slopeY
	toX, toY := width+0.5*slopeX, height+0.5*slopeY
	if rising {
		fromY, toY = toY, fromY
	}
	g.stroke(NewPath(float32(g.width), float32(g.height),
		MoveTo(float32(fromX), float32(fromY)),
		LineTo(float32(toX), float32(toY))))
}

func round32(v float64) int32 { return int32(math.Floor(v + 0.5)) }
