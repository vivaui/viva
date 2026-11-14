//go:build !darwin && !windows

package app

import "fmt"

func nativeSetClipboardImagePNG([]byte) error {
	return fmt.Errorf("write clipboard image: %w", ErrUnsupported)
}
