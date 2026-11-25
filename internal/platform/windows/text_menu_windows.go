package windows

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// textMenuKeys is the chord shown at the end of a text menu's row, as the
// system's own edit menus show it.
func textMenuKeys(command platform.TextMenuCommand) string {
	switch command {
	case platform.TextMenuCut:
		return "\tCtrl+X"
	case platform.TextMenuCopy:
		return "\tCtrl+C"
	case platform.TextMenuPaste:
		return "\tCtrl+V"
	case platform.TextMenuSelectAll:
		return "\tCtrl+A"
	}
	return ""
}

// popTextMenu shows the text menu at the screen point (x, y) in device pixels
// and returns the command the user chose, or TextMenuNone. The window that
// receives its messages must be the foreground window while the menu is up, or a
// click elsewhere would not close it.
func popTextMenu(window uintptr, items []platform.TextMenuItem, x, y int32) platform.TextMenuCommand {
	if len(items) == 0 {
		return platform.TextMenuNone
	}
	allowDarkMenus()
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return platform.TextMenuNone
	}
	defer destroyMenu.Call(menu)
	for _, item := range items {
		if item.Command == platform.TextMenuNone {
			appendMenu.Call(menu, mfSeparator, 0, 0)
			continue
		}
		flags := uintptr(mfString)
		if item.Disabled {
			flags |= mfGrayed
		}
		appendMenu.Call(menu, flags, uintptr(item.Command), uintptr(unsafe.Pointer(menuTitle(item.Title+textMenuKeys(item.Command)))))
	}
	setForegroundWindow.Call(window)
	chosen, _, _ := trackPopupMenuEx.Call(menu, tpmRightButton|tpmNoNotify|tpmReturnCmd, uintptr(x), uintptr(y), window, 0)
	// The documented way to end a menu that belongs to a window.
	postMessage.Call(window, wmNull, 0, 0)
	return platform.TextMenuCommand(chosen)
}

// showTextMenu opens the runtime's text menu for the press that has just been
// released, at the pointer, as the system's own edit controls do, and runs the
// command chosen. The pointer capture a press holds is let go first: a menu that
// tracks the pointer cannot while another window owns it.
func (state *windowState) showTextMenu(window uintptr, at point) {
	request, ok := state.config.TakeTextMenu()
	if !ok || request.Perform == nil {
		return
	}
	releaseCapture.Call()
	chosen := popTextMenu(window, request.Items, at.X, at.Y)
	if chosen == platform.TextMenuNone {
		return
	}
	frame, err := request.Perform(chosen)
	state.frame(frame, err)
}
