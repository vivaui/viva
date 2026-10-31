package app

import "github.com/vivaui/viva/internal/platform"

// trackEdge follows which resize edge of a window with Viva-drawn controls the
// pointer is over, and reports whether that changed. The top edge yields to a
// control under the pointer, as it does when pressed.
func (painter *painter) trackEdge(event platform.PointerEvent) bool {
	next := platform.EdgeNone
	if painter.controls != nil && event.Kind != platform.PointerExit && !painter.controls.maximized() {
		overControl := painter.pressAt(event.X, event.Y) == platform.PressControl
		next = platform.EdgeAt(event.X, event.Y, painter.size.Width, painter.size.Height, overControl)
	}
	changed := next != painter.edge
	painter.edge = next
	return changed
}

// cursorForEdge is the resize cursor of a window edge.
func cursorForEdge(edge platform.WindowEdge) Cursor {
	switch edge {
	case platform.EdgeLeft, platform.EdgeRight:
		return CursorResizeHorizontal
	case platform.EdgeTop, platform.EdgeBottom:
		return CursorResizeVertical
	case platform.EdgeTopLeft, platform.EdgeBottomRight:
		return CursorResizeNWSE
	case platform.EdgeTopRight, platform.EdgeBottomLeft:
		return CursorResizeNESW
	}
	return CursorArrow
}
