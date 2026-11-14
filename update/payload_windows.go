//go:build windows

package update

import "fmt"

// installedPayload has nothing to describe on Windows: the NSIS installer owns
// the installation and patches are not offered there.
func installedPayload() (root, kind string, err error) {
	return "", "", fmt.Errorf("%w: Windows installs updates from an installer, not a payload", ErrUnsupported)
}
