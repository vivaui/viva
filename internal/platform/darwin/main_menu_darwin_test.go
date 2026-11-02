//go:build darwin

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The menu bar is what makes Command+H and Command+M work: AppKit only runs a
// system shortcut that a menu item carries. This compiles the adapter's own
// menu with a program that reads it back, and fails if Command+W becomes a menu
// key equivalent, which would take the key from an application that closes a tab
// with it.
func TestMainMenuCarriesTheSystemShortcutsAndLeavesCommandW(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "main-menu")
	command := exec.Command("xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/main_menu.m", "main_menu_darwin.m",
		"-o", executable, "-framework", "Cocoa")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native main menu test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native main menu: %v\n%s", err, output)
	}
}
