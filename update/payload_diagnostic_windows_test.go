//go:build windows

package update

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCopiedSystemExecutableExitStatus(t *testing.T) {
	for _, sample := range []struct {
		name string
		args []string
	}{
		{"whoami.exe", nil},
		{"cmd.exe", []string{"/c", "exit 0"}},
		{"cmd.exe", []string{"/c", "exit 1"}},
	} {
		t.Run(sample.name+" "+strings.Join(sample.args, " "), func(t *testing.T) {
			native := systemExecutable(t, sample.name)
			copy := filepath.Join(t.TempDir(), "app.exe")
			copyWindowsFile(t, native, copy)
			for _, executable := range []string{native, copy} {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				output, err := exec.CommandContext(ctx, executable, sample.args...).CombinedOutput()
				cancel()
				t.Logf("executable=%s args=%q result=%v output=%q", executable, sample.args, err, output)
			}
		})
	}
}
