package textmetrics

import "math"

// BaselineBelowCenter is how far below the middle of a text's line box the
// baseline sits, in units of the font size. It is half the difference between
// the ascent and the descent of the system sans fonts (SF Pro's 0.378), which is
// where the baseline lands when a font's own line box is centered: the capitals
// are centered optically, and a font with a taller or lower line box than another
// does not move its letters. native.h repeats the constant for the CoreText
// rasterizer; a test keeps the two equal.
const BaselineBelowCenter = 0.378

// LabelBaseline is the row, counted from the top of a label's texture, on which
// the baseline of a proportional line lies, for a texture heightPixels tall and
// a font sizePixels high, both in device pixels. Every backend places the glyphs
// of an interface-font label on this row, whatever the ascent and descent of the
// font it draws with, so such a line stands at the same height on all of them; a
// fixed-pitch label instead carries its font's own baseline, as the platform's
// TextBaseline dispatches. The baseline is a whole device pixel, so glyph edges
// land on the pixel grid.
func LabelBaseline(heightPixels int, sizePixels float64) int {
	return int(math.Floor(float64(heightPixels)/2 + BaselineBelowCenter*sizePixels + 1e-6))
}
