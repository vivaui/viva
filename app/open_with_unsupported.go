//go:build !windows && !darwin && !linux

package app

import "fmt"

func nativeOpenWith(_, _ string) error {
	return fmt.Errorf("%w: OpenWith", ErrUnsupported)
}
