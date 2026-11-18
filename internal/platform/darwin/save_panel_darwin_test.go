//go:build darwin

package darwin

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeSavePanelDoesNotDuplicateSuggestedExtension(t *testing.T) {
	testContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	executable := filepath.Join(t.TempDir(), "save-panel")
	command := exec.CommandContext(testContext, "xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/save_panel.m", "-o", executable,
		"-framework", "Cocoa")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native save panel test: %v\n%s", err, output)
	}
	if output, err := exec.CommandContext(testContext, executable, t.TempDir()).CombinedOutput(); err != nil {
		t.Fatalf("native save panel suggested name: %v\n%s", err, output)
	}
}
