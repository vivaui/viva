// Package windowframe is the geometry of the frame Viva draws around a window it
// draws the frame of: the margin that holds the shadow, the rounded corners of the
// content, and the grip that takes resize input. It is pure arithmetic in device
// pixels, shared by the renderers that draw it and the windows that place it.
package windowframe

import "math"

// A window the application draws the frame of (platform.WindowCustomTitlebar)
// has no frame from the window manager, so on a desktop that composites windows
// it also has no shadow and square corners, where the macOS and Windows windows
// have both. Viva draws them: the X window is larger than the content by a margin
// on every side, the margin is transparent except for a soft shadow, and the
// content's corners are rounded. The margin is announced to the window manager
// as _GTK_FRAME_EXTENTS, which is how it knows where the window really is: it
// places, snaps and maximizes the content, not the margin.
//
// Everything here is in device pixels unless it says logical.

const (
	// MarginLogical is the margin around the content, which holds the shadow.
	MarginLogical = 24
	// RadiusLogical is the radius of the content's corners.
	RadiusLogical = 10
	// GripLogical is how much of the margin takes input: a press there
	// resizes the window, the rest of the margin lets the pointer through to the
	// window behind.
	GripLogical = 8
	// frameShadow is the shadow's blur, offset down, and strength (logical).
	ShadowBlur   = 18
	ShadowOffset = 4
	ShadowAlpha  = 0.38
)

// Margin is the margin in device pixels at a display scale.
func Margin(scale float32) uint32 {
	return uint32(math.Round(float64(MarginLogical) * float64(max(scale, 1))))
}

// Box is a rectangle in device pixels.
type Box struct{ X, Y, Width, Height int32 }

// ContentSize is the size of the content of a window of a total size with a
// margin on every side; a margin that leaves nothing leaves one pixel.
func ContentSize(total, margin uint32) uint32 {
	if total <= 2*margin {
		return 1
	}
	return total - 2*margin
}

// Content is the content rectangle of a window of width x height.
func Content(width, height, margin uint32) Box {
	return Box{int32(margin), int32(margin), int32(ContentSize(width, margin)), int32(ContentSize(height, margin))}
}

// Bands are the four strips of the margin, which the shadow is drawn in:
// top and bottom across the whole width, left and right between them.
func Bands(width, height, margin uint32) [4]Box {
	m, w, h := int32(margin), int32(width), int32(height)
	return [4]Box{
		{0, 0, w, m},
		{0, h - m, w, m},
		{0, m, m, h - 2*m},
		{w - m, m, m, h - 2*m},
	}
}

// Corners are the squares at the four corners of the content that its
// rounded corners are cut in; each is a pixel larger than the radius so the
// edge of the cut is not clipped.
func Corners(width, height, margin uint32, radius float32) [4]Box {
	side := int32(math.Ceil(float64(radius))) + 1
	c := Content(width, height, margin)
	return [4]Box{
		{c.X, c.Y, side, side},
		{c.X + c.Width - side, c.Y, side, side},
		{c.X, c.Y + c.Height - side, side, side},
		{c.X + c.Width - side, c.Y + c.Height - side, side, side},
	}
}

// Input is the part of the window that takes input: the content and a grip
// around it that is used to resize.
func Input(width, height, margin, grip uint32) Box {
	reach := min(grip, margin)
	return Box{int32(margin - reach), int32(margin - reach), int32(width - 2*(margin-reach)), int32(height - 2*(margin-reach))}
}
