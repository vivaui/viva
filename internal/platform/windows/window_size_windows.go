package windows

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// SetSize queues a content-size request on the window thread. It reports only
// whether the request was accepted; the resulting WM_SIZE is authoritative.
func (window *nativeWindow) SetSize(width, height int) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	if width < 1 || height < 1 || width > 65535 || height > 65535 {
		return fmt.Errorf("windows: invalid window size %dx%d", width, height)
	}
	packed := uintptr(uint16(width)) | uintptr(uint16(height))<<16
	if ok, _, err := postMessage.Call(window.state.window, wmSetSize, packed, 0); ok == 0 {
		return fmt.Errorf("windows: request window size: %w", err)
	}
	return nil
}

func (state *windowState) resizeClient(width, height int) {
	if state.window == 0 || width < 1 || height < 1 || state.maximized || state.minimized || state.fullscreen.Load() {
		return
	}
	scale := state.scale
	if scale <= 0 {
		scale = 1
	}
	bounds := rect{
		Right:  int32(math.Round(float64(width) * float64(scale))),
		Bottom: int32(math.Round(float64(height) * float64(scale))),
	}
	style := uintptr(wsOverlappedWindow)
	if state.config.WindowMode == platform.WindowTitled {
		if adjustWindow.Find() == nil {
			adjustWindow.Call(uintptr(unsafe.Pointer(&bounds)), style, 0, 0, uintptr(scale*96))
		} else {
			adjustWindowLegacy.Call(uintptr(unsafe.Pointer(&bounds)), style, 0, 0)
		}
	}
	minimum := state.minimumTrackSize()
	w, h := max(minimum.X, bounds.Right-bounds.Left), max(minimum.Y, bounds.Bottom-bounds.Top)
	const noZOrder, noActivate, noMove = 0x4, 0x10, 0x2
	flags := uintptr(noZOrder | noActivate | noMove)
	var x, y int32
	monitor, _, _ := monitorFromWindow.Call(state.window, monitorNearest)
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	var current rect
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		if ok, _, _ := getWindowRect.Call(state.window, uintptr(unsafe.Pointer(&current))); ok != 0 {
			w, h = min(w, info.Work.Right-info.Work.Left), min(h, info.Work.Bottom-info.Work.Top)
			x = min(max(current.Left, info.Work.Left), info.Work.Right-w)
			y = min(max(current.Top, info.Work.Top), info.Work.Bottom-h)
			flags &^= noMove
		}
	}
	setWindowPosition.Call(state.window, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), flags)
}
