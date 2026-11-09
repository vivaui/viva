package app

import (
	"os"
	"strconv"
	"strings"

	linuxplatform "github.com/vivaui/viva/internal/platform/linux"
)

func nativeRegisterGlobalShortcut(s GlobalShortcut, pressed func()) (func(), error) {
	// An XWayland grab does not reach native Wayland clients.
	if os.Getenv("DISPLAY") == "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") {
		return nil, ErrUnsupported
	}
	var symbol uint32
	switch {
	case len(s.Key) == 1:
		symbol = uint32(s.Key[0])
		if symbol >= 'A' && symbol <= 'Z' {
			symbol += 'a' - 'A'
		}
	case s.Key[0] == 'F':
		n, _ := strconv.Atoi(s.Key[1:])
		symbol = 0xffbe + uint32(n-1)
	default:
		switch s.Key {
		case "SPACE":
			symbol = ' '
		case "ENTER":
			symbol = 0xff0d
		case "ESCAPE":
			symbol = 0xff1b
		case "TAB":
			symbol = 0xff09
		}
	}
	return linuxplatform.RegisterGlobalShortcut(s.Modifiers, symbol, pressed)
}
