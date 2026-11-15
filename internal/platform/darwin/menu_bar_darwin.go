//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "menu_bar_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// menuBarActions is what the current menu bar's entries run, by the id the C
// menu target carries. The menu bar is built on the event-loop thread and its
// entries run there too.
var menuBarActions struct {
	sync.Mutex
	actions map[int32]func()
}

//export vivaMenuAction
func vivaMenuAction(action C.int32_t) {
	menuBarActions.Lock()
	fn := menuBarActions.actions[int32(action)]
	menuBarActions.Unlock()
	if fn != nil {
		fn()
	}
}

// SetMenuBar installs the application's own menus between the application menu
// and the Window menu.
func (application *nativeApplication) SetMenuBar(menus []platform.MenuBarMenu) error {
	menuBarActions.Lock()
	menuBarActions.actions = make(map[int32]func())
	menuBarActions.Unlock()
	var next int32
	C.viva_menu_bar_begin()
	for _, menu := range menus {
		title := C.CString(menu.Title)
		handle := C.viva_menu_create(title)
		C.free(unsafe.Pointer(title))
		addMenuBarItems(handle, menu.Items, &next)
		C.viva_menu_bar_add(handle)
	}
	C.viva_menu_bar_install()
	return nil
}

func addMenuBarItems(menu unsafe.Pointer, items []platform.MenuBarItem, next *int32) {
	for _, item := range items {
		if item.Separator {
			C.viva_menu_add_separator(menu)
			continue
		}
		title := C.CString(item.Title)
		if len(item.Items) > 0 {
			submenu := C.viva_menu_create(title)
			addMenuBarItems(submenu, item.Items, next)
			C.viva_menu_add_submenu(menu, title, submenu)
			C.free(unsafe.Pointer(title))
			continue
		}
		key := C.CString(item.Key)
		action := *next
		*next++
		if item.OnSelect != nil {
			menuBarActions.Lock()
			menuBarActions.actions[action] = item.OnSelect
			menuBarActions.Unlock()
		}
		enabled := C.int(0)
		if !item.Disabled {
			enabled = 1
		}
		checked := C.int(0)
		if item.Checked {
			checked = 1
		}
		C.viva_menu_add_item(menu, title, key, C.uint32_t(item.Modifiers), enabled, checked, C.int32_t(action))
		C.free(unsafe.Pointer(title))
		C.free(unsafe.Pointer(key))
	}
}
