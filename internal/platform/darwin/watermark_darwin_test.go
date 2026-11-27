//go:build darwin && !ios

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWatermarkNativeStackingClippingAndInputTransparency(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "watermark-views")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror", "-framework", "Cocoa", "-framework", "QuartzCore", "-framework", "CoreText", "testdata/watermark.m", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native watermark view test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native watermark view test: %v\n%s", err, output)
	}
}
