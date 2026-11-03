//go:build darwin || windows || linux

package app

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// SetFullscreenMonitor sets the simulated monitor's full bounds in logical
// pixels, not its work area. The default monitor is 1920 by 1080 at (0, 0).
// Updating the monitor while fullscreen resizes the window without changing
// the placement that will be restored.
func (driver *Driver) SetFullscreenMonitor(bounds ui.Rect) error {
	if bounds.Width < 1 || bounds.Height < 1 ||
		!finiteFullscreenCoordinate(bounds.X) || !finiteFullscreenCoordinate(bounds.Y) ||
		!finiteFullscreenCoordinate(bounds.Width) || !finiteFullscreenCoordinate(bounds.Height) {
		return fmt.Errorf("app: fullscreen monitor dimensions must be positive")
	}
	if driver.closed {
		return fmt.Errorf("app: driver is closed")
	}
	driver.fullscreenMonitor = bounds
	if driver.fullscreen {
		return driver.applyFullscreenFrame(bounds)
	}
	return nil
}

// SetFullscreen changes the simulated native state and restores the previous
// content placement on exit. Like other Driver methods it is not concurrent.
func (driver *Driver) SetFullscreen(fullscreen bool) error {
	if driver.closed {
		return fmt.Errorf("app: driver is closed")
	}
	if err := driver.Pump(); err != nil {
		return err
	}
	driver.fullscreenTarget = fullscreen
	if driver.fullscreenChanging {
		return nil
	}
	driver.fullscreenChanging = true
	defer func() { driver.fullscreenChanging = false }()
	for driver.fullscreen != driver.fullscreenTarget {
		if err := driver.transitionFullscreen(driver.fullscreenTarget); err != nil {
			return err
		}
	}
	return nil
}

func (driver *Driver) transitionFullscreen(fullscreen bool) error {
	if driver.fullscreen == fullscreen {
		return nil
	}
	bounds := driver.fullscreenRestore
	if fullscreen {
		driver.fullscreenRestore = driver.Frame()
		bounds = driver.fullscreenMonitor
		if bounds.Width == 0 {
			bounds = ui.Rect{Width: 1920, Height: 1080}
		}
	}
	driver.fullscreen = fullscreen
	driver.painter.fullscreen = fullscreen
	if driver.config.OnFullscreen != nil {
		driver.config.OnFullscreen(fullscreen)
	}
	err := driver.applyFullscreenFrame(bounds)
	if err == nil && !fullscreen && driver.config.OnFrameChange != nil {
		driver.config.OnFrameChange(driver.Frame())
	}
	return err
}

func finiteFullscreenCoordinate(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

func (driver *Driver) applyFullscreenFrame(bounds ui.Rect) error {
	driver.origin = ui.Point{X: bounds.X, Y: bounds.Y}
	driver.storeFrame(bounds)
	_, err := driver.painter.resize(platform.Size{Width: bounds.Width, Height: bounds.Height, Scale: driver.painter.scale})
	return err
}

// Fullscreen reports the state SetFullscreen last reported.
func (driver *Driver) Fullscreen() bool { return driver.fullscreen }
