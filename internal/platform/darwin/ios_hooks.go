//go:build ios

package darwin

import (
	"errors"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// What a macOS window or application has that an iOS one does not, or has not
// yet (ADR 0095): each is a plain stand-in here, and moves out of this file as
// the iOS backend gets its own.

// webState is the pages of one window; iOS hosts none yet, so a window never
// makes one (newWebState answers nil) and these are never called.
type webState struct {
	token unsafe.Pointer
}

func newWebState(*runState, *platform.WebHost, []platform.NativeView) *webState { return nil }
func (*webState) frame(platform.Frame)                                          {}
func (*webState) layout(bool)                                                   {}
func (*webState) closeAll()                                                     {}

// AddStatusItem: iOS has no status area.
func (application *nativeApplication) AddStatusItem(platform.StatusItemConfig) (platform.StatusItem, error) {
	return nil, errors.New("darwin: iOS has no status item")
}

// setApplicationIcon: an iOS application's icon is its bundle's.
func setApplicationIcon(platform.WindowIcon) {}

// Quit does nothing: an iOS application does not end itself; the system does.
func (application *nativeApplication) Quit() {}

// SetDockBadge and SetProgress: there is no Dock; an icon badge needs a permission
// this backend does not ask for yet.
func (application *nativeApplication) SetDockBadge(string) {}
func (application *nativeApplication) SetProgress(float32) {}

// SetMenuBar: iOS has no menu bar.
func (application *nativeApplication) SetMenuBar([]platform.MenuBarMenu) error {
	return errors.New("darwin: iOS has no menu bar")
}
