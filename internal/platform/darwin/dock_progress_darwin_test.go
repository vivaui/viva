package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The Dock tile belongs to the system: this compiles the adapter's own function
// with a program that asks AppKit what the tile shows afterwards, so a fraction
// puts a bar on the tile, a second one reuses it, and a negative or undefined
// one leaves it drawing the icon alone. The view is deliberately not taken away
// when a bar stops: a program run from a build directory would get the system's
// icon for a bare executable instead of the one it was drawing a moment before.
// Drawing is part of it: the program paints the view once into a bitmap, which
// is where a bar that draws nothing would show up.
func TestNativeDockProgressDrawsAndClearsTheTileBar(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "dock-progress")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/dock_progress.m", "dock_progress_darwin.m",
		"-o", executable, "-framework", "Cocoa")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native Dock progress test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native Dock progress: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
