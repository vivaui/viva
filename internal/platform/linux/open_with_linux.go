//go:build linux

package linux

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/vivaui/viva/internal/platform"
)

// OpenWith starts executable with path as its only argument; the child is
// reaped in the background once it exits.
func OpenWith(executable, path string) error {
	if err := platform.StartReaped(exec.Command(executable, path)); err != nil {
		return fmt.Errorf("linux: launch %s: %w", filepath.Base(executable), err)
	}
	return nil
}
