package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// windowPreview is the window mode of a DragPreview. It is not a public
// WindowMode: a preview is opened only through OpenDragPreview, which keeps a
// click-through window above every other from being used as a general overlay.
const windowPreview = WindowMode(platform.WindowPreview)

// DragPreviewConfig describes the picture that follows a drag.
type DragPreviewConfig struct {
	// Width and Height are the picture's size in logical pixels.
	Width, Height int
	// Origin is where the picture's top-left starts, in screen coordinates
	// (see Window.Frame).
	Origin ui.Point
	// Paint draws the picture on a transparent surface of the given size.
	Paint func(canvas *ui.Canvas, size ui.Size)
}

// DragPreview is a picture of what the user is dragging, kept under the pointer
// where the drag has left every window of the application, such as a tab pulled
// out of its window. It sits above other windows, never takes focus, and the
// pointer passes through it, so the drag goes on in the window that has it.
type DragPreview struct {
	window *Window
}

// ErrNoDragPreview is returned, wrapping ErrUnsupported, by OpenDragPreview on
// a platform that cannot show one. The drag works without it; the application
// shows what it can inside its own windows.
var ErrNoDragPreview = fmt.Errorf("%w: drag preview", ErrUnsupported)

// previewHost is implemented by the hosts whose windows can serve as a preview.
type previewHost interface {
	CanPreview() bool
}

// OpenDragPreview shows a preview at config.Origin. Call it on the application
// thread, like OpenWindow, and Close it when the drag ends.
func (application *Application) OpenDragPreview(config DragPreviewConfig) (*DragPreview, error) {
	if application == nil || application.host == nil {
		return nil, errors.New("app: application is not running")
	}
	if host, ok := application.host.(previewHost); !ok || !host.CanPreview() {
		return nil, ErrNoDragPreview
	}
	origin := config.Origin
	window, err := application.OpenWindow(Config{
		Width:      config.Width,
		Height:     config.Height,
		Origin:     &origin,
		WindowMode: windowPreview,
		Paint:      config.Paint,
	}, nil)
	if err != nil {
		return nil, err
	}
	if _, ok := window.control.(platform.WindowMover); !ok {
		window.Close()
		return nil, ErrNoDragPreview
	}
	return &DragPreview{window: window}, nil
}

// MoveTo puts the picture's top-left at origin, in screen coordinates. Call it
// on the application thread, as from a pointer callback.
func (preview *DragPreview) MoveTo(origin ui.Point) {
	if preview == nil || preview.window == nil || preview.window.closed.Load() {
		return
	}
	if mover, ok := preview.window.control.(platform.WindowMover); ok {
		mover.MoveTo(origin)
	}
}

// Close takes the picture off the screen. Closing a closed preview does nothing.
func (preview *DragPreview) Close() {
	if preview != nil && preview.window != nil {
		preview.window.Close()
	}
}
