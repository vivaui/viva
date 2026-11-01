//go:build windows

package app

import windowsplatform "github.com/vivaui/viva/internal/platform/windows"

func nativeSchemeHandlerAvailable(scheme string) (bool, error) {
	return windowsplatform.SchemeHandlerAvailable(scheme)
}
