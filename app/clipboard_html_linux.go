//go:build linux

package app

import linuxplatform "github.com/vivaui/viva/internal/platform/linux"

func nativeSetClipboardHTML(html, alt string) error {
	return linuxSystemError(linuxplatform.SetClipboardHTML(html, alt))
}
