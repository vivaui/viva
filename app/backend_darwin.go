//go:build !ios

package app

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/darwin"
)

func runNative(config platform.Config) error { return darwin.Run(config) }
func runNativeApplication(start platform.ApplicationStart) error {
	return darwin.RunApplication(start)
}

func checkNativeApplicationThread() error { return darwin.CheckApplicationThread() }
