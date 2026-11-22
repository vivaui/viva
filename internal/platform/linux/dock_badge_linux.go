//go:build linux && !android

package linux

// SetDockBadge does nothing: the desktops Viva runs on show a launcher icon,
// not a Dock tile with a badge, and have no labelless badge to set.
func (application *linuxApplication) SetDockBadge(text string) {}
