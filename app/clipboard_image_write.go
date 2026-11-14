package app

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/vivaui/viva/internal/platform"
)

// SetClipboardImagePNG replaces the clipboard with a PNG image. It validates
// the encoded and decoded size before changing the clipboard, and retains none
// of the caller's bytes. Driver runs use their private clipboard.
// Native writing is supported on macOS and Windows; other targets currently
// return an error wrapping ErrUnsupported. Call from the window thread.
func SetClipboardImagePNG(data []byte) error {
	if err := platform.CheckClipboardPNG(data); err != nil {
		return err
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("app: invalid clipboard PNG: %w", err)
	}
	const bytesPerPixel = 4
	if config.Width <= 0 || config.Height <= 0 || config.Width > platform.MaxClipboardRawBytes/bytesPerPixel/config.Height {
		return ErrTooLarge
	}
	if system := activeDriverSystem.Load(); system != nil {
		return system.driver.SetClipboardImage(data)
	}
	return nativeSetClipboardImagePNG(data)
}
