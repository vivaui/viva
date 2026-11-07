//go:build !darwin && !windows && !linux && !js

package app

import (
	"fmt"
	"runtime"
)

// Run reports that the current platform backend has not been implemented yet.
func Run(config Config) error {
	if _, err := config.normalize(); err != nil {
		return err
	}
	return fmt.Errorf("app: %s/%s is not supported yet", runtime.GOOS, runtime.GOARCH)
}
