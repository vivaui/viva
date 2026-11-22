//go:build !ios

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The Dock tile badge belongs to the system: this compiles the adapter's own
// function with a program that asks AppKit what the tile shows afterwards, so a
// label reaches the tile and an empty one takes it away.
func TestNativeDockBadgeSetsAndClearsTheTileLabel(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "dock-badge")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/dock_badge.m", "dock_badge_darwin.m",
		"-o", executable, "-framework", "Cocoa")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native Dock badge test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native Dock badge: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
