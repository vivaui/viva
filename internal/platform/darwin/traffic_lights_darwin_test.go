package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The native traffic-light placement runs on a window that is never shown, so
// the test opens nothing on screen.
func TestNativeTrafficLightPlacementAndWindowLevel(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "traffic-lights")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/traffic_lights.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native traffic light test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native traffic light placement: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
