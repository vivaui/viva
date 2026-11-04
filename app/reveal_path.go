package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RevealPath shows an existing local file or directory in the system file
// manager, selecting the entry when the platform supports it. Unlike OpenPath,
// it never runs or opens the item, including executable files and shortcuts.
// A symbolic link is selected as a directory entry, not followed for launching.
// The path must be absolute and contain no NUL. Driver-backed runs record the
// request without asking the system to open a window. A native call can wait for
// the file manager, so callers on an interactive window thread should run it
// away from that thread and handle its result when it arrives.
func RevealPath(path string) error {
	if strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("app: reveal path contains NUL")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("app: %q is not an absolute path", path)
	}
	path = filepath.Clean(path)
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	if err, handled := driverRevealPath(path); handled {
		return err
	}
	return nativeRevealPath(path)
}
