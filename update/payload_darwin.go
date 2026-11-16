//go:build darwin

package update

import (
	"fmt"
	"os"
)

// installedPayload returns the .app bundle the running program belongs to.
func installedPayload() (root, kind string, err error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("update: locating the running program: %w", err)
	}
	bundle, err := bundleRootFor(executable)
	if err != nil {
		return "", "", err
	}
	return bundle, "app", nil
}
