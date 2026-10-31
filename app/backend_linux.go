package app

import (
	"github.com/vivaui/viva/internal/platform"
	linuxplatform "github.com/vivaui/viva/internal/platform/linux"
)

// runNative runs a single-window Viva application using the platform-selected native backend.
func runNative(config platform.Config) error { return linuxplatform.Run(config) }

// runNativeApplication runs a multi-window Viva application using the platform-selected native backend.
func runNativeApplication(start platform.ApplicationStart) error {
	return linuxplatform.RunApplication(start)
}
