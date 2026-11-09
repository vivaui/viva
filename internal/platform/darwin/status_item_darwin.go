//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "native.h"
#include "status_item_darwin.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

type nativeStatusItem struct {
	token          unsafe.Pointer
	handle         cgo.Handle
	removed        atomic.Bool
	onClick        func(platform.StatusClick)
	menu           func() []platform.MenuItem
	currentEntries []platform.MenuEntry
}

// AddStatusItem puts an icon in the menu bar. Press events arrive on the
// event-loop thread.
func (application *nativeApplication) AddStatusItem(config platform.StatusItemConfig) (platform.StatusItem, error) {
	if !config.Icon.Valid() {
		return nil, errors.New("darwin: status icon pixels do not match its size")
	}
	item := &nativeStatusItem{onClick: config.OnClick, menu: config.Menu}
	item.handle = cgo.NewHandle(item)
	tooltip := C.CString(config.Tooltip)
	defer C.free(unsafe.Pointer(tooltip))
	pixels := C.CBytes(config.Icon.RGBA)
	defer C.free(pixels)
	hasMenu := config.Menu != nil
	item.token = C.viva_status_item_create(tooltip, nativeStatusIcon(config.Icon, pixels), C.uintptr_t(item.handle), C.int(boolInt(hasMenu)))
	if item.token == nil {
		item.handle.Delete()
		return nil, errors.New("darwin: failed to create the status item")
	}
	return item, nil
}

func nativeStatusIcon(icon platform.StatusIcon, pixels unsafe.Pointer) C.viva_status_icon {
	return C.viva_status_icon{size: C.int(icon.Size), rgba: (*C.uchar)(pixels), template_image: C.int(boolInt(icon.Template))}
}

func (item *nativeStatusItem) SetIcon(icon platform.StatusIcon) error {
	if !icon.Valid() {
		return errors.New("darwin: status icon pixels do not match its size")
	}
	if item.removed.Load() {
		return errors.New("darwin: status item is removed")
	}
	pixels := C.CBytes(icon.RGBA)
	defer C.free(pixels)
	if C.viva_status_item_set_icon(item.token, nativeStatusIcon(icon, pixels)) == 0 {
		return errors.New("darwin: failed to replace the status icon")
	}
	return nil
}

func (item *nativeStatusItem) SetTooltip(tooltip string) {
	if item.removed.Load() {
		return
	}
	text := C.CString(tooltip)
	defer C.free(unsafe.Pointer(text))
	C.viva_status_item_set_tooltip(item.token, text)
}

// Remove posts the removal to the event loop, so it may be called anywhere.
func (item *nativeStatusItem) Remove() {
	if !item.removed.Swap(true) {
		C.viva_application_post_status_remove(item.token)
	}
}

//export vivaStatusItemClicked
func vivaStatusItemClicked(handle C.uintptr_t, button C.int, x, y, width, height C.float, dismissed C.int) {
	item := cgo.Handle(handle).Value().(*nativeStatusItem)
	if item.removed.Load() || item.onClick == nil {
		return
	}
	pointer := gfx.PointerButtonPrimary
	if button == 2 {
		pointer = gfx.PointerButtonSecondary
	}
	item.onClick(platform.StatusClick{
		Button:         pointer,
		Anchor:         platform.ScreenRect{X: float32(x), Y: float32(y), Width: float32(width), Height: float32(height)},
		PanelDismissed: dismissed != 0,
	})
}

//export vivaStatusItemUpdateMenu
func vivaStatusItemUpdateMenu(handle C.uintptr_t) {
	item := cgo.Handle(handle).Value().(*nativeStatusItem)
	if item.removed.Load() || item.menu == nil {
		return
	}
	item.updateMenu()
}

func (item *nativeStatusItem) updateMenu() {
	entries := platform.NumberMenu(item.menu())
	item.currentEntries = entries
	if len(entries) == 0 {
		C.viva_status_item_set_menu_entries(item.token, nil, 0)
		return
	}
	size := unsafe.Sizeof(C.viva_menu_entry{})
	memory := C.malloc(C.size_t(size) * C.size_t(len(entries)))
	defer C.free(memory)
	native := unsafe.Slice((*C.viva_menu_entry)(memory), len(entries))
	for index, entry := range entries {
		native[index] = C.viva_menu_entry{id: C.int(entry.ID), disabled: C.int(boolInt(entry.Disabled)), checked: C.int(boolInt(entry.Checked)), children: C.int(entry.Children)}
		if !entry.Separator {
			native[index].title = C.CString(entry.Title)
		}
	}
	defer func() {
		for _, entry := range native {
			if entry.title != nil {
				C.free(unsafe.Pointer(entry.title))
			}
		}
	}()
	C.viva_status_item_set_menu_entries(item.token, &native[0], C.int(platform.TopLevel(entries)))
}

//export vivaStatusItemMenuChosen
func vivaStatusItemMenuChosen(handle C.uintptr_t, id C.int) {
	item := cgo.Handle(handle).Value().(*nativeStatusItem)
	if item.removed.Load() {
		return
	}
	platform.Select(item.currentEntries, int(id))
}

// Quit closes every window, close requests notwithstanding, and ends the loop.
func (application *nativeApplication) Quit() { C.viva_application_quit() }
