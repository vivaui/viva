//go:build linux

package app

import "os/exec"

func openWithCommand(executable, path string) *exec.Cmd {
	return exec.Command(executable, path)
}

func nativeOpenWith(executable, path string) error {
	return startOpenWith(openWithCommand(executable, path))
}
