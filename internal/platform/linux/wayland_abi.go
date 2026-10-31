//go:build linux

package linux

import (
	"math"
	"runtime"
	"unsafe"
)

// wlFixed is a 24.8 signed fixed-point number used by Wayland.
type wlFixed int32

// wlFixedToFloat64 converts a 24.8 fixed-point number to float64.
func wlFixedToFloat64(f wlFixed) float64 {
	return float64(f) / 256.0
}

// float64ToWlFixed converts a float64 to a 24.8 fixed-point number.
func float64ToWlFixed(v float64) wlFixed {
	return wlFixed(math.Round(v * 256.0))
}

// wlMessage describes a Wayland request or event signature (C ABI compatible).
type wlMessage struct {
	Name      *byte
	Signature *byte
	Types     **wlInterface
}

// wlInterface describes a Wayland interface (C ABI compatible).
type wlInterface struct {
	Name        *byte
	Version     int32
	MethodCount int32
	Methods     *wlMessage
	EventCount  int32
	Padding     uint32
	Events      *wlMessage
}

// wlArray describes a dynamically sized buffer in Wayland (C ABI compatible).
type wlArray struct {
	Size  uintptr
	Alloc uintptr
	Data  unsafe.Pointer
}

// pinnedListener pins callback function tables and user data to prevent Go runtime GC movement.
type pinnedListener struct {
	pinner runtime.Pinner
}

// pin pins any heap pointers passed so they can be safely handed to C callbacks.
func (p *pinnedListener) pin(targets ...any) {
	for _, target := range targets {
		if target != nil {
			p.pinner.Pin(target)
		}
	}
}

// unpin unpins all pinned objects.
func (p *pinnedListener) unpin() {
	p.pinner.Unpin()
}
