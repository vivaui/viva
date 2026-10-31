//go:build linux

package linux

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

func TestWaylandKeyboardKeyTranslation(t *testing.T) {
	app := &waylandApplication{}
	win := &waylandWindow{
		app:     app,
		surface: 200,
	}

	var keyEvents []platform.KeyEvent

	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keyEvents = append(keyEvents, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{
		config: state.config,
	}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	// 1. Press Space (evdev scancode 57)
	app.handleKey(1, 100, 57, 1)

	if len(keyEvents) != 1 {
		t.Fatalf("expected 1 key event, got %d", len(keyEvents))
	}
	if keyEvents[0].Key != input.KeySpace {
		t.Errorf("expected KeySpace, got %v", keyEvents[0].Key)
	}
	if keyEvents[0].Text != " " {
		t.Errorf("expected text ' ', got %q", keyEvents[0].Text)
	}
	if keyEvents[0].Release {
		t.Errorf("expected Release == false for press")
	}

	// 2. Release Space
	app.handleKey(2, 120, 57, 0)
	// Default router without wantsKeyReleases does not forward key releases for normal apps
	if len(keyEvents) != 1 {
		t.Fatalf("expected release not to be delivered without wantsKeyReleases, got %d", len(keyEvents))
	}
}

func TestWaylandKeyboardRepeat(t *testing.T) {
	now := time.Unix(100, 0)
	app := &waylandApplication{
		repeatRate:  25, // 25 Hz = 40ms interval
		repeatDelay: 50,
		now:         func() time.Time { return now },
	}
	win := &waylandWindow{
		app:     app,
		surface: 201,
	}

	var repeatCount atomic.Int32
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Repeat {
					repeatCount.Add(1)
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(10, 100, 28, 1)
	if timeout := app.pollTimeout(now); timeout != 50 {
		t.Fatalf("initial repeat timeout = %dms, want 50ms", timeout)
	}
	app.runKeyRepeat(now.Add(49 * time.Millisecond))
	if repeatCount.Load() != 0 {
		t.Fatal("repeat fired before its initial delay")
	}

	now = now.Add(50 * time.Millisecond)
	app.runKeyRepeat(now)
	if repeatCount.Load() != 1 {
		t.Fatalf("first repeat count = %d, want 1", repeatCount.Load())
	}
	if want := now.Add(40 * time.Millisecond); !app.repeatDeadline.Equal(want) {
		t.Fatalf("next repeat deadline = %v, want %v", app.repeatDeadline, want)
	}

	now = now.Add(500 * time.Millisecond)
	app.runKeyRepeat(now)
	app.runKeyRepeat(now)
	if repeatCount.Load() != 2 {
		t.Fatalf("late repeat catch-up count = %d, want exactly one new event", repeatCount.Load())
	}
	if !app.repeatDeadline.After(now) {
		t.Fatalf("late repeat deadline %v did not move beyond %v", app.repeatDeadline, now)
	}

	app.handleKey(11, 250, 28, 0)
	current := repeatCount.Load()
	app.runKeyRepeat(now.Add(time.Second))
	if repeatCount.Load() != current {
		t.Fatalf("key repeat continued after release: was %d, now %d", current, repeatCount.Load())
	}
	if !app.repeatDeadline.IsZero() {
		t.Fatal("release retained a repeat deadline")
	}
}

func TestWaylandKeyRepeatQueuedAfterDestroyIsIgnored(t *testing.T) {
	app := &waylandApplication{loop: &waylandLoop{}}
	win := &waylandWindow{app: app, surface: 204}
	delivered := false
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				delivered = true
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.windows = []*waylandWindowState{state}
	app.keyboardFocus = win
	app.repeatScancode = 28
	app.repeatGeneration = 1
	app.repeatWindow = win

	win.Close()
	app.post(func() {
		app.dispatchKeyRepeat(win, 28, 1)
	})
	app.loop.drainCommands()

	if delivered {
		t.Fatal("queued key repeat was delivered after window destruction")
	}
	if win.state != nil {
		t.Fatal("expected window state to be destroyed")
	}
}

func TestWaylandIBusModifiersUseXMask(t *testing.T) {
	app := &waylandApplication{
		modifiers: input.ModifierShift | input.ModifierControl | input.ModifierAlt | input.ModifierMeta,
	}
	context := &fakeIMEContext{}
	ime := &linuxIME{app: app, active: true, context: context}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	state.router.ime = ime
	ime.router = state.router
	win.state = state
	app.keyboardFocus = win

	app.handleKey(1, 100, 57, 1)
	if len(context.keys) != 1 {
		t.Fatalf("IBus received %d keys, want 1", len(context.keys))
	}
	want := uint32(xcbModShift | xcbModControl | xcbMod1 | xcbMod4)
	if got := context.keys[0].state; got != want {
		t.Fatalf("IBus modifier mask = %#x, want X mask %#x", got, want)
	}
}

func TestWaylandIBusKeyReleaseUsesReleaseMask(t *testing.T) {
	app := &waylandApplication{modifiers: input.ModifierShift}
	context := &fakeIMEContext{}
	ime := &linuxIME{app: app, active: true, context: context}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	state.router.ime = ime
	ime.router = state.router
	win.state = state
	app.keyboardFocus = win

	app.handleKey(1, 100, 57, 1)
	app.handleKey(2, 120, 57, 0)
	if len(context.keys) != 2 {
		t.Fatalf("IBus received %d keys, want press and release", len(context.keys))
	}
	release := context.keys[1]
	want := uint32(xcbModShift) | ibus.ReleaseMask
	if release.state != want {
		t.Fatalf("IBus release mask = %#x, want %#x", release.state, want)
	}
	if release.done != nil {
		t.Fatal("IBus release unexpectedly has an application delivery callback")
	}
	if release.keysym != context.keys[0].keysym || release.keycode != context.keys[0].keycode {
		t.Fatalf("press and release identities differ: press=%+v release=%+v", context.keys[0], release)
	}
}

func TestWaylandKeyboardErrorsReachApplication(t *testing.T) {
	expectedErr := errors.New("key failed")
	app := &waylandApplication{}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(platform.KeyEvent) (platform.Frame, error) {
				return platform.Frame{}, expectedErr
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win

	app.handleKey(1, 100, 57, 1)
	if !errors.Is(app.firstErr, expectedErr) {
		t.Fatalf("application error = %v, want %v", app.firstErr, expectedErr)
	}
}

func TestWaylandKeyboardFocusAndModifiers(t *testing.T) {
	focusLostCalled := false
	app := &waylandApplication{}
	win := &waylandWindow{
		app:     app,
		surface: 202,
	}
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnFocusLost: func() (platform.Frame, error) {
				focusLostCalled = true
				return platform.Frame{}, nil
			},
		},
	}
	win.state = state
	app.windows = []*waylandWindowState{state}

	initKeyboardListeners()

	// Enter window
	app.keyboardFocus = win

	if app.keyboardFocus != win {
		t.Fatal("expected keyboard focus to be set to win")
	}

	// Set modifiers
	app.modifiers = input.ModifierShift | input.ModifierControl
	if app.modifiers&(input.ModifierShift|input.ModifierControl) != (input.ModifierShift | input.ModifierControl) {
		t.Fatalf("unexpected modifiers: %v", app.modifiers)
	}

	// Leave window
	app.stopKeyRepeat()
	if app.keyboardFocus != nil && app.keyboardFocus.state != nil && app.keyboardFocus.state.config.OnFocusLost != nil {
		app.keyboardFocus.state.config.OnFocusLost()
	}
	app.keyboardFocus = nil

	if !focusLostCalled {
		t.Fatal("expected OnFocusLost to be called on keyboard leave")
	}
	if app.keyboardFocus != nil {
		t.Fatal("expected keyboardFocus to be nil after leave")
	}
}

func TestWaylandKeyRepeatCleanupAndIndependentRelease(t *testing.T) {
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 50,
	}
	win := &waylandWindow{
		app:     app,
		surface: 203,
	}
	repeatEvents := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Repeat {
					repeatEvents++
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, scale: win.displayScale}
	win.state = state
	app.windows = []*waylandWindowState{state}
	app.keyboardFocus = win

	app.handleKey(1, 100, 30, 1)
	app.mu.Lock()
	generation := app.repeatGeneration
	if app.repeatScancode != 30 || app.repeatWindow != win || app.repeatDeadline.IsZero() {
		app.mu.Unlock()
		t.Fatal("key press did not initialize main-loop repeat state")
	}
	app.mu.Unlock()

	// Releasing a different key must not stop the active repeat generation.
	app.handleKey(2, 120, 42, 0)
	app.mu.Lock()
	if app.repeatScancode != 30 || app.repeatGeneration != generation {
		app.mu.Unlock()
		t.Fatal("release of a different key changed the repeat generation")
	}
	app.mu.Unlock()

	app.handleKey(3, 140, 30, 0)
	app.mu.Lock()
	if app.repeatScancode != 0 || app.repeatWindow != nil || !app.repeatDeadline.IsZero() || app.repeatGeneration == generation {
		app.mu.Unlock()
		t.Fatal("releasing the repeating key did not invalidate its state")
	}
	app.mu.Unlock()

	app.dispatchKeyRepeat(win, 30, generation)
	if repeatEvents != 0 {
		t.Fatal("released repeat generation delivered an event")
	}
}

func TestWaylandKeyRepeatRejectsPreviousPressGeneration(t *testing.T) {
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 50,
		xkbState:    1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 'a' },
		},
	}
	win := &waylandWindow{app: app, surface: 205}
	repeatEvents := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Repeat {
					repeatEvents++
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	oldGeneration := app.repeatGeneration
	app.handleKey(2, 120, 30, 0)
	app.handleKey(3, 140, 30, 1)
	if app.repeatGeneration == oldGeneration {
		t.Fatal("same-key press reused the previous generation")
	}

	app.dispatchKeyRepeat(win, 30, oldGeneration)
	if repeatEvents != 0 {
		t.Fatal("previous press generation delivered after same-key re-press")
	}
	app.dispatchKeyRepeat(win, 30, app.repeatGeneration)
	if repeatEvents != 1 {
		t.Fatalf("current press generation delivered %d events, want 1", repeatEvents)
	}
}

func TestWaylandShortcutIsOfferedBeforeInputMethod(t *testing.T) {
	app := &waylandApplication{
		modifiers: input.ModifierControl,
		xkbState:  1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 's' },
		},
	}
	context := &fakeIMEContext{}
	ime := &linuxIME{app: app, active: true, context: context}
	win := &waylandWindow{app: app}
	shortcuts := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
				shortcuts++
				if event.Text != "s" {
					t.Errorf("shortcut text = %q, want s", event.Text)
				}
				return platform.Frame{}, true, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	state.router.ime = ime
	ime.router = state.router
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 31, 1)
	if shortcuts != 1 {
		t.Fatalf("OnShortcut calls = %d, want 1", shortcuts)
	}
	if len(context.keys) != 0 {
		t.Fatalf("input method received a handled shortcut: %+v", context.keys)
	}
}

func TestWaylandTextFieldOwnsClipboardChord(t *testing.T) {
	app := &waylandApplication{
		modifiers: input.ModifierControl,
		xkbState:  1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 'c' },
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}}
	win := &waylandWindow{app: app}
	shortcuts, keys := 0, 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnShortcut: func(platform.KeyEvent) (platform.Frame, bool, error) {
				shortcuts++
				return platform.Frame{}, true, nil
			},
			OnKey: func(platform.KeyEvent) (platform.Frame, error) {
				keys++
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 46, 1)
	if shortcuts != 0 {
		t.Fatalf("OnShortcut took Ctrl+C from the text field (%d calls)", shortcuts)
	}
	if keys != 0 {
		t.Fatalf("OnKey took an owned chord from the text field (%d calls)", keys)
	}
}

func TestWaylandDeadKeyComposes(t *testing.T) {
	keysyms := map[uint32]uint32{
		13: 0xfe51, // dead_acute
		18: 'e',
	}
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 { return keysyms[key-8] },
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}}
	var inserted []string
	text.input.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		if event.Kind == platform.TextInsert {
			inserted = append(inserted, event.Text)
		}
		return platform.TextState{}, nil
	}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	state.router = eventRouter{config: state.config, text: text}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 13, 1)
	app.handleKey(2, 120, 13, 0)
	app.handleKey(3, 140, 18, 1)
	if len(inserted) != 1 || inserted[0] != "é" {
		t.Fatalf("composed text = %q, want é", inserted)
	}
}

func TestWaylandShortcutTextPrefersLatinLayout(t *testing.T) {
	var sym uint32
	app := &waylandApplication{
		modifiers:        input.ModifierControl,
		xkbState:         1,
		xkbKeymap:        3,
		xkbShortcutState: 2,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 0x6d3 },
			stateUpdateMask:   func(uintptr, uint32, uint32, uint32, uint32, uint32, uint32) uint32 { return 0 },
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if level != 0 {
					return 0
				}
				if layout == 0 {
					sym = 0x6d3 // Cyrillic small es, what the active layout types
				} else {
					sym = 'c'
				}
				*out = &sym
				return 1
			},
		},
	}
	win := &waylandWindow{app: app}
	var shortcuts, keys []string
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
				shortcuts = append(shortcuts, event.Text)
				return platform.Frame{}, true, nil
			},
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event.Text)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 46, 1)
	if len(shortcuts) != 1 || shortcuts[0] != "c" {
		t.Fatalf("Ctrl+C shortcut text = %q, want c", shortcuts)
	}

	app.modifiers = 0
	app.handleKey(2, 120, 46, 1)
	if len(keys) != 1 || keys[0] != "с" {
		t.Fatalf("plain key text = %q, want the layout character с", keys)
	}
}

func TestWaylandModifierKeysAreNotRepeated(t *testing.T) {
	now := time.Unix(100, 0)
	app := &waylandApplication{
		repeatRate:  25,
		repeatDelay: 50,
		now:         func() time.Time { return now },
		xkbState:    1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return keysymShiftL },
		},
	}
	win := &waylandWindow{app: app}
	var keys []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 42, 1)
	if len(keys) != 0 {
		t.Fatalf("modifier press reached OnKey: %+v", keys)
	}
	if app.repeatScancode != 0 {
		t.Fatal("modifier press armed key repeat")
	}
	app.runKeyRepeat(now.Add(time.Second))
	if len(keys) != 0 {
		t.Fatalf("modifier repeat reached OnKey: %+v", keys)
	}
}

func TestWaylandTerminalReceivesModifiersOnceAndNoReleaseShortcut(t *testing.T) {
	app := &waylandApplication{
		modifiers: input.ModifierShift,
		xkbState:  1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return keysymShiftL },
		},
	}
	text := &linuxTextInput{enabled: true, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var keys []platform.KeyEvent
	shortcuts := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event)
				return platform.Frame{}, nil
			},
			OnShortcut: func(platform.KeyEvent) (platform.Frame, bool, error) {
				shortcuts++
				return platform.Frame{}, false, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 42, 1)
	app.handleKey(2, 120, 42, 0)
	if len(keys) != 2 {
		t.Fatalf("terminal received %d modifier events, want press and release", len(keys))
	}
	if keys[0].Key != input.KeyShiftLeft || keys[0].Modifiers != input.ModifierShift || keys[0].Release {
		t.Fatalf("modifier press = %+v", keys[0])
	}
	if keys[1].Key != input.KeyShiftLeft || keys[1].Modifiers != 0 || !keys[1].Release {
		t.Fatalf("modifier release = %+v", keys[1])
	}
	if shortcuts != 0 {
		t.Fatalf("modifier events offered %d shortcuts", shortcuts)
	}
}

func TestWaylandModifierPressLeavesRepeatRunning(t *testing.T) {
	now := time.Unix(100, 0)
	keysyms := map[uint32]uint32{
		30: 'a',
		42: keysymShiftL,
	}
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 50,
		now:         func() time.Time { return now },
		xkbState:    1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 { return keysyms[key-8] },
		},
	}
	win := &waylandWindow{app: app}
	repeats := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Repeat {
					repeats++
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	app.handleKey(2, 120, 42, 1)
	if app.repeatScancode != 30 {
		t.Fatal("pressing a modifier cancelled the repeat of the held key")
	}

	now = now.Add(100 * time.Millisecond)
	app.runKeyRepeat(now)
	if repeats != 1 {
		t.Fatalf("repeat count after modifier press = %d, want 1", repeats)
	}
}

func TestWaylandLockKeysDoNotRepeat(t *testing.T) {
	now := time.Unix(100, 0)
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 50,
		now:         func() time.Time { return now },
		xkbState:    1,
		xkbKeymap:   3,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return keysymScrollLock },
			// The evdev keymap marks <SCLK> repeatable; lock keys still must not.
			keymapKeyRepeats: func(uintptr, uint32) int32 { return 1 },
		},
	}
	win := &waylandWindow{app: app}
	keys := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(platform.KeyEvent) (platform.Frame, error) {
				keys++
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 70, 1)
	if app.repeatScancode != 0 {
		t.Fatal("lock key armed key repeat")
	}
	app.runKeyRepeat(now.Add(time.Second))
	if keys != 0 {
		t.Fatalf("lock key produced %d key events", keys)
	}
}

func TestWaylandTerminalTextComesFromShortcutState(t *testing.T) {
	app := &waylandApplication{
		modifiers:        input.ModifierShift,
		xkbState:         1,
		xkbShortcutState: 2,
		xkb: &xkbAPI{
			stateUpdateMask: func(uintptr, uint32, uint32, uint32, uint32, uint32, uint32) uint32 { return 0 },
			stateKeyGetOneSym: func(state uintptr, key uint32) uint32 {
				if state == 2 {
					return 0x6d3 // Cyrillic small es, the base character
				}
				return 'X' // what the live state types with Shift
			},
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var keys []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 54, 1)
	if len(keys) != 1 {
		t.Fatalf("terminal received %d presses, want 1", len(keys))
	}
	if keys[0].Text != "с" {
		t.Fatalf("raw terminal Text = %q, want the base character с", keys[0].Text)
	}
	if keys[0].Typed != "X" {
		t.Fatalf("raw terminal Typed = %q, want X", keys[0].Typed)
	}
}

func TestWaylandShortcutTextIgnoresLevel3(t *testing.T) {
	var sym uint32
	app := &waylandApplication{
		modifiers:        input.ModifierControl | input.ModifierAlt,
		xkbState:         1,
		xkbKeymap:        3,
		xkbShortcutState: 2,
		xkbDepressed:     8, // AltGr
		xkb: &xkbAPI{
			stateUpdateMask: func(uintptr, uint32, uint32, uint32, uint32, uint32, uint32) uint32 { return 0 },
			stateKeyGetOneSym: func(uintptr, uint32) uint32 {
				return '@' // what AltGr types
			},
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 0 || level != 0 {
					return 0
				}
				sym = 'q'
				*out = &sym
				return 1
			},
		},
	}
	win := &waylandWindow{app: app}
	var text string
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
				text = event.Text
				return platform.Frame{}, true, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 24, 1)
	if text != "q" {
		t.Fatalf("Ctrl+AltGr shortcut text = %q, want q", text)
	}
}

func TestWaylandComposedKeyLeavesTypedEmptyForApplications(t *testing.T) {
	keysyms := map[uint32]uint32{
		13: 0xfe51, // dead_acute
		18: 'e',
	}
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 { return keysyms[key-8] },
		},
	}
	win := &waylandWindow{app: app}
	var keys []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event)
				return platform.Frame{}, nil
			},
		},
	}
	text := &linuxTextInput{config: &platform.Config{}} // disabled: the composed key reaches the application
	state.router = eventRouter{config: state.config, text: text}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 13, 1)
	app.handleKey(2, 120, 13, 0)
	app.handleKey(3, 140, 18, 1)
	if len(keys) != 1 {
		t.Fatalf("application received %d keys, want the composed one", len(keys))
	}
	if keys[0].Text != "é" || keys[0].Typed != "" {
		t.Fatalf("composed event = Text %q Typed %q, want é and no Typed", keys[0].Text, keys[0].Typed)
	}
}

func TestWaylandShortcutTextUsesLatchedShift(t *testing.T) {
	var sym uint32
	app := &waylandApplication{
		modifiers:    input.ModifierControl | input.ModifierShift,
		xkbState:     1,
		xkbKeymap:    3,
		xkbShiftMask: 1,
		xkbLatched:   1, // sticky Shift is latched, not depressed
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 0 },
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 0 {
					return 0
				}
				if level == 0 {
					sym = 'c'
				} else if level == 1 {
					sym = 'C'
				} else {
					return 0
				}
				*out = &sym
				return 1
			},
		},
	}
	win := &waylandWindow{app: app}
	var text string
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
				text = event.Text
				return platform.Frame{}, true, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 46, 1)
	if text != "C" {
		t.Fatalf("latched Shift shortcut text = %q, want C", text)
	}
}

func TestWaylandShortcutTextUsesNumLock(t *testing.T) {
	var sym uint32
	app := &waylandApplication{
		modifiers:      input.ModifierControl,
		xkbState:       1,
		xkbKeymap:      3,
		xkbNumLockMask: 4,
		xkbLocked:      4, // Num Lock on
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 0 },
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 0 {
					return 0
				}
				if level == 0 {
					sym = keysymKPHome
				} else if level == 1 {
					sym = '7'
				} else {
					return 0
				}
				*out = &sym
				return 1
			},
		},
	}
	win := &waylandWindow{app: app}
	var text string
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
				text = event.Text
				return platform.Frame{}, true, nil
			},
		},
	}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 79, 1)
	if text != "7" {
		t.Fatalf("Num Lock shortcut text = %q, want 7", text)
	}
}

func TestWaylandRepeatFollowsKeymapFlag(t *testing.T) {
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 50,
		xkbState:    1,
		xkbKeymap:   3,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 'a' },
			keymapKeyRepeats: func(_ uintptr, key uint32) int32 {
				if key == 30+8 {
					return 0 // the keymap marks this key non-repeating
				}
				return 1
			},
		},
	}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	if app.repeatScancode != 0 {
		t.Fatal("key the keymap marks non-repeating armed repeat")
	}
	app.handleKey(2, 120, 31, 1)
	if app.repeatScancode != 31 {
		t.Fatal("repeatable key did not arm repeat")
	}
}

func TestWaylandKeyboardTeardownForgetsHeldState(t *testing.T) {
	app := &waylandApplication{
		xkbDepressed:     1,
		xkbLatched:       2,
		xkbLocked:        4,
		xkbGroup:         1,
		modifiers:        input.ModifierControl,
		xkbShiftMask:     1,
		xkbLockMask:      2,
		xkbNumLockMask:   4,
		xkbLevels:        map[uint32][][2]uint32{46: {{'c', 'C'}}},
		repeatRate:       50,
		repeatScancode:   46,
		xkbShortcutState: 0,
	}

	app.teardownKeyboard()

	if app.xkbDepressed != 0 || app.xkbLatched != 0 || app.xkbLocked != 0 || app.xkbGroup != 0 {
		t.Fatalf("held modifiers survived teardown: %d/%d/%d group %d", app.xkbDepressed, app.xkbLatched, app.xkbLocked, app.xkbGroup)
	}
	if app.modifiers != 0 {
		t.Fatalf("modifiers survived teardown: %#x", app.modifiers)
	}
	if app.xkbShiftMask != 0 || app.xkbLockMask != 0 || app.xkbNumLockMask != 0 {
		t.Fatalf("maskes survived teardown: %d/%d/%d", app.xkbShiftMask, app.xkbLockMask, app.xkbNumLockMask)
	}
	if app.xkbLevels != nil {
		t.Fatal("cached key levels survived teardown")
	}
}

func TestWaylandShortcutTextWithoutShortcutStateUsesNoLiveLayout(t *testing.T) {
	var sym uint32
	liveRead := false
	app := &waylandApplication{
		xkbKeymap: 3,
		xkb: &xkbAPI{
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 0 || level != 0 {
					return 0
				}
				sym = 0x6d3 // Cyrillic small es, no ASCII shortcut
				*out = &sym
				return 1
			},
			stateKeyGetOneSym: func(uintptr, uint32) uint32 {
				liveRead = true
				return 'x'
			},
		},
	}

	if got := app.shortcutText(46, 0, 0, 0, 0); got != "" {
		t.Fatalf("shortcut text = %q, want empty without a shortcut state", got)
	}
	if liveRead {
		t.Fatal("shortcut text read the live layout state")
	}
}

func TestWaylandRepeatFallbackIgnoresModifierScancodes(t *testing.T) {
	app := &waylandApplication{}
	for _, scancode := range []uint32{29, 97, 42, 54, 56, 100, 125, 126, 58, 69, 70} {
		if !app.keepsRepeat(0, 0, scancode) {
			t.Errorf("scancode %d is not treated as a modifier or lock key", scancode)
		}
	}
	if app.keepsRepeat(0, 0, 30) {
		t.Fatal("a letter scancode is treated as a modifier")
	}
	if !app.keyRepeats(30) {
		t.Fatal("letter scancode does not repeat without a keymap")
	}
}

func TestWaylandNumLockMaskFollowsModifierMap(t *testing.T) {
	xkb := &xkbAPI{
		keymapModGetIndex: func(_ uintptr, name *byte) uint32 {
			switch unsafe.String(name, 4) {
			case "Mod3":
				return 5 // this keymap binds Num_Lock to Mod3
			case "Mod2":
				return 4
			case "Mod5":
				return 7 // the AltGr fallback
			}
			t.Errorf("unexpected modifier name %q", unsafe.String(name, 4))
			return 0
		},
	}
	// What xkbcommon serializes: modifier_map names keys, not keysyms.
	keymap := []byte("xkb_keymap {\n key <NMLK> { [ Num_Lock ] };\n modifier_map Mod3 { <NMLK> };\n};\n")
	if numLock, _ := modifierMasks(xkb, 3, keymap); numLock != 1<<5 {
		t.Fatalf("NumLock mask = %#x, want %#x", numLock, uint32(1<<5))
	}
	// With no Num_Lock binding, follow X11's Mod2 fallback.
	if numLock, _ := modifierMasks(xkb, 3, []byte("xkb_keymap { modifier_map Mod2 { <CAPS> }; };")); numLock != 1<<4 {
		t.Fatalf("NumLock fallback mask = %#x, want %#x", numLock, uint32(1<<4))
	}
}

func TestWaylandShortcutTextScansEveryLayout(t *testing.T) {
	var sym uint32
	app := &waylandApplication{
		xkbKeymap: 3,
		xkb: &xkbAPI{
			keymapLayoutsForKey: func(uintptr, uint32) uint32 { return 3 },
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 2 || level != 0 {
					return 0
				}
				sym = 'z'
				*out = &sym
				return 1
			},
		},
	}

	if got := app.shortcutText(46, 0, 0, 0, 0); got != "z" {
		t.Fatalf("shortcut text = %q, want z from the third layout", got)
	}
}

func TestWaylandShortcutLevelsAreCached(t *testing.T) {
	var sym uint32
	calls := 0
	app := &waylandApplication{
		xkbKeymap: 3,
		xkb: &xkbAPI{
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				calls++
				if level != 0 {
					return 0
				}
				sym = 'c'
				*out = &sym
				return 1
			},
		},
	}

	if got := app.shortcutText(46, 0, 0, 0, 0); got != "c" {
		t.Fatalf("first shortcut text = %q, want c", got)
	}
	first := calls
	if got := app.shortcutText(46, 0, 0, 0, 0); got != "c" {
		t.Fatalf("second shortcut text = %q, want c", got)
	}
	if calls != first {
		t.Fatalf("second lookup read the keymap again: %d calls after %d", calls, first)
	}
}

func TestRegisterOptionalLibFuncIgnoresMissingSymbol(t *testing.T) {
	api, err := loadXKB()
	if err != nil {
		t.Skipf("libxkbcommon unavailable: %v", err)
	}
	defer api.close()

	var fn func() int
	registerOptionalLibFunc(&fn, api.library, "xkb_symbol_viva_test_does_not_exist")
	if fn != nil {
		t.Fatal("missing symbol was bound instead of skipped")
	}
}

func TestWaylandModifierReleaseKeepsOtherSideHeld(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 {
				switch key {
				case 42 + 8:
					return keysymShiftL
				case 54 + 8:
					return keysymShiftR
				}
				return 0
			},
			stateModNameIsActive: func(uintptr, *byte, uint32) int32 { return 0 },
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var releases []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Release {
					releases = append(releases, event)
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text, modifierKeys: newModifierTracker()}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}
	app.modifiers = input.ModifierShift

	app.handleKey(1, 100, 42, 1) // Shift_L down
	app.handleKey(2, 110, 54, 1) // Shift_R down
	app.handleKey(3, 120, 54, 0) // Shift_R up, Shift_L still held
	if len(releases) != 1 {
		t.Fatalf("got %d releases, want 1", len(releases))
	}
	if releases[0].Modifiers&input.ModifierShift == 0 {
		t.Fatal("Shift_R release dropped Shift while Shift_L is held")
	}

	app.handleKey(4, 130, 42, 0) // Shift_L up
	if len(releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(releases))
	}
	if releases[1].Modifiers&input.ModifierShift != 0 {
		t.Fatalf("last Shift release still reports Shift: %#x", releases[1].Modifiers)
	}
}

func TestWaylandNonRepeatingKeyStopsRepeat(t *testing.T) {
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 50,
		xkbState:    1,
		xkbKeymap:   3,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 {
				if key == 30+8 {
					return 'a'
				}
				return 0 // an unmapped key
			},
			keymapKeyRepeats: func(_ uintptr, key uint32) int32 {
				if key == 30+8 {
					return 1
				}
				return 0
			},
		},
	}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	state.router = eventRouter{config: state.config}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	if app.repeatScancode != 30 {
		t.Fatal("repeating key did not arm repeat")
	}
	app.handleKey(2, 110, 86, 1) // non-repeating key
	if app.repeatScancode != 0 {
		t.Fatal("repeat survived a non-repeating key")
	}
}

func TestWaylandModifiersRefreshAfterKeymapReload(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateModNameIsActive: func(_ uintptr, name *byte, _ uint32) int32 {
				if unsafe.String(name, 5) == "Shift" {
					return 1
				}
				return 0
			},
		},
	}

	app.updateModifiers()

	if app.modifiers != input.ModifierShift {
		t.Fatalf("modifiers after keymap reload = %#x, want Shift", app.modifiers)
	}
}

func TestWaylandTerminalChordHasNoTypedText(t *testing.T) {
	app := &waylandApplication{
		modifiers: input.ModifierControl,
		xkbState:  1,
		xkb: &xkbAPI{
			stateKeyGetOneSym:    func(uintptr, uint32) uint32 { return 'c' },
			stateModNameIsActive: func(uintptr, *byte, uint32) int32 { return 0 },
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var keys []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				keys = append(keys, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 46, 1)
	if len(keys) != 1 {
		t.Fatalf("terminal received %d presses, want 1", len(keys))
	}
	if keys[0].Text != "c" || keys[0].Typed != "" {
		t.Fatalf("Ctrl+C event = Text %q Typed %q, want c and no Typed", keys[0].Text, keys[0].Typed)
	}
}

func TestModMaskFallsBackToRealIndices(t *testing.T) {
	xkb := &xkbAPI{}
	if got := modMask(xkb, 0, strModShift); got != 1<<0 {
		t.Fatalf("Shift mask = %#x, want %#x", got, uint32(1<<0))
	}
	if got := modMask(xkb, 0, strModNum); got != 1<<4 {
		t.Fatalf("Mod2 mask = %#x, want %#x", got, uint32(1<<4))
	}
}

func TestWaylandKeyboardTeardownReleasesHeldKeys(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 {
				if key == 30+8 {
					return 'a'
				}
				return 0
			},
			stateUnref: func(uintptr) {},
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var releases []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Release {
					releases = append(releases, event)
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text, modifierKeys: newModifierTracker()}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1) // 'a' down
	app.teardownKeyboard()

	if len(releases) != 1 {
		t.Fatalf("got %d releases after teardown, want 1", len(releases))
	}
	if releases[0].Code != 30 || !releases[0].Release {
		t.Fatalf("teardown release = %+v, want code 30 up", releases[0])
	}
}

func TestWaylandKeyboardTeardownSurvivesReleaseError(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 'a' },
			stateUnref:        func(uintptr) {},
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	releaseCalls := 0
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Release {
					releaseCalls++
					return platform.Frame{}, errors.New("release failed")
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text, modifierKeys: newModifierTracker()}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	app.handleKey(2, 110, 31, 1)
	app.teardownKeyboard()

	if app.firstErr == nil {
		t.Fatal("the failed release did not reach the application")
	}
	if win.state != nil {
		t.Fatal("the failed release did not tear the window down")
	}
	if releaseCalls != 1 {
		t.Fatalf("delivered %d synthetic releases after the first failed, want 1", releaseCalls)
	}
}

func TestWaylandLevel3MaskFollowsModifierMap(t *testing.T) {
	xkb := &xkbAPI{
		keymapModGetIndex: func(_ uintptr, name *byte) uint32 {
			switch got := unsafe.String(name, 4); got {
			case "Mod2":
				return 4 // the Num Lock fallback
			case "Mod5":
				return 7
			default:
				t.Errorf("modifier name = %q, want Mod2 or Mod5", got)
				return 0
			}
		},
	}
	keymap := []byte("xkb_keymap {\n key <RALT> { [ ISO_Level3_Shift ] };\n modifier_map Mod5 { <RALT> };\n};\n")
	if _, level3 := modifierMasks(xkb, 3, keymap); level3 != 1<<7 {
		t.Fatalf("Level3 mask = %#x, want %#x", level3, uint32(1<<7))
	}
	if _, level3 := modifierMasks(&xkbAPI{}, 3, keymap); level3 != 1<<7 {
		t.Fatalf("Level3 fallback mask = %#x, want Mod5", level3)
	}
}

func TestWaylandShortcutFallbackIgnoresLevel3(t *testing.T) {
	var appliedDepressed, appliedLatched uint32
	app := &waylandApplication{
		xkbKeymap:        3,
		xkbShortcutState: 1,
		xkbLevel3Mask:    1 << 7,
		xkb: &xkbAPI{
			stateUpdateMask: func(_ uintptr, depressed, latched, _, _, _, _ uint32) uint32 {
				appliedDepressed, appliedLatched = depressed, latched
				return 0
			},
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 'q' },
		},
	}

	if got := app.shortcutText(46, 1<<7, 1<<7, 0, 0); got != "q" {
		t.Fatalf("shortcut text = %q, want q", got)
	}
	if appliedDepressed&(1<<7) != 0 || appliedLatched&(1<<7) != 0 {
		t.Fatalf("shortcut fallback applied Level3: depressed %#x latched %#x", appliedDepressed, appliedLatched)
	}
}

func TestModifierReleaseFallbackKeepsAltGrOutOfAlt(t *testing.T) {
	router := eventRouter{modifierKeys: newModifierTracker()}
	if got := router.releaseModifiers(input.ModifierAlt, input.KeyAltRight, keysymLevel3, 100); got != input.ModifierAlt {
		t.Fatalf("untracked AltGr release changed Alt: %#x", got)
	}
	if got := router.releaseModifiers(input.ModifierAlt, input.KeyAltRight, keysymModeSwitch, 100); got != input.ModifierAlt {
		t.Fatalf("untracked Mode_switch release changed Alt: %#x", got)
	}
}

func TestWaylandReleaseTellsIMEThePreReleaseModifiers(t *testing.T) {
	context := &fakeIMEContext{}
	ime := &linuxIME{active: true, context: context}
	router := eventRouter{ime: ime, modifierKeys: newModifierTracker()}
	keyEvent := platform.KeyEvent{Key: input.KeyShiftLeft, Release: true}

	if err := router.waylandKeyRelease(keyEvent, keysymShiftL, 50, input.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if len(context.keys) != 1 || context.keys[0].state != ibusModifierMask(input.ModifierShift)|ibus.ReleaseMask {
		t.Fatalf("IME release state = %+v, want Shift", context.keys)
	}
}

func TestWaylandKeyboardLeaveForAnotherWindowCleansItAndKeepsFocus(t *testing.T) {
	app := &waylandApplication{}
	focusLost, leavingLost := 0, 0
	focus := &waylandWindow{state: &waylandWindowState{}}
	focus.state.config.OnFocusLost = func() (platform.Frame, error) {
		focusLost++
		return platform.Frame{}, nil
	}
	leaving := &waylandWindow{state: &waylandWindowState{}}
	leaving.state.config.OnFocusLost = func() (platform.Frame, error) {
		leavingLost++
		return platform.Frame{}, nil
	}
	app.keyboardFocus = focus

	// The enter for the new focus can arrive before this leave.
	app.handleKeyboardLeave(leaving)

	if app.keyboardFocus != focus {
		t.Fatal("a leave for another surface cleared the current focus")
	}
	if focusLost != 0 {
		t.Fatalf("the focused window reported focus lost %d times", focusLost)
	}
	if leavingLost != 1 {
		t.Fatalf("the leaving window reported focus lost %d times, want 1", leavingLost)
	}
}

func TestWaylandModeSwitchLeavesRepeatRunning(t *testing.T) {
	app := &waylandApplication{}
	if !app.keepsRepeat(input.KeyUnknown, keysymModeSwitch, 100) {
		t.Fatal("Mode_switch stops another key's repeat")
	}
	if !app.keepsRepeat(input.KeyUnknown, 0, 100) {
		t.Fatal("scancode 100 does not count as a layout modifier without a keymap")
	}
}

func TestWaylandKeyboardLeaveReleasesHeldKeys(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 {
				if key == 30+8 {
					return 'a'
				}
				return 0
			},
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var releases []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Release {
					releases = append(releases, event)
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text, modifierKeys: newModifierTracker()}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	app.handleKeyboardLeave(win)

	if len(releases) != 1 || releases[0].Code != 30 {
		t.Fatalf("leave releases = %+v, want one release of code 30", releases)
	}
	if app.keyboardFocus != nil {
		t.Fatal("keyboard focus survived leave")
	}
	if len(state.router.modifierKeys.down) != 0 {
		t.Fatalf("held keys survived leave: %v", state.router.modifierKeys.down)
	}
}

func TestModifierReleaseReadsTheTrackedBit(t *testing.T) {
	tracker := newModifierTracker()
	tracker.press(100, keysymLevel3, input.KeyAltRight) // AltGr is not Alt
	router := eventRouter{modifierKeys: tracker}
	if got := router.releaseModifiers(input.ModifierAlt, input.KeyAltRight, keysymLevel3, 100); got != input.ModifierAlt {
		t.Fatalf("AltGr release changed Alt: %#x", got)
	}

	tracker.press(56, keysymAltL, input.KeyAltLeft)
	tracker.press(100, keysymAltR, input.KeyAltRight)
	if got := router.releaseModifiers(input.ModifierAlt, input.KeyAltLeft, keysymAltL, 56); got&input.ModifierAlt == 0 {
		t.Fatal("Alt_L release dropped Alt while Alt_R is held")
	}
}

func TestWaylandSyntheticReleasesClearDistinctModifiers(t *testing.T) {
	app := &waylandApplication{
		xkbState:  1,
		modifiers: input.ModifierShift | input.ModifierControl,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 {
				switch key {
				case 42 + 8:
					return keysymShiftL
				case 29 + 8:
					return keysymControlL
				}
				return 0
			},
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	var releases []platform.KeyEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
				if event.Release {
					releases = append(releases, event)
				}
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text, modifierKeys: newModifierTracker()}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 42, 1) // Shift down
	app.handleKey(2, 110, 29, 1) // Control down
	app.handleKeyboardLeave(win)

	if len(releases) != 2 {
		t.Fatalf("got %d synthetic releases, want 2", len(releases))
	}
	if releases[0].Code != 29 || releases[1].Code != 42 {
		t.Fatalf("synthetic releases are not in scancode order: %d then %d", releases[0].Code, releases[1].Code)
	}
	if releases[1].Modifiers != 0 {
		t.Fatalf("second release re-introduced a modifier: %#x", releases[1].Modifiers)
	}
}

func TestWaylandUnknownPressStaysFiltered(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 0 },
		},
	}
	text := &linuxTextInput{enabled: true, config: &platform.Config{}, input: platform.TextInput{Terminal: true, KeyReleases: true}}
	win := &waylandWindow{app: app}
	called := false
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnKey: func(platform.KeyEvent) (platform.Frame, error) {
				called = true
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, text: text, modifierKeys: newModifierTracker()}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 86, 1)
	if called {
		t.Fatal("an unknown press reached the application")
	}
}

func TestWaylandShortcutUsesShiftedOnlyLevel(t *testing.T) {
	var sym uint32
	app := &waylandApplication{
		xkbKeymap: 3,
		xkb: &xkbAPI{
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 0 || level != 1 {
					return 0
				}
				sym = 'c'
				*out = &sym
				return 1
			},
		},
	}
	if got := app.shortcutText(46, 0, 0, 0, 0); got != "c" {
		t.Fatalf("shortcut text = %q, want c from the shifted level", got)
	}
}

func TestWaylandRepeatReachesInputMethod(t *testing.T) {
	context := &fakeIMEContext{}
	ime := &linuxIME{active: true, context: context}
	app := &waylandApplication{
		repeatRate:  50,
		repeatDelay: 10,
		xkbState:    1,
		xkbKeymap:   3,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return 'a' },
			keymapKeyRepeats:  func(uintptr, uint32) int32 { return 1 },
		},
	}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	state.router = eventRouter{config: state.config, ime: ime}
	win.state = state
	app.keyboardFocus = win
	app.windows = []*waylandWindowState{state}

	app.handleKey(1, 100, 30, 1)
	if app.repeatScancode != 30 {
		t.Fatal("a letter the input method takes did not arm repeat")
	}
	if len(context.keys) != 1 {
		t.Fatalf("the input method saw %d presses, want 1", len(context.keys))
	}

	app.mu.Lock()
	app.repeatDeadline = time.Now().Add(-time.Second)
	app.mu.Unlock()
	app.runKeyRepeat(time.Now())

	if len(context.keys) != 2 {
		t.Fatalf("the input method saw %d presses after a repeat, want 2", len(context.keys))
	}
}

func TestWaylandFocusLossNotifiesOnce(t *testing.T) {
	app := &waylandApplication{}
	win := &waylandWindow{state: &waylandWindowState{}}
	lost := 0
	win.state.config.OnFocusLost = func() (platform.Frame, error) {
		lost++
		return platform.Frame{}, nil
	}
	app.keyboardFocus = win

	// leave arrives first, then the toplevel deactivation.
	app.handleKeyboardLeave(win)
	notifyFocusLost(win)
	if lost != 1 {
		t.Fatalf("focus loss reported %d times, want 1", lost)
	}

	// A new focus span makes the next blur news again.
	win.focusLostSent = false
	notifyFocusLost(win)
	if lost != 2 {
		t.Fatalf("second focus loss reported %d times, want 2", lost)
	}
}

func TestWaylandKeyboardTeardownNotifiesFocusLossOnce(t *testing.T) {
	app := &waylandApplication{}
	win := &waylandWindow{state: &waylandWindowState{}}
	lost := 0
	win.state.config.OnFocusLost = func() (platform.Frame, error) {
		lost++
		return platform.Frame{}, nil
	}
	app.keyboardFocus = win

	app.handleKeyboardLeave(win)
	app.teardownKeyboard()
	if lost != 1 {
		t.Fatalf("focus loss reported %d times, want 1", lost)
	}

	// Without a leave the capability removal alone must tell the window.
	other := &waylandWindow{state: &waylandWindowState{}}
	otherLost := 0
	other.state.config.OnFocusLost = func() (platform.Frame, error) {
		otherLost++
		return platform.Frame{}, nil
	}
	app.keyboardFocus = other
	app.teardownKeyboard()
	if otherLost != 1 {
		t.Fatalf("capability removal reported focus loss %d times, want 1", otherLost)
	}
}

func TestWaylandShortcutTextCountsLockedShift(t *testing.T) {
	app := &waylandApplication{
		xkbKeymap:    3,
		xkbShiftMask: 1 << 0,
		xkb: &xkbAPI{
			keymapKeyGetSyms: func(_ uintptr, key, layout, level uint32, out **uint32) int32 {
				if layout != 0 {
					return 0
				}
				switch level {
				case 0:
					*out = ptrToUint32('c')
				case 1:
					*out = ptrToUint32('C')
				}
				return 1
			},
		},
	}

	if got := app.shortcutText(46, 0, 0, 1<<0, 0); got != "C" {
		t.Fatalf("shortcut text with a locked shift = %q, want C", got)
	}
}

func ptrToUint32(value uint32) *uint32 { return &value }

func TestWaylandEnterRebuildsHeldKeys(t *testing.T) {
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(_ uintptr, key uint32) uint32 {
				switch key {
				case 42 + 8:
					return keysymShiftL
				case 29 + 8:
					return keysymControlL
				}
				return 0
			},
		},
	}
	tracker := newModifierTracker()
	tracker.press(99, 0, input.KeyUnknown) // a key released while unfocused
	keys := []uint32{42, 29}
	array := wlArray{Size: uintptr(len(keys)) * 4, Data: unsafe.Pointer(&keys[0])}

	app.syncModifierKeys(eventRouter{modifierKeys: tracker}, unsafe.Pointer(&array))

	if len(tracker.down) != 2 {
		t.Fatalf("held keys after enter = %v, want the two the compositor named", tracker.down)
	}
	if tracker.down[42] != input.ModifierShift || tracker.down[29] != input.ModifierControl {
		t.Fatalf("held keys after enter = %v", tracker.down)
	}
}

func TestWaylandHeldKeysFollowKeymapReload(t *testing.T) {
	sym := uint32(keysymAltR)
	app := &waylandApplication{
		xkbState: 1,
		xkb: &xkbAPI{
			stateKeyGetOneSym: func(uintptr, uint32) uint32 { return sym },
		},
	}
	tracker := newModifierTracker()
	tracker.press(100, keysymLevel3, input.KeyAltRight) // held as AltGr
	win := &waylandWindow{state: &waylandWindowState{}}
	win.state.router = eventRouter{modifierKeys: tracker}
	app.keyboardFocus = win

	app.refreshModifierKeys()
	if tracker.down[100] != input.ModifierAlt {
		t.Fatalf("held key bit after reload = %#x, want Alt", tracker.down[100])
	}

	sym = keysymLevel3
	app.refreshModifierKeys()
	if tracker.down[100] != 0 {
		t.Fatalf("held key bit after reload = %#x, want AltGr", tracker.down[100])
	}
}
