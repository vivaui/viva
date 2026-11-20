package gfx

import (
	"strings"
	"unicode/utf8"
)

// LabelHighlighted paints case-insensitive query matches behind a shaped label.
// Match offsets are UTF-8 byte boundaries, as required by the text engine; the
// original label is emitted once, preserving accessibility and native text.
func (canvas *Canvas) LabelHighlighted(value string, bounds Rect, style TextStyle, alignment Alignment, query string, color Color) {
	if query == "" || color.Alpha <= 0 {
		canvas.LabelAligned(value, bounds, style, alignment)
		return
	}
	value, bounds.X = canvas.placeLabel(value, bounds, style, alignment)
	count := utf8.RuneCountInString(query)
	var storage [8]TextSpan
	canvas.Clip(bounds)
	for start := 0; start < len(value); {
		end := start
		for i := 0; i < count && end < len(value); i++ {
			_, size := utf8.DecodeRuneInString(value[end:])
			end += size
		}
		if strings.EqualFold(value[start:end], query) {
			for _, span := range canvas.TextSelectionStyle(storage[:0], value, style, start, end) {
				canvas.Fill(Rect{X: bounds.X + span.X, Y: bounds.Y, Width: span.Width, Height: bounds.Height}, color)
			}
			start = end
		} else {
			_, size := utf8.DecodeRuneInString(value[start:])
			start += size
		}
	}
	canvas.Unclip()
	canvas.LabelAligned(value, bounds, style, AlignLeft)
}
