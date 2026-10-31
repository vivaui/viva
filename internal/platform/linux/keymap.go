package linux

import (
	"unicode"

	"github.com/vivaui/viva/internal/input"
)

// Modifier bits of an X key event's state, and where the keyboard group is.
const (
	xcbModShift   = 1 << 0
	xcbModLock    = 1 << 1
	xcbModControl = 1 << 2
	xcbMod1       = 1 << 3
	xcbMod2       = 1 << 4
	xcbMod4       = 1 << 6
	xcbMod5       = 1 << 7

	// The server puts the effective keyboard group (the layout in use) in bits
	// 13 and 14 of the state.
	xcbGroupShift = 13
	xcbGroupMask  = 3
)

// xcbKeymap is the server's core keyboard mapping: for each keycode a row of
// keysyms in the order the protocol and XKB agree on. Columns 0 and 1 are the
// first layout's unshifted and shifted symbols, 2 and 3 the second layout's,
// and 4 to 7 the third and fourth levels (what AltGr selects) of the first and
// then the second layout. A key whose layout has fewer levels leaves zeros.
type xcbKeymap struct {
	first      uint8
	last       uint8
	perKeycode uint8
	keysyms    []uint32
	masks      keyMasks
}

// keyMasks are the modifier bits the keyboard's special keys sit on; a zero
// means the modifier map has not been read, and the conventional bit stands.
type keyMasks struct {
	level3, modeSwitch, numLock uint16
}

func (keymap *xcbKeymap) level3Mask() uint16 {
	if keymap.masks.level3 != 0 {
		return keymap.masks.level3
	}
	return xcbMod5
}

func (keymap *xcbKeymap) numLockMask() uint16 {
	if keymap.masks.numLock != 0 {
		return keymap.masks.numLock
	}
	return xcbMod2
}

// setModifierMap finds which modifier each of AltGr, Mode_switch and Num Lock is
// on from the server's modifier map: keycodes holds perModifier keycodes for each
// of the eight modifiers in order.
func (keymap *xcbKeymap) setModifierMap(keycodes []uint8, perModifier int) {
	keymap.masks = keyMasks{}
	if perModifier <= 0 {
		return
	}
	for modifier := 0; modifier < 8 && (modifier+1)*perModifier <= len(keycodes); modifier++ {
		for _, keycode := range keycodes[modifier*perModifier : (modifier+1)*perModifier] {
			if keycode == 0 {
				continue
			}
			for _, keysym := range keymap.symbols(keycode) {
				switch keysym {
				case keysymLevel3:
					keymap.masks.level3 |= 1 << modifier
				case keysymModeSwitch:
					keymap.masks.modeSwitch |= 1 << modifier
				case keysymNumLock:
					keymap.masks.numLock |= 1 << modifier
				}
			}
		}
	}
}

func (keymap *xcbKeymap) key(keycode uint8) input.Key {
	for _, keysym := range keymap.symbols(keycode) {
		if key := normalizedKeysym(keysym); key != input.KeyUnknown {
			return key
		}
	}
	return input.KeyUnknown
}

func (keymap *xcbKeymap) symbols(keycode uint8) []uint32 {
	if keycode < keymap.first || keycode > keymap.last || keymap.perKeycode == 0 {
		return nil
	}
	offset := int(keycode-keymap.first) * int(keymap.perKeycode)
	end := offset + int(keymap.perKeycode)
	if offset < 0 || end > len(keymap.keysyms) {
		return nil
	}
	return keymap.keysyms[offset:end]
}

// text is the characters a key types under the modifier state, empty for a key
// that types none.
func (keymap *xcbKeymap) text(keycode uint8, state uint16) string {
	character, ok := keysymRune(keymap.keysym(keycode, state))
	if !ok {
		return ""
	}
	return string(character)
}

// keysym is the symbol a key produces under the modifier state: the layout the
// state names, the level Shift, Caps Lock and AltGr choose, and Num Lock's digit
// on the keypad. It is zero for a key the mapping does not know.
func (keymap *xcbKeymap) keysym(keycode uint8, state uint16) uint32 {
	symbols := keymap.symbols(keycode)
	if len(symbols) == 0 {
		return 0
	}
	group := int(state>>xcbGroupShift) & xcbGroupMask
	// A server that puts Mode_switch on the modifier AltGr is on (the evdev rules
	// do) means the third level by it, not the second layout.
	if modeSwitch := keymap.masks.modeSwitch &^ keymap.masks.level3; modeSwitch != 0 && state&modeSwitch != 0 {
		group = 1
	}
	return pickKeysym(symbols, group, state&xcbModShift != 0, state&xcbModLock != 0, state&keymap.level3Mask() != 0, state&keymap.numLockMask() != 0)
}

// shortcutText is the character a key stands for in a shortcut: the Latin one,
// whatever layout is active. Ctrl+C is the key that types c on the first layout
// that has Latin letters, so a Russian or Greek layout does not break it, as the
// virtual-key codes keep it on Windows. A key no layout gives an ASCII character
// (a function key, a layout of its own letters only) stands for what it types.
func (keymap *xcbKeymap) shortcutText(keycode uint8, state uint16) string {
	symbols := keymap.symbols(keycode)
	shift, lock, numLock := state&xcbModShift != 0, state&xcbModLock != 0, state&keymap.numLockMask() != 0
	pairs := make([][2]uint32, 0, 2)
	for column := 0; column < len(symbols) && column < 4; column += 2 {
		var high uint32
		if column+1 < len(symbols) {
			high = symbols[column+1]
		}
		pairs = append(pairs, [2]uint32{symbols[column], high})
	}
	if character, ok := shortcutFromLevels(pairs, shift, lock, numLock); ok {
		return string(character)
	}
	// A key no level scan gives ASCII text falls back to what it types with the
	// passed modifiers, without the AltGr level, as Wayland's fallback does.
	return keymap.text(keycode, state&^keymap.level3Mask())
}

// shortcutFromLevels is the character a key stands for in a shortcut: the
// first ASCII character among its unshifted and shifted level pairs, scanned in
// layout order. A pair that has only the shifted level still names the key.
// Both backends use it, so a chord means the same key on each.
func shortcutFromLevels(pairs [][2]uint32, shift, lock, numLock bool) (rune, bool) {
	for _, pair := range pairs {
		chosen := pickLevel(pair[0], pair[1], shift, lock, numLock)
		if chosen == 0 {
			chosen = pair[1]
		}
		if chosen >= 0x20 && chosen <= 0x7e {
			return rune(chosen), true
		}
	}
	return 0, false
}

// hasPair reports whether the row has a symbol at column.
func hasPair(symbols []uint32, column int) bool {
	return column < len(symbols) && (symbols[column] != 0 || column+1 < len(symbols) && symbols[column+1] != 0)
}

// pickKeysym chooses among one key's symbols. A layout the key has no symbols
// for falls back to the first, and a level it has none for to the one below.
func pickKeysym(symbols []uint32, group int, shift, lock, level3, numLock bool) uint32 {
	if group != 1 || !hasPair(symbols, 2) {
		group = 0 // the third and fourth layouts are not in the core mapping's reach
	}
	if level3 && hasPair(symbols, 4+group*2) {
		if keysym := pickFromPair(symbols, 4+group*2, shift, lock, numLock); keysym != 0 {
			return keysym
		}
	}
	return pickFromPair(symbols, group*2, shift, lock, numLock)
}

// pickFromPair chooses the unshifted or shifted symbol of the pair that starts at
// column. Caps Lock shifts a letter that has a capital, Num Lock turns a
// keypad key's navigation symbol into its digit and Shift turns it back.
func pickFromPair(symbols []uint32, column int, shift, lock, numLock bool) uint32 {
	if column >= len(symbols) {
		return 0
	}
	low, high := symbols[column], uint32(0)
	if column+1 < len(symbols) {
		high = symbols[column+1]
	}
	return pickLevel(low, high, shift, lock, numLock)
}

// pickLevel chooses the unshifted or shifted symbol of a pair. Caps Lock shifts
// a letter that has a capital, Num Lock turns a keypad key's navigation symbol
// into its digit and Shift turns it back.
func pickLevel(low, high uint32, shift, lock, numLock bool) uint32 {
	switch {
	case isKeypadKeysym(low) || isKeypadKeysym(high):
		if numLock {
			shift = !shift
		}
	case lock && hasCapital(low, high):
		shift = !shift
	}
	if shift && high != 0 {
		return high
	}
	return low
}

// hasCapital reports whether high is the capital of the letter low.
func hasCapital(low, high uint32) bool {
	small, smallOK := keysymRune(low)
	capital, capitalOK := keysymRune(high)
	return smallOK && capitalOK && small != capital && unicode.ToUpper(small) == capital
}
