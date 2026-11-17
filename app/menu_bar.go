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
	// the application still receives that key through its own key handler.
	// Before OnSelect runs, the chord is offered as pressing it would deliver
	// it: a character chord to the shortcut handler, a named key to the key
	// handler, with the focused control as the target. A handler that takes it
	// owns the command.
	Shortcut keymap.Chord
	// Submenu, when not empty, makes the entry open a menu of these.
	Submenu []MenuBarItem
	// OnSelect runs on the event-loop thread when the user chooses the entry and
	// returns what the command did, as a key handler does. The runtime applies
	// the focus, clipboard and activation it names to the window the user is
	// working in and asks for the repaint it requests, so an entry that runs the
	// same command as a shortcut moves the keyboard the same way. Return the
	// zero result for an entry that changes nothing.
	OnSelect func() gfx.EventResult
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
	return setter.SetMenuBar(application.menuBar(menus))
}

// menuBar copies menus, wrapping every action so the runtime carries out what
// it did on the window the user is working in. A native menu runs its entry
// outside the window's event handlers, so the result would otherwise be
// dropped, and a menu entry that runs the same command as a shortcut would not
// move the keyboard, write the clipboard or repaint.
func (application *Application) menuBar(menus []Menu) []Menu {
	out := make([]Menu, len(menus))
	for index, menu := range menus {
		out[index] = Menu{Title: menu.Title, Items: application.menuBarItems(menu.Items)}
	}
	return out
}

func (application *Application) menuBarItems(items []MenuBarItem) []MenuBarItem {
	out := make([]MenuBarItem, len(items))
	for index, item := range items {
		item.Submenu = application.menuBarItems(item.Submenu)
		if selectItem := item.OnSelect; selectItem != nil {
			shortcut := item.Shortcut
			item.OnSelect = func() gfx.EventResult {
				// A menu's key equivalent is a fallback in AppKit: the focused
				// content sees the chord first, and the menu entry runs only when
				// nothing there takes it. A choice from the open menu goes through
				// the same handler, so a click and its shortcut run one command.
				if result, handled := application.runMenuShortcut(shortcut); handled {
					application.applyMenuResult(result)
					return result
				}
				result := selectItem()
				application.applyMenuResult(result)
				return result
			}
		}
		out[index] = item
	}
	return out
}

// runMenuShortcut offers a menu entry's chord to the shortcut handler of the
// window the user is working in, with the control that holds the keyboard as
// the target, and reports whether the handler took it.
func (application *Application) runMenuShortcut(chord keymap.Chord) (gfx.EventResult, bool) {
	window := application.activeWindow()
	if window == nil || window.runShortcut == nil {
		return gfx.EventResult{}, false
	}
	return window.runShortcut(chord)
}

// applyMenuResult carries out a menu action's result on the active window:
// the focus, clipboard and activation it names, and a repaint when it asks for
// one. A window created without Config.Invalidator has no waker, so the runtime
// cannot ask it for the frame that shows the result; such an application asks
// for its own repaint.
func (application *Application) applyMenuResult(result gfx.EventResult) {
	window := application.activeWindow()
	if window == nil || window.applyResult == nil {
		return
	}
	changed := window.applyResult(result)
	if (changed || result.Repaint) && window.requestRedraw != nil {
		window.requestRedraw()
	}
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
		var onSelect func()
		if item.OnSelect != nil {
			selectItem := item.OnSelect
			onSelect = func() { selectItem() }
		}
		out[index] = platform.MenuBarItem{
			Title: item.Title, Separator: item.Separator, Disabled: item.Disabled, Checked: item.Checked,
			Key: key, Modifiers: modifiers, OnSelect: onSelect, Items: platformMenuItems(item.Submenu),
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
