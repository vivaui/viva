//go:build linux

package linux

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// StartFileDrag cannot offer files from a window on Linux yet. Dragging out is
// the XDND protocol from the source's side: the window becomes the source,
// offers the paths as text/uri-list on the XdndSelection, answers the target's
// XdndStatus and the conversion requests while the drag lasts, and finishes it
// when the pointer is released. This adapter speaks XDND as a target only (see
// drop_linux.go), so the window can offer its files only by copying them (see
// SetClipboardFiles), which is a different promise to the user.
func StartFileDrag(paths []string, allowed platform.DragEffect) (platform.DragEffect, error) {
	return platform.DragEffectNone, fmt.Errorf("%w: the X11 adapter takes XDND drops but does not offer them", platform.ErrDragUnsupported)
}
