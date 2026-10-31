//go:build linux

package linux

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

// waylandAPI holds dynamic bindings to libwayland-client.so.0 via purego.
type waylandAPI struct {
	library uintptr

	// Core connection and event dispatching
	displayConnect         func(name *byte) uintptr
	displayDisconnect      func(display uintptr)
	displayGetFd           func(display uintptr) int32
	displayGetError        func(display uintptr) int32
	displayDispatch        func(display uintptr) int32
	displayDispatchPending func(display uintptr) int32
	displayPrepareRead     func(display uintptr) int32
	displayReadEvents      func(display uintptr) int32
	displayCancelRead      func(display uintptr)
	displayFlush           func(display uintptr) int32
	displayRoundtrip       func(display uintptr) int32

	// Proxy marshaling and listeners
	proxyMarshalFlags func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr
	proxyAddListener  func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32
	proxyDestroy      func(proxy uintptr)
	proxyGetVersion   func(proxy uintptr) uint32
}

// loadWaylandClient attempts to dynamically load libwayland-client.so.0 and bind core functions.
func loadWaylandClient() (*waylandAPI, error) {
	library, err := purego.Dlopen("libwayland-client.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("linux: failed to load libwayland-client.so.0: %w", err)
	}
	api := &waylandAPI{library: library}
	purego.RegisterLibFunc(&api.displayConnect, library, "wl_display_connect")
	purego.RegisterLibFunc(&api.displayDisconnect, library, "wl_display_disconnect")
	purego.RegisterLibFunc(&api.displayGetFd, library, "wl_display_get_fd")
	purego.RegisterLibFunc(&api.displayGetError, library, "wl_display_get_error")
	purego.RegisterLibFunc(&api.displayDispatch, library, "wl_display_dispatch")
	purego.RegisterLibFunc(&api.displayDispatchPending, library, "wl_display_dispatch_pending")
	purego.RegisterLibFunc(&api.displayPrepareRead, library, "wl_display_prepare_read")
	purego.RegisterLibFunc(&api.displayReadEvents, library, "wl_display_read_events")
	purego.RegisterLibFunc(&api.displayCancelRead, library, "wl_display_cancel_read")
	purego.RegisterLibFunc(&api.displayFlush, library, "wl_display_flush")
	purego.RegisterLibFunc(&api.displayRoundtrip, library, "wl_display_roundtrip")
	purego.RegisterLibFunc(&api.proxyMarshalFlags, library, "wl_proxy_marshal_flags")
	purego.RegisterLibFunc(&api.proxyAddListener, library, "wl_proxy_add_listener")
	purego.RegisterLibFunc(&api.proxyDestroy, library, "wl_proxy_destroy")
	purego.RegisterLibFunc(&api.proxyGetVersion, library, "wl_proxy_get_version")
	return api, nil
}

func (api *waylandAPI) close() {
	if api != nil && api.library != 0 {
		purego.Dlclose(api.library)
		api.library = 0
	}
}
