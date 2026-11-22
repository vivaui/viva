//go:build (!linux || android) && (!darwin || ios) && !windows

package filewatch

func openBackend(string, *Watch) (backend, error) { return nil, ErrUnsupported }
