package windows

import (
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

var monitorEffectiveDPI = syscall.NewLazyDLL("shcore.dll").NewProc("GetDpiForMonitor")

func (state *windowState) placeNotification() {
	config := state.config
	monitor := cursorMonitor()
	if config.HasOrigin {
		x, y := int32(config.Origin.X), int32(config.Origin.Y)
		monitor, _, _ = monitorFromPoint.Call(uintptr(uint32(x))|uintptr(uint32(y))<<32, monitorNearest)
	}
	var info monitorInfo
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return
	}
	scale := state.scale
	if !state.scaleFixed && monitorEffectiveDPI.Find() == nil {
		var dpiX, dpiY uint32
		hr, _, _ := monitorEffectiveDPI.Call(monitor, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
		if int32(hr) == 0 && dpiX != 0 {
			scale = float32(dpiX) / 96
		}
	}
	area := platform.ScreenRect{
		X: float32(info.Work.Left), Y: float32(info.Work.Top),
		Width: float32(info.Work.Right - info.Work.Left), Height: float32(info.Work.Bottom - info.Work.Top),
	}
	width := min(float32(int32(float32(config.Width)*scale)), area.Width)
	height := min(float32(int32(float32(config.Height)*scale)), area.Height)
	x, y := platform.PlaceNotification(width, height, 12*scale, area)
	if config.HasOrigin {
		x, y = platform.RestoreNotification(config.Origin.X, config.Origin.Y, width, height, area)
	}
	// The selected monitor's DPI is already included. A synchronous
	// WM_DPICHANGED still updates the scale but must not move the window again.
	state.placingNotification = true
	setWindowPosition.Call(state.window, hwndTopmost, uintptr(int32(x)), uintptr(int32(y)), uintptr(int32(width)), uintptr(int32(height)), 0x10)
	state.placingNotification = false
}
