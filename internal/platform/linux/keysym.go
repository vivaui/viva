package linux

import (
	"sort"

	"github.com/vivaui/viva/internal/input"
)

const (
	keysymBackspace  = 0xff08
	keysymTab        = 0xff09
	keysymEnter      = 0xff0d
	keysymEscape     = 0xff1b
	keysymScrollLock = 0xff14
	keysymMultiKey   = 0xff20
	keysymHome       = 0xff50
	keysymLeft       = 0xff51
	keysymUp         = 0xff52
	keysymRight      = 0xff53
	keysymDown       = 0xff54
	keysymEnd        = 0xff57
	keysymDelete     = 0xffff
	keysymISOTab     = 0xfe20
	keysymPageUp     = 0xff55
	keysymPageDown   = 0xff56
	keysymInsert     = 0xff63
	keysymModeSwitch = 0xff7e
	keysymNumLock    = 0xff7f
	keysymF1         = 0xffbe
	keysymF12        = 0xffc9
	keysymShiftL     = 0xffe1
	keysymShiftR     = 0xffe2
	keysymControlL   = 0xffe3
	keysymControlR   = 0xffe4
	keysymCapsLock   = 0xffe5
	keysymMetaL      = 0xffe7
	keysymMetaR      = 0xffe8
	keysymAltL       = 0xffe9
	keysymAltR       = 0xffea
	keysymSuperL     = 0xffeb
	keysymSuperR     = 0xffec
	keysymLevel3     = 0xfe03
	keysymKPSpace    = 0xff80
	keysymKPEnter    = 0xff8d
	keysymKPHome     = 0xff95
	keysymKPLeft     = 0xff96
	keysymKPUp       = 0xff97
	keysymKPRight    = 0xff98
	keysymKPDown     = 0xff99
	keysymKPEnd      = 0xff9c
	keysymKPDelete   = 0xff9f
	keysymKPEqual    = 0xffbd
	keysymKPFirst    = keysymKPSpace
	keysymKPLast     = keysymKPEqual
)

// isKeypadKeysym reports whether a keysym belongs to the numeric keypad. A
// keypad key's two keysyms are its navigation meaning and its digit; Num Lock
// (or Shift) chooses the digit.
func isKeypadKeysym(keysym uint32) bool {
	return keysym >= keysymKPFirst && keysym <= keysymKPLast
}

// keypadRune is the character a keypad keysym types. Keypad Enter, Tab and the
// navigation forms type none.
func keypadRune(keysym uint32) (rune, bool) {
	switch {
	case keysym == keysymKPSpace:
		return ' ', true
	case keysym >= 0xffaa && keysym <= 0xffb9: // * + , - . / 0..9
		return rune(keysym - 0xffaa + '*'), true
	case keysym == keysymKPEqual:
		return '=', true
	}
	return 0, false
}

// keysymRune is the character a keysym types: Latin-1 as itself, the Unicode
// keysyms (0x01000000 plus the code point), the legacy national blocks through
// the generated table, and the keypad's characters.
func keysymRune(keysym uint32) (rune, bool) {
	switch {
	case keysym >= 0x20 && keysym <= 0x7e, keysym >= 0xa0 && keysym <= 0xff:
		return rune(keysym), true
	case keysym&0xff000000 == 0x01000000:
		value := rune(keysym & 0x00ffffff)
		if value <= 0x10ffff {
			return value, true
		}
	case keysym > 0xff && keysym <= 0x20ff:
		index := sort.Search(len(legacyKeysymRunes), func(i int) bool { return uint32(legacyKeysymRunes[i].keysym) >= keysym })
		if index < len(legacyKeysymRunes) && uint32(legacyKeysymRunes[index].keysym) == keysym {
			return rune(legacyKeysymRunes[index].character), true
		}
	case isKeypadKeysym(keysym):
		return keypadRune(keysym)
	}
	return 0, false
}

func normalizedKeysym(keysym uint32) input.Key {
	switch keysym {
	case keysymTab, keysymISOTab:
		return input.KeyTab
	case keysymEnter, keysymKPEnter:
		return input.KeyEnter
	case ' ', keysymKPSpace:
		return input.KeySpace
	case keysymEscape:
		return input.KeyEscape
	case keysymLeft, keysymKPLeft:
		return input.KeyLeft
	case keysymRight, keysymKPRight:
		return input.KeyRight
	case keysymUp, keysymKPUp:
		return input.KeyUp
	case keysymDown, keysymKPDown:
		return input.KeyDown
	case keysymHome, keysymKPHome:
		return input.KeyHome
	case keysymEnd, keysymKPEnd:
		return input.KeyEnd
	case keysymBackspace:
		return input.KeyBackspace
	case keysymDelete, keysymKPDelete:
		return input.KeyDelete
	case keysymPageUp:
		return input.KeyPageUp
	case keysymPageDown:
		return input.KeyPageDown
	case keysymInsert:
		return input.KeyInsert
	case keysymShiftL:
		return input.KeyShiftLeft
	case keysymShiftR:
		return input.KeyShiftRight
	case keysymControlL:
		return input.KeyControlLeft
	case keysymControlR:
		return input.KeyControlRight
	case keysymAltL, keysymMetaL:
		return input.KeyAltLeft
	case keysymAltR, keysymMetaR, keysymLevel3:
		return input.KeyAltRight
	case keysymSuperL:
		return input.KeyMetaLeft
	case keysymSuperR:
		return input.KeyMetaRight
	default:
		if keysym >= keysymF1 && keysym <= keysymF12 {
			return input.KeyF1 + input.Key(keysym-keysymF1)
		}
		return input.KeyUnknown
	}
}
