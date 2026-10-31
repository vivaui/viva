package platform

// WindowEdge is the part of a frameless window's border that resizes it. A
// window the system draws no frame for still resizes from its edges, and each
// adapter, the painter's cursor and the hit-test agree on where they are.
type WindowEdge uint8

const (
	EdgeNone WindowEdge = iota
	// The eight edges run clockwise from the top left, which is the order of
	// the X11 _NET_WM_MOVERESIZE directions (offset by one, since 0 is none).
	EdgeTopLeft
	EdgeTop
	EdgeTopRight
	EdgeRight
	EdgeBottomRight
	EdgeBottom
	EdgeBottomLeft
	EdgeLeft
)

const (
	// EdgeBorder is the width of the strip around the window that resizes it, and
	// EdgeCorner the length of the corner zone along each side, in logical pixels.
	// Aligned with community standards (GTK4 RESIZE_HANDLE_CORNER_SIZE = 24, Qt6,
	// and libdecor), the corner zone extends 24px along the perimeter to provide
	// accessible and ergonomic diagonal resize handles.
	EdgeBorder = 6
	EdgeCorner = 24
)

// EdgeAt reports which edge a point of a width x height window is on, or
// EdgeNone. overControl says the application has a control under the point: the
// top edge yields to it, so a button stays usable up to the top of the strip,
// while the sides and the bottom do not. Outer grip points (outside the client
// area, where x < 0 or y < 0 or x >= width or y >= height) never yield to controls.
func EdgeAt(x, y, width, height float32, overControl bool) WindowEdge {
	if width <= 0 || height <= 0 {
		return EdgeNone
	}
	left, right := x < EdgeBorder, x >= width-EdgeBorder
	top, bottom := y < EdgeBorder, y >= height-EdgeBorder
	nearLeft, nearRight := x < EdgeCorner, x >= width-EdgeCorner
	nearTop, nearBottom := y < EdgeCorner, y >= height-EdgeCorner
	topYields := overControl && y >= 0
	switch {
	case left && nearTop, top && nearLeft && !topYields:
		return EdgeTopLeft
	case right && nearTop, top && nearRight && !topYields:
		return EdgeTopRight
	case left && nearBottom, bottom && nearLeft:
		return EdgeBottomLeft
	case right && nearBottom, bottom && nearRight:
		return EdgeBottomRight
	case left:
		return EdgeLeft
	case right:
		return EdgeRight
	case bottom:
		return EdgeBottom
	case top && !topYields:
		return EdgeTop
	}
	return EdgeNone
}
