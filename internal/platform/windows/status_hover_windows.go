package windows

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	ninPopupOpen        = 0x0406
	ninPopupClose       = 0x0407
	statusHoverGapTimer = 2
	statusHoverGap      = 150
)

var (
	statusWindowClassName = user.NewProc("GetClassNameW")
	statusAncestor        = user.NewProc("GetAncestor")
)

type statusHover struct {
	iconID   uint32
	anchor   platform.ScreenRect
	panelID  uint32
	panel    *nativeWindow
	gapUntil int64
}

// Explorer sends the version-4 popup pair for visible taskbar icons. An
// overflow flyout may omit the close, so leave that surface to the shell.
func (item *statusItem) hoverBounds() (platform.ScreenRect, bool) {
	if !item.version4 {
		return platform.ScreenRect{}, false
	}
	r := item.anchor(item.application.status.window, 0)
	if r.Width <= 0 || r.Height <= 0 {
		return r, false
	}
	x, y := int32(r.X+r.Width/2), int32(r.Y+r.Height/2)
	window, _, _ := windowFromPoint.Call(uintptr(uint32(x)) | uintptr(uint32(y))<<32)
	root, _, _ := statusAncestor.Call(window, 2) // GA_ROOT
	var name [64]uint16
	n, _, _ := statusWindowClassName.Call(root, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	return r, n != 0 && taskbarSupportsHover(syscall.UTF16ToString(name[:n]))
}

func taskbarSupportsHover(class string) bool {
	return class == "Shell_TrayWnd" || class == "Shell_SecondaryTrayWnd"
}

func (item *statusItem) SetHoverPanel(open func(platform.ScreenRect) platform.Window) bool {
	if item.removed.Load() {
		return false
	}
	supported := true
	if open != nil {
		_, supported = item.hoverBounds()
		if !supported {
			open = nil
		}
	}
	wasEnabled := item.hoverPanel != nil
	item.hoverPanel = open
	if wasEnabled == (open != nil) {
		return supported
	}
	if open == nil {
		item.application.status.stopItemHover(item.id)
	}
	if err := item.notify(notifyModify); err != nil {
		item.hoverPanel = nil
		return false
	}
	return supported
}

func (status *statusState) openHover(item *statusItem, anchor platform.ScreenRect) {
	h := &status.hover
	if h.iconID == item.id || item.hoverPanel == nil || item.removed.Load() || item.menuOpen {
		return
	}
	h.iconID, h.anchor = item.id, anchor
	if h.panelID == item.id {
		status.cancelHoverGap()
		return
	}
	status.closeHoverPanel()
	panel, ok := item.hoverPanel(anchor).(*nativeWindow)
	if !ok || panel == nil || panel.closed.Load() {
		return
	}
	if h.iconID != item.id || item.removed.Load() || item.hoverPanel == nil {
		panel.Close()
		return
	}
	h.panelID, h.panel = item.id, panel
	panel.state.hoverStatus = status
}

func (status *statusState) leaveHover(id uint32) {
	if status.hover.iconID == id {
		status.hover.iconID = 0
		status.startHoverGap()
	}
}

func (status *statusState) startHoverGap() {
	h := &status.hover
	if h.panel != nil && h.gapUntil == 0 {
		h.gapUntil = monotonicNow() + statusHoverGap*int64(time.Millisecond)
		if timer, _, _ := setTimer.Call(status.window, statusHoverGapTimer, statusHoverGap, 0); timer == 0 {
			status.closeHoverPanel()
		}
	}
}

func (status *statusState) cancelHoverGap() {
	if status.hover.gapUntil != 0 {
		killTimer.Call(status.window, statusHoverGapTimer)
		status.hover.gapUntil = 0
	}
}

func (status *statusState) finishHoverGap() {
	h := &status.hover
	if h.gapUntil == 0 || monotonicNow() < h.gapUntil {
		return
	}
	status.cancelHoverGap()
	var cursor point
	var bounds rect
	if panel := h.panel; panel != nil && !panel.closed.Load() {
		if ok, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok != 0 {
			if h.iconID == h.panelID && hoverContains(h.anchor, cursor) {
				return
			}
			if ok, _, _ := getWindowRect.Call(panel.state.window, uintptr(unsafe.Pointer(&bounds))); ok != 0 &&
				cursor.X >= bounds.Left && cursor.X < bounds.Right && cursor.Y >= bounds.Top && cursor.Y < bounds.Bottom {
				return
			}
		}
	}
	status.closeHoverPanel()
}

func (status *statusState) closeHoverPanel() {
	h := &status.hover
	status.cancelHoverGap()
	if panel := h.panel; panel != nil {
		h.panelID, h.panel = 0, nil
		panel.state.hoverStatus = nil
		panel.Close()
	}
}

func (status *statusState) stopItemHover(id uint32) {
	status.leaveHover(id)
	if status.hover.panelID == id {
		status.closeHoverPanel()
	}
}

func (status *statusState) resetHover() {
	status.hover.iconID = 0
	status.closeHoverPanel()
}

func hoverContains(bounds platform.ScreenRect, cursor point) bool {
	x, y := float32(cursor.X), float32(cursor.Y)
	return x >= bounds.X && x < bounds.X+bounds.Width && y >= bounds.Y && y < bounds.Y+bounds.Height
}
