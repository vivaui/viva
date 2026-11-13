//go:build windows

package app

import windowsplatform "github.com/vivaui/viva/internal/platform/windows"

func nativeFileIcon(path string, pixels int) ([]byte, error) {
	return windowsplatform.FileIconBGRA(path, pixels)
}
