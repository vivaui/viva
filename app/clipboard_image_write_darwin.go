//go:build darwin

package app

import darwinplatform "github.com/vivaui/viva/internal/platform/darwin"

func nativeSetClipboardImagePNG(data []byte) error { return darwinplatform.SetClipboardImagePNG(data) }
