//go:build windows

package app

import (
	"errors"
	"fmt"

	windowsplatform "github.com/vivaui/viva/internal/platform/windows"
)

func nativePreventSleep(reason string) (func(), error) {
	release, err := windowsplatform.PreventSleep(reason)
	if errors.Is(err, windowsplatform.ErrSleepUnavailable) {
		return nil, fmt.Errorf("%w: %v", ErrSleepPreventionUnavailable, err)
	}
	return release, err
}
