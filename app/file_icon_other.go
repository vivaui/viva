//go:build !darwin && !windows

package app

func nativeFileIcon(string, int) ([]byte, error) { return nil, ErrUnsupported }
