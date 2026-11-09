package darwin

import (
	"os"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestNativeCarbonGlobalShortcut(t *testing.T) {
	if os.Getenv("VIVA_NATIVE_GLOBAL_SHORTCUT_TEST") != "1" {
		t.Skip("requires a macOS desktop session")
	}
	const modifiers = gfx.ModifierControl | gfx.ModifierAlt | gfx.ModifierShift
	calls := make(chan struct{}, 2)
	binding, err := registerCarbonShortcut(modifiers, "G", func() { calls <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer binding.close()
	if other, err := registerCarbonShortcut(modifiers, "G", func() {}); err == nil {
		other.close()
		t.Fatal("Carbon accepted a conflicting binding")
	}
	send := func(down bool) {
		t.Helper()
		if status := sendCarbonShortcutTestEvent(binding.id, down); status != 0 {
			t.Fatalf("dispatch Carbon hotkey event: OSStatus %d", status)
		}
	}
	wait := func() {
		t.Helper()
		select {
		case <-calls:
		case <-time.After(2 * time.Second):
			t.Fatal("Carbon event did not deliver its callback")
		}
	}
	send(true)
	wait()
	send(true)
	select {
	case <-calls:
		t.Fatal("repeated press retriggered callback")
	default:
	}
	send(false)
	send(true)
	wait()
	select {
	case <-calls:
		t.Fatal("repeated press retriggered callback")
	default:
	}
	binding.close()
	binding.close()
	if status := sendCarbonShortcutTestEvent(binding.id, true); status == 0 {
		t.Fatal("stopped binding still claims the event")
	}
	again, err := registerCarbonShortcut(modifiers, "G", func() {})
	if err != nil {
		t.Fatalf("stop did not release Carbon hotkey: %v", err)
	}
	again.close()
}
