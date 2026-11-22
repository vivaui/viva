//go:build android

package app

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/android"
)

func runNative(config platform.Config) error { return android.Run(config) }
func runNativeApplication(start platform.ApplicationStart) error {
	return android.RunApplication(start)
}
