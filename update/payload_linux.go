//go:build linux

package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// installedPayload returns the directory holding the running executable, which
// is all a Linux update installs.
func installedPayload() (root, kind string, err error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("update: locating the running executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", "", fmt.Errorf("update: resolving the running executable: %w", err)
	}
	return filepath.Dir(resolved), "file", nil
}
