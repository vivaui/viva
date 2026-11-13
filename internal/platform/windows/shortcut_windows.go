package windows

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

var (
	toUnicodeEx       = user.NewProc("ToUnicodeEx")
	getKeyboardLayout = user.NewProc("GetKeyboardLayout")
)

const toUnicodeKeepState = 0x4 // do not change the keyboard state: a dead key stays pending

// keyIsDown reports whether a virtual key is held, as of the message being handled.
func keyIsDown(vk uintptr) bool {
	value, _, _ := getKeyState.Call(vk)
	return value&0x8000 != 0
}

// altGrHeld reports whether AltGr is held. Windows reports it as a left Control
// and a right Alt together, so a layout's third-level character reads like the
// chord Control+Alt.
func altGrHeld() bool {
	return platform.IsAltGr(keyIsDown(0xa5), keyIsDown(0xa2), keyIsDown(0xa3))
}

// chordKeyText is the character a key message names in a chord: the letter or
// digit of its virtual key, which does not depend on the keyboard layout, or the
// character a punctuation key types under the layout, as "," in Control+ or "="
// in Control+=. shift says whether the character is the one Shift types. Empty
// for any other key, or for a dead key.
func chordKeyText(first, second uintptr, shift bool) string {
	if text := shortcutText(first); text != "" || !platform.ShortcutPunctuationKey(first) {
		return text
	}
	var keys [256]byte
	if shift {
		keys[0x10] = 0x80
	}
	var typed [4]uint16
	layout, _, _ := getKeyboardLayout.Call(0)
	count, _, _ := toUnicodeEx.Call(first, uintptr(uint32(second>>16)&0xff), uintptr(unsafe.Pointer(&keys[0])),
		uintptr(unsafe.Pointer(&typed[0])), uintptr(len(typed)), toUnicodeKeepState, layout)
	if int32(count) != 1 {
		return ""
	}
	return string(rune(typed[0]))
}

// offerShortcut offers a key message that is a modified printable chord to the
// application before native text handling, and reports whether the application
// took it (or failed doing it), which ends the message. AltGr is no chord: it
// types a character of the layout.
func (state *windowState) offerShortcut(first, second uintptr, mods input.Modifiers) bool {
	if state.config.OnShortcut == nil || state.imeComposing || altGrHeld() ||
		(normalizedKey(first) != input.KeyUnknown && normalizedKey(first) != input.KeySpace && normalizedKey(first) != input.KeyEnter) {
		return false
	}
	event := platform.KeyEvent{Modifiers: mods, Repeat: second&(1<<30) != 0, Text: chordKeyText(first, second, mods&input.ModifierShift != 0)}
	if normalizedKey(first) == input.KeyEnter {
		event.Key = input.KeyEnter
	}
	if normalizedKey(first) == input.KeySpace {
		event.Key, event.Text = input.KeyUnknown, " "
	}
	// A focused text field owns editing chords before application shortcuts.
	// Otherwise a background terminal's copy/paste binding steals form input.
	if state.textEnabled && !state.textInput.Terminal {
		if state.imeComposing {
			return false
		}
		if _, editing := platform.TextEditingCommand(event); editing || platform.TextClipboardAction(event) != 0 {
			return false
		}
	}
	if !platform.ShortcutCandidate(event) {
		return false
	}
	frame, handled, err := state.config.OnShortcut(event)
	state.frame(frame, err)
	return handled || err != nil
}

// offerQueuedShortcut runs before character translation, on the same window
// thread and with the same fault handling as windowProc. Unhandled messages
// continue through TranslateMessage and the normal native editor/IME path.
func offerQueuedShortcut(event *message) (handled bool) {
	if event.Kind != wmKeyDown && event.Kind != wmSysKeyDown {
		return false
	}
	value, ok := windowStates.Load(event.Window)
	if !ok {
		return false
	}
	state := value.(*windowState)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("windows: shortcut handler", recovered)
			state.fail(fmt.Errorf("windows: shortcut handler panicked: %v", recovered))
			handled = true
		}
	}()
	active.Store(state)
	return state.offerShortcut(event.First, event.Second, modifiers())
}
