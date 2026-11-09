package linux

import (
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// fixedAdvanceEngine gives every character the same advance, half the font size when
// advance is zero, so a test can put a caret or a click where it wants it.
type fixedAdvanceEngine struct{ advance float32 }

func (e fixedAdvanceEngine) per(size float32) float32 {
	if e.advance > 0 {
		return e.advance
	}
	return size / 2
}

func (e fixedAdvanceEngine) Width(value string, size, _ float32) float32 {
	return float32(utf8.RuneCountInString(value)) * e.per(size)
}

func (e fixedAdvanceEngine) Caret(value string, size, _ float32, offset int) float32 {
	return float32(utf8.RuneCountInString(value[:max(0, min(offset, len(value)))])) * e.per(size)
}

func (e fixedAdvanceEngine) Selection(dst []gfx.TextSpan, value string, size, weight float32, start, end int) []gfx.TextSpan {
	left := e.Caret(value, size, weight, start)
	return append(dst, gfx.TextSpan{X: left, Width: e.Caret(value, size, weight, end) - left})
}

func (e fixedAdvanceEngine) OffsetAt(value string, size, _ float32, x float32) int {
	best, distance := 0, x
	for offset := range value {
		if d := abs32(x - e.Caret(value, size, 0, offset)); d < distance {
			best, distance = offset, d
		}
	}
	if abs32(x-e.Caret(value, size, 0, len(value))) < distance {
		best = len(value)
	}
	return best
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
