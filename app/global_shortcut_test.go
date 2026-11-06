//go:build darwin || windows || linux

package app

import (
	"github.com/vivaui/viva/ui"
	"testing"
)

func TestGlobalShortcutDriverRegistrationLifetime(t *testing.T) {
	d, err := NewDriver(Config{Width: 160, Height: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := GlobalShortcut{Modifiers: ui.ModifierControl | ui.ModifierAlt, Key: "g"}
	calls := 0
	stop, err := RegisterGlobalShortcut(s, func() { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RegisterGlobalShortcut(s, func() {}); err == nil {
		t.Fatal("duplicate binding accepted")
	}
	if ok, err := d.PressGlobalShortcut(s); !ok || err != nil || calls != 1 {
		t.Fatalf("press: %v %v %d", ok, err, calls)
	}
	stop()
	stop()
	if ok, _ := d.PressGlobalShortcut(s); ok {
		t.Fatal("stopped binding still active")
	}
	stop, err = RegisterGlobalShortcut(s, func() { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	stop()
	if ok, _ := d.PressGlobalShortcut(s); ok {
		t.Fatal("closed driver accepted input")
	}
}
func TestGlobalShortcutValidation(t *testing.T) {
	for _, s := range []GlobalShortcut{{Key: "G"}, {Modifiers: ui.ModifierControl, Key: "invalid"}} {
		if _, err := RegisterGlobalShortcut(s, func() {}); err == nil {
			t.Fatal("invalid shortcut accepted")
		}
	}
	if _, err := RegisterGlobalShortcut(GlobalShortcut{Modifiers: ui.ModifierAlt, Key: "g"}, nil); err == nil {
		t.Fatal("nil callback accepted")
	}
}
