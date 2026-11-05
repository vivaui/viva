//go:build darwin

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// Status item menus belong to the system. This compiles the adapter's own
// status item implementation with a test runner that verifies dynamic menu
// population via NSMenuDelegate and item action dispatching.
func TestNativeStatusItemMenuDelegateAndSelection(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "status-item")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/status_item.m", "status_item_darwin.m",
		"-o", executable, "-framework", "Cocoa")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native status item test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native status item: %v\n%s", err, output)
	}
}
