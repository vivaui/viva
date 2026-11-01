//go:build darwin

package app

import darwinplatform "github.com/vivaui/viva/internal/platform/darwin"

func nativeSchemeHandlerAvailable(scheme string) (bool, error) {
	return darwinplatform.SchemeHandlerAvailable(scheme)
}
