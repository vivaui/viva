//go:build !darwin && !windows && !linux && !(js && wasm)

package app

import (
	"fmt"
	"runtime"

	"github.com/vivaui/viva/gfx"
)

// RunCanvas reports that the current native backend is not implemented yet.
func RunCanvas(config Config, canvas *gfx.Canvas) error {
	if _, err := config.normalize(); err != nil {
		return err
	}
	return fmt.Errorf("app: canvas backend for %s/%s is not supported yet", runtime.GOOS, runtime.GOARCH)
}
