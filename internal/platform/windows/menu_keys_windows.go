package windows

import (
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

const wmMenuKey = wmUpdateIcon + 1

var imeVirtualKey = imm.NewProc("ImmGetVirtualKey")

type menuKeyboard struct {
	open       bool
	generation uint16 // an already-posted key must not run in a later menu
	// taken holds the presses the menu owns until their releases come. A key is
	// identified by its scan code, the physical key, because Windows reports the
	// release with the layout current at release time: a layout change while a
	// key is held would otherwise strand the press and keep the global hook
	// installed. A key that carries no scan code is identified by its virtual
	// key in takenNoScan instead.
	taken       [256]bool
	takenNoScan [256]bool
	// shiftHeld is a Shift press the menu swallowed; a later key sees Shift in
	// its modifiers even though the system never saw the press.
	shiftHeld bool
	shiftScan uint16
	held      int
}

func (keys *menuKeyboard) owns(scan, vk uintptr) bool {
	if scan != 0 {
		return keys.taken[scan]
	}
	return keys.takenNoScan[vk]
}

func (keys *menuKeyboard) own(scan, vk uintptr, take bool) {
	if scan != 0 {
		keys.taken[scan] = take
		return
	}
	keys.takenNoScan[vk] = take
}

// The foreground window uses Viva's existing low-level keyboard hook before
// Windows gives the key to any input method. The focused terminal keeps its
// HIMC and edit buffer throughout the menu.
func (state *windowState) syncMenuKeyboard() {
	if state.config.CaptureMenuKeys == nil {
		return
	}
	keys := &state.menuKeys
	open := state.focused && state.config.CaptureMenuKeys()
	if open != keys.open {
		keys.generation++
		keys.open = open
		state.syncShortcutHook()
	}
}

func (state *windowState) releaseMenuKeyboard() {
	gen := state.menuKeys.generation
	state.menuKeys = menuKeyboard{generation: gen}
}

func (state *windowState) takePhysicalMenuKey(kind uintptr, event *kbdLLHook) bool {
	vk := uintptr(event.VkCode)
	if vk >= 256 {
		return false
	}
	scan := uintptr(event.ScanCode & 0xff)
	down := kind == wmKeyDown || kind == wmSysKeyDown
	alt := event.Flags&llkhfAltDown != 0
	flags := shortcutKeyLParam(event, down, alt, state.menuKeys.owns(scan, vk))
	// This callback runs before the asynchronous key state reflects *this*
	// transition. Other modifiers already held are visible; a Shift press
	// eaten by the menu is tracked in taken and added in takeMenuKey below.
	var mods input.Modifiers
	if asyncKeyDown(0x10) {
		mods |= input.ModifierShift
	}
	if asyncKeyDown(0x11) {
		mods |= input.ModifierControl
	}
	if asyncKeyDown(vkLeftWin) || asyncKeyDown(vkRightWin) {
		mods |= input.ModifierMeta
	}
	if alt {
		mods |= input.ModifierAlt
	}
	if down && !state.menuKeys.owns(scan, vk) && asyncKeyDown(vk) {
		flags |= 1 << 30 // was already down when the menu opened
	}
	return state.takeMenuKey(vk, flags, mods, true)
}

// takeMenuKey owns a press through its release, even if the first press
// closes the menu. A release from before the menu still belongs to the
// original key owner. The hook only posts a private message; application
// callbacks and repaint run in the ordinary dispatcher.
func (state *windowState) takeMenuKey(vk, flags uintptr, mods input.Modifiers, fromHook bool) bool {
	keys := &state.menuKeys
	if !keys.open && keys.held == 0 || vk >= 256 {
		return false
	}
	scan := (flags >> 16) & 0xff
	if vk == vkProcessKey && !fromHook {
		// Some input methods expose VK_PROCESSKEY rather than the letter.
		// The scan code still names the original key on this thread's layout.
		if scan != 0 {
			if original, _, _ := mapVirtualKey.Call(scan, 1 /* MAPVK_VSC_TO_VK */); original > 0 && original < 256 {
				vk = original
			}
		}
		if vk == vkProcessKey {
			// A virtual-key-only injection can have no scan code. Ask IMM
			// before TranslateMessage, while it still knows the original VK.
			if original, _, _ := imeVirtualKey.Call(state.window); original > 0 && original < 256 {
				vk = original
			}
		}
	}
	if flags&(1<<31) != 0 {
		if !keys.owns(scan, vk) {
			return false
		}
		keys.own(scan, vk, false)
		if keys.shiftHeld && (scan != 0 && keys.shiftScan == uint16(scan) || scan == 0 && isShiftKey(vk)) {
			keys.shiftHeld, keys.shiftScan = false, 0
		}
		keys.held--
		if keys.held == 0 && !keys.open {
			// Unhooking a low-level hook from inside its own callback is not
			// safe; let the dispatcher re-sync once this hook returns.
			if fromHook {
				postMessage.Call(state.window, wmCaptureShortcuts, 0, 0)
			} else {
				state.syncShortcutHook()
			}
		}
		return true
	}
	repeat := flags&(1<<30) != 0
	if keys.owns(scan, vk) {
		// A later press of a key the menu already owns is its auto-repeat.
		// Deliver it with Repeat set so a menu can move like a native one.
		if repeat {
			state.sendMenuKey(vk, flags, mods, fromHook)
		}
		return true
	}
	if !keys.open {
		return false
	}
	if repeat {
		// A repeat of a key that was down before the menu opened: its press
		// never reached the menu, so there is nothing to repeat.
		return true
	}
	// Leave system chords such as Alt+Tab and Win+L with Windows. Swallow a
	// plain Shift press: some input methods use it to toggle language, which
	// could end the composition while the menu is up.
	mod := modifierKey(vk, flags)
	systemChord := mods&input.ModifierMeta != 0 || mods&input.ModifierControl != 0 && vk == vkEscape ||
		mods&input.ModifierAlt != 0 && (vk == vkTab || vk == vkEscape || vk == 0x73 /* F4 */ || vk == 0x20)
	if systemChord || mod != input.KeyUnknown && mod.Modifier() != input.ModifierShift {
		return false
	}
	// AltGr types a character of the layout; it is not a menu chord. Leave the
	// key to text and the input method, as offerShortcut does.
	if altGrHeld() {
		return false
	}
	if keys.shiftHeld {
		mods |= input.ModifierShift
	}
	keys.own(scan, vk, true)
	keys.held++
	if mod != input.KeyUnknown {
		if mod.Modifier() == input.ModifierShift {
			keys.shiftHeld, keys.shiftScan = true, uint16(scan)
		}
		return true
	}
	state.sendMenuKey(vk, flags, mods, fromHook)
	return true
}

// sendMenuKey hands an owned press to the dispatcher. The hook posts it; a key
// message already on the dispatcher's thread delivers it in place.
func (state *windowState) sendMenuKey(vk, flags uintptr, mods input.Modifiers, fromHook bool) {
	first := vk | uintptr(mods)<<8 | uintptr(state.menuKeys.generation)<<16
	if fromHook {
		postMessage.Call(state.window, wmMenuKey, first, flags)
		return
	}
	state.deliverMenuMessage(first, flags)
}

func isShiftKey(vk uintptr) bool {
	return vk == 0x10 || vk == 0xa0 || vk == 0xa1
}

func (state *windowState) deliverMenuMessage(first, flags uintptr) {
	if !state.menuKeys.open || uint16(first>>16) != state.menuKeys.generation {
		return
	}
	vk, mods := first&0xff, input.Modifiers((first>>8)&0xff)
	event := platform.KeyEvent{Key: normalizedKey(vk), Modifiers: mods, Repeat: flags&(1<<30) != 0}
	if event.Key != input.KeyUnknown {
		if state.config.OnKey != nil {
			state.sendKey(event)
		}
	} else if state.config.OnShortcut != nil {
		event.Text = chordKeyText(vk, flags, mods&input.ModifierShift != 0)
		if event.Text != "" {
			frame, _, err := state.config.OnShortcut(event)
			state.frame(frame, err)
		}
	}
}
