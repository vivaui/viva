package windows

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const wmGetMinMaxInfo = 0x0024

// minMaxInfo is Win32 MINMAXINFO. Only MinTrack is changed: the OS retains its
// maximize, snap, and maximum tracking geometry.
type minMaxInfo struct {
	Reserved, MaxSize, MaxPosition, MinTrack, MaxTrack point
}

func (state *windowState) minimumTrackSize() point {
	if state.config.MinWidth <= 0 && state.config.MinHeight <= 0 {
		return point{}
	}
	var frame rect
	if state.config.WindowMode == platform.WindowTitled && !state.decoration.removed {
		if adjustWindow.Find() == nil {
			adjustWindow.Call(uintptr(unsafe.Pointer(&frame)), wsOverlappedWindow, 0, 0, uintptr(max(state.scale, 1)*96))
		} else {
			adjustWindowLegacy.Call(uintptr(unsafe.Pointer(&frame)), wsOverlappedWindow, 0, 0)
		}
	}
	extraWidth, extraHeight := int(frame.Right-frame.Left), int(frame.Bottom-frame.Top)
	var availableWidth, availableHeight int
	monitor, _, _ := monitorFromWindow.Call(state.window, monitorNearest)
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		availableWidth = max(1, int(info.Work.Right-info.Work.Left)-extraWidth)
		availableHeight = max(1, int(info.Work.Bottom-info.Work.Top)-extraHeight)
	}
	width, height := state.config.MinimumContentSize(state.scale, availableWidth, availableHeight)
	var minimum point
	if width > 0 {
		minimum.X = int32(width + extraWidth)
	}
	if height > 0 {
		minimum.Y = int32(height + extraHeight)
	}
	return minimum
}

func (state *windowState) constrainTracking(info *minMaxInfo) {
	minimum := state.minimumTrackSize()
	info.MinTrack.X = max(info.MinTrack.X, minimum.X)
	info.MinTrack.Y = max(info.MinTrack.Y, minimum.Y)
}
