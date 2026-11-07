package linux

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

func TestGlobalShortcutRepeatPairsDoNotRetrigger(t *testing.T) {
	var state shortcutKeyState
	if !state.event(xcbKeyPress, 1) || state.event(xcbKeyPress, 2) {
		t.Fatal("detectable auto-repeat retriggered the chord")
	}
	state.event(xcbKeyRelease, 3)
	if state.event(xcbKeyPress, 3) {
		t.Fatal("core X auto-repeat pair retriggered the chord")
	}
	state.event(xcbKeyRelease, 4)
	if !state.event(xcbKeyPress, 5) {
		t.Fatal("fresh key press was suppressed")
	}
}

func TestNativeX11GlobalShortcut(t *testing.T) {
	if os.Getenv("VIVA_NATIVE_GLOBAL_SHORTCUT_TEST") != "1" {
		t.Skip("requires an isolated X11 display")
	}
	const modifiers = ui.ModifierControl | ui.ModifierAlt | ui.ModifierShift
	presses := make(chan struct{}, 1)
	stop, err := RegisterGlobalShortcut(modifiers, 'g', func() { presses <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if duplicate, err := RegisterGlobalShortcut(modifiers, 'g', func() {}); err == nil {
		duplicate()
		t.Fatal("X11 accepted a conflicting grab")
	}
	if _, err := exec.LookPath("xdotool"); err != nil {
		t.Fatal("isolated display check needs xdotool")
	}
	press := func(chord string) {
		t.Helper()
		if out, err := exec.Command("xdotool", "key", chord).CombinedOutput(); err != nil {
			t.Fatalf("trigger on Xvfb: %v: %s", err, out)
		}
		select {
		case <-presses:
		case <-time.After(2 * time.Second):
			t.Fatal("X11 hotkey did not dispatch")
		}
	}
	press("ctrl+alt+shift+g")
	press("Caps_Lock ctrl+alt+shift+g")
	stop()
	stop()
	again, err := RegisterGlobalShortcut(modifiers, 'g', func() {})
	if err != nil {
		t.Fatalf("stop did not release the grab: %v", err)
	}
	again()
}
