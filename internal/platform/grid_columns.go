package platform

import "math"

// GridColumns is the device columns, left to right from the texture's left edge, that the cells
// of one run own in an LCD grid label's texture width columns wide: the run starts at left and
// is span wide, both in device pixels. Both edges round to the nearest column, so the runs of a
// line, which meet at the edge of a cell, own columns that meet and never overlap, and a run that
// holds no cell owns none.
//
// A column two runs both owned would be drawn twice, once with each run's color, over the same
// coverage; a glyph that fills its cell, such as a powerline arrow, has full coverage in the
// shared column, so the second color would cover the first as a line the height of the cell. The
// cell width is a whole number of device pixels (ui.Canvas.MonoCellSize), so the edges of a cell
// are columns already and rounding only absorbs the float error of the division by the scale.
func GridColumns(left, span float64, width int) (from, to int) {
	from = min(max(roundColumn(left), 0), width)
	to = min(roundColumn(left+span), width)
	return from, max(to, from)
}

func roundColumn(edge float64) int {
	return int(math.Floor(edge + 0.5))
}
