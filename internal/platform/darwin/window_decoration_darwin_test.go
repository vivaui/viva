//go:build !ios

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWindowDecorationRequestsCompleteOnceWhenClosing(t *testing.T) {
	seen := postedDrainResult(t, func() []int {
		window := &nativeWindow{mode: WindowTitled}
		var seen []int
		if err := window.SetDecorated(false, func(err error) {
			if err != nil {
				seen = append(seen, 1)
			} else {
				seen = append(seen, -1)
			}
		}); err != nil || len(seen) != 0 {
			return nil
		}
		if err := window.SetDecorated(true, func(error) { seen = append(seen, -2) }); err == nil {
			return nil
		}
		window.Close()
		vivaApplicationDrain()
		return seen
	})
	if len(seen) != 1 || seen[0] != 1 {
		t.Fatalf("completion sequence = %v, want one error for the accepted request", seen)
	}
}

func TestNativeWindowDecorationPreservesContent(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "window-decoration")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/window_decoration.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native window decoration test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native window decoration: %v\n%s", err, output)
	}
}
