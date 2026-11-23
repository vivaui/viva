package windows

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// Use the Win32 queue (including TranslateMessage), not direct calls to a
// shortcut handler. Preedit updates stand in for the installed IME so this
// test also runs on a machine without a Pinyin keyboard.
func TestQueuedMenuKeysLeaveTerminalCompositionWithItsOwner(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key, sent uintptr
		composing bool
		fromHook  bool
	}{
		{"C before composition", 'C', 'C', false, false},
		{"E during composition", 'E', 'E', true, false},
		{"IME-processed C", 'C', vkProcessKey, true, false},
		{"IME-processed E", 'E', vkProcessKey, true, false},
		{"keyboard hook", 'E', 'E', true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			var menuOpen bool
			var shortcuts, marked, inserted []string
			var raw []platform.KeyEvent
			var state *windowState
			var sendPress func()
			var originalContext uintptr
			contextOf := func() uintptr {
				context, _, _ := getInputContext.Call(state.window)
				if context != 0 {
					releaseInputContext.Call(state.window, context)
				}
				return context
			}
			checkOwner := func(stage string) {
				if context := contextOf(); context != originalContext ||
					state.imeDisabled || state.imeComposing != tc.composing ||
					state.textState.Composing != tc.composing || tc.composing && state.textState.Value != "ni" {
					t.Errorf("%s: context %v (was %v), disabled=%v ime=%v state=%+v",
						stage, context, originalContext, state.imeDisabled, state.imeComposing, state.textState)
				}
			}
			field := testTextInput("", 10)
			field.Terminal, field.KeyReleases = true, true
			field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				switch event.Kind {
				case platform.TextSetMarked:
					marked = append(marked, event.Text)
					return platform.TextState{Value: event.Text, Composing: true,
						Composition: text.Range{End: len([]rune(event.Text))}}, nil
				case platform.TextInsert:
					inserted = append(inserted, event.Text)
				case platform.TextUnmark:
					t.Error("the menu cancelled the pending composition")
				}
				return platform.TextState{}, nil
			}
			err := RunApplication(func(application platform.Application) error {
				opened, err := application.OpenWindow(platform.Config{
					Title: "Menu input owner", Width: 180, Height: 90,
					TextInput: &field, DynamicTextInput: true,
					CaptureMenuKeys: func() bool { return menuOpen },
					OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
						if event.Kind == platform.PointerDown && event.Button == gfx.PointerButtonSecondary {
							menuOpen = true
						}
						if tc.fromHook && event.Kind == platform.PointerUp && event.Button == gfx.PointerButtonSecondary {
							sendPress()
						}
						return platform.Frame{}, nil
					},
					OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
						shortcuts = append(shortcuts, event.Text)
						if menuOpen {
							checkOwner("shortcut")
							menuOpen = false
							return platform.Frame{}, true, nil
						}
						return platform.Frame{}, false, nil
					},
					OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
						raw = append(raw, event)
						return platform.Frame{}, nil
					},
				})
				if err != nil {
					return err
				}
				state = opened.(*nativeWindow).state
				originalContext = contextOf()
				if tc.composing {
					state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "ni"})
				}
				scan, _, _ := mapVirtualKey.Call(tc.key, 0)
				flags := scan<<16 | 1
				postMessage.Call(state.window, wmRightDown, 2, 10<<16|10)
				postMessage.Call(state.window, wmRightUp, 0, 10<<16|10)
				sendPress = func() {
					if tc.fromHook {
						// Exercise the installed hook's key route and private message without
						// sending hardware input to the user's foreground application.
						if !state.shortcutsHeld || shortcutHook.handle == 0 ||
							!state.takePhysicalMenuKey(wmKeyDown, &kbdLLHook{VkCode: uint32(tc.sent), ScanCode: uint32(scan)}) {
							t.Error("the installed hook did not consume the menu press")
						}
					} else {
						postMessage.Call(state.window, wmKeyDown, tc.sent, flags)
					}
					postMessage.Call(state.window, wmKeyDown, tc.sent, flags|1<<30)
					postMessage.Call(state.window, wmKeyUp, tc.sent, flags|1<<30|1<<31)
					application.Post(func() {
						checkOwner("after key up")
						if state.shortcutsHeld || shortcutHook.handle != 0 {
							t.Error("the menu kept its keyboard hook after the closing key was released")
						}
						if tc.composing {
							state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "nihao"})
							state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "你好"})
						}
						// Close posts a message behind any WM_CHAR generated above.
						opened.Close()
					})
				}
				if !tc.fromHook {
					sendPress()
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if menuOpen || !slices.Equal(shortcuts, []string{strings.ToLower(string(rune(tc.key)))}) || len(raw) != 0 {
				t.Fatalf("menu open=%v shortcuts=%v raw=%+v", menuOpen, shortcuts, raw)
			}
			if tc.composing {
				if !slices.Equal(marked, []string{"ni", "nihao"}) || !slices.Equal(inserted, []string{"你好"}) {
					t.Fatalf("preedit=%q inserted=%q", marked, inserted)
				}
			} else if len(marked) != 0 || len(inserted) != 0 {
				t.Fatalf("menu key entered the text service: preedit=%q inserted=%q", marked, inserted)
			}
		})
	}
}

func TestMenuKeysPreserveSystemChordsAndShift(t *testing.T) {
	state := windowState{menuKeys: menuKeyboard{open: true}}
	for _, key := range []struct {
		vk   uintptr
		mods input.Modifiers
	}{
		{vkTab, input.ModifierAlt}, {0x73, input.ModifierAlt},
		{vkEscape, input.ModifierControl}, {'L', input.ModifierMeta},
	} {
		if state.takeMenuKey(key.vk, 1, key.mods, false) {
			t.Fatalf("captured system chord: %+v", key)
		}
	}
	var mods []input.Modifiers
	state.config.OnShortcut = func(event platform.KeyEvent) (platform.Frame, bool, error) {
		mods = append(mods, event.Modifiers)
		return platform.Frame{}, false, nil
	}
	if !state.takeMenuKey(0x10, 1, 0, false) || !state.takeMenuKey('E', 1, 0, false) ||
		!slices.Equal(mods, []input.Modifiers{input.ModifierShift}) {
		t.Errorf("Shift+E should remain modified when the menu consumes Shift, got %v", mods)
	}
}

func TestTerminalMenuStopsCapturingWhenWindowLosesFocus(t *testing.T) {
	state := &windowState{ready: true, focused: true}
	state.config.CaptureMenuKeys = func() bool { return true }
	state.menuKeys = menuKeyboard{open: true, held: 1}
	state.menuKeys.takenNoScan['E'] = true
	// Invoke the native dispatcher's focus-loss path without taking keyboard
	// focus away from the user during this test.
	const handle = ^uintptr(0)
	previous := active.Load()
	windowStates.Store(handle, state)
	t.Cleanup(func() {
		windowStates.Delete(handle)
		active.Store(previous)
	})
	windowProc(handle, wmKillFocus, 0, 0)
	if state.focused || state.shortcutsHeld || state.menuKeys.open || state.menuKeys.held != 0 ||
		state.takeMenuKey('E', 1<<31, 0, false) || state.takeMenuKey('C', 1, 0, false) {
		t.Fatal("the background window still takes menu keys")
	}
}
