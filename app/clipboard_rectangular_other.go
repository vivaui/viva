//go:build !windows

package app

func nativeSetRectangularClipboardText(value string) error {
	return nativeSetClipboardText(value)
}

func nativeClipboardTextWithFormat() (string, bool, error) {
	value, err := nativeClipboardText()
	return value, false, err
}
