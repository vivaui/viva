//go:build darwin || windows || linux

package app

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// keyProbe is a screen with a terminal that asks for key releases and a button
// beside it, and records what the terminal's key handler and the pointer saw.
type keyProbe struct {
	releases bool // whether the terminal asks for key releases
	keys     []gfx.KeyEvent
	pointers []gfx.PointerEvent
	text     []TextEvent
}

func (probe *keyProbe) config() Config {
	config := Config{Title: "keys", Width: 400, Height: 200}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 200, Height: size.Height}, "Terminal", gfx.HitOptions{Role: gfx.RoleTerminal})
		canvas.HitWithOptions(gfx.Item(2), gfx.Rect{X: 200, Width: 200, Height: size.Height}, "Other", gfx.HitOptions{Role: gfx.RoleButton})
	}
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		probe.keys = append(probe.keys, event)
		return gfx.EventResult{Handled: true}
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		probe.pointers = append(probe.pointers, event)
		return gfx.EventResult{}
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if !context.Terminal {
			return nil
		}
		return &TextInput{
			Bounds: gfx.Rect{X: 8, Y: 16, Width: 8, Height: 16}, FontSize: 13, KeyReleases: probe.releases,
			OnEvent: func(event TextEvent) (TextState, error) {
				probe.text = append(probe.text, event)
				return TextState{}, nil
			},
		}
	}
	return config
}

func newKeyDriver(t *testing.T, probe *keyProbe) *Driver {
	t.Helper()
	driver, err := NewDriver(probe.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	probe.keys, probe.pointers = nil, nil
	return driver
}

// summary reduces key events to what a remote desktop acts on.
func summary(events []gfx.KeyEvent) []string {
	var out []string
	for _, event := range events {
		name := event.Text
		switch {
		case event.Key.IsModifier():
			name = map[gfx.Key]string{gfx.KeyShiftLeft: "shift", gfx.KeyControlLeft: "ctrl", gfx.KeyAltLeft: "alt"}[event.Key]
		case event.Key == gfx.KeyLeft:
			name = "left"
		}
		if event.Release {
			name += " up"
		} else if event.Repeat {
			name += " repeat"
		}
		out = append(out, name)
	}
	return out
}

func TestKeyDownAndUpDeliverPairedPressesAndReleases(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	steps := []func() error{
		func() error { return driver.KeyDown(gfx.KeyShiftLeft, "") },
		func() error { return driver.KeyDown(gfx.KeyUnknown, "A") },
		func() error { return driver.KeyDown(gfx.KeyUnknown, "A") }, // the remote's own repeat stays the platform's
		func() error { return driver.KeyDown(gfx.KeyLeft, "") },
		func() error { return driver.KeyUp(gfx.KeyShiftLeft, "") }, // Shift comes up before the letter does
		func() error { return driver.KeyUp(gfx.KeyUnknown, "a") },
		func() error { return driver.KeyUp(gfx.KeyLeft, "") },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"shift", "a", "a repeat", "left", "shift up", "a up", "left up"}
	if got := summary(probe.keys); !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	if probe.keys[1].Modifiers != gfx.ModifierShift || probe.keys[5].Modifiers != 0 {
		t.Fatalf("modifiers at press and at release = %v, %v", probe.keys[1].Modifiers, probe.keys[5].Modifiers)
	}
	if len(probe.text) != 0 {
		t.Fatalf("a key delivered as a key must not also be text: %+v", probe.text)
	}
	if probe.keys[1].Text != "a" || probe.keys[1].Typed != "A" {
		t.Fatalf("Shift+a is key %q typing %q, want \"a\" typing \"A\"", probe.keys[1].Text, probe.keys[1].Typed)
	}
}

func TestKeyUpOfAKeyThatWentDownElsewhereIsDropped(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	if err := driver.KeyUp(gfx.KeyUnknown, "x"); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 0 {
		t.Fatalf("an orphan release reached the handler: %+v", probe.keys)
	}
}

func TestTerminalsThatDidNotAskNeverSeeReleasesOrModifierKeys(t *testing.T) {
	probe := &keyProbe{}
	driver := newKeyDriver(t, probe)
	for _, step := range []func() error{
		func() error { return driver.KeyDown(gfx.KeyShiftLeft, "") },
		func() error { return driver.KeyDown(gfx.KeyLeft, "") },
		func() error { return driver.KeyUp(gfx.KeyLeft, "") },
		func() error { return driver.KeyUp(gfx.KeyShiftLeft, "") },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range probe.keys {
		if event.Release || event.Key.IsModifier() {
			t.Fatalf("unrequested event %+v", event)
		}
	}
	if len(probe.keys) != 1 || probe.keys[0].Key != gfx.KeyLeft || probe.keys[0].Modifiers != gfx.ModifierShift {
		t.Fatalf("keys = %+v", probe.keys)
	}
}

func TestHeldModifiersApplyToPointerEvents(t *testing.T) {
	probe := &keyProbe{}
	driver := newKeyDriver(t, probe)
	if err := driver.KeyDown(gfx.KeyControlLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.Click(50, 50); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyUp(gfx.KeyControlLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.Click(50, 50); err != nil {
		t.Fatal(err)
	}
	var held, plain int
	for _, event := range probe.pointers {
		if event.Kind != gfx.PointerDown {
			continue
		}
		if event.Modifiers == gfx.ModifierControl {
			held++
		} else if event.Modifiers == 0 {
			plain++
		}
	}
	if held != 1 || plain != 1 {
		t.Fatalf("presses with Control held=%d without=%d: %+v", held, plain, probe.pointers)
	}
}

func TestLosingTheWindowFocusReleasesEveryHeldKey(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	for _, key := range []gfx.Key{gfx.KeyAltLeft, gfx.KeyControlLeft, gfx.KeyLeft} {
		if err := driver.KeyDown(key, ""); err != nil {
			t.Fatal(err)
		}
	}
	probe.keys = nil
	if err := driver.LoseFocus(); err != nil {
		t.Fatal(err)
	}
	if got, want := summary(probe.keys), []string{"left up", "ctrl up", "alt up"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("releases = %v, want %v", got, want)
	}
	if err := driver.KeyUp(gfx.KeyLeft, ""); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 3 {
		t.Fatalf("a key released by the focus loss was released again: %+v", probe.keys)
	}
}

func TestMovingFocusOffTheTerminalReleasesItsKeys(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	if err := driver.KeyDown(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyDown(gfx.KeyUnknown, "q"); err != nil {
		t.Fatal(err)
	}
	probe.keys = nil
	if err := driver.Click(300, 100); err != nil { // the other control takes the focus
		t.Fatal(err)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if got, want := summary(probe.keys), []string{"q up", "shift up"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("releases = %v, want %v", got, want)
	}
	if probe.keys[0].Target != gfx.Item(1) {
		t.Fatalf("a release goes to the hit the key was pressed on: %+v", probe.keys[0])
	}
}

func TestStoppingTheRequestForReleasesReleasesHeldKeys(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	if err := driver.KeyDown(gfx.KeyLeft, ""); err != nil {
		t.Fatal(err)
	}
	probe.keys, probe.releases = nil, false
	if err := driver.redraw(); err != nil {
		t.Fatal(err)
	}
	if got := summary(probe.keys); !reflect.DeepEqual(got, []string{"left up"}) {
		t.Fatalf("keys = %v", got)
	}
}

// A key an input method consumed has no press the handler saw, so its release
// is an orphan; one that reached the handler is paired even when it was the
// text system that decided.
func TestOnlyKeysTheHandlerSawGoDownAreReleased(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	// The input method has the key: it arrives as composition, not a key event.
	if err := driver.textEvent(platformSetMarked("ni")); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.painter.key(platform.KeyEvent{Release: true, Code: 'n'}); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 0 {
		t.Fatalf("release of a consumed key reached the handler: %+v", probe.keys)
	}
}

// A release carries the identity the press had. A modifier released first
// changes the platform's release character, but the handler must see the
// character the press named.
func TestKeyReleaseKeepsThePressConsoleIdentity(t *testing.T) {
	probe := &keyProbe{releases: true}
	driver := newKeyDriver(t, probe)
	press := gfx.Win32Key{VirtualKey: 'C', ScanCode: 46, Unicode: 3, ControlState: 0x0008, Repeat: 1}
	release := gfx.Win32Key{VirtualKey: 'C', ScanCode: 46, Unicode: 'c', ControlState: 0, Repeat: 1}
	if _, err := driver.painter.key(platform.KeyEvent{Code: 46, Win32: press}); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.painter.key(platform.KeyEvent{Release: true, Code: 46, Win32: release}); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 2 {
		t.Fatalf("events = %+v", probe.keys)
	}
	if got := probe.keys[1].Win32; got != press {
		t.Fatalf("release Win32 = %+v, want the press's %+v", got, press)
	}
}

func BenchmarkKeyPressAndRelease(b *testing.B) {
	probe := &keyProbe{releases: true}
	config := probe.config()
	config.OnKey = func(gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{Handled: true} }
	driver, err := NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		b.Fatal(err)
	}
	press := platform.KeyEvent{Key: gfx.KeyLeft, Code: 7}
	release := platform.KeyEvent{Release: true, Code: 7}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := driver.painter.key(press); err != nil {
			b.Fatal(err)
		}
		if _, err := driver.painter.key(release); err != nil {
			b.Fatal(err)
		}
	}
}
