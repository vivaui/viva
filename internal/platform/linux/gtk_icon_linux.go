//go:build linux && vivagtk

package linux

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

func (w *gtkWindow) SetIcon(icon platform.WindowIcon) error {
	if w == nil || w.app == nil {
		return fmt.Errorf("linux: window is closed")
	}
	w.app.enqueue(func() {
		if !w.closed {
			w.config.Icon = icon
			w.setIcon()
		}
	})
	return nil
}

// setIcon gives the window the application's picture where the desktop shows one (the title bar and
// the task switcher on X11; Wayland takes the icon of the application's desktop entry instead).
func (w *gtkWindow) setIcon() {
	icon := w.config.Icon
	if !icon.Valid() {
		return
	}
	g := w.g
	var newPixbuf func(colorspace, alpha, bits, width, height int32) uintptr
	var pixbufPixels func(uintptr) *byte
	var pixbufStride func(uintptr) int32
	library, err := purego.Dlopen("libgdk_pixbuf-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	defer purego.Dlclose(library)
	if !g.lookup(&newPixbuf, "gdk_pixbuf_new") || !g.lookup(&pixbufPixels, "gdk_pixbuf_get_pixels") || !g.lookup(&pixbufStride, "gdk_pixbuf_get_rowstride") {
		return
	}
	var list uintptr
	var pixbufs []uintptr
	for _, size := range netWMIconSizes {
		if size > icon.Size && len(pixbufs) > 0 {
			break
		}
		// The title bar and the task switcher draw these small, so the mark
		// fills the icon rather than the canvas's margin.
		tile := icon.TileMark(min(size, icon.Size))
		if !tile.Valid() {
			continue
		}
		pixbuf := newPixbuf(0, 1, 8, int32(tile.Size), int32(tile.Size))
		if pixbuf != 0 {
			// GTK can retain a pixbuf after the window replaces its icon. Give
			// it native-owned pixels instead of retaining Go slices indefinitely
			// or releasing them while another GTK object still reads them.
			stride := int(pixbufStride(pixbuf))
			pixels := unsafe.Slice(pixbufPixels(pixbuf), (tile.Size-1)*stride+tile.Size*4)
			for row := 0; row < tile.Size; row++ {
				copy(pixels[row*stride:row*stride+tile.Size*4], tile.RGBA[row*tile.Size*4:(row+1)*tile.Size*4])
			}
			pixbufs = append(pixbufs, pixbuf)
			list = g.listAppend(list, pixbuf)
		}
	}
	if list == 0 {
		return
	}
	g.windowSetIconList(w.toplevel, list)
	g.listFree(list)
	for _, pixbuf := range pixbufs {
		g.objectUnref(pixbuf)
	}
}

// placeAtOrigin puts the window where the application asked for its content, on X11. The window
// manager's frame is not known until the window is shown, so the first time the window has a place
// the difference between the content and the asked-for origin is corrected once.
func (w *gtkWindow) placeAtOrigin() {
	config := w.config
	if !config.HasOrigin || w.app.wayland || config.WindowMode == platform.WindowPanel {
		return
	}
	x, y := int32(math.Round(float64(config.Origin.X*w.ratio))), int32(math.Round(float64(config.Origin.Y*w.ratio)))
	w.g.windowMove(w.toplevel, x, y)
	w.originPending = true
}

// correctOrigin runs when the window has a place on screen: it moves the frame by what the
// decoration added.
func (w *gtkWindow) correctOrigin() {
	if !w.originPending || w.closed {
		return
	}
	w.originPending = false
	window := w.g.widgetGetWindow(w.area)
	if window == 0 {
		return
	}
	var cx, cy int32
	w.g.gdkWindowOrigin(window, &cx, &cy)
	wantX, wantY := int32(math.Round(float64(w.config.Origin.X*w.ratio))), int32(math.Round(float64(w.config.Origin.Y*w.ratio)))
	if cx == wantX && cy == wantY {
		return
	}
	var fx, fy int32
	w.g.windowGetPosition(w.toplevel, &fx, &fy)
	w.g.windowMove(w.toplevel, fx+wantX-cx, fy+wantY-cy)
}
