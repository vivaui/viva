package windows

import (
	"unicode"
	"unicode/utf16"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

const (
	wmKeyUp      = 257
	wmDeadChar   = 259
	wmSysKeyUp   = 261
	vkProcessKey = 0xe5
)

var (
	mapVirtualKey    = user.NewProc("MapVirtualKeyW")
	getKeyboardState = user.NewProc("GetKeyboardState")
)

// pendingKey is a printable key whose WM_KEYDOWN has been seen and whose
// WM_CHAR decides what it is: a key, when the character is what the key types,
// or text when an input method or a dead key took part.
type pendingKey struct {
	set    bool
	code   uint32
	vk     uintptr
	repeat bool
}

// wantsKeyReleases reports whether the focused terminal asked for presses and
// releases of every key (platform.TextInput.KeyReleases).
func (state *windowState) wantsKeyReleases() bool {
	return state.textEnabled && state.textInput.Terminal && state.textInput.KeyReleases
}

// keyCode names the physical key in a key message: its scan code and the
// extended bit, the same for the press and the release. Input injected by
// virtual key alone (keybd_event without a scan code, some remote-control
// tools) carries no scan code; the layout maps the virtual key to one then.
func keyCode(first, second uintptr) uint32 {
	if code := uint32(second>>16) & 0x1ff; code != 0 {
		return code
	}
	const mapVKToVSC = 0
	scan, _, _ := mapVirtualKey.Call(first, mapVKToVSC)
	return uint32(scan) & 0xff
}

// modifierKey names the modifier key a key message is for.
func modifierKey(first, second uintptr) input.Key {
	extended := second&(1<<24) != 0
	switch first {
	case 16, 0xa0, 0xa1:
		if first == 0xa1 || first == 16 && uint32(second>>16)&0xff == 0x36 {
			return input.KeyShiftRight
		}
		return input.KeyShiftLeft
	case 17, 0xa2, 0xa3:
		if first == 0xa3 || extended {
			return input.KeyControlRight
		}
		return input.KeyControlLeft
	case 18, 0xa4, 0xa5:
		if first == 0xa5 || extended {
			return input.KeyAltRight
		}
		return input.KeyAltLeft
	case 0x5b:
		return input.KeyMetaLeft
	case 0x5c:
		return input.KeyMetaRight
	}
	return input.KeyUnknown
}

// printableKey reports whether a virtual key types a character, the space
// included: Win32 reports a space as both VK_SPACE and WM_CHAR " ".
func printableKey(vk uintptr) bool {
	return vk == 0x20 || vk >= '0' && vk <= '9' || vk >= 'A' && vk <= 'Z' || vk >= 0x60 && vk <= 0x6f ||
		vk >= 0xba && vk <= 0xc0 || vk >= 0xdb && vk <= 0xdf || vk == 0xe2
}

// spaceIsText reports whether a space press is the character the text system
// inserts rather than a key the application handles. Windows sends a plain
// space twice, as the VK_SPACE key and as the character " "; a modifier makes
// it a chord (Control+Space is NUL, Alt+Space an Escape and a space).
func spaceIsText(vk uintptr, mods input.Modifiers) bool {
	return vk == 0x20 && mods&(input.ModifierControl|input.ModifierAlt|input.ModifierMeta) == 0
}

// baseCharacter is the character a virtual key types with no modifiers, and
// false for a dead key or a key with none.
func baseCharacter(vk uintptr) (string, bool) {
	const mapVKToChar = 2
	value, _, _ := mapVirtualKey.Call(vk, mapVKToChar)
	if value == 0 || value&0x80000000 != 0 {
		return "", false
	}
	return string(unicode.ToLower(rune(utf16.Decode([]uint16{uint16(value)})[0]))), true
}

// typedText is the character a WM_CHAR carries, empty for a control character
// or half of a surrogate pair.
func typedText(char uintptr) string {
	if char < 0x20 || char == 0x7f || utf16.IsSurrogate(rune(char)) {
		return ""
	}
	return string(rune(char))
}

// modifierTap reports the press or release of a modifier key to the runtime
// when the application listens for a modifier pressed on its own
// (platform.Config.ModifierTaps). It only tells the runtime: the message goes
// on to the text system, the input method and the system menu as before, so
// nothing about typing or Alt's menu changes. A terminal that asked for the
// whole keyboard gets the same events from keyDown and keyUp.
func (state *windowState) modifierTap(first, second uintptr, release bool, mods input.Modifiers) {
	if !state.config.ModifierTaps || state.config.OnKey == nil || state.imeComposing {
		return
	}
	key := modifierKey(first, second)
	if key == input.KeyUnknown {
		return
	}
	event := platform.KeyEvent{Key: key, Release: release, Code: keyCode(first, second)}
	if release {
		event.Modifiers = mods &^ key.Modifier()
	} else {
		event.Modifiers, event.Repeat = mods|key.Modifier(), second&(1<<30) != 0
	}
	state.sendKey(event)
}

// keyDown handles WM_KEYDOWN and WM_SYSKEYDOWN for a terminal that wants
// presses and releases, and reports whether it consumed the message. A plain
// space is held for its WM_CHAR like any other character, so the release can
// carry the press's identity.
func (state *windowState) keyDown(first, second uintptr, mods input.Modifiers) bool {
	repeat := second&(1<<30) != 0
	state.pendingKey = pendingKey{}
	if first == vkProcessKey || state.imeComposing || state.config.OnKey == nil {
		return false
	}
	if key := modifierKey(first, second); key != input.KeyUnknown {
		state.sendKey(platform.KeyEvent{Key: key, Modifiers: mods | key.Modifier(), Repeat: repeat, Code: keyCode(first, second), Win32: win32Key(first, second, 0, uint16(second&0xffff))})
		return true
	}
	code := keyCode(first, second)
	if key := normalizedKey(first); key != input.KeyUnknown && !spaceIsText(first, mods) {
		if key == input.KeyF4 && mods&input.ModifierAlt != 0 {
			return false // Alt+F4 closes the window
		}
		state.sendKey(platform.KeyEvent{Key: key, Modifiers: mods, Repeat: repeat, Code: code, Win32: win32Key(first, second, 0, uint16(second&0xffff))})
		return true
	}
	chord := mods&(input.ModifierControl|input.ModifierMeta) != 0
	if text := shortcutText(first); chord && text != "" {
		state.sendKey(platform.KeyEvent{Modifiers: mods, Repeat: repeat, Code: code, Text: text, Win32: win32Key(first, second, 0, uint16(second&0xffff))})
		return true
	}
	if printableKey(first) && !chord && mods&input.ModifierAlt != 0 {
		// Alt+printable produces WM_SYSCHAR, not WM_CHAR, so waiting for a
		// character would lose it. A terminal that asked for key releases
		// needs the raw key; its encoder decides whether Alt means Escape or
		// a kitty modifier.
		if text, ok := baseCharacter(first); ok {
			state.sendKey(platform.KeyEvent{Modifiers: mods, Repeat: repeat, Code: code, Text: text, Win32: win32Key(first, second, 0, uint16(second&0xffff))})
			return true
		}
	}
	if printableKey(first) && !chord && mods&input.ModifierAlt == 0 {
		state.pendingKey = pendingKey{set: true, code: code, vk: first, repeat: repeat}
	}
	return false
}

// textKey handles WM_KEYDOWN and WM_SYSKEYDOWN for the focused text input while
// it did not ask for key releases, and reports whether it consumed the message.
// A text field applies the editing command the key means; a terminal has no
// text model of its own, so its named keys and control chords go to the
// application raw, as they do on macOS and Linux.
func (state *windowState) textKey(first, second uintptr, mods input.Modifiers) bool {
	if state.textInput.Terminal {
		event, ok := state.terminalKeyEvent(first, second, mods)
		if !ok {
			return false
		}
		if state.config.OnKey != nil {
			state.sendKey(event)
		}
		return true
	}
	repeat := second&(1<<30) != 0
	editKey := platform.KeyEvent{Key: normalizedKey(first), Modifiers: mods, Repeat: repeat, Text: shortcutText(first)}
	if action := platform.TextClipboardAction(editKey); action != 0 {
		event, changed, err := platform.TextClipboardEvent(state.textInput.ClipboardState(action), state.textInput.Secure, action, ClipboardText, SetClipboardText)
		if err != nil {
			messageBeep.Call(0) // a busy clipboard must not close the window or delete a selection
		} else if changed {
			state.handleTextEvent(event)
		}
		return true
	}
	// Placed editors own visual row navigation (wrapping, scrolling, and so on).
	if state.textInput.Placed && (editKey.Key == input.KeyHome || editKey.Key == input.KeyEnd) {
		return false
	}
	if command, ok := platform.TextEditingCommand(editKey); ok {
		state.handleTextEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
		return true
	}
	if command, ok := state.textCommandWith(first, mods&input.ModifierShift != 0); ok {
		state.handleTextEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
		return true
	}
	return false
}

// terminalKeyEvent is the raw event a key means to a terminal: a named key, or
// a control chord. Backspace, Delete, the arrows and Enter are named keys here,
// so a terminal passes them to the program instead of editing text it does not
// keep. A printable key is not one: the text system inserts it when WM_CHAR
// arrives. Alt is not a chord, because AltGr arrives as Control+Alt and types a
// character.
func (state *windowState) terminalKeyEvent(first, second uintptr, mods input.Modifiers) (platform.KeyEvent, bool) {
	repeat := second&(1<<30) != 0
	if key := normalizedKey(first); key != input.KeyUnknown && !spaceIsText(first, mods) {
		return platform.KeyEvent{Key: key, Modifiers: mods, Repeat: repeat, Code: keyCode(first, second), Win32: win32Key(first, second, 0, uint16(second&0xffff))}, true
	}
	if mods&(input.ModifierControl|input.ModifierMeta) != 0 && mods&input.ModifierAlt == 0 {
		if text := shortcutText(first); text != "" {
			return platform.KeyEvent{Modifiers: mods, Repeat: repeat, Text: text, Code: keyCode(first, second), Win32: win32Key(first, second, 0, uint16(second&0xffff))}, true
		}
	}
	return platform.KeyEvent{}, false
}

// character handles WM_CHAR for a terminal that wants presses and releases. A
// character that follows a printable key's WM_KEYDOWN is that key, unless a dead
// key or an input method composed it.
func (state *windowState) character(typed, second uintptr) bool {
	pending := state.pendingKey
	state.pendingKey = pendingKey{}
	dead := state.deadKey
	state.deadKey = false
	if !pending.set || dead || state.imeComposing {
		return false
	}
	text, ok := baseCharacter(pending.vk)
	if !ok {
		return false
	}
	state.sendKey(platform.KeyEvent{
		Modifiers: modifiers(), Repeat: pending.repeat, Code: pending.code, Text: text, Typed: typedText(typed),
		Win32: win32Key(pending.vk, second, uint16(typed), uint16(second&0xffff)),
	})
	return true
}

// keyUp handles WM_KEYUP and WM_SYSKEYUP. The runtime drops the release of a
// key whose press it never saw, so every release is passed on.
func (state *windowState) keyUp(first, second uintptr, mods input.Modifiers) bool {
	if state.config.OnKey == nil {
		return false
	}
	event := platform.KeyEvent{Release: true, Code: keyCode(first, second), Modifiers: mods, Win32: win32Key(first, second, 0, 1)}
	if key := modifierKey(first, second); key != input.KeyUnknown {
		event.Key, event.Modifiers = key, mods&^key.Modifier()
	} else {
		event.Key = normalizedKey(first)
	}
	state.sendKey(event)
	return true
}

func (state *windowState) sendKey(event platform.KeyEvent) {
	state.keyEvents++
	frame, err := state.config.OnKey(event)
	state.frame(frame, err)
}

// focusLost tells the runtime the keyboard went elsewhere.
func (state *windowState) focusLost() {
	if state.config.OnFocusLost == nil {
		return
	}
	frame, err := state.config.OnFocusLost()
	state.frame(frame, err)
}

// win32ControlState is the console control-key-state bitset of a key message.
// It names left and right modifiers and the lock toggles exactly, which the
// normalized input.Modifiers cannot carry.
func win32ControlState(second uintptr) uint16 {
	var state uint16
	down := func(vk uintptr) bool {
		value, _, _ := getKeyState.Call(vk)
		return value&0x8000 != 0
	}
	toggled := func(vk uintptr) bool {
		value, _, _ := getKeyState.Call(vk)
		return value&1 != 0
	}
	if down(0xa5) { // VK_RMENU
		state |= 0x0001
	}
	if down(0xa4) { // VK_LMENU
		state |= 0x0002
	}
	if down(0xa3) { // VK_RCONTROL
		state |= 0x0004
	}
	if down(0xa2) { // VK_LCONTROL
		state |= 0x0008
	}
	if down(0x10) { // VK_SHIFT
		state |= 0x0010
	}
	if toggled(0x90) { // VK_NUMLOCK
		state |= 0x0020
	}
	if toggled(0x91) { // VK_SCROLL
		state |= 0x0040
	}
	if toggled(0x14) { // VK_CAPITAL
		state |= 0x0080
	}
	if second&(1<<24) != 0 {
		state |= 0x0100
	}
	return state
}

// win32Key is the console INPUT_RECORD identity of one key message.
func win32Key(first, second uintptr, unicode, repeat uint16) gfx.Win32Key {
	return win32KeyRecord(first, second, unicode, repeat, unicode == 0 && altGrHeld())
}

// win32KeyRecord builds one console INPUT_RECORD identity. altGr says AltGr is
// held for a message that named no character: AltGr types a layout character
// that WM_CHAR delivers as text, so translating it here as well would hand the
// terminal that character twice. Named keys keep their console character.
func win32KeyRecord(first, second uintptr, unicode, repeat uint16, altGr bool) gfx.Win32Key {
	scan := uint16((second >> 16) & 0xff)
	if scan == 0 {
		scan = uint16(keyCode(first, second) & 0xff)
	}
	if repeat == 0 {
		repeat = 1
	}
	if unicode == 0 && !(altGr && printableKey(first)) {
		// Named keys and control chords are dispatched at WM_KEYDOWN, before
		// WM_CHAR. ConPTY still needs their character (CR for Enter, for
		// example), including on release. Preserve characters supplied by
		// WM_CHAR and translate the others without consuming a pending dead key.
		var keys [256]byte
		if ok, _, _ := getKeyboardState.Call(uintptr(unsafe.Pointer(&keys[0]))); ok != 0 {
			unicode = consoleKeyCharacter(first, scan, &keys)
		}
	}
	return gfx.Win32Key{
		VirtualKey: uint16(first), ScanCode: scan, Unicode: unicode,
		ControlState: win32ControlState(second), Repeat: repeat,
	}
}

func consoleKeyCharacter(vk uintptr, scan uint16, keys *[256]byte) uint16 {
	var typed [8]uint16
	layout, _, _ := getKeyboardLayout.Call(0)
	count, _, _ := toUnicodeEx.Call(vk, uintptr(scan), uintptr(unsafe.Pointer(&keys[0])),
		uintptr(unsafe.Pointer(&typed[0])), uintptr(len(typed)), toUnicodeKeepState, layout)
	if int32(count) == 1 {
		return typed[0]
	}
	return 0
}
