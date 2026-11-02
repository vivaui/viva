//go:build linux

package app

import "os"

// currentDesktopTheme reads the desktop's name from the environment once; the
// desktop a session runs on cannot change without logging out again.
func currentDesktopTheme() desktopTheme {
	return detectDesktopTheme(os.Getenv("XDG_CURRENT_DESKTOP"))
}
