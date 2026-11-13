//go:build js && wasm

package app

import webplatform "github.com/vivaui/viva/internal/platform/web"

func nativeSetClipboardHTML(html, alt string) error {
	return webSystemError("write clipboard html", webplatform.SetClipboardHTML(html, alt))
}
