//go:build linux && !android

package vulkan

import (
	"errors"
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

// The renderer is made for any window system's surface; these tests give it a real
// one, an X window, with the few libxcb calls it takes to make one. Where there is
// no X display they use a headless surface, and skip where there is neither.

const vkStructureXcbSurfaceCreateInfo = 1000005000

type vkXcbSurfaceCreateInfo struct {
	SType      uint32
	_          uint32
	Next       uintptr
	Flags      uint32
	_          uint32
	Connection uintptr
	Window     uint32
	_          uint32
}

type xcbTestWindow struct {
	connection uintptr
	id         uint32
	configure  func(connection uintptr, window uint32, mask uint16, values *uint32) uint32
	flush      func(connection uintptr) int32
	disconnect func(connection uintptr)
}

// newXCBTestWindow opens a window of width x height, closed when the test ends.
func newXCBTestWindow(t testing.TB, width, height uint32) *xcbTestWindow {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	library, err := purego.Dlopen("libxcb.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		t.Skipf("no libxcb: %v", err)
	}
	var (
		connect    func(display uintptr, screen *int32) uintptr
		connErr    func(connection uintptr) int32
		getSetup   func(connection uintptr) unsafe.Pointer
		generateID func(connection uintptr) uint32
		create     func(connection uintptr, depth uint8, window, parent uint32, x, y int16, width, height, border, class uint16, visual, mask uint32, values uintptr) uint32
		mapWindow  func(connection uintptr, window uint32) uint32
	)
	window := &xcbTestWindow{}
	purego.RegisterLibFunc(&connect, library, "xcb_connect")
	purego.RegisterLibFunc(&connErr, library, "xcb_connection_has_error")
	purego.RegisterLibFunc(&getSetup, library, "xcb_get_setup")
	purego.RegisterLibFunc(&generateID, library, "xcb_generate_id")
	purego.RegisterLibFunc(&create, library, "xcb_create_window")
	purego.RegisterLibFunc(&mapWindow, library, "xcb_map_window")
	purego.RegisterLibFunc(&window.configure, library, "xcb_configure_window")
	purego.RegisterLibFunc(&window.flush, library, "xcb_flush")
	purego.RegisterLibFunc(&window.disconnect, library, "xcb_disconnect")
	var screen int32
	window.connection = connect(0, &screen)
	if window.connection == 0 || connErr(window.connection) != 0 {
		t.Skip("cannot connect to the X display")
	}
	t.Cleanup(func() { window.disconnect(window.connection) })
	// The first screen's root window sits after the setup's fixed 40 bytes, the vendor
	// string padded to four and the eight-byte pixmap formats.
	setup := getSetup(window.connection)
	vendor := uint32(*(*uint16)(unsafe.Add(setup, 24)))
	formats := uint32(*(*uint8)(unsafe.Add(setup, 29)))
	root := *(*uint32)(unsafe.Add(setup, 40+int((vendor+3)&^3)+int(formats)*8))
	window.id = generateID(window.connection)
	create(window.connection, 0, window.id, root, 0, 0, uint16(width), uint16(height), 0, 1, 0, 0, 0)
	mapWindow(window.connection, window.id)
	window.flush(window.connection)
	return window
}

// resize asks for a size and returns once the server has it, as the surface sees it.
func (window *xcbTestWindow) resize(width, height uint32) {
	size := [2]uint32{width, height}
	window.configure(window.connection, window.id, 0x4|0x8, &size[0])
	window.flush(window.connection)
}

// xcbSurfaceSource makes the surface of the X window.
func xcbSurfaceSource(t testing.TB, window *xcbTestWindow) SurfaceSource {
	t.Helper()
	var create func(uintptr, *vkXcbSurfaceCreateInfo, unsafe.Pointer, *uintptr) int32
	if err := Bind(&create, "vkCreateXcbSurfaceKHR"); err != nil {
		if errors.Is(err, platform.ErrNoGraphics) {
			t.Skipf("no Vulkan device: %v", err)
		}
		t.Fatal(err)
	}
	return SurfaceSource{
		Extension: "VK_KHR_xcb_surface",
		Create: func(instance uintptr) (uintptr, error) {
			info := vkXcbSurfaceCreateInfo{SType: vkStructureXcbSurfaceCreateInfo, Connection: window.connection, Window: window.id}
			var surface uintptr
			if err := vkCheck(create(instance, &info, nil, &surface), "create XCB surface"); err != nil {
				return 0, err
			}
			return surface, nil
		},
	}
}

func init() {
	windowSystemSurface = func(t testing.TB, width, height uint32) (SurfaceSource, func(width, height uint32) bool) {
		if os.Getenv("DISPLAY") == "" {
			return headlessSurfaceSource(t), func(uint32, uint32) bool { return false }
		}
		window := newXCBTestWindow(t, width, height)
		return xcbSurfaceSource(t, window), func(width, height uint32) bool { window.resize(width, height); return true }
	}
}
