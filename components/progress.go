package components

import "github.com/vivaui/viva/gfx"

// progressHeight is the track's h-1.5.
const progressHeight = 6

// Progress paints the official bounded horizontal progress track and
// indicator. A non-finite or negative fraction is treated as zero.
//
// Like the source, the indicator is a plain bar inside a rounded track that
// clips it: its left end follows the track's round end at any width, and its
// right end is flat until the bar fills the track.
func Progress(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, fraction float32) {
	if canvas == nil || bounds.Width <= 0 {
		return
	}
	if bounds.Height <= 0 {
		bounds.Height = progressHeight
	}
	if fraction != fraction || fraction < 0 {
		fraction = 0
	} else if fraction > 1 {
		fraction = 1
	}
	canvas.Rounded(bounds, bounds.Height/2, theme.Muted)
	paintProgressIndicator(canvas, theme, bounds, 0, bounds.Width*fraction)
}

// ProgressIndeterminate paints a track whose indicator is a segment sliding
// across it, for work of unknown length. phase is the caller's position in the
// cycle: 0 puts the segment off the left end, 1 off the right end. The caller
// schedules the frames; nothing here keeps time.
func ProgressIndeterminate(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, phase float32) {
	if canvas == nil || bounds.Width <= 0 {
		return
	}
	if bounds.Height <= 0 {
		bounds.Height = progressHeight
	}
	phase = min(max(phase, 0), 1)
	canvas.Rounded(bounds, bounds.Height/2, theme.Muted)
	segment := bounds.Width * 0.4
	start := -segment + (bounds.Width+segment)*phase
	paintProgressIndicator(canvas, theme, bounds, max(start, 0), min(start+segment, bounds.Width))
}

// paintProgressIndicator fills the track between two offsets from its left
// edge, by painting the full pill in the indicator color under a clip.
func paintProgressIndicator(canvas *gfx.Canvas, theme gfx.Theme, track gfx.Rect, from, to float32) {
	if to-from <= 0 {
		return
	}
	canvas.Clip(gfx.Rect{X: track.X + from, Y: track.Y, Width: to - from, Height: track.Height})
	canvas.Rounded(track, track.Height/2, theme.Primary)
	canvas.Unclip()
}

// ProgressRowHeight is the height of a labeled progress row: a text-sm line, the
// root's gap-3 and the track.
func ProgressRowHeight(theme gfx.Theme) float32 {
	return theme.Metrics.FontSizeM + 7 + theme.Metrics.GapItem*1.5 + progressHeight
}

// PaintProgressRow paints a ProgressLabel at the start and a ProgressValue at
// the end of a line, with the track on the line below, and returns the track's
// rectangle. bounds.Height is ignored; the row is ProgressRowHeight tall.
// value is the caller-formatted text, such as "64%".
func PaintProgressRow(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, label, value string, fraction float32) gfx.Rect {
	metrics := theme.Metrics
	line := metrics.FontSizeM + 7
	valueWidth := canvas.TextWidth(value, metrics.FontSizeM)
	canvas.Label(label, gfx.Rect{X: bounds.X, Y: bounds.Y, Width: max(bounds.Width-valueWidth-metrics.GapItem*1.5, 0), Height: line},
		gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground})
	canvas.LabelAligned(value, gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: line},
		gfx.TextStyle{Size: metrics.FontSizeM, Color: theme.MutedForeground}, gfx.AlignRight)
	track := gfx.Rect{X: bounds.X, Y: bounds.Y + line + metrics.GapItem*1.5, Width: bounds.Width, Height: progressHeight}
	Progress(canvas, theme, track, fraction)
	return track
}
