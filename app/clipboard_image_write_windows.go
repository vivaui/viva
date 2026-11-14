//go:build windows

package app

import windowsplatform "github.com/vivaui/viva/internal/platform/windows"

func nativeSetClipboardImagePNG(data []byte) error { return windowsplatform.SetClipboardImagePNG(data) }
