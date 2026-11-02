//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

const openWithCreateNoWindow = 0x08000000

func openWithCommand(executable, path string) *exec.Cmd {
	command := exec.Command(executable, path)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: openWithCreateNoWindow,
	}
	return command
}

func nativeOpenWith(executable, path string) error {
	return startOpenWith(openWithCommand(executable, path))
}
