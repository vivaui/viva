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
	taken      [256]bool
	held       int
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
	down := kind == wmKeyDown || kind == wmSysKeyDown
	alt := event.Flags&llkhfAltDown != 0
	flags := shortcutKeyLParam(event, down, alt, state.menuKeys.taken[vk])
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
	if down && !state.menuKeys.taken[vk] && asyncKeyDown(vk) {
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
	if vk == vkProcessKey && !fromHook {
		// Some input methods expose VK_PROCESSKEY rather than the letter.
		// The scan code still names the original key on this thread's layout.
		if scan := (flags >> 16) & 0xff; scan != 0 {
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
		if !keys.taken[vk] {
			return false
		}
		keys.taken[vk] = false
		keys.held--
		if keys.held == 0 && !keys.open {
			state.syncShortcutHook()
		}
		return true
	}
	repeat := flags&(1<<30) != 0
	if keys.taken[vk] {
		return true
	}
	if !keys.open {
		return false
	}
	if repeat {
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
	if keys.taken[0x10] || keys.taken[0xa0] || keys.taken[0xa1] {
		mods |= input.ModifierShift
	}
	keys.taken[vk] = true
	keys.held++
	if mod != input.KeyUnknown {
		return true
	}
	first := vk | uintptr(mods)<<8 | uintptr(keys.generation)<<16
	if fromHook {
		postMessage.Call(state.window, wmMenuKey, first, flags)
	} else {
		state.deliverMenuMessage(first, flags)
	}
	return true
}

func (state *windowState) deliverMenuMessage(first, flags uintptr) {
	if !state.menuKeys.open || uint16(first>>16) != state.menuKeys.generation {
		return
	}
	vk, mods := first&0xff, input.Modifiers((first>>8)&0xff)
	event := platform.KeyEvent{Key: normalizedKey(vk), Modifiers: mods}
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
