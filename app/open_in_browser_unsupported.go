//go:build !windows && !darwin && !linux

package app

import "fmt"

func nativeOpenInBrowser(string) error {
	return fmt.Errorf("%w: OpenPathInBrowser", ErrUnsupported)
}
