//go:build !darwin && !linux && !windows

package update

// installedPayload has no application to describe on a target without a native
// executable.
func installedPayload() (root, kind string, err error) {
	return "", "", ErrUnsupported
}
