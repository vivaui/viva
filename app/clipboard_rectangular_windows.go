package app

import windowsplatform "github.com/vivaui/viva/internal/platform/windows"

func nativeSetRectangularClipboardText(value string) error {
	return windowsplatform.SetClipboardRectangularText(value)
}

func nativeClipboardTextWithFormat() (string, bool, error) {
	return windowsplatform.ClipboardTextWithFormat()
}
