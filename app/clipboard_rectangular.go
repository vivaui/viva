package app

import (
	"fmt"
	"strings"
)

// SetRectangularClipboardText writes text with column-selection metadata on
// Windows. Other native targets retain the plain-text clipboard fallback.
func SetRectangularClipboardText(value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("app: clipboard text contains NUL")
	}
	if err, handled := driverSetRectangularClipboardText(value); handled {
		return err
	}
	return nativeSetRectangularClipboardText(value)
}

// ClipboardTextWithFormat reads text and its rectangular-selection marker from
// one clipboard snapshot. A target without that format returns false.
func ClipboardTextWithFormat() (value string, rectangular bool, err error) {
	if value, rectangular, err, handled := driverClipboardTextWithFormat(); handled {
		return value, rectangular, err
	}
	return nativeClipboardTextWithFormat()
}
