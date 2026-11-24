//go:build darwin || windows || linux

package app

import (
	"runtime"

	"github.com/vivaui/viva/internal/platform"
)

// Run starts the native application loop and blocks until the final window
// closes. It must be called from the program's main goroutine.
// On macOS, call runtime.LockOSThread in an init function in package main to
// keep that goroutine on the process main thread for AppKit.
func Run(config Config) error {
	attendFaults()
	normalized, err := config.normalize()
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	return runNative(platform.Config{
		Title:             normalized.Title,
		Class:             normalized.Class,
		Icon:              normalized.Icon.platform(),
		Width:             normalized.Width,
		Height:            normalized.Height,
		MinWidth:          normalized.MinWidth,
		MinHeight:         normalized.MinHeight,
		Maximized:         normalized.Maximized,
		WindowMode:        platform.WindowMode(normalized.WindowMode),
		WindowMaterials:   normalized.windowMaterials(),
		TitlebarHeight:    normalized.TitlebarHeight,
		TrafficLightScale: normalized.TrafficLightScale,
		ClearColor: platform.Color{
			Red:   normalized.ClearColor.Red,
			Green: normalized.ClearColor.Green,
			Blue:  normalized.ClearColor.Blue,
			Alpha: normalized.ClearColor.Alpha,
		},
		WindowBorderColor: borderColorHandler(normalized.WindowBorderColor),
	})
}
