//go:build darwin

package app

import darwinplatform "github.com/vivaui/viva/internal/platform/darwin"

func nativeSetClipboardHTML(html, alt string) error {
	return darwinplatform.SetClipboardHTML(html, alt)
}
