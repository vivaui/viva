//go:build windows

package windows

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

// Input injected by virtual key alone has no scan code in its message; the
// press and the release must still name the same physical key.
func TestKeyCodeFallsBackToTheLayoutWhenTheMessageHasNoScanCode(t *testing.T) {
	const vkA = 0x41
	if code := keyCode(vkA, 0); code == 0 {
		t.Fatal("a key injected without a scan code has no code")
	}
	if keyCode(vkA, 0) != keyCode(vkA, 0x1e<<16) {
		t.Fatalf("injected %#x differs from the keyboard's %#x", keyCode(vkA, 0), keyCode(vkA, 0x1e<<16))
	}
}

// A terminal has no text model of its own: the editing keys a text field would
// consume, and the control chords it would undo with, are the program's keys.
// They must reach the application instead of becoming text commands, which is
// what Backspace, Delete and the arrows did before.
func TestATerminalGetsEditingKeysAndChordsAsRawKeys(t *testing.T) {
	state := &windowState{textEnabled: true}
	state.textInput.Terminal = true
	var events []platform.KeyEvent
	state.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
		events = append(events, event)
		return platform.Frame{}, errors.New("stop before the renderer")
	}
	for _, test := range []struct {
		name string
		vk   uintptr
		mods input.Modifiers
		key  input.Key
		text string
	}{
		{"Backspace", 8, 0, input.KeyBackspace, ""},
		{"Delete", 46, 0, input.KeyDelete, ""},
		{"Left", 37, 0, input.KeyLeft, ""},
		{"Shift+Right", 39, input.ModifierShift, input.KeyRight, ""},
		{"Enter", 13, 0, input.KeyEnter, ""},
		{"Ctrl+R", 'R', input.ModifierControl, input.KeyUnknown, "r"},
		{"Ctrl+Space", 0x20, input.ModifierControl, input.KeySpace, ""},
	} {
		events = events[:0]
		if !state.textKey(test.vk, 0, test.mods) {
			t.Fatalf("%s was not consumed", test.name)
		}
		if len(events) != 1 || events[0].Key != test.key || events[0].Text != test.text || events[0].Modifiers != test.mods {
			t.Fatalf("%s = %+v", test.name, events)
		}
	}
	// A printable key is text, inserted when its WM_CHAR arrives, and AltGr
	// (Control+Alt) types a character too.
	for _, test := range []struct {
		name string
		vk   uintptr
		mods input.Modifiers
	}{
		{"a", 'A', 0},
		{"space", 0x20, 0},
		{"AltGr+a", 'A', input.ModifierControl | input.ModifierAlt},
	} {
		events = events[:0]
		if state.textKey(test.vk, 0, test.mods) {
			t.Fatalf("%s was consumed as a key", test.name)
		}
		if len(events) != 0 {
			t.Fatalf("%s sent %+v", test.name, events)
		}
	}
}

// A text field keeps taking the editing commands, and never sees them as keys.
func TestATextFieldStillGetsEditingCommands(t *testing.T) {
	state := &windowState{textEnabled: true}
	var commands []platform.TextCommand
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		commands = append(commands, event.Command)
		return platform.TextState{}, errors.New("stop before the renderer")
	}
	state.config.OnKey = func(platform.KeyEvent) (platform.Frame, error) {
		t.Fatal("a text field sent its editing key to the application")
		return platform.Frame{}, nil
	}
	if !state.textKey(8, 0, 0) {
		t.Fatal("Backspace in a text field was not consumed")
	}
	if len(commands) != 1 || commands[0] != platform.TextDeleteBackward {
		t.Fatalf("text field commands = %v", commands)
	}
}

// A win32-input-mode packet needs the console record's own fields, not a
// reconstruction from the normalized key.
func TestWin32KeyRecordCarriesTheConsoleFields(t *testing.T) {
	key := win32Key('A', 0x1e<<16|3, 'a', 3)
	if key.VirtualKey != 'A' || key.ScanCode != 0x1e || key.Unicode != 'a' || key.Repeat != 3 {
		t.Fatalf("win32 key = %+v", key)
	}
}

// ConPTY raw readers use UnicodeChar, not just the virtual key. In particular
// Enter with a zero character reaches PowerShell but is lost by Node's TTY.
func TestTerminalControlKeysCarryConsoleCharacters(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, vk := range []uintptr{8, 9, 13, 27} {
			state := &windowState{}
			var events []platform.KeyEvent
			state.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
				events = append(events, event)
				return platform.Frame{}, errors.New("stop before the renderer")
			}
			if raw {
				state.keyDown(vk, 1, 0)
			} else {
				event, ok := state.terminalKeyEvent(vk, 1, 0)
				if !ok {
					t.Fatal("control key was not handled")
				}
				events = append(events, event)
			}
			state.keyUp(vk, 1, 0)
			if len(events) != 2 {
				t.Fatalf("events = %+v", events)
			}
			for _, event := range events {
				if event.Win32.Unicode != uint16(vk) {
					t.Errorf("raw=%v vk=%d release=%v: Unicode=%d, want %d", raw, vk, event.Release, event.Win32.Unicode, vk)
				}
			}
		}
	}
}

func TestConsoleKeyCharacterTranslatesControlChords(t *testing.T) {
	for _, test := range []struct {
		vk      uintptr
		control bool
		want    uint16
	}{
		{13, false, 13}, {13, true, 10}, {8, true, 127},
		{'C', true, 3}, {'D', true, 4}, {'Z', true, 26},
		{37, false, 0}, {16, false, 0},
	} {
		var keys [256]byte
		if test.control {
			keys[17] = 0x80
		}
		scan := uint16(keyCode(test.vk, 0) & 0xff)
		if got := consoleKeyCharacter(test.vk, scan, &keys); got != test.want {
			t.Errorf("vk=%d control=%v: character=%d, want %d", test.vk, test.control, got, test.want)
		}
	}
}

// AltGr types a layout character that WM_CHAR delivers as text. Putting it in
// the raw record as well would hand the terminal the same character twice.
func TestAltGrLeavesPrintableCharactersToText(t *testing.T) {
	for _, vk := range []uintptr{'E', '4', 0xbd} {
		if got := win32KeyRecord(vk, 0, 0, 1, true); got.Unicode != 0 {
			t.Errorf("vk=%#x with AltGr: record Unicode=%#x, want none", vk, got.Unicode)
		}
	}
	// A named key still carries its console character on an AltGr chord.
	scan := uintptr(keyCode(13, 0)&0xff) << 16
	if got := win32KeyRecord(13, scan, 0, 1, true); got.Unicode != 13 {
		t.Errorf("Enter with AltGr: record Unicode=%d, want 13", got.Unicode)
	}
	// Without AltGr the key is translated as before.
	if got := win32KeyRecord('E', 0, 0, 1, false); got.Unicode == 0 {
		t.Error("E without AltGr was not translated")
	}
}

func BenchmarkConsoleKeyRecord(b *testing.B) {
	for _, test := range []struct {
		name      string
		vk        uintptr
		character uint16
	}{
		{"enter", 13, 0}, {"printable", 'A', 'a'},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = win32Key(test.vk, 0x1c<<16|1, test.character, 1)
			}
		})
	}
}

// Alt+printable arrives as WM_SYSCHAR, not WM_CHAR. A terminal that asked for
// releases must still see it as a raw key rather than losing it.
func TestAltPrintableIsSentAsARawKey(t *testing.T) {
	state := &windowState{}
	var events []platform.KeyEvent
	state.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
		events = append(events, event)
		return platform.Frame{}, nil
	}
	if !state.keyDown('A', 0x1e<<16, input.ModifierAlt) {
		t.Fatal("Alt+A was not consumed")
	}
	if len(events) != 1 || events[0].Text != "a" || events[0].Modifiers != input.ModifierAlt || events[0].Win32.VirtualKey != 'A' || events[0].Win32.ScanCode != 0x1e {
		t.Fatalf("Alt+A = %+v", events)
	}
}

// WM_CHAR completes a printable key's press with the typed code unit.
func TestCharCompletesTheConsoleRecord(t *testing.T) {
	state := &windowState{}
	var events []platform.KeyEvent
	state.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
		events = append(events, event)
		return platform.Frame{}, nil
	}
	state.pendingKey = pendingKey{set: true, vk: 'A', code: 0x1e}
	if !state.character('a', 0x1e<<16) {
		t.Fatal("the character did not complete the key")
	}
	if len(events) != 1 || events[0].Win32.Unicode != 'a' || events[0].Win32.VirtualKey != 'A' || events[0].Win32.ScanCode != 0x1e {
		t.Fatalf("character = %+v", events)
	}
}

// A plain space is one character, not a key and a character. Win32 reports it
// as VK_SPACE and as WM_CHAR " ", and taking both typed it twice: a terminal
// took the key as the space it sends and then inserted the character too.
func TestAPlainSpaceIsOneCharacter(t *testing.T) {
	state := &windowState{textEnabled: true}
	state.textInput.Terminal = true
	state.textInput.KeyReleases = true
	var events []platform.KeyEvent
	state.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
		events = append(events, event)
		return platform.Frame{}, nil
	}
	if state.keyDown(0x20, 0, 0) {
		t.Fatal("a plain space was taken as a key at WM_KEYDOWN")
	}
	if len(events) != 0 {
		t.Fatalf("a plain space sent %+v before its character", events)
	}
	if !state.character(0x20, 0) {
		t.Fatal("the space did not complete its key")
	}
	if len(events) != 1 || events[0].Key != input.KeyUnknown || events[0].Text != " " || events[0].Typed != " " || events[0].Code == 0 {
		t.Fatalf("space = %+v", events)
	}
	// Without key releases a terminal takes the space from WM_CHAR alone: no
	// named key, or the shell would read two.
	state.textInput.KeyReleases = false
	events = events[:0]
	if state.textKey(0x20, 0, 0) {
		t.Fatal("a plain space was consumed as a terminal key")
	}
	if len(events) != 0 {
		t.Fatalf("a terminal keyed a plain space: %+v", events)
	}
}

func TestAModifierIsReportedOnlyWhenTheApplicationListensForTaps(t *testing.T) {
	var events []platform.KeyEvent
	state := &windowState{}
	state.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
		events = append(events, event)
		return platform.Frame{}, errors.New("stop before the renderer")
	}
	leftShift := uintptr(0x2a) << 16
	state.modifierTap(0x10, leftShift, false, input.ModifierShift)
	state.modifierTap(0x10, leftShift, true, 0)
	if len(events) != 0 {
		t.Fatalf("a window that did not ask heard %+v", events)
	}

	state.config.ModifierTaps = true
	state.modifierTap(0x10, leftShift, false, input.ModifierShift)
	state.modifierTap(0x10, leftShift, true, 0)
	state.modifierTap('A', 0x1e<<16, false, 0) // not a modifier
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	press, release := events[0], events[1]
	if press.Key != input.KeyShiftLeft || press.Release || press.Modifiers != input.ModifierShift || press.Code == 0 {
		t.Errorf("press = %+v", press)
	}
	if release.Key != input.KeyShiftLeft || !release.Release || release.Modifiers != 0 {
		t.Errorf("release = %+v", release)
	}
	if press.Code != release.Code {
		t.Errorf("the press and release of one key have codes %d and %d", press.Code, release.Code)
	}
}
