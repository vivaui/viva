//go:build darwin || windows || (linux && !android && !vivagtk)

package app

// capabilityTransparentWindow reports that this build can composite a window
// over the desktop: Metal layers on macOS, DirectComposition on Windows, and an
// ARGB frame window on the X11 shell of Linux, where OnTransparentChange says
// whether a compositing manager is there. The GTK shell does not draw one.
const capabilityTransparentWindow = CapabilityTransparentWindow
