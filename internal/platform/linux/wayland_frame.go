//go:build linux

package linux

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	frameListenerOnce sync.Once
	rawFrameListener  [1]uintptr
	pinnerFrame       runtime.Pinner

	errWaylandFrameCallback   = errors.New("linux: failed to create Wayland frame callback")
	errWaylandFramePending    = errors.New("linux: Wayland frame callback is still pending")
	errWaylandSurfaceNotReady = errors.New("linux: Wayland surface is not ready for a buffer commit")
)

func initFrameListener() {
	frameListenerOnce.Do(func() {
		rawFrameListener[0] = purego.NewCallback(func(data unsafe.Pointer, callback uintptr, callbackData uint32) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.onFrameDone(callback, callbackData)
		})
		pinnerFrame.Pin(&rawFrameListener)
	})
}

// requestFrameCallback mounts a wl_callback on wl_surface to drive presentation backpressure.
func (win *waylandWindow) requestFrameCallback() bool {
	if win.framePending {
		return true
	}
	if win.surface == 0 || win.app == nil || win.app.api == nil {
		return false
	}
	cbIface := win.app.api.getInterface("wl_callback_interface", &wlCallbackInterface)
	cb := win.app.api.proxyMarshalFlags(win.surface, wlSurfaceFrameOpcode, cbIface, 1, 0, uintptr(0))
	if cb == 0 {
		return false
	}
	win.framePending = true
	win.frameCallback = cb

	initFrameListener()
	if win.app.api.proxyAddListener(cb, uintptr(unsafe.Pointer(&rawFrameListener[0])), unsafe.Pointer(win)) != 0 {
		win.app.api.proxyDestroy(cb)
		win.frameCallback = 0
		win.framePending = false
		return false
	}
	return true
}

// presentWithFrameCallback installs the next frame clock before acknowledging
// and presenting its commit. A failure destroys only that new callback. Close
// cancels a pending commit without turning normal shutdown into an error.
func (win *waylandWindow) presentWithFrameCallback(present func() error, ackConfigure func()) error {
	if present == nil {
		return nil
	}
	if !win.canPresent() {
		if win.closed.Load() {
			return nil
		}
		return errWaylandSurfaceNotReady
	}
	if win.framePending {
		return errWaylandFramePending
	}
	if !win.requestFrameCallback() {
		return errWaylandFrameCallback
	}
	callback := win.frameCallback
	if ackConfigure != nil {
		ackConfigure()
	}
	err := present()
	if err != nil && callback != 0 {
		if win.app != nil && win.app.api != nil {
			win.app.api.proxyDestroy(callback)
		}
		if win.frameCallback == callback {
			win.frameCallback = 0
			win.framePending = false
		}
	}
	if err == nil {
		win.surfaceState = waylandSurfaceMapped
		win.mappedOnce = true
		if win.app != nil && win.app.api != nil && win.app.api.displayFlush != nil && win.app.display != 0 {
			win.app.api.displayFlush(win.app.display)
		}
	}
	return err
}

func (win *waylandWindow) onFrameDone(callback uintptr, callbackData uint32) {
	if win.app != nil && win.app.api != nil && callback != 0 {
		win.app.api.proxyDestroy(callback)
	}
	win.frameCallback = 0
	win.framePending = false

	if win.resizeDeferred {
		win.applyPendingResize()
	} else if win.needsRedraw || win.forcePresent {
		win.renderFrame()
	}
}
