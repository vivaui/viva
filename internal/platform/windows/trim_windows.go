package windows

import (
	"time"
	"unsafe"
)

// A graphics driver keeps what a frame needed (staging copies of uploads,
// destroyed textures it has not yet reused) until it is told it may let go, and
// on an integrated GPU that memory is the process's own. A window that is idle
// or not visible tells it with IDXGIDevice3::Trim, which Microsoft asks every
// Direct3D 11 application to call when it is suspended or has nothing to draw.
//
// The timer runs once after a burst of painting and then stops: an idle window
// does nothing further, and the next paint arms it again.

const (
	trimTimerID = 2
	// trimIdle is how long a window must go without presenting before its driver
	// memory is trimmed.
	trimIdle = 2 * time.Second
)

// dxgiDevice3IID is IID_IDXGIDevice3.
var dxgiDevice3IID = guid{0x6007896c, 0x3244, 0x4afd, [8]byte{0xbf, 0x18, 0xa6, 0xd3, 0xbe, 0xda, 0x50, 0x23}}

// dxgiDevice3Trim is the vtable index of IDXGIDevice3::Trim: IUnknown (3),
// IDXGIObject (4), IDXGIDevice (5), IDXGIDevice1 (2), IDXGIDevice2 (3).
const dxgiDevice3Trim = 17

// trim flushes the device's deferred work and asks the driver to release the
// memory it caches. A renderer without a device does nothing.
func (renderer *renderer) trim() {
	renderer.labelPictures.evict(0)
	if renderer.device == 0 || renderer.context == 0 {
		return
	}
	comCall(renderer.context, 111) // ID3D11DeviceContext::Flush: lets destroyed objects go
	var dxgi uintptr
	if hresult(comCall(renderer.device, 0, uintptr(unsafe.Pointer(&dxgiDevice3IID)), uintptr(unsafe.Pointer(&dxgi)))) != nil {
		return // Windows 8.1 and earlier have no Trim
	}
	comCall(dxgi, dxgiDevice3Trim)
	release(&dxgi)
}

// scheduleTrim arms the idle timer after a present, once per burst of painting.
func (state *windowState) scheduleTrim() {
	state.lastPresent = time.Now()
	if state.trimArmed || state.window == 0 {
		return
	}
	if timer, _, _ := setTimer.Call(state.window, trimTimerID, uintptr(trimIdle/time.Millisecond), 0); timer != 0 {
		state.trimArmed = true
	}
}

// handleTrimTimer trims once the window has really been idle for trimIdle, and
// otherwise waits for the rest of it.
func (state *windowState) handleTrimTimer() {
	if remaining := trimIdle - time.Since(state.lastPresent); remaining > 0 {
		setTimer.Call(state.window, trimTimerID, uintptr(remaining/time.Millisecond)+1, 0)
		return
	}
	killTimer.Call(state.window, trimTimerID)
	state.trimArmed = false
	state.renderer.trim()
}
