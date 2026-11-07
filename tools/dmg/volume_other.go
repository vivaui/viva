//go:build !darwin

package dmg

import "errors"

// Describe reports that disk images are only laid out on a Mac, which is also
// where hdiutil is.
func Describe(mount, volume, name string) (File, error) {
	return File{}, errors.New("dmg: disk images are made on macOS")
}
