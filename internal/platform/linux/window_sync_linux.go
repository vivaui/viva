//go:build !android

package linux

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// syncCounter is the window's XSYNC counter and the part of libxcb-sync that
// drives it. The library is optional: a machine without it, or an X server
// without the extension, keeps the window as it was, resized without a sync.
type syncCounter struct {
	library uintptr
	id      uint32
	set     func(uintptr, uint32, uint64) xcbVoidCookie
	destroy func(uintptr, uint32) xcbVoidCookie
}

type syncInitializeCookie struct{ Sequence uint32 }

type syncInitializeReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Major, Minor uint8
	Pad          [22]byte
}

// openSync creates the counter and publishes it, and returns the protocol atoms
// the window adds to WM_PROTOCOLS: _NET_WM_PING always, and
// _NET_WM_SYNC_REQUEST when the counter exists. A failure of any part leaves
// out that part only.
func (win *window) openSync() []uint32 {
	api, connection := win.api, win.connection
	var protocols []uint32
	if ping, err := api.atom(connection, "_NET_WM_PING"); err == nil {
		win.pingAtom = ping
		protocols = append(protocols, ping)
	}
	request, err := api.atom(connection, "_NET_WM_SYNC_REQUEST")
	if err != nil {
		return protocols
	}
	counterAtom, err := api.atom(connection, "_NET_WM_SYNC_REQUEST_COUNTER")
	if err != nil {
		return protocols
	}
	library, err := purego.Dlopen("libxcb-sync.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return protocols
	}
	var initialize func(uintptr, uint8, uint8) syncInitializeCookie
	var initializeReply func(uintptr, syncInitializeCookie, *unsafe.Pointer) *syncInitializeReply
	var create func(uintptr, uint32, uint64) xcbVoidCookie
	counter := &syncCounter{library: library}
	purego.RegisterLibFunc(&initialize, library, "xcb_sync_initialize")
	purego.RegisterLibFunc(&initializeReply, library, "xcb_sync_initialize_reply")
	purego.RegisterLibFunc(&create, library, "xcb_sync_create_counter")
	purego.RegisterLibFunc(&counter.set, library, "xcb_sync_set_counter")
	purego.RegisterLibFunc(&counter.destroy, library, "xcb_sync_destroy_counter")
	var nativeError unsafe.Pointer
	reply := initializeReply(connection, initialize(connection, 3, 0), &nativeError)
	if nativeError != nil {
		api.free(nativeError)
	}
	if reply == nil {
		purego.Dlclose(library)
		return protocols
	}
	api.free(unsafe.Pointer(reply))
	counter.id = api.generateID(connection)
	create(connection, counter.id, 0)
	value := counter.id
	api.changeProperty(connection, xcbPropertyReplace, win.id, counterAtom, xcbAtomCardinal, 32, 1, unsafe.Pointer(&value))
	win.syncCounter = counter
	win.syncRequestAtom = request
	return append(protocols, request)
}

// closeSync releases the counter. The connection is still open.
func (win *window) closeSync() {
	counter := win.syncCounter
	if counter == nil {
		return
	}
	win.syncCounter = nil
	counter.destroy(win.connection, counter.id)
	purego.Dlclose(counter.library)
}

// protocolMessage answers a WM_PROTOCOLS message that is not a close request:
// a ping goes back to the root window, and a sync request is kept until the
// next configure has been drawn. It reports whether the message was one of them.
func (win *window) protocolMessage(message *xcbClientEvent) bool {
	atom := uint32(message.Data[0]) | uint32(message.Data[1])<<8 | uint32(message.Data[2])<<16 | uint32(message.Data[3])<<24
	switch {
	case atom != 0 && atom == win.pingAtom:
		// The window manager asks whether the window still answers; sending the
		// message back with the window replaced by the root is the reply.
		reply := *message
		reply.Window = win.screen.Root
		win.api.sendEvent(win.connection, 0, win.screen.Root, xcbEventSubstructureRedirect|xcbEventSubstructureNotify, (*byte)(unsafe.Pointer(&reply)))
		win.api.flush(win.connection)
		return true
	case atom != 0 && atom == win.syncRequestAtom && win.syncCounter != nil:
		lo, hi := syncRequestValue(message.Data)
		win.syncPending.request(lo, hi)
		return true
	}
	return false
}

// answerSync tells the window manager the frame it asked about is drawn.
func (win *window) answerSync() {
	counter := win.syncCounter
	if counter == nil {
		return
	}
	if lo, hi, ok := win.syncPending.take(); ok {
		counter.set(win.connection, counter.id, syncInt64(lo, hi))
		win.api.flush(win.connection)
	}
}
