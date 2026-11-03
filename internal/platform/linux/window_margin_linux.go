package linux

import (
	"os"
	"strconv"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The X side of a window whose frame Viva draws (window_frame.go): a 32-bit
// visual so the margin can be transparent, the margin announced to the window
// manager, and the input restricted to the content and a grip around it.

const (
	xcbCWBorderPixel = 1 << 3
	xcbCWColormap    = 1 << 13

	// xcbVisualTrueColor is the visual class of a visual that carries its colors
	// in its pixels.
	xcbVisualTrueColor = 4

	shapeSet   = 0
	shapeInput = 2
)

type xcbDepth struct {
	Depth      uint8
	Padding0   uint8
	VisualsLen uint16
	Padding1   [4]byte
}

type xcbDepthIterator struct {
	Data      *xcbDepth
	Remaining int32
	Index     int32
}

type xcbVisualType struct {
	ID                 uint32
	Class              uint8
	BitsPerRGB         uint8
	ColormapEntries    uint16
	RedMask, GreenMask uint32
	BlueMask           uint32
	Padding            [4]byte
}

type xcbVisualIterator struct {
	Data      *xcbVisualType
	Remaining int32
	Index     int32
}

// frameMode is VIVA_LINUX_FRAME. "off" never draws a frame. "on" draws it where the
// desktop composites. "force" draws it without asking, for tests. Anything else
// is automatic: where the desktop composites and the graphics device is a GPU.
func frameMode() string { return os.Getenv("VIVA_LINUX_FRAME") }

// argbVisual finds the 32-bit TrueColor visual with the usual alpha-red-green-blue
// masks, which every compositing X server offers.
func (win *window) argbVisual() (uint32, bool) {
	var depths func(*xcbScreen) xcbDepthIterator
	var nextDepth func(*xcbDepthIterator)
	var visuals func(*xcbDepth) xcbVisualIterator
	var nextVisual func(*xcbVisualIterator)
	purego.RegisterLibFunc(&depths, win.api.library, "xcb_screen_allowed_depths_iterator")
	purego.RegisterLibFunc(&nextDepth, win.api.library, "xcb_depth_next")
	purego.RegisterLibFunc(&visuals, win.api.library, "xcb_depth_visuals_iterator")
	purego.RegisterLibFunc(&nextVisual, win.api.library, "xcb_visualtype_next")
	for iterator := depths(win.screen); iterator.Remaining > 0; nextDepth(&iterator) {
		if iterator.Data.Depth != 32 {
			continue
		}
		for visual := visuals(iterator.Data); visual.Remaining > 0; nextVisual(&visual) {
			candidate := visual.Data
			if candidate.Class == xcbVisualTrueColor && candidate.RedMask == 0xff0000 && candidate.GreenMask == 0xff00 && candidate.BlueMask == 0xff {
				return candidate.ID, true
			}
		}
	}
	return 0, false
}

// compositing reports whether a compositing manager owns the screen: windows are
// drawn with their alpha only then.
func (win *window) compositing(screenIndex int32) bool {
	atom, err := win.api.atom(win.connection, "_NET_WM_CM_S"+strconv.Itoa(int(screenIndex)))
	if err != nil {
		return false
	}
	var nativeError unsafe.Pointer
	reply := win.api.selectionOwnerReply(win.connection, win.api.getSelectionOwner(win.connection, atom), &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return false
	}
	defer win.api.free(unsafe.Pointer(reply))
	return reply.Owner != 0
}

// wantsFrame says whether a window of this kind gets a drawn frame here, and
// the visual to make it with. A desktop that does not composite shows the
// transparent margin as black, so there the window keeps the plain frame.
func (win *window) wantsFrame(screenIndex int32) (visual uint32, ok bool) {
	mode := frameMode()
	if mode == "off" {
		return 0, false
	}
	if mode != "force" {
		if !win.compositing(screenIndex) {
			return 0, false
		}
		// Software drivers before Mesa 23 draw nothing into a window with alpha,
		// and a machine drawing in software is a virtual one where a shadow is not
		// worth the risk of an invisible window. "on" asks for it anyway.
		if mode != "on" && vulkanDeviceIsSoftware() {
			return 0, false
		}
	}
	return win.argbVisual()
}

// createFrameWindow creates the window with the 32-bit visual and its colormap.
func (win *window) createFrameWindow(visual uint32, eventMask uint32, x, y int16) {
	api, connection := win.api, win.connection
	colormap := api.generateID(connection)
	var createColormap func(uintptr, uint8, uint32, uint32, uint32) xcbVoidCookie
	purego.RegisterLibFunc(&createColormap, api.library, "xcb_create_colormap")
	createColormap(connection, 0, colormap, win.screen.Root, visual)
	// Values follow the order of their bits: background, border, events, colormap.
	values := [4]uint32{0, 0, eventMask, colormap}
	api.createWindow(connection, 32, win.id, win.screen.Root, x, y, uint16(win.width), uint16(win.height), 0, xcbWindowClassInput, visual,
		xcbCWBackPixel|xcbCWBorderPixel|xcbCWEventMask|xcbCWColormap, &values[0])
	win.colormap = colormap
}

// contentOrigin is where the content's top-left corner is on the screen, inside
// the margin the window manager does not count.
func (win *window) contentOrigin() (int32, int32) {
	x, y := win.rootOrigin()
	return x + int32(win.margin), y + int32(win.margin)
}

// setFrameExtents tells the window manager how much of the window is margin.
func (win *window) setFrameExtents(margin uint32) {
	atom, err := win.api.atom(win.connection, "_GTK_FRAME_EXTENTS")
	if err != nil {
		return
	}
	extents := [4]uint32{margin, margin, margin, margin}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, atom, xcbAtomCardinal, 32, 4, unsafe.Pointer(&extents[0]))
}

// updateInputRegion makes the window take input only in the content and a grip
// around it, so a press in the rest of the margin goes to the window behind.
// The extension is optional; without it the whole window takes input.
func (win *window) updateInputRegion() {
	if win.margin == 0 {
		if win.shapeRectangles != nil {
			rectangle := [4]uint16{0, 0, uint16(win.width), uint16(win.height)}
			win.shapeRectangles(win.connection, shapeSet, shapeInput, 0, win.id, 0, 0, 1, unsafe.Pointer(&rectangle[0]))
		}
		return
	}
	if win.shapeRectangles == nil {
		library, err := purego.Dlopen("libxcb-shape.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			win.shapeRectangles = func(uintptr, uint8, uint8, uint8, uint32, int16, int16, uint32, unsafe.Pointer) xcbVoidCookie {
				return xcbVoidCookie{}
			}
			return
		}
		purego.RegisterLibFunc(&win.shapeRectangles, library, "xcb_shape_rectangles")
		win.shapeLibrary = library
	}
	grip := uint32(float32(frameGripLogical) * win.displayScale())
	box := frameInput(win.width, win.height, win.margin, grip)
	rectangle := [4]uint16{uint16(box.x), uint16(box.y), uint16(box.width), uint16(box.height)}
	win.shapeRectangles(win.connection, shapeSet, shapeInput, 0, win.id, 0, 0, 1, unsafe.Pointer(&rectangle[0]))
}

// closeFrame releases what a drawn frame loaded.
func (win *window) closeFrame() {
	if win.shapeLibrary != 0 {
		purego.Dlclose(win.shapeLibrary)
		win.shapeLibrary = 0
	}
	win.shapeRectangles = nil
}

// dropFrame takes the margin away from a window that has one: it is maximized,
// or the renderer cannot composite with alpha.
func (win *window) setMargin(margin uint32) {
	win.margin = margin
	win.setFrameExtents(margin)
	if margin == 0 && win.shapeRectangles != nil {
		// A window without margin takes input everywhere: the whole window is
		// the input region.
		rectangle := [4]uint16{0, 0, uint16(win.width), uint16(win.height)}
		win.shapeRectangles(win.connection, shapeSet, shapeInput, 0, win.id, 0, 0, 1, unsafe.Pointer(&rectangle[0]))
	}
}

// shiftPointer moves a pointer event's position from the window's own
// coordinates to the content's, which start at the margin. A press in the grip
// of the margin has a negative position, which is how it reaches the edge test.
func (win *window) shiftPointer(kind uint8, event unsafe.Pointer) {
	switch kind {
	case xcbButtonPress, xcbButtonRelease, xcbMotionNotify, xcbEnterNotify, xcbLeaveNotify:
		input := (*xcbInputEvent)(event)
		input.EventX -= int16(win.margin)
		input.EventY -= int16(win.margin)
	}
}

// syncMargin updates win.margin to match the window's maximized state.
// When maximized, a window has no margin so its content fills what the
// window manager grants.
func (state *linuxWindowState) syncMargin() (bool, error) {
	win := state.win
	if !win.frameWanted {
		return false, nil
	}
	margin := uint32(0)
	if !win.frame.maximized && !win.fullscreen.Load() {
		margin = frameMargin(win.displayScale())
	}
	if margin == win.margin {
		return false, nil
	}
	win.setMargin(margin)
	if err := state.renderer.setFrameMargin(margin); err != nil {
		return false, err
	}
	return true, nil
}
