package linux

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
	// frameMarginLogical is the margin around the content, which holds the shadow.
	frameMarginLogical = 24
	// frameRadiusLogical is the radius of the content's corners.
	frameRadiusLogical = 10
	// frameGripLogical is how much of the margin takes input: a press there
	// resizes the window, the rest of the margin lets the pointer through to the
	// window behind.
	frameGripLogical = 8
	// frameShadow is the shadow's blur, offset down, and strength (logical).
	frameShadowBlur   = 18
	frameShadowOffset = 4
	frameShadowAlpha  = 0.38
)

// frameMargin is the margin in device pixels at a display scale.
func frameMargin(scale float32) uint32 {
	return uint32(math.Round(float64(frameMarginLogical) * float64(max(scale, 1))))
}

// frameGrip is the grip region in device pixels at a display scale.
func frameGrip(scale float32) uint32 {
	return uint32(math.Round(float64(frameGripLogical) * float64(max(scale, 1))))
}

// pixelBox is a rectangle in device pixels.
type pixelBox struct{ x, y, width, height int32 }

// contentSize is the size of the content of a window of a total size with a
// margin on every side; a margin that leaves nothing leaves one pixel.
func contentSize(total, margin uint32) uint32 {
	if total <= 2*margin {
		return 1
	}
	return total - 2*margin
}

// frameContent is the content rectangle of a window of width x height.
func frameContent(width, height, margin uint32) pixelBox {
	return pixelBox{int32(margin), int32(margin), int32(contentSize(width, margin)), int32(contentSize(height, margin))}
}

// frameBands are the four strips of the margin, which the shadow is drawn in:
// top and bottom across the whole width, left and right between them.
func frameBands(width, height, margin uint32) [4]pixelBox {
	m, w, h := int32(margin), int32(width), int32(height)
	return [4]pixelBox{
		{0, 0, w, m},
		{0, h - m, w, m},
		{0, m, m, h - 2*m},
		{w - m, m, m, h - 2*m},
	}
}

// frameCorners are the squares at the four corners of the content that its
// rounded corners are cut in; each is a pixel larger than the radius so the
// edge of the cut is not clipped.
func frameCorners(width, height, margin uint32, radius float32) [4]pixelBox {
	side := int32(math.Ceil(float64(radius))) + 1
	c := frameContent(width, height, margin)
	return [4]pixelBox{
		{c.x, c.y, side, side},
		{c.x + c.width - side, c.y, side, side},
		{c.x, c.y + c.height - side, side, side},
		{c.x + c.width - side, c.y + c.height - side, side, side},
	}
}

// frameInput is the part of the window that takes input: the content and a grip
// around it that is used to resize.
func frameInput(width, height, margin, grip uint32) pixelBox {
	reach := min(grip, margin)
	return pixelBox{int32(margin - reach), int32(margin - reach), int32(width - 2*(margin-reach)), int32(height - 2*(margin-reach))}
}
