//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestGlobalShortcutDriverRegistrationLifetime(t *testing.T) {
	d, err := NewDriver(Config{Width: 160, Height: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := GlobalShortcut{Modifiers: gfx.ModifierControl | gfx.ModifierAlt, Key: "F1"}
	calls := 0
	stop, err := RegisterGlobalShortcut(s, func() { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	alias := GlobalShortcut{Modifiers: s.Modifiers, Key: " f01 "}
	if duplicate, err := RegisterGlobalShortcut(alias, func() {}); err == nil {
		duplicate()
		t.Fatal("alternate spelling registered the same function key twice")
	}
	if ok, err := d.PressGlobalShortcut(alias); !ok || err != nil || calls != 1 {
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
	for _, s := range []GlobalShortcut{{Key: "G"}, {Modifiers: gfx.ModifierControl, Key: "invalid"}} {
		if _, err := RegisterGlobalShortcut(s, func() {}); err == nil {
			t.Fatal("invalid shortcut accepted")
		}
	}
	if _, err := RegisterGlobalShortcut(GlobalShortcut{Modifiers: gfx.ModifierAlt, Key: "g"}, nil); err == nil {
		t.Fatal("nil callback accepted")
	}
}

func BenchmarkDriverGlobalShortcutRegistration(b *testing.B) {
	d, err := NewDriver(Config{Width: 160, Height: 100}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	s := GlobalShortcut{Modifiers: gfx.ModifierControl | gfx.ModifierAlt, Key: "g"}
	callback := func() {}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		stop, err := RegisterGlobalShortcut(s, callback)
		if err != nil {
			b.Fatal(err)
		}
		stop()
	}
}
