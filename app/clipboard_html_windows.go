//go:build windows

package app

import windowsplatform "github.com/vivaui/viva/internal/platform/windows"

func nativeSetClipboardHTML(html, alt string) error {
	return windowsplatform.SetClipboardHTML(html, alt)
}
