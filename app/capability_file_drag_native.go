//go:build (darwin && !ios) || windows || (linux && !android && !vivagtk)

package app

// capabilityFileDrag reports that this build can take files out of a window:
// an NSDraggingSession on macOS, an OLE drag on Windows, and the XDND source of
// the X11 shell on Linux. The GTK shell speaks Wayland, whose drag source this
// adapter does not bind, and the mobile targets have no file manager to take
// them.
const capabilityFileDrag = CapabilityFileDrag
