//go:build !ios

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The native drop target runs on a view that is never shown, with a stand-in for
// what AppKit passes it, so the test opens nothing on screen.
func TestNativeFileDropTarget(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "file-drop")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/file_drop.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native file drop test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native file drop target: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
