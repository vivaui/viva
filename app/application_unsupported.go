//go:build !darwin && !windows && !linux && !js

package app

import (
	"fmt"
	"runtime"
)

// RunApplication reports that the current native backend is unavailable.
func RunApplication(start func(*Application) error) error {
	return fmt.Errorf("app: multi-window backend for %s/%s is not supported yet", runtime.GOOS, runtime.GOARCH)
}
