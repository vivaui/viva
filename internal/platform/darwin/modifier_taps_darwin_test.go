package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The native view takes a modifier's press and release only when the
// application listens for taps. It runs on a view that is never shown, with
// events made by hand, so the test opens nothing on screen.
func TestNativeModifierTaps(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "modifier-taps")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/modifier_taps.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native modifier tap test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native modifier taps: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
