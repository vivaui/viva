//go:build windows

package windows

import (
	"os"
	"testing"
)

func TestNativeGlobalShortcutRegistration(t *testing.T) {
	if os.Getenv("VIVA_NATIVE_GLOBAL_SHORTCUT_TEST") != "1" {
		t.Skip("opt in to native desktop hotkey registration")
	}
	// Ctrl+Alt+Shift+F22 avoids common application and debugger shortcuts.
	stop, err := RegisterGlobalShortcut(7, 0x85, func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if duplicate, err := RegisterGlobalShortcut(7, 0x85, func() {}); err == nil {
		duplicate()
		t.Fatal("OS accepted a conflicting registration")
	}
	stop()
	stop()
	replacement, err := RegisterGlobalShortcut(7, 0x85, func() {})
	if err != nil {
		t.Fatalf("stop did not release OS registration: %v", err)
	}
	replacement()
}
