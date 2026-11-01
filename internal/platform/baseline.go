package platform

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
// the baseline of a single line of text lies, for a texture heightPixels tall and
// a font sizePixels high, both in device pixels. Every backend places its glyphs
// on this row, whatever the ascent and descent of the font it draws with, so a
// line of text stands at the same height on all of them. The baseline is a whole
// device pixel, so glyph edges land on the pixel grid.
func LabelBaseline(heightPixels int, sizePixels float64) int {
	return int(math.Floor(float64(heightPixels)/2 + BaselineBelowCenter*sizePixels + 1e-6))
}

// TextCaretBand is the vertical extent, in logical pixels, of the caret and of the
// selection highlight in a text input at y that is height tall: a band as tall as a line
// of text, centered in the field, so the highlight neither fills the field's padding nor
// stands taller than the caret.
func TextCaretBand(y, height, fontSize float32) (top, bandHeight float32) {
	bandHeight = min(height, fontSize*1.35)
	return y + (height-bandHeight)/2, bandHeight
}

// CompositionUnderlineTop is the top, in logical pixels, of the stroke that underlines a
// composition in a text input at y that is height tall. Text stands on LabelBaseline of
// the field's device-pixel texture, not on a multiple of the font size, so the stroke is
// placed from that baseline and stays clear of descenders; thickness is the widest stroke
// drawn, which the field's bottom edge clamps. scale is device pixels per logical pixel,
// zero meaning one.
func CompositionUnderlineTop(y, height, fontSize, scale, thickness float32) float32 {
	if scale <= 0 {
		scale = 1
	}
	baseline := float32(LabelBaseline(int(math.Ceil(float64(height*scale))), float64(fontSize*scale))) / scale
	return y + min(height-thickness, baseline+max(2, fontSize*0.2))
}
