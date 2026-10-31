//go:build linux

package linux

// SetDockBadge does nothing: the desktops Viva runs on show a launcher icon,
// not a Dock tile with a badge, and have no labelless badge to set.
func (application *linuxApplication) SetDockBadge(text string) {}

// SetDockBadge does nothing on the Wayland backend for the same reason: the
// launcher shows an application icon without a badge.
func (application *waylandApplication) SetDockBadge(text string) {}
