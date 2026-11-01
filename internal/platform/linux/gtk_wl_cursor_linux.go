//go:build linux && vivagtk

package linux

import (
	"os"
	"strconv"

	"github.com/ebitengine/purego"
)

// The cursor of the Wayland subsurface for a compositor without wp_cursor_shape_v1 (GNOME before 47,
// for one): the client draws the cursor itself, from the user's cursor theme through
// libwayland-cursor, on a wl_surface of its own.

type wlCursorTheme struct {
	loaded     bool
	theme      uintptr
	surface    uintptr
	size       int32
	themeLoad  func(name *byte, size int32, shm uintptr) uintptr
	themeGet   func(theme uintptr, name *byte) uintptr
	imageGetBF func(image uintptr) uintptr
}

// themeCursor sets the cursor of the pointer from the theme. It reports whether it could.
func (s *wlSession) themeCursor(shape uint8) bool {
	t := &s.cursorTheme
	if !t.loaded {
		t.loaded = true
		library, err := purego.Dlopen("libwayland-cursor.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return false
		}
		purego.RegisterLibFunc(&t.themeLoad, library, "wl_cursor_theme_load")
		purego.RegisterLibFunc(&t.themeGet, library, "wl_cursor_theme_get_cursor")
		purego.RegisterLibFunc(&t.imageGetBF, library, "wl_cursor_image_get_buffer")
		t.size = 24
		if v, err := strconv.Atoi(os.Getenv("XCURSOR_SIZE")); err == nil && v > 0 {
			t.size = int32(v)
		}
		var name *byte
		if v := os.Getenv("XCURSOR_THEME"); v != "" {
			b := cString(v)
			name = &b[0]
		}
		t.theme = t.themeLoad(name, t.size, s.shm)
		c := s.c
		t.surface = c.create(s.compositor, 0, c.core["wl_surface"], c.version(s.compositor), 0)
	}
	if t.theme == 0 || int(shape) >= len(cursorNames) {
		return false
	}
	name := cString(cursorNames[shape])
	cursor := t.themeGet(t.theme, &name[0])
	if cursor == 0 {
		fallback := cString("left_ptr")
		if cursor = t.themeGet(t.theme, &fallback[0]); cursor == 0 {
			return false
		}
	}
	// struct wl_cursor { unsigned int image_count; struct wl_cursor_image **images; char *name; }
	images := *(*uintptr)(unsafePointer(cursor + 8))
	image := *(*uintptr)(unsafePointer(images))
	// struct wl_cursor_image { uint32 width, height, hotspot_x, hotspot_y, delay; }
	hotX, hotY := *(*uint32)(unsafePointer(image + 8)), *(*uint32)(unsafePointer(image + 12))
	buffer := t.imageGetBF(image)
	if buffer == 0 {
		return false
	}
	c := s.c
	c.request(t.surface, 1, wlArg(buffer), 0, 0)
	c.request(t.surface, 9, 0, 0, 0x7fffffff, 0x7fffffff) // damage_buffer: all of it
	c.request(t.surface, 6)
	c.request(s.pointer, 0, wlArg(s.enterSerial), wlArg(t.surface), wlArg(hotX), wlArg(hotY)) // set_cursor
	c.flush(s.display)
	return true
}
