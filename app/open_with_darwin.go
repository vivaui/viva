//go:build darwin

package app

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func openWithCommand(executable, path string) *exec.Cmd {
	if strings.EqualFold(filepath.Ext(executable), ".app") {
		return exec.Command("/usr/bin/open", "-a", executable, path)
	}
	return exec.Command(executable, path)
}

func nativeOpenWith(executable, path string) error {
	return startOpenWith(openWithCommand(executable, path))
}
