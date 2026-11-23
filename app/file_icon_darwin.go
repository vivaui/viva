//go:build darwin && !ios

package app

import darwinplatform "github.com/vivaui/viva/internal/platform/darwin"

func nativeFileIcon(path string, pixels int) ([]byte, error) {
	return darwinplatform.FileIconBGRA(path, pixels)
}
