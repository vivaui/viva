package base

import (
	"strings"

	"github.com/vivaui/viva/gfx"
)

func ParseColorHex(value string, alpha bool) (gfx.Color, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 0 || value[0] != '#' {
		return gfx.Color{}, false
	}
	value = value[1:]
	short := len(value) == 3 || alpha && len(value) == 4
	if !short && len(value) != 6 && !(alpha && len(value) == 8) {
		return gfx.Color{}, false
	}
	channels := [4]byte{0, 0, 0, 255}
	count := len(value) / 2
	if short {
		count = len(value)
	}
	for i := 0; i < count; i++ {
		at := i * 2
		if short {
			at = i
		}
		a, ok := pickerHexDigit(value[at])
		if !ok {
			return gfx.Color{}, false
		}
		b := a
		if !short {
			b, ok = pickerHexDigit(value[i*2+1])
			if !ok {
				return gfx.Color{}, false
			}
		}
		channels[i] = a*16 + b
	}
	return gfx.Color{Red: float32(channels[0]) / 255, Green: float32(channels[1]) / 255, Blue: float32(channels[2]) / 255, Alpha: float32(channels[3]) / 255}, true
}

func pickerHexDigit(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}
