//go:build darwin

package app

import (
	"errors"
	"fmt"

	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

func nativePreventSleep(reason string) (func(), error) {
	release, err := darwinplatform.PreventSleep(reason)
	if errors.Is(err, darwinplatform.ErrSleepUnavailable) {
		return nil, fmt.Errorf("%w: %v", ErrSleepPreventionUnavailable, err)
	}
	return release, err
}
