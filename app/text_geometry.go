package app

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// textGeometry is where the text of the focused field is on the window: the character
// boundary nearest a point, the caret at a boundary and the rectangles a range covers. The
// framework draws the text, so it is the one that can say; a phone's text interaction (the
// caret a tap places, the word a long press selects, the handles and the magnifier) is
// built on it. Indexes are runes of the field's value, as in TextState.
//
// A field that lays its rows out itself answers through its gfx.TextPlacement. Any other
// is one line, measured the way the components paint it: from the line's origin in the
// declared text bounds, with the canvas's text engine and the field's size, weight and
// alignment. A password field is measured on the bullets it shows, one per rune.
type textGeometry struct {
	canvas    *gfx.Canvas
	placement gfx.TextPlacement
	bounds    gfx.Rect
	style     gfx.TextStyle
	alignment gfx.Alignment
	value     string // the field's value
	shown     string // what is drawn: the value, or one bullet per rune
}

// focusedTextGeometry is the geometry of the focused field, false when no field with text
// bounds has the focus.
func (painter *painter) focusedTextGeometry() (textGeometry, bool) {
	context, placement, _ := painter.focusedTextField()
	if context.ID.IsZero() || painter.textInput == nil {
		return textGeometry{}, false
	}
	input := painter.textInput(context)
	if input == nil {
		return textGeometry{}, false
	}
	value := input.State.Value
	if input.Editor != nil {
		value = input.Editor.String()
	}
	geometry := textGeometry{
		canvas: painter.canvas, placement: placement, bounds: context.TextBounds,
		style:     gfx.TextStyle{Size: context.FontSize, Weight: context.FontWeight},
		alignment: context.Alignment, value: value, shown: value,
	}
	if placement == nil && (context.TextBounds.Width <= 0 || context.TextBounds.Height <= 0 || context.FontSize <= 0) {
		return textGeometry{}, false
	}
	if context.Secure {
		geometry.shown = strings.Repeat("•", utf8.RuneCountInString(value))
	}
	return geometry, true
}

// textGeometryOf is the geometry of the focused field as the platform asks for it, nil
// while no field has the focus.
func (painter *painter) textGeometryOf() platform.TextGeometry {
	geometry, ok := painter.focusedTextGeometry()
	if !ok {
		return nil
	}
	return geometry
}

// IndexAt is the character boundary nearest a window point.
func (geometry textGeometry) IndexAt(x, y float32) (int, bool) {
	if geometry.placement != nil {
		offset, ok := geometry.placement.OffsetAt(gfx.Point{X: x, Y: y})
		if !ok {
			return 0, false
		}
		return utf8.RuneCountInString(geometry.value[:max(0, min(offset, len(geometry.value)))]), true
	}
	origin := geometry.origin()
	offset := geometry.canvas.TextOffsetAtStyle(geometry.shown, geometry.style, x-origin)
	return utf8.RuneCountInString(geometry.shown[:max(0, min(offset, len(geometry.shown)))]), true
}

// CaretRect is the caret at a boundary, as tall as the line.
func (geometry textGeometry) CaretRect(index int) (gfx.Rect, bool) {
	if geometry.placement != nil {
		return gfx.Rect{}, false // it paints one caret, wherever the caret is
	}
	offset := byteOffsetOfRune(geometry.shown, index)
	x := geometry.origin() + geometry.canvas.TextCaretStyle(geometry.shown, geometry.style, offset)
	return gfx.Rect{X: x, Y: geometry.bounds.Y, Width: 1, Height: geometry.bounds.Height}, true
}

// RangeRects appends the rectangles the runes [start, end) cover, inside the field.
func (geometry textGeometry) RangeRects(dst []gfx.Rect, start, end int) []gfx.Rect {
	if geometry.placement != nil || end <= start {
		return dst
	}
	from, to := byteOffsetOfRune(geometry.shown, start), byteOffsetOfRune(geometry.shown, end)
	origin, bounds := geometry.origin(), geometry.bounds
	var spans [4]gfx.TextSpan
	for _, span := range geometry.canvas.TextSelectionStyle(spans[:0], geometry.shown, geometry.style, from, to) {
		left := max(origin+span.X, bounds.X)
		right := min(origin+span.X+span.Width, bounds.X+bounds.Width)
		if right > left {
			dst = append(dst, gfx.Rect{X: left, Y: bounds.Y, Width: right - left, Height: bounds.Height})
		}
	}
	return dst
}

func (geometry textGeometry) origin() float32 {
	return geometry.canvas.LabelOrigin(geometry.shown, geometry.bounds, geometry.style, geometry.alignment)
}

// byteOffsetOfRune is the byte offset of rune index in value, clamped to its ends.
func byteOffsetOfRune(value string, index int) int {
	if index <= 0 {
		return 0
	}
	for offset := range value {
		if index == 0 {
			return offset
		}
		index--
	}
	return len(value)
}
