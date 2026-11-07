package darwin

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A live resize changes the layer's bounds before the drawable for the new size
// is presented. The view must present that drawable in the same Core Animation
// transaction as the bounds (presentsWithTransaction), so the compositor never
// scales the previous frame over the new window. The drawable must also follow
// the view at the screen's scale, so the frame is drawn one texel per device
// pixel.
func TestLiveResizePresentsInTheWindowTransaction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	executable := filepath.Join(t.TempDir(), "live-resize")
	command := exec.CommandContext(ctx, "xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/live_resize.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native live resize test: %v\n%s", err, output)
	}
	if output, err := exec.CommandContext(ctx, executable).CombinedOutput(); err != nil {
		t.Fatalf("native live resize: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
