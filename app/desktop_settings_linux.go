//go:build linux

package app

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// watchDesktopSettings keeps a canvas in step with the desktop while its window is open: a
// change of the desktop's fonts makes it measure again, a change of the preference for reduced
// motion reaches its transitions, and the window is redrawn. It costs nothing until the
// desktop says something changed.
func watchDesktopSettings(canvas *gfx.Canvas, waker *platform.Waker) (stop func()) {
	platform.SetDesktopObserver(func(changed platform.DesktopSetting) {
		if changed&platform.SettingMotion != 0 {
			canvas.SetReduceMotion(nativeReduceMotion())
		}
		if changed&platform.SettingFonts != 0 {
			canvas.SetTextEngine(nativeTextEngine())
		}
		if waker != nil {
			waker.Wake()
		}
	})
	return func() { platform.SetDesktopObserver(nil) }
}
