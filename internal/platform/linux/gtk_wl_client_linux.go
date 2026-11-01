//go:build linux && vivagtk

package linux

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The Wayland client side of the GTK shell (ADR 0064). GDK owns the connection; the shell adds a
// subsurface to the window's surface and talks to it with libwayland-client directly, through
// wl_proxy_marshal_array_flags (which, unlike the generated wrappers, is not variadic). The
// protocols libwayland does not ship are written out below as the scanner would.

// wlArg is one union wl_argument.
type wlArg uintptr

// wl_interface is 40 bytes: name, version int32, method_count int32, methods, event_count int32,
// events. wl_message is 24 bytes: name, signature, types. The tables the interfaces point to must
// stay where they are for as long as the proxies live, so they come from an arena that is never
// freed.
var (
	wlArena    = make([]byte, 16<<10)
	wlArenaOff int
)

func wlAlloc(n int) uintptr {
	offset := (wlArenaOff + 7) &^ 7
	wlArenaOff = offset + n
	if wlArenaOff > len(wlArena) {
		panic("linux: Wayland interface arena exhausted")
	}
	return uintptr(unsafe.Pointer(&wlArena[offset]))
}

func wlPut(base uintptr, offset int, value uintptr) {
	*(*uintptr)(unsafePointer(base + uintptr(offset))) = value
}

func wlPut32(base uintptr, offset int, value int32) {
	*(*int32)(unsafePointer(base + uintptr(offset))) = value
}

func wlCStr(value string) uintptr {
	p := wlAlloc(len(value) + 1)
	// Not copy: the compiler cannot tell the memmove of a string into memory that is an address
	// from C is not overlapping, and fails to compile it where this is inlined.
	for i := 0; i < len(value); i++ {
		*(*byte)(unsafePointer(p + uintptr(i))) = value[i]
	}
	return p
}

type wlMsg struct {
	name, signature string
	types           []uintptr // one per argument, 0 where it names no interface
}

func wlMessages(messages []wlMsg) uintptr {
	if len(messages) == 0 {
		return 0
	}
	array := wlAlloc(24 * len(messages))
	for i, m := range messages {
		types := wlAlloc(8 * (len(m.types) + 1))
		for j, t := range m.types {
			wlPut(types, 8*j, t)
		}
		wlPut(array, 24*i, wlCStr(m.name))
		wlPut(array, 24*i+8, wlCStr(m.signature))
		wlPut(array, 24*i+16, types)
	}
	return array
}

func wlInterface(name string, version int32, methods, events []wlMsg) uintptr {
	p := wlAlloc(40)
	wlPut(p, 0, wlCStr(name))
	wlPut32(p, 8, version)
	wlPut32(p, 12, int32(len(methods)))
	wlPut(p, 16, wlMessages(methods))
	wlPut32(p, 24, int32(len(events)))
	wlPut(p, 32, wlMessages(events))
	return p
}

// wlClient is libwayland-client and the interfaces the shell uses.
type wlClient struct {
	marshal     func(proxy uintptr, opcode uint32, iface uintptr, version uint32, flags uint32, args *wlArg) uintptr
	addListener func(proxy, listener, data uintptr) int32
	version     func(proxy uintptr) uint32
	roundtrip   func(display uintptr) int32
	flush       func(display uintptr) int32
	core        map[string]uintptr

	viewporter, viewport, fractionalManager, fractional, cursorManager, cursorDevice uintptr
}

var (
	wlOnce   sync.Once
	wlShared *wlClient
	wlError  error
)

// loadWayland opens libwayland-client once.
func loadWayland() (*wlClient, error) {
	wlOnce.Do(func() {
		library, err := purego.Dlopen("libwayland-client.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			wlError = fmt.Errorf("linux: libwayland-client is not installed: %w", err)
			return
		}
		c := &wlClient{core: map[string]uintptr{}}
		for _, symbol := range []string{"wl_proxy_marshal_array_flags", "wl_proxy_add_listener", "wl_proxy_get_version", "wl_display_roundtrip", "wl_display_flush"} {
			if _, err := purego.Dlsym(library, symbol); err != nil {
				wlError = fmt.Errorf("linux: libwayland-client lacks %s (1.20 or newer is needed): %w", symbol, err)
				return
			}
		}
		purego.RegisterLibFunc(&c.marshal, library, "wl_proxy_marshal_array_flags")
		purego.RegisterLibFunc(&c.addListener, library, "wl_proxy_add_listener")
		purego.RegisterLibFunc(&c.version, library, "wl_proxy_get_version")
		purego.RegisterLibFunc(&c.roundtrip, library, "wl_display_roundtrip")
		purego.RegisterLibFunc(&c.flush, library, "wl_display_flush")
		for _, name := range []string{"wl_registry", "wl_compositor", "wl_subcompositor", "wl_shm", "wl_shm_pool", "wl_buffer", "wl_surface", "wl_subsurface", "wl_region", "wl_pointer", "wl_callback", "wl_data_device_manager", "wl_data_device", "wl_data_offer"} {
			symbol, err := purego.Dlsym(library, name+"_interface")
			if err != nil {
				wlError = fmt.Errorf("linux: libwayland-client lacks %s_interface: %w", name, err)
				return
			}
			c.core[name] = symbol
		}
		surface, pointer := c.core["wl_surface"], c.core["wl_pointer"]
		c.viewport = wlInterface("wp_viewport", 1, []wlMsg{
			{"destroy", "", nil},
			{"set_source", "ffff", []uintptr{0, 0, 0, 0}},
			{"set_destination", "ii", []uintptr{0, 0}},
		}, nil)
		c.viewporter = wlInterface("wp_viewporter", 1, []wlMsg{
			{"destroy", "", nil},
			{"get_viewport", "no", []uintptr{c.viewport, surface}},
		}, nil)
		c.fractional = wlInterface("wp_fractional_scale_v1", 1, []wlMsg{{"destroy", "", nil}},
			[]wlMsg{{"preferred_scale", "u", []uintptr{0}}})
		c.fractionalManager = wlInterface("wp_fractional_scale_manager_v1", 1, []wlMsg{
			{"destroy", "", nil},
			{"get_fractional_scale", "no", []uintptr{c.fractional, surface}},
		}, nil)
		c.cursorDevice = wlInterface("wp_cursor_shape_device_v1", 1, []wlMsg{
			{"destroy", "", nil},
			{"set_shape", "uu", []uintptr{0, 0}},
		}, nil)
		c.cursorManager = wlInterface("wp_cursor_shape_manager_v1", 1, []wlMsg{
			{"destroy", "", nil},
			{"get_pointer", "no", []uintptr{c.cursorDevice, pointer}},
			{"get_tablet_tool_v2", "no", []uintptr{0, 0}},
		}, nil)
		wlShared = c
	})
	return wlShared, wlError
}

// request sends a request that creates nothing.
func (c *wlClient) request(proxy uintptr, opcode uint32, args ...wlArg) {
	if len(args) == 0 {
		args = []wlArg{0}
	}
	c.marshal(proxy, opcode, 0, c.version(proxy), 0, &args[0])
}

// destroy sends a destructor request, which frees the proxy.
func (c *wlClient) destroy(proxy uintptr, opcode uint32) {
	argument := wlArg(0)
	c.marshal(proxy, opcode, 0, c.version(proxy), 1, &argument)
}

// create sends a request whose new_id argument is the slot holding 0 in args, and returns the new
// proxy.
func (c *wlClient) create(proxy uintptr, opcode uint32, iface uintptr, version uint32, args ...wlArg) uintptr {
	return c.marshal(proxy, opcode, iface, version, 0, &args[0])
}

// wlObjects maps the data pointer of a listener to the Go value it is for. Listeners are static: a
// callback is made once per event signature, because purego never frees one.
var (
	wlObjects = map[uintptr]any{}
	wlNext    uintptr
)

func wlRegister(object any) uintptr {
	wlNext++
	wlObjects[wlNext] = object
	return wlNext
}

func wlUnregister(id uintptr) { delete(wlObjects, id) }

// wlListener returns the table of C function pointers for wl_proxy_add_listener.
func wlListener(functions ...any) uintptr {
	table := wlAlloc(8 * len(functions))
	for i, function := range functions {
		wlPut(table, 8*i, purego.NewCallback(function))
	}
	return table
}

func wlFixed(v uintptr) float64 { return float64(int32(v)) / 256 }
