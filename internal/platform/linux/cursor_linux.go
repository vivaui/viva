package linux

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// themedCursors loads cursors from the user's cursor theme through libxcb-cursor,
// which sizes them from Xcursor.size in the resource database, or from Xft.dpi
// when that is unset: a HiDPI desktop that sets either gets large cursors. The
// library is optional; without it, or for a name the theme lacks, the cursor font's
// glyph is used, which is one fixed small size.
type themedCursors struct {
	library uintptr
	context uintptr
	load    func(uintptr, *byte) uint32
	free    func(uintptr)
}

// cursorNames are the theme names of the cursors, in app.Cursor order.
var cursorNames = [...]string{
	"left_ptr", "xterm", "hand2", "crosshair", "watch",
	"sb_h_double_arrow", "sb_v_double_arrow", "size_fdiag", "size_bdiag", "crossed_circle", "grabbing",
}

// openThemedCursors returns nil when the library or the theme is not there.
func openThemedCursors(api *xcbAPI, connection uintptr, screen *xcbScreen) *themedCursors {
	library, err := purego.Dlopen("libxcb-cursor.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	var create func(uintptr, *xcbScreen, *uintptr) int32
	purego.RegisterLibFunc(&create, library, "xcb_cursor_context_new")
	cursors := &themedCursors{library: library}
	purego.RegisterLibFunc(&cursors.load, library, "xcb_cursor_load_cursor")
	purego.RegisterLibFunc(&cursors.free, library, "xcb_cursor_context_free")
	if create(connection, screen, &cursors.context) != 0 || cursors.context == 0 {
		purego.Dlclose(library)
		return nil
	}
	return cursors
}

// cursor returns a new X cursor for the shape, 0 when the theme has none.
func (cursors *themedCursors) cursor(shape uint8) uint32 {
	if cursors == nil || int(shape) >= len(cursorNames) {
		return 0
	}
	name := append([]byte(cursorNames[shape]), 0)
	return cursors.load(cursors.context, (*byte)(unsafe.Pointer(&name[0])))
}

func (cursors *themedCursors) close() {
	if cursors != nil && cursors.context != 0 {
		cursors.free(cursors.context)
		cursors.context = 0
		purego.Dlclose(cursors.library)
	}
}
