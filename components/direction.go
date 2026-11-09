package components

import "github.com/vivaui/viva/gfx"

// Direction is an explicit local inline-flow direction. The zero value and
// unknown values resolve to LTR; no platform or process-global state is read or
// changed.
type Direction uint8

const (
	DirectionLTR Direction = iota + 1
	DirectionRTL

	LTR = DirectionLTR
	RTL = DirectionRTL
)

// LogicalRect describes geometry from inline-start and block-start.
type LogicalRect struct {
	InlineStart float32
	BlockStart  float32
	InlineSize  float32
	BlockSize   float32
}

// LogicalInsets names inline edges independently from physical left and right.
type LogicalInsets struct {
	Start  float32
	Top    float32
	End    float32
	Bottom float32
}

// IsRTL reports whether logical inline-start is the physical right edge.
func (direction Direction) IsRTL() bool {
	return direction == DirectionRTL
}

// Rect resolves logical geometry inside a physical containing rectangle.
func (direction Direction) Rect(container gfx.Rect, logical LogicalRect) gfx.Rect {
	x := container.X + logical.InlineStart
	if direction.IsRTL() {
		x = container.X + container.Width - logical.InlineStart - logical.InlineSize
	}
	return gfx.Rect{
		X:      x,
		Y:      container.Y + logical.BlockStart,
		Width:  max(logical.InlineSize, 0),
		Height: max(logical.BlockSize, 0),
	}
}

// Inset resolves logical insets and clamps an over-constrained result to an
// empty physical rectangle.
func (direction Direction) Inset(bounds gfx.Rect, insets LogicalInsets) gfx.Rect {
	start := max(insets.Start, 0)
	top := max(insets.Top, 0)
	end := max(insets.End, 0)
	bottom := max(insets.Bottom, 0)
	left, right := start, end
	if direction.IsRTL() {
		left, right = end, start
	}
	width := max(bounds.Width-left-right, 0)
	height := max(bounds.Height-top-bottom, 0)
	return gfx.Rect{X: bounds.X + left, Y: bounds.Y + top, Width: width, Height: height}
}

// Start returns a strip at logical inline-start.
func (direction Direction) Start(bounds gfx.Rect, size float32) gfx.Rect {
	size = min(max(size, 0), max(bounds.Width, 0))
	return direction.Rect(bounds, LogicalRect{InlineSize: size, BlockSize: max(bounds.Height, 0)})
}

// End returns a strip at logical inline-end.
func (direction Direction) End(bounds gfx.Rect, size float32) gfx.Rect {
	size = min(max(size, 0), max(bounds.Width, 0))
	return direction.Rect(bounds, LogicalRect{InlineStart: max(bounds.Width-size, 0), InlineSize: size, BlockSize: max(bounds.Height, 0)})
}

// InlineDelta converts a logical positive movement toward inline-end into a
// physical x delta.
func (direction Direction) InlineDelta(delta float32) float32 {
	if direction.IsRTL() {
		return -delta
	}
	return delta
}

// VisualIndex maps a logical index to its physical order. Out-of-range inputs
// return -1 rather than silently wrapping.
func (direction Direction) VisualIndex(logicalIndex, count int) int {
	if logicalIndex < 0 || logicalIndex >= count {
		return -1
	}
	if direction.IsRTL() {
		return count - logicalIndex - 1
	}
	return logicalIndex
}
