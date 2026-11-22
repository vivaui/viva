//go:build !linux || android

package app

func nativeSetPrimarySelection(string) error  { return ErrUnsupported }
func nativePrimarySelection() (string, error) { return "", ErrUnsupported }
