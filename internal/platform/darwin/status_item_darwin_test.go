//go:build darwin

package darwin

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Status item menus belong to the system. This compiles the adapter's own
// status item implementation with a test runner that verifies dynamic menu
// population via NSMenuDelegate and item action dispatching.
func TestNativeStatusItemMenuDelegateAndSelection(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "status-item")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/status_item.m", "status_item_darwin.m",
		"-o", executable, "-framework", "Cocoa")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native status item test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native status item: %v\n%s", err, output)
	}
	// Exercise the .app launch path too: startup may retain a bundled icon, but
	// an explicit preference must still override the running Dock image.
	bundle := filepath.Join(t.TempDir(), "IconTest.app", "Contents")
	if err := os.MkdirAll(filepath.Join(bundle, "MacOS"), 0700); err != nil {
		t.Fatal(err)
	}
	info := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>status-item</string><key>CFBundleIdentifier</key><string>dev.viva.icon-test</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(bundle, "Info.plist"), []byte(info), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	bundled := filepath.Join(bundle, "MacOS", "status-item")
	if err := os.WriteFile(bundled, data, 0700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(bundled).CombinedOutput(); err != nil {
		t.Fatalf("bundled Dock/status icon: %v\n%s", err, output)
	}
}
