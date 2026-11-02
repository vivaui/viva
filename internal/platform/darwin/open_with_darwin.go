//go:build darwin

package darwin

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/internal/platform"
)

// OpenWith starts executable with path as its only argument. An application
// bundle goes through open, a short-lived launcher whose exit status reports
// whether the handoff to the application worked, so its failure comes back to
// the caller; anything else starts directly and is reaped in the background
// once it exits.
func OpenWith(executable, path string) error {
	if strings.EqualFold(filepath.Ext(executable), ".app") {
		command := exec.Command("/usr/bin/open", "-a", executable, path)
		if err := command.Start(); err != nil {
			return fmt.Errorf("darwin: launch %s: %w", filepath.Base(executable), err)
		}
		if err := command.Wait(); err != nil {
			return fmt.Errorf("darwin: launch %s: %w", filepath.Base(executable), err)
		}
		return nil
	}
	if err := platform.StartReaped(exec.Command(executable, path)); err != nil {
		return fmt.Errorf("darwin: launch %s: %w", filepath.Base(executable), err)
	}
	return nil
}
