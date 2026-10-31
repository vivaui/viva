//go:build darwin || windows || linux

package app

import (
	"runtime"

	"github.com/vivaui/viva/internal/platform"
)

// Run starts the native application loop and blocks until the final window
// closes. It must be called from the program's main goroutine.
func Run(config Config) error {
	normalized, err := config.normalize()
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	return runNative(platform.Config{
		Title:           normalized.Title,
		Class:           normalized.Class,
		Icon:            normalized.Icon.platform(),
		Width:           normalized.Width,
		Height:          normalized.Height,
		MinWidth:        normalized.MinWidth,
		MinHeight:       normalized.MinHeight,
		WindowMode:      platform.WindowMode(normalized.WindowMode),
		WindowMaterials: normalized.WindowMaterials,
		TitlebarHeight:  normalized.TitlebarHeight,
		ClearColor: platform.Color{
			Red:   normalized.ClearColor.Red,
			Green: normalized.ClearColor.Green,
			Blue:  normalized.ClearColor.Blue,
			Alpha: normalized.ClearColor.Alpha,
		},
	})
}
