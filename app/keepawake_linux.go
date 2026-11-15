//go:build linux

package app

import (
	"errors"
	"fmt"

	linuxplatform "github.com/vivaui/viva/internal/platform/linux"
)

func nativePreventSleep(reason string) (func(), error) {
	release, err := linuxplatform.PreventSleep(reason)
	if errors.Is(err, linuxplatform.ErrSleepUnavailable) {
		return nil, fmt.Errorf("%w: %v", ErrSleepPreventionUnavailable, err)
	}
	return release, err
}
