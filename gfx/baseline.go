package gfx

import (
	"math"

	"github.com/vivaui/viva/internal/textmetrics"
)

// LabelBaseline reports the y, in logical pixels, of the baseline of a
// proportional label drawn in bounds with style. Every backend puts the glyphs
// of an interface-font line on this row, whatever the font's own ascent and
// descent, so text drawn on it stands at the same height on every platform and
// in a window and an offscreen image. A fixed-pitch line instead stands on its
// own font's baseline, MonoBaseline; textRow is the row that dispatches.
func (canvas *Canvas) LabelBaseline(bounds Rect, style TextStyle) float32 {
	return bounds.Y + canvas.labelBaselineRow(bounds.Height, style.Size)
}

// LabelOnBaseline appends an immutable system-font line whose baseline sits at
// baselineY. bounds gives the line box: its X and Width place the line, and its
// Height is the line height whose baseline row the platform uses. bounds.Y is
// ignored, since baselineY decides the vertical position. The line is aligned to
// the start of its text, as Label.
//
// A label drawn with Label in a box at y has its baseline at LabelBaseline, so a
// caller that knows where the first line stands can place the whole line there
// instead of computing a box top from a guessed ascent. A Mono style stands on
// its own font's baseline, so its row comes from MonoBaseline when the canvas
// measured the face.
func (canvas *Canvas) LabelOnBaseline(value string, bounds Rect, baselineY float32, style TextStyle) {
	canvas.LabelAlignedOnBaseline(value, bounds, baselineY, style, AlignStart)
}

// LabelAlignedOnBaseline is LabelOnBaseline with explicit horizontal placement,
// as LabelAligned.
func (canvas *Canvas) LabelAlignedOnBaseline(value string, bounds Rect, baselineY float32, style TextStyle, alignment Alignment) {
	bounds.Y = baselineY - canvas.textRow(bounds, style)
	canvas.LabelAligned(value, bounds, style, alignment)
}

// textRow is the row of a line of style in bounds: the mono font's own baseline
// when the style is fixed-pitch and the canvas measured the face, and
// labelBaselineRow for interface-font lines and a face no engine measured, which
// has no row of its own.
func (canvas *Canvas) textRow(bounds Rect, style TextStyle) float32 {
	if style.Mono {
		if row := canvas.MonoBaseline(style.Size, style.MonoFace, bounds.Height); row > 0 {
			return row
		}
	}
	return canvas.labelBaselineRow(bounds.Height, style.Size)
}

// labelBaselineRow is the distance from the top of a label's texture to its
// baseline, in logical pixels, for a label box height logical pixels tall and a
// font size pixels high. The platform rasters the label at the canvas's pixel
// scale, on a row that is a whole device pixel.
func (canvas *Canvas) labelBaselineRow(height, size float32) float32 {
	scale := canvas.PixelScale()
	rows := int(math.Ceil(float64(height * scale)))
	return float32(textmetrics.LabelBaseline(rows, float64(size)*float64(scale))) / scale
}
