package linux

import (
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// The X11 adapter can preview where an ARGB visual, compositor and input
// shaping are available. OpenWindow checks those before mapping the surface.
func (*linuxApplication) SupportsDragPreview() {}

func (win *window) markPreview() error {
	library, err := purego.Dlopen("libxcb-shape.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return err
	}
	win.shapeLibrary = library
	// The checked request rejects servers without the input-shape extension.
	purego.RegisterLibFunc(&win.shapeRectangles, library, "xcb_shape_rectangles_checked")
	if err := win.checkRequest("preview input shape", win.shapeRectangles(win.connection, shapeSet, shapeInput, 0, win.id, 0, 0, 0, nil)); err != nil {
		return err
	}
	return win.setPreviewProperties()
}

func (win *window) setPreviewProperties() error {
	// Override-redirect keeps the window manager from decorating, activating,
	// repositioning or adding a taskbar entry for this transient drag surface.
	const overrideRedirect = 1 << 9
	value := uint32(1)
	if err := win.checkRequest("preview override redirect", win.api.changeWindow(win.connection, win.id, overrideRedirect, &value)); err != nil {
		return err
	}
	for _, property := range []struct {
		name   string
		values []string
	}{
		{"_NET_WM_WINDOW_TYPE", []string{"_NET_WM_WINDOW_TYPE_DND"}},
		{"_NET_WM_STATE", []string{"_NET_WM_STATE_ABOVE", "_NET_WM_STATE_SKIP_TASKBAR", "_NET_WM_STATE_SKIP_PAGER"}},
	} {
		atom, err := win.api.atom(win.connection, property.name)
		if err != nil {
			return err
		}
		values := make([]uint32, len(property.values))
		for i, name := range property.values {
			values[i], err = win.api.atom(win.connection, name)
			if err != nil {
				return err
			}
		}
		win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, atom, xcbAtomAtom, 32, uint32(len(values)), unsafe.Pointer(&values[0]))
	}
	atom, err := win.api.atom(win.connection, "WM_HINTS")
	if err != nil {
		return err
	}
	hints := [9]uint32{1, 0} // InputHint, input=false
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, atom, atom, 32, uint32(len(hints)), unsafe.Pointer(&hints[0]))
	return nil
}

// MoveTo runs on the application thread, like OpenDragPreview. Only position
// changes: no resize, focus request or redraw is needed to move a retained image.
func (window *nativeWindow) MoveTo(origin gfx.Point) {
	if window == nil || window.closed.Load() || window.state == nil {
		return
	}
	win := window.state.win
	win.mu.Lock()
	defer win.mu.Unlock()
	if win.connection == 0 {
		return
	}
	scale := win.displayScale()
	position := [2]uint32{
		uint32(int32(clampInt16(int(origin.X*scale) - int(win.margin)))),
		uint32(int32(clampInt16(int(origin.Y*scale) - int(win.margin)))),
	}
	win.api.configureWindow(win.connection, win.id, 1|2, &position[0])
	win.api.flush(win.connection)
}

var _ platform.WindowMover = (*nativeWindow)(nil)
