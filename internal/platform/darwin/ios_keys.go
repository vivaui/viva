//go:build ios

package darwin

/*
#include "native.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
)

// UIKeyModifierFlags, as UIKey reports them.
const (
	uiKeyModifierShift   = 1 << 17
	uiKeyModifierControl = 1 << 18
	uiKeyModifierAlt     = 1 << 19
	uiKeyModifierCommand = 1 << 20
)

// keyOfUsage is the key a USB HID usage of the keyboard page names, for the keys that
// are not text: what a connected keyboard reports in UIKey.keyCode.
func keyOfUsage(usage int) gfx.Key {
	switch {
	case usage >= 0x3A && usage <= 0x45: // F1 to F12
		return gfx.KeyF1 + gfx.Key(usage-0x3A)
	}
	switch usage {
	case 0x28, 0x58: // Return, keypad Enter
		return gfx.KeyEnter
	case 0x29:
		return gfx.KeyEscape
	case 0x2A:
		return gfx.KeyBackspace
	case 0x2B:
		return gfx.KeyTab
	case 0x2C:
		return gfx.KeySpace
	case 0x49:
		return gfx.KeyInsert
	case 0x4A:
		return gfx.KeyHome
	case 0x4B:
		return gfx.KeyPageUp
	case 0x4C:
		return gfx.KeyDelete
	case 0x4D:
		return gfx.KeyEnd
	case 0x4E:
		return gfx.KeyPageDown
	case 0x4F:
		return gfx.KeyRight
	case 0x50:
		return gfx.KeyLeft
	case 0x51:
		return gfx.KeyDown
	case 0x52:
		return gfx.KeyUp
	}
	return gfx.KeyUnknown
}

func keyModifiers(flags int) input.Modifiers {
	var modifiers input.Modifiers
	if flags&uiKeyModifierShift != 0 {
		modifiers |= input.ModifierShift
	}
	if flags&uiKeyModifierControl != 0 {
		modifiers |= input.ModifierControl
	}
	if flags&uiKeyModifierAlt != 0 {
		modifiers |= input.ModifierAlt
	}
	if flags&uiKeyModifierCommand != 0 {
		modifiers |= input.ModifierMeta
	}
	return modifiers
}

// vivaIOSKeyEvent is a key of a connected keyboard. A key that is not text is a key
// event; a chord with Control or Command and a character is offered as a shortcut,
// and is a key event only if nothing took it. What a key types is the text input's.
//
//export vivaIOSKeyEvent
func vivaIOSKeyEvent(handle C.uintptr_t, view unsafe.Pointer, usage C.int, flags C.int, repeat C.int, utf8 *C.uchar, utf8Length C.size_t) {
	state, ok := runStateFor(handle)
	if !ok || state.closed {
		return
	}
	modifiers := keyModifiers(int(flags))
	key := keyOfUsage(int(usage))
	chord := modifiers&(input.ModifierControl|input.ModifierMeta) != 0 && utf8Length > 0
	if chord && vivaShortcutEvent(handle, view, C.int(key), C.int(modifiers), repeat, utf8, utf8Length) != 0 {
		return
	}
	if key == gfx.KeyUnknown && !chord {
		return
	}
	deliverKey(handle, view, KeyEvent{Key: key, Modifiers: modifiers, Repeat: repeat != 0}, utf8, utf8Length)
}

// vivaIOSVisibility follows the application going behind another and coming back.
// Coming back, the window is drawn again: what the layer holds may not be the scene.
//
//export vivaIOSVisibility
func vivaIOSVisibility(handle C.uintptr_t, view unsafe.Pointer, visible C.int) {
	state, ok := runStateFor(handle)
	if !ok || state.closed {
		return
	}
	vivaWindowOccluded(handle, view, visible)
	if visible == 0 || state.redrawHandler == nil {
		return
	}
	frame, err := state.redrawHandler()
	if err == nil {
		err = presentFrame(state, view, frame, false)
	}
	if err != nil {
		state.err = fmt.Errorf("darwin: drawing after the application returned: %w", err)
		C.viva_view_close(view)
	}
}
