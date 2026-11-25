//go:build (darwin && !ios) || windows

package app

// nativeTextMenus reports that this platform shows the system's own menu for a
// text field.
func nativeTextMenus() bool { return true }
