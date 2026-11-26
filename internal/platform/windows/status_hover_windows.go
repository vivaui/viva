package windows

import (
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

var statusRootWindow = user.NewProc("GetAncestor")
var statusWindowClass = user.NewProc("GetClassNameW")

func (item *statusItem) setVersion() {
	data := item.data()
	data.Version = notifyVersion4
	ok, _, _ := shellNotifyIcon.Call(notifySetVersion, uintptr(unsafe.Pointer(&data)))
	item.version4 = ok != 0
}

func (item *statusItem) SetHover(handler func(platform.StatusHover)) bool {
	if item.removed.Load() {
		return false
	}
	if handler != nil {
		if !item.version4 {
			return false
		}
		if _, ok := item.visibleTrayAnchor(item.application.status.window); !ok {
			return false
		}
	} else if item.hover == nil {
		// Disabling is idempotent even on shells without paired hover events.
		return true
	}
	item.endHover()
	previous := item.hover
	item.hover = handler
	if err := item.notify(notifyModify); err != nil {
		item.hover = previous
		return false
	}
	return true
}

func (item *statusItem) RefreshMenu() {}

func (item *statusItem) beginHover(window uintptr) {
	if item.hover == nil || item.hovering {
		return
	}
	anchor, ok := item.visibleTrayAnchor(window)
	if ok {
		item.hovering = true
		item.hover(platform.StatusHover{Anchor: anchor, Entered: true})
	}
}

func (item *statusItem) endHover() {
	if item.hovering {
		item.hovering = false
		item.hover(platform.StatusHover{})
	}
}

func (item *statusItem) visibleTrayAnchor(window uintptr) (platform.ScreenRect, bool) {
	anchor, ok := item.iconRect(window)
	if !ok {
		return anchor, false
	}
	// Only the visible taskbar supplies paired notifications. Shell overflow
	// hosts can report a rectangle but omit the matching close notification.
	x, y := int32(anchor.X+anchor.Width/2), int32(anchor.Y+anchor.Height/2)
	target, _, _ := windowFromPoint.Call(uintptr(uint32(x)) | uintptr(uint32(y))<<32)
	root, _, _ := statusRootWindow.Call(target, 2) // GA_ROOT
	var name [64]uint16
	count, _, _ := statusWindowClass.Call(root, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if count == 0 {
		return anchor, false
	}
	switch syscall.UTF16ToString(name[:count]) {
	case "Shell_TrayWnd", "Shell_SecondaryTrayWnd":
		return anchor, true
	}
	return anchor, false
}
