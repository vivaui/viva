//go:build darwin

package darwin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpenWithReportsBundleLaunchFailures pins the handoff through open: a
// bundle whose executable is gone must fail the call, not launch nothing
// silently, because every mainstream editor is a bundle.
func TestOpenWithReportsBundleLaunchFailures(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "Broken.app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "notes.yml")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := OpenWith(bundle, target); err == nil {
		t.Fatal("an application bundle without an executable launched without an error")
	}
}

func TestOpenWithStartsProgramsDirectly(t *testing.T) {
	if err := OpenWith("/usr/bin/true", t.TempDir()); err != nil {
		t.Fatalf("direct launch: %v", err)
	}
}
