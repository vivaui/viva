//go:build !linux

package app

// currentDesktopTheme is the default theme where no desktop environment names
// itself in the environment; macOS and Windows draw their own window buttons.
func currentDesktopTheme() desktopTheme { return desktopDefault }
