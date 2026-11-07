//go:build windows

package app

import (
	"strconv"

	windowsplatform "github.com/vivaui/viva/internal/platform/windows"
	"github.com/vivaui/viva/ui"
)

func nativeRegisterGlobalShortcut(s GlobalShortcut, pressed func()) (func(), error) {
	var modifiers uint32
	if s.Modifiers&ui.ModifierAlt != 0 {
		modifiers |= 1
	}
	if s.Modifiers&ui.ModifierControl != 0 {
		modifiers |= 2
	}
	if s.Modifiers&ui.ModifierShift != 0 {
		modifiers |= 4
	}
	if s.Modifiers&ui.ModifierMeta != 0 {
		modifiers |= 8
	}
	key := uint32(0)
	if len(s.Key) == 1 {
		key = uint32(s.Key[0])
	} else if s.Key[0] == 'F' {
		n, _ := strconv.Atoi(s.Key[1:])
		key = uint32(0x70 + n - 1)
	} else {
		switch s.Key {
		case "SPACE":
			key = 0x20
		case "ENTER":
			key = 0x0d
		case "ESCAPE":
			key = 0x1b
		case "TAB":
			key = 9
		}
	}
	return windowsplatform.RegisterGlobalShortcut(modifiers, key, pressed)
}
