//go:build darwin

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise cursor ownership with real NSView hit testing rather than a duplicate Go model.
func TestNativeWebCursorOwnership(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "cursor-owner")
	compile := exec.Command("xcrun", "clang", "-x", "objective-c", "-fno-objc-arc", "-framework", "AppKit", "-I", ".", "testdata/cursor_owner.m", "-o", binary)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile native cursor ownership regression: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native cursor ownership regression failed: %v\n%s", err, output)
	}
}
