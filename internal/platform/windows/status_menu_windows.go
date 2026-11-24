package windows

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfPopup     = 0x0010
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100
	tpmBottomAlign = 0x0020

	wmNull = 0

	// The ordinals uxtheme.dll exports for the two calls that let an
	// application's menus follow the system's dark setting. Windows documents
	// neither; every release since 1809 that draws dark menus has them.
	uxthemeSetPreferredAppMode = 135
	uxthemeFlushMenuThemes     = 136
	preferredAppModeAllowDark  = 1
)

var createPopupMenu = user.NewProc("CreatePopupMenu")
var appendMenu = user.NewProc("AppendMenuW")
var destroyMenu = user.NewProc("DestroyMenu")
var trackPopupMenuEx = user.NewProc("TrackPopupMenuEx")
var getProcAddress = kernel.NewProc("GetProcAddress")
var uxtheme = systemDLL("uxtheme.dll")
var allowDarkMenusOnce sync.Once

// allowDarkMenus lets the system draw the application's menus dark when the
// system is set to dark, as it does for its own. Without it a menu stays light
// beside a dark taskbar. It does nothing where the system lacks the calls.
func allowDarkMenus() {
	allowDarkMenusOnce.Do(func() {
		if uxtheme.Load() != nil {
			return
		}
		module := uxtheme.Handle()
		mode, _, _ := getProcAddress.Call(module, uxthemeSetPreferredAppMode)
		flush, _, _ := getProcAddress.Call(module, uxthemeFlushMenuThemes)
		if mode == 0 || flush == 0 {
			return
		}
		syscall.SyscallN(mode, preferredAppModeAllowDark)
		syscall.SyscallN(flush)
	})
}

// menuTitle is a title as a menu item shows it: '&' would mark a shortcut, and
// a title is shown as written.
func menuTitle(title string) *uint16 {
	escaped := make([]byte, 0, len(title))
	for index := 0; index < len(title); index++ {
		switch title[index] {
		case '&':
			escaped = append(escaped, '&', '&')
		case 0:
		default:
			escaped = append(escaped, title[index])
		}
	}
	text, _ := syscall.UTF16PtrFromString(string(escaped))
	return text
}

// buildMenu fills menu with the count entries that start at *cursor, taking each
// submenu's entries from the ones that follow its parent.
func buildMenu(menu uintptr, entries []platform.MenuEntry, cursor *int, count int) {
	for index := 0; index < count; index++ {
		entry := entries[*cursor]
		*cursor++
		flags := uintptr(mfString)
		if entry.Disabled {
			flags |= mfGrayed
		}
		if entry.Checked {
			flags |= mfChecked
		}
		switch {
		case entry.Separator:
			appendMenu.Call(menu, mfSeparator, 0, 0)
		case entry.Children > 0:
			submenu, _, _ := createPopupMenu.Call()
			buildMenu(submenu, entries, cursor, entry.Children)
			appendMenu.Call(menu, flags|mfPopup, submenu, uintptr(unsafe.Pointer(menuTitle(entry.Title))))
		default:
			appendMenu.Call(menu, flags, uintptr(entry.ID), uintptr(unsafe.Pointer(menuTitle(entry.Title))))
		}
	}
}

// popMenu shows the item's menu above its icon, and runs the entry the user
// chooses once the menu has closed. window is the one that receives the item's
// messages; it must be the foreground window while the menu is up, or a click
// elsewhere would not close it.
func (item *statusItem) popMenu(window uintptr, anchor platform.ScreenRect) {
	if item.menuOpen {
		return
	}
	item.menuOpen = true
	defer func() { item.menuOpen = false }()
	entries := platform.NumberMenu(item.menu())
	if len(entries) == 0 {
		return
	}
	allowDarkMenus()
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	cursor := 0
	buildMenu(menu, entries, &cursor, platform.TopLevel(entries))
	setForegroundWindow.Call(window)
	x, y := int32(anchor.X+anchor.Width/2), int32(anchor.Y)
	chosen, _, _ := trackPopupMenuEx.Call(menu, tpmRightButton|tpmNoNotify|tpmReturnCmd|tpmBottomAlign, uintptr(x), uintptr(y), window, 0)
	// The documented way to end a menu that belongs to a notification icon.
	postMessage.Call(window, wmNull, 0, 0)
	platform.Select(entries, int(chosen))
}
