//go:build !darwin && !windows && !linux && !(js && wasm)

package app

func nativeSetClipboardHTML(html, alt string) error { return ErrUnsupported }
