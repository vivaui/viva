//go:build linux && !android && !vivaxembed

package linux

import "github.com/vivaui/viva/internal/platform"

// The default XCB backend serves StatusNotifier over D-Bus. It links no GTK
// tray, adds no window state, and goes straight to the ordinary blocking poll.
type trayApplication struct{}
type trayWindow struct{}

func (*linuxApplication) openEmbeddedTray(platform.StatusItemConfig) (platform.StatusItem, error) {
	return nil, platform.ErrStatusAreaUnavailable
}
func (*linuxApplication) closeEmbeddedTray() {}
func (*trayApplication) prepareTrayWait(fds []pollFD, timeout int32) ([]pollFD, int32) {
	return fds, timeout
}
func (*trayApplication) dispatchTray([]pollFD, []pollFD) {}
func (*trayWindow) detachTrayPanel()                     {}
func (*trayWindow) trayPanelCrossing(bool)               {}
