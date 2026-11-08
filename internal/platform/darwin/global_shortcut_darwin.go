package darwin

/*
#cgo LDFLAGS: -framework Carbon
#include <Carbon/Carbon.h>
#include <stdint.h>
int32_t viva_global_shortcut_register(uint32_t code, uint32_t modifiers, uint32_t identifier, EventHotKeyRef *reference);
void viva_global_shortcut_unregister(EventHotKeyRef reference);
int32_t viva_global_shortcut_test_event(uint32_t identifier, uint32_t kind);
*/
import "C"

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/vivaui/viva/ui"
)

type carbonShortcut struct {
	id     uint32
	ref    C.EventHotKeyRef
	events chan struct{}
	stop   chan struct{}
	closed atomic.Bool
	down   bool // main thread only
}

var nextCarbonShortcut atomic.Uint32
var carbonShortcuts sync.Map // uint32 -> *carbonShortcut

// RegisterGlobalShortcut uses the system hotkey API. It does not read other
// keystrokes or need Accessibility or Input Monitoring permissions.
func RegisterGlobalShortcut(modifiers ui.Modifiers, key string, pressed func()) (func(), error) {
	binding, err := registerCarbonShortcut(modifiers, key, pressed)
	if err != nil {
		return nil, err
	}
	return binding.close, nil
}

func registerCarbonShortcut(modifiers ui.Modifiers, key string, pressed func()) (*carbonShortcut, error) {
	code, ok := carbonKeycode(key)
	if !ok {
		return nil, fmt.Errorf("darwin: global shortcut key %q is unsupported", key)
	}
	mask := C.uint32_t(0)
	if modifiers&ui.ModifierShift != 0 {
		mask |= C.shiftKey
	}
	if modifiers&ui.ModifierControl != 0 {
		mask |= C.controlKey
	}
	if modifiers&ui.ModifierAlt != 0 {
		mask |= C.optionKey
	}
	if modifiers&ui.ModifierMeta != 0 {
		mask |= C.cmdKey
	}
	id := nextCarbonShortcut.Add(1)
	binding := &carbonShortcut{id: id, events: make(chan struct{}, 1), stop: make(chan struct{})}
	carbonShortcuts.Store(id, binding)
	var ref C.EventHotKeyRef
	if status := C.viva_global_shortcut_register(code, mask, C.uint32_t(id), &ref); status != 0 {
		carbonShortcuts.Delete(id)
		return nil, fmt.Errorf("darwin: register global shortcut (OSStatus %d)", status)
	}
	binding.ref = ref
	go func() {
		for {
			select {
			case <-binding.stop:
				return
			case <-binding.events:
				select {
				case <-binding.stop:
					return
				default:
				}
				pressed()
			}
		}
	}()
	return binding, nil
}

func (b *carbonShortcut) close() {
	// Do not make the AppKit thread wait for a worker that is itself waiting
	// to unregister on AppKit. Only the first caller performs the release.
	if b.closed.Swap(true) {
		return
	}
	carbonShortcuts.Delete(b.id)
	close(b.stop)
	C.viva_global_shortcut_unregister(b.ref)
}

//export vivaGlobalShortcutEvent
func vivaGlobalShortcutEvent(identifier, kind C.uint32_t) C.int {
	value, ok := carbonShortcuts.Load(uint32(identifier))
	if !ok {
		return 0
	}
	b := value.(*carbonShortcut)
	if kind == C.kEventHotKeyReleased {
		b.down = false
	} else if !b.down {
		b.down = true
		select {
		case b.events <- struct{}{}:
		default:
		}
	}
	return 1
}

func carbonKeycode(key string) (C.uint32_t, bool) {
	letters := [...]C.uint32_t{C.kVK_ANSI_A, C.kVK_ANSI_B, C.kVK_ANSI_C, C.kVK_ANSI_D, C.kVK_ANSI_E, C.kVK_ANSI_F,
		C.kVK_ANSI_G, C.kVK_ANSI_H, C.kVK_ANSI_I, C.kVK_ANSI_J, C.kVK_ANSI_K, C.kVK_ANSI_L, C.kVK_ANSI_M,
		C.kVK_ANSI_N, C.kVK_ANSI_O, C.kVK_ANSI_P, C.kVK_ANSI_Q, C.kVK_ANSI_R, C.kVK_ANSI_S, C.kVK_ANSI_T,
		C.kVK_ANSI_U, C.kVK_ANSI_V, C.kVK_ANSI_W, C.kVK_ANSI_X, C.kVK_ANSI_Y, C.kVK_ANSI_Z}
	digits := [...]C.uint32_t{C.kVK_ANSI_0, C.kVK_ANSI_1, C.kVK_ANSI_2, C.kVK_ANSI_3, C.kVK_ANSI_4,
		C.kVK_ANSI_5, C.kVK_ANSI_6, C.kVK_ANSI_7, C.kVK_ANSI_8, C.kVK_ANSI_9}
	functions := [...]C.uint32_t{C.kVK_F1, C.kVK_F2, C.kVK_F3, C.kVK_F4, C.kVK_F5, C.kVK_F6, C.kVK_F7,
		C.kVK_F8, C.kVK_F9, C.kVK_F10, C.kVK_F11, C.kVK_F12, C.kVK_F13, C.kVK_F14, C.kVK_F15, C.kVK_F16,
		C.kVK_F17, C.kVK_F18, C.kVK_F19, C.kVK_F20}
	if len(key) == 1 {
		c := key[0]
		if 'A' <= c && c <= 'Z' {
			return letters[c-'A'], true
		}
		if '0' <= c && c <= '9' {
			return digits[c-'0'], true
		}
	}
	if len(key) > 1 && key[0] == 'F' {
		n, err := strconv.Atoi(key[1:])
		if err == nil && n >= 1 && n <= len(functions) {
			return functions[n-1], true
		}
	}
	switch key {
	case "SPACE":
		return C.kVK_Space, true
	case "ENTER":
		return C.kVK_Return, true
	case "ESCAPE":
		return C.kVK_Escape, true
	case "TAB":
		return C.kVK_Tab, true
	}
	return 0, false
}

// sendCarbonShortcutTestEvent exercises the real application event handler
// without injecting keyboard input into the host desktop.
func sendCarbonShortcutTestEvent(id uint32, down bool) int32 {
	kind := C.uint32_t(C.kEventHotKeyReleased)
	if down {
		kind = C.kEventHotKeyPressed
	}
	return int32(C.viva_global_shortcut_test_event(C.uint32_t(id), kind))
}
