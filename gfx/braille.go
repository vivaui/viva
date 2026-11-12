package gfx

import "math"

// IsBraille reports whether r is a Braille Pattern, U+2800 to U+28FF: the dots a terminal draws
// as squares of its cell, so a spinner or a braille picture follows the cell's size.
func IsBraille(r rune) bool { return r >= 0x2800 && r <= 0x28FF }

// brailleDots is where each bit of a pattern draws: the left column's four rows, the right
// column's four, in the order Unicode numbers them.
var brailleDots = [8]struct{ column, row int32 }{
	{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3},
}

// braille draws the dots of a Braille Pattern as squares in the cell: as large as the cell's
// quarters and eighths allow, each in the middle of its slot so that one gap falls between any two
// dots — inside a cell and across the edge to the next one. A dot is a quarter of the cell wide
// and an eighth of it tall, the size ghostty's sprite gives it (src/font/sprite/draw/braille.zig).
//
// ponytail: a cell is rarely a whole number of dots and gaps, so one gap may be a device pixel
// wider than its neighbour. ghostty spends that leftover on a wider inner gap instead, which
// leaves the gap between two cells narrower than the gap inside one (a 20x40 device cell: 6 and
// 4); a dot grid reads as uneven then, so it is spread evenly here, and the dots land a device
// pixel off ghostty's on some cell sizes.
func (g *cellGlyph) braille(r rune) {
	width, height := g.width, g.height
	dot := min(width/4, height/8)
	if dot <= 0 {
		return
	}
	// A dot's centre is the middle of its slot, so the slots — not the dots — tile the cell. A
	// half device pixel lands on the lower index, which is where ghostty's own rounding puts it.
	place := func(index, slots, extent int32) int32 {
		return int32(math.Floor((float64(index)+0.5)*float64(extent)/float64(slots) - float64(dot)/2))
	}
	x := [2]int32{place(0, 2, width), place(1, 2, width)}
	y := [4]int32{place(0, 4, height), place(1, 4, height), place(2, 4, height), place(3, 4, height)}
	pattern := uint8(r)
	for bit, at := range brailleDots {
		if pattern&(1<<uint(bit)) != 0 {
			g.rect(x[at.column], y[at.row], x[at.column]+dot, y[at.row]+dot)
		}
	}
}
