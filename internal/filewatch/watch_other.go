//go:build !linux && !darwin && !windows

package filewatch

func openBackend(string, *Watch) (backend, error) { return nil, ErrUnsupported }
