package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The native content-size path runs on a window that is never shown, so the
// test opens nothing on screen.
func TestNativeWindowContentSize(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "window-size")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/window_size.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native window size test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native window content size: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
