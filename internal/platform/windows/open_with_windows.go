//go:build windows

package windows

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/vivaui/viva/internal/platform"
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

// OpenWith starts executable with path as its only argument and without a
// console window; the child is reaped in the background once it exits.
func OpenWith(executable, path string) error {
	if err := platform.StartReaped(openWithCommand(executable, path)); err != nil {
		return fmt.Errorf("windows: launch %s: %w", filepath.Base(executable), err)
	}
	return nil
}
