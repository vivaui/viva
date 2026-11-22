//go:build darwin

package darwin

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// StartFileDrag cannot offer files from a window on macOS yet. A drag out of a
// window is a dragging session: the view under the pointer starts an
// NSDraggingSession whose pasteboard items are file URLs, and the session's
// dragging source reports where it ended and with which operation. This adapter
// binds the view and the general pasteboard, but neither NSDraggingSession nor
// NSDraggingSource is bound, so the window can offer its files only by copying
// them (see SetClipboardFiles), which is a different promise to the user.
func StartFileDrag(paths []string, allowed platform.DragEffect) (platform.DragEffect, error) {
	return platform.DragEffectNone, fmt.Errorf("%w: macOS drags files out with an NSDraggingSession, which this adapter does not bind", platform.ErrDragUnsupported)
}
