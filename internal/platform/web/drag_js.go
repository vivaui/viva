//go:build js && wasm

package web

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// StartFileDrag cannot offer files from the page. A browser drags files out of a
// dragstart handler that puts them on the event's DataTransfer, which is a DOM
// object the page's own script builds and the browser takes over; the canvas this
// adapter paints into has no DataTransfer, and a canvas drag reaches the browser
// as a drag of pixels, not of files. So the page can offer its files only by
// copying them (see SetClipboardFiles), which is a different promise to the user.
func StartFileDrag(paths []string, allowed platform.DragEffect) (platform.DragEffect, error) {
	return platform.DragEffectNone, fmt.Errorf("%w: a browser drags files from a DOM DataTransfer a canvas does not have", platform.ErrDragUnsupported)
}
