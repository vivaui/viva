//go:build linux && !vivagtk

package app

import (
	"github.com/vivaui/viva/internal/platform"
	linuxplatform "github.com/vivaui/viva/internal/platform/linux"
)

func runNative(config platform.Config) error { return linuxplatform.Run(config) }
func runNativeApplication(start platform.ApplicationStart) error {
	return linuxplatform.RunApplication(start)
}
