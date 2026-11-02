//go:build windows

package instance

import "syscall"

// shortPath returns the 8.3 short form of an existing absolute path. Volumes
// with short names disabled, and paths already short, return ok false.
func shortPath(path string) (string, bool) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	size, err := syscall.GetShortPathName(ptr, nil, 0)
	if err != nil || size == 0 {
		return "", false
	}
	buf := make([]uint16, size)
	if n, err := syscall.GetShortPathName(ptr, &buf[0], size); err != nil || n == 0 {
		return "", false
	}
	short := syscall.UTF16ToString(buf)
	if short == "" || short == path {
		return "", false
	}
	return short, true
}
