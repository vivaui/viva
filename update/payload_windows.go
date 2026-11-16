//go:build windows

package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// installedPayload returns the directory holding the running program. Both an
// NSIS installation and a portable copy keep the application's files there:
// the executable and the folders it loads. A patch touches only the files it
// names, so the installer's uninstaller and shortcuts stay as they are.
func installedPayload() (root, kind string, err error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("update: locating the running program: %w", err)
	}
	resolved, err := filepath.Abs(executable)
	if err != nil {
		return "", "", fmt.Errorf("update: resolving %s: %w", executable, err)
	}
	return filepath.Dir(resolved), "dir", nil
}
