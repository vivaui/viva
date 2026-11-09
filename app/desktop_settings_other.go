//go:build !linux

package app

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// watchDesktopSettings has nothing to watch here: these targets read the fonts and the
// motion preference when the window opens.
func watchDesktopSettings(*gfx.Canvas, *platform.Waker) (stop func()) { return func() {} }
