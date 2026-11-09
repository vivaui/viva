package platform

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/textmetrics"
)

// ResolveTextAlignment turns a start- or end-relative alignment into the left or
// right alignment a native editor can place its glyphs with. The decision is the
// one gfx.Canvas makes for a label: AlignStart is the left edge for left-to-right
// text and the right edge for right-to-left text, and AlignEnd is the other one.
// The direction is read from the line the caret is on, so a right-to-left cell
// in a left-to-right document aligns by its own text.
//
// A caller that places a native editor must resolve before handing it over:
// TextAlignmentInset only knows the physical edges, so an unresolved AlignStart
// would silently render left-aligned.
func ResolveTextAlignment(alignment gfx.Alignment, value string, caret int) gfx.Alignment {
	switch alignment {
	case gfx.AlignStart, gfx.AlignEnd:
		start, end, _ := TextLine(value, caret)
		rightToLeft := textmetrics.RightToLeft(value[start:end])
		if alignment == gfx.AlignStart && rightToLeft || alignment == gfx.AlignEnd && !rightToLeft {
			return gfx.AlignRight
		}
		return gfx.AlignLeft
	case gfx.AlignCenter:
		return gfx.AlignCenter
	case gfx.AlignRight:
		return gfx.AlignRight
	default:
		return gfx.AlignLeft
	}
}

// TextAlignmentInset is how far into its field a fitted line starts: zero for
// left alignment, half the free space for center, all of it for right. Terminal
// and self-placed inputs keep their old origin, and a line wider than its field
// is never pushed, so an overlong value keeps the clipped behavior it had.
//
// Alignment must already be physical: callers holding an AlignStart or AlignEnd
// resolve it through ResolveTextAlignment first.
func TextAlignmentInset(alignment gfx.Alignment, fieldWidth, lineWidth float32, terminal, placed bool) float32 {
	if terminal || placed || lineWidth >= fieldWidth {
		return 0
	}
	switch alignment {
	case gfx.AlignCenter:
		return max(0, fieldWidth-lineWidth) / 2
	case gfx.AlignRight:
		return max(0, fieldWidth-lineWidth)
	default:
		return 0
	}
}

func TextCaretInsideField(alignment gfx.Alignment, fieldX, fieldWidth, caretX, strokeWidth float32) float32 {
	if alignment != gfx.AlignRight {
		return caretX
	}
	return min(caretX, fieldX+max(0, fieldWidth-strokeWidth))
}
