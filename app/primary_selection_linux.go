//go:build linux && !android

package app

import linuxplatform "github.com/vivaui/viva/internal/platform/linux"

func nativeSetPrimarySelection(text string) error {
	return linuxSystemError(linuxplatform.SetPrimaryText(text))
}

func nativePrimarySelection() (string, error) {
	text, err := linuxplatform.PrimaryText()
	return text, linuxSystemError(err)
}
