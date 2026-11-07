package darwin

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A live resize changes the layer's bounds before the next drawable is
// presented. The view must keep the previous frame at its own size (top left)
// instead of letting the compositor stretch it to the new bounds, and it must
// size the drawable to the view at the screen's scale so the new frame is drawn
// one texel per device pixel.
func TestLiveResizePinsTheStaleFrameToTheTopLeft(t *testing.T) {
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
