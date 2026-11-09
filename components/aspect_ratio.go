package components

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// AspectRatio returns the largest centered rectangle with ratio width/height
// that fits bounds. A non-positive or unordered ratio produces an empty rect
// at the center, making invalid layout input deterministic.
func AspectRatio(bounds gfx.Rect, ratio float32) gfx.Rect {
	centerX := bounds.X + bounds.Width/2
	centerY := bounds.Y + bounds.Height/2
	if !(ratio > 0) || math.IsInf(float64(ratio), 0) || bounds.Width <= 0 || bounds.Height < 0 {
		return gfx.Rect{X: centerX, Y: centerY}
	}
	if bounds.Height == 0 {
		return gfx.Rect{
			X: bounds.X, Y: bounds.Y,
			Width: bounds.Width, Height: bounds.Width / ratio,
		}
	}

	width := bounds.Width
	height := width / ratio
	if height > bounds.Height {
		height = bounds.Height
		width = height * ratio
	}
	return gfx.Rect{
		X:     centerX - width/2,
		Y:     centerY - height/2,
		Width: width, Height: height,
	}
}
