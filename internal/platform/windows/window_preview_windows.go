package windows

import (
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

var errTransparentPreviewUnavailable = fmt.Errorf("%w: windows has no compositor for it", platform.ErrDragPreviewUnsupported)

// SupportsDragPreview marks the Windows adapter as able to show a transparent
// preview window that never takes focus or input.
func (application *windowsApplication) SupportsDragPreview() {}

// MoveTo puts the top-left of a preview's content at origin without changing
// its size, stacking level or activation.
func (window *nativeWindow) MoveTo(origin gfx.Point) {
	if window == nil || window.closed.Load() || window.state == nil || window.state.window == 0 {
		return
	}
	state := window.state
	scale := state.scale
	if scale <= 0 {
		scale = 1
	}
	x := int32(origin.X * scale)
	y := int32(origin.Y * scale)
	setWindowPosition.Call(state.window, 0, uintptr(x), uintptr(y), 0, 0, 0x15) // nosize, nozorder, noactivate
}

var _ platform.WindowMover = (*nativeWindow)(nil)
