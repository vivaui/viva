package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/keymap"
)

// Menu is one top-level menu of the application's own menu bar entries, shown
// between the application menu and the Window menu macOS supplies. Windows and
// Linux do not have an application menu bar in this API yet: SetMenuBar returns
// ErrUnsupported there, and an application uses the in-window components.Menubar.
type Menu struct {
	Title string
	Items []MenuBarItem
}

// MenuBarItem is one entry of a Menu.
type MenuBarItem struct {
	Title string
	// Separator makes the entry a dividing line; its other fields are ignored.
	Separator bool
	// Disabled shows the entry dimmed and not selectable.
	Disabled bool
	// Checked puts a check mark beside the entry. Set it when the menu is built;
	// call SetMenuBar again to show a new state.
	Checked bool
	// Shortcut is the key the system offers for the entry. Only a character key
	// is represented in a menu; a chord with a named key shows no shortcut, and
	// the application still receives that key through its own handler.
	Shortcut keymap.Chord
	// Submenu, when not empty, makes the entry open a menu of these.
	Submenu []MenuBarItem
	// OnSelect runs on the event-loop thread when the user chooses the entry.
	OnSelect func()
}

// SetMenuBar replaces the application's own menus. It is macOS-only today and
// returns an error wrapping ErrUnsupported elsewhere. Call it on the
// event-loop thread; the menus are built now, so an application that shows
// changing state (a check mark) calls it again with the new value.
//
// The standard application and Window menus stay where the system puts them;
// these menus are inserted between them.
func (application *Application) SetMenuBar(menus []Menu) error {
	if application == nil || application.host == nil {
		return errors.New("app: the application is not running")
	}
	setter, ok := application.host.(interface{ SetMenuBar([]Menu) error })
	if !ok {
		return fmt.Errorf("%w: the application menu bar", ErrUnsupported)
	}
	return setter.SetMenuBar(menus)
}

// platform menus carry a key as a character and a modifier mask, so the
// platform layer does not depend on keymap.
func platformMenuBar(menus []Menu) []platform.MenuBarMenu {
	out := make([]platform.MenuBarMenu, len(menus))
	for index, menu := range menus {
		out[index] = platform.MenuBarMenu{Title: menu.Title, Items: platformMenuItems(menu.Items)}
	}
	return out
}

func platformMenuItems(items []MenuBarItem) []platform.MenuBarItem {
	out := make([]platform.MenuBarItem, len(items))
	for index, item := range items {
		key := ""
		var modifiers uint32
		if item.Shortcut.Char != 0 {
			key = strings.ToLower(string(item.Shortcut.Char))
			modifiers = menuModifiers(item.Shortcut.Modifiers)
		}
		out[index] = platform.MenuBarItem{
			Title: item.Title, Separator: item.Separator, Disabled: item.Disabled, Checked: item.Checked,
			Key: key, Modifiers: modifiers, OnSelect: item.OnSelect, Items: platformMenuItems(item.Submenu),
		}
	}
	return out
}

func menuModifiers(modifiers gfx.Modifiers) uint32 {
	var out uint32
	if modifiers&gfx.ModifierMeta != 0 {
		out |= 1 << 20 // NSEventModifierFlagCommand
	}
	if modifiers&gfx.ModifierShift != 0 {
		out |= 1 << 17 // NSEventModifierFlagShift
	}
	if modifiers&gfx.ModifierAlt != 0 {
		out |= 1 << 19 // NSEventModifierFlagOption
	}
	if modifiers&gfx.ModifierControl != 0 {
		out |= 1 << 18 // NSEventModifierFlagControl
	}
	return out
}
