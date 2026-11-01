//go:build linux

package app

import linuxplatform "github.com/vivaui/viva/internal/platform/linux"

func nativeSchemeHandlerAvailable(scheme string) (bool, error) {
	available, err := linuxplatform.SchemeHandlerAvailable(scheme)
	return available, linuxSystemError(err)
}
