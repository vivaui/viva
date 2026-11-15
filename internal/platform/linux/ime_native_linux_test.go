package linux

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestNativeIBusFactoryDeclinedShortcut(t *testing.T) {
	if os.Getenv("VIVA_IBUS_NATIVE") != "1" {
		t.Skip("native factory requires a private IBus daemon")
	}
	for _, previous := range []string{"none", "retired"} {
		for _, session := range []string{"ordinary", "empty", "hidden"} {
			t.Run(previous+"/"+session, func(t *testing.T) {
				bus, err := ibus.Dial()
				if err != nil {
					t.Fatal(err)
				}
				defer bus.Close()
				h := newTentativeIMEHarness(t)
				router := h.ime.router
				if previous == "none" {
					router.ime = nil
				} else {
					router.ime = &linuxIME{text: h.input}
				}
				shortcuts := 0
				router.config.OnShortcut = func(platform.KeyEvent) (platform.Frame, bool, error) {
					shortcuts++
					return platform.Frame{}, true, nil
				}
				app := &linuxApplication{ibus: bus, ibusTried: true}
				ime := app.newIME(&window{}, h.input, router)
				if ime == nil {
					t.Fatal("native factory returned no context")
				}
				defer ime.context.Close()
				defer ime.close()
				ime.origin = h.ime.origin
				h.ime = ime
				// Keep the production factory/router; only the declined answer is synthetic.
				ime.context = h.context
				h.focus(t)
				if session != "ordinary" {
					ime.nativePreedit(nativeMode("preview", ibus.PreeditModeClear))
					if session == "empty" {
						ime.nativePreedit(nativeMode("", ibus.PreeditModeClear))
					} else {
						ime.nativePreedit(ibus.PreeditEvent{Kind: ibus.PreeditHide})
					}
					if h.input.state.Composing || !ime.composing() {
						t.Fatal("fixture did not retain an active empty native session")
					}
				}
				if !ime.processKey('x', &xcbInputEvent{Detail: 53}, platform.KeyEvent{Text: "x"}) || len(h.context.keys) != 1 {
					t.Fatal("plain key did not reach the input method")
				}
				if shortcuts != 0 {
					t.Fatal("shortcut ran before the key answer")
				}
				h.context.keys[0].done(false)
				want := 0
				if session == "ordinary" {
					want = 1
				}
				if shortcuts != want {
					t.Fatalf("factory declined-key shortcuts=%d want=%d", shortcuts, want)
				}
				if app.firstErr != nil {
					t.Fatal(app.firstErr)
				}
			})
		}
	}
}

func TestNativeIBusFrontend(t *testing.T) {
	if os.Getenv("VIVA_IBUS_NATIVE") != "1" {
		t.Skip("native frontend requires a private IBus daemon")
	}
	engine := os.Getenv("VIVA_IBUS_ENGINE")
	if engine == "" {
		engine = "libpinyin"
	}
	var keycodes map[uint32]uint32
	if err := json.Unmarshal([]byte(os.Getenv("VIVA_IBUS_KEYCODES")), &keycodes); err != nil {
		t.Fatalf("native keymap: %v", err)
	}
	for _, tentative := range []bool{false, true} {
		for _, ending := range []string{"commit", "escape", "reset", "focus-out", "pending-focus-out", "pending-release-loss", "pending-modifier-loss"} {
			t.Run(ending+map[bool]string{false: "/legacy", true: "/tentative"}[tentative], func(t *testing.T) {
				bus, err := ibus.Dial()
				if err != nil {
					t.Fatal(err)
				}
				defer bus.Close()
				h := newIMEHarness(t)
				h.editor = text.NewEditor("abc")
				h.editor.SetSelection(text.Range{Start: 1, End: 2})
				if tentative && !h.editor.EnableTentativeComposition() {
					t.Fatal("enable refused")
				}
				var events []platform.TextEvent
				field := h.input.input
				field.State = platform.EditorState(h.editor)
				field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
					events = append(events, event)
					event.Apply(h.editor)
					return platform.EditorState(h.editor), nil
				}
				if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
					t.Fatal(err)
				}
				app := &linuxApplication{ibus: bus, ibusTried: true}
				router := h.ime.router
				router.ime = nil
				ime := app.newIME(&window{}, h.input, router)
				if ime == nil {
					t.Fatal("native context creation failed")
				}
				defer ime.close()
				ime.origin = h.ime.origin
				h.ime = ime
				h.focus(t)
				selectNativeIBusEngine(t, bus, ime.context, engine)
				send := func(symbol uint32) {
					t.Helper()
					event := &xcbInputEvent{Detail: uint8(keycodes[symbol] + 8)}
					key := platform.KeyEvent{Key: normalizedKeysym(symbol)}
					if character, ok := keysymRune(symbol); ok {
						key.Text = string(character)
					}
					if !ime.processKey(symbol, event, key) {
						t.Fatal("native processKey refused")
					}
				}
				press := func(symbol uint32) {
					t.Helper()
					send(symbol)
					nativeFrontendDrain(t, bus, func() bool { _, waiting := bus.Deadline(); return !waiting })
				}
				keys, live := "nihao", "a你好c"
				if engine == "hangul" {
					press(0xff31)
					keys, live = "rksk", "a가나c"
				}
				for _, symbol := range keys {
					press(uint32(symbol))
				}
				if h.editor.String() != live {
					t.Fatalf("native live preedit=%q", h.editor.String())
				}
				switch ending {
				case "commit":
					press(' ')
				case "escape":
					press(0xff1b)
				case "reset":
					if err := resetNativeIBusFrontend(ime); err != nil {
						t.Fatal(err)
					}
				case "focus-out":
					ime.setFocused(false)
				case "pending-focus-out":
					send(' ')
					ime.setFocused(false)
				case "pending-release-loss", "pending-modifier-loss":
					if ending == "pending-release-loss" {
						keymap, event := letterKey(0)
						ime.release(keymap, event)
					} else {
						ime.pressModifier(0xffe1, &xcbInputEvent{Detail: 50})
					}
					ime.setFocused(false)
					bus.Close()
				}
				lost := ending == "pending-release-loss" || ending == "pending-modifier-loss"
				if !lost {
					until := time.Now().Add(150 * time.Millisecond)
					nativeFrontendDrain(t, bus, func() bool { return time.Now().After(until) })
				} else if !ime.dead || app.firstErr != nil {
					t.Errorf("daemon failure did not fall back: dead=%t error=%v", ime.dead, app.firstErr)
				}
				terminals := 0
				wantPhase := text.CompositionCancel
				result := ending == "commit" || ending == "pending-focus-out" || engine == "hangul" || lost
				if result {
					wantPhase = text.CompositionResult
				}
				for _, event := range events {
					t.Logf("event kind=%d phase=%d text=%q", event.Kind, event.CompositionPhase, event.Text)
					if tentative {
						if event.Kind == platform.TextSetMarked && event.CompositionPhase != text.CompositionUpdate {
							t.Error("native preedit was not Update")
						}
						if event.CompositionPhase == text.CompositionResult || event.CompositionPhase == text.CompositionCancel || event.CompositionPhase == text.CompositionCommit {
							terminals++
							want := wantPhase
							if lost && (engine != "hangul" || terminals == 2) {
								want = text.CompositionCommit
							}
							if event.CompositionPhase != want {
								t.Errorf("terminal phase=%d want=%d", event.CompositionPhase, want)
							}
						}
						if event.Kind == platform.TextUnmark && event.CompositionPhase != text.CompositionCommit {
							t.Error("tentative path emitted legacy Unmark")
						}
					} else if event.CompositionPhase != text.CompositionNone {
						t.Error("legacy event acquired a phase")
					}
				}
				wantTerminals := 1
				if engine == "hangul" {
					wantTerminals = 2
				}
				if tentative && terminals != wantTerminals {
					t.Errorf("terminal count=%d, want=%d", terminals, wantTerminals)
				}
				want := "ac"
				if result {
					want = live
				}
				if engine == "hangul" && (ending == "commit" || ending == "pending-focus-out") {
					// Hangul commits the syllable but declines the space key.
					want = "a가나 c"
				}
				if h.editor.String() != want || h.input.state.Composing {
					t.Errorf("settled text=%q composing=%t", h.editor.String(), h.input.state.Composing)
				}
				if tentative {
					if engine == "hangul" {
						for _, want := range []string{"a가c", "ac", "abc"} {
							if !h.editor.Undo() || h.editor.String() != want {
								t.Fatalf("Hangul Undo=%q want=%q", h.editor.String(), want)
							}
						}
						return
					}
					wantUndo := "abc"
					if result {
						wantUndo = "ac"
					}
					if !h.editor.Undo() || h.editor.String() != wantUndo {
						t.Errorf("first Undo=%q want=%q", h.editor.String(), wantUndo)
					}
					if result && (!h.editor.Undo() || h.editor.String() != "abc") {
						t.Errorf("second Undo=%q want abc", h.editor.String())
					}
				}
			})
		}
	}
}

func TestNativeIBusOwnerReactivation(t *testing.T) {
	if os.Getenv("VIVA_IBUS_NATIVE") != "1" {
		t.Skip("native owner lifecycle requires a private daemon")
	}
	for _, tentative := range []bool{false, true} {
		bus, err := ibus.Dial()
		if err != nil {
			t.Fatal(err)
		}
		h := newIMEHarness(t)
		if tentative {
			h.editor.EnableTentativeComposition()
		}
		field := h.input.input
		field.Target, field.State = gfx.Item(42), platform.EditorState(h.editor)
		if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
			t.Fatal(err)
		}
		app := &linuxApplication{ibus: bus, ibusTried: true}
		router := h.ime.router
		router.ime = nil
		ime := app.newIME(&window{}, h.input, router)
		if ime == nil {
			t.Fatal("initial native context missing")
		}
		ime.origin = h.ime.origin
		h.ime = ime
		h.focus(t)
		original := ime.context
		if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true}}); err != nil {
			t.Fatal(err)
		}
		ime.sync()
		if ime.dead || ime.context != nil {
			t.Fatal("temporary field removal killed the native input method")
		}
		if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
			t.Fatal(err)
		}
		ime.sync()
		if ime.context == nil || ime.context == original || !ime.active {
			t.Fatal("restored field did not receive a fresh native context")
		}
		selectNativeIBusEngine(t, bus, ime.context, os.Getenv("VIVA_IBUS_ENGINE"))
		t.Logf("tentative=%t native context retired and reopened", tentative)
		ime.close()
		bus.Close()
	}
}

func resetNativeIBusFrontend(ime *linuxIME) error {
	if frontend, ok := any(ime).(interface{ reset() error }); ok {
		return frontend.reset()
	}
	return ime.context.Reset()
}

func selectNativeIBusEngine(t *testing.T, bus *ibus.Bus, context inputMethodContext, engine string) {
	t.Helper()
	connection, err := dbus.Dial(os.Getenv("IBUS_ADDRESS"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	const service, path = "org.freedesktop.IBus", "/org/freedesktop/IBus"
	reply, err := connection.Call(service, path, service, "GetUseGlobalEngine", "", nil)
	if err != nil || reply.Signature != "b" {
		t.Fatalf("native global mode: reply=%+v error=%v", reply, err)
	}
	args := reply.Args()
	global := args.Bool()
	if args.Err() != nil {
		t.Fatal(args.Err())
	}
	selector, ok := context.(interface{ SetEngine(string) error })
	if !ok {
		t.Fatal("input context has no native engine selection")
	}
	err = selector.SetEngine(engine)
	var refusal *dbus.Error
	if err != nil && (!global || !errors.As(err, &refusal) || refusal.Name != "org.freedesktop.DBus.Error.Failed" ||
		refusal.Text != "Cannot set engines when use-global-engine is enabled.") || bus.Err() != nil {
		t.Fatalf("native engine selection: %v bus=%v", err, bus.Err())
	}
	t.Logf("native global=%t context selection refused=%t", global, err != nil)
	if global {
		// Use this private bus's wire API, as in the independent service gate.
		for _, name := range []string{"xkb:us::eng", engine} {
			var selection dbus.Encoder
			selection.String(name)
			if _, err := connection.Call(service, path, service, "SetGlobalEngine", "s", selection.Bytes()); err != nil {
				t.Fatalf("native global engine %q: %v", name, err)
			}
		}
		reply, err := connection.Call(service, path, service, "GetGlobalEngine", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		args := reply.Args()
		args.Signature()
		args.Align(8)
		typeName := args.String()
		args.Skip("a{sv}")
		actual := args.String()
		if args.Err() != nil || typeName != "IBusEngineDesc" || actual != engine {
			t.Fatalf("native selected engine=%q type=%q want=%q error=%v", actual, typeName, engine, args.Err())
		}
		t.Logf("native selected engine=%s", actual)
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
}

func nativeFrontendDrain(t *testing.T, bus *ibus.Bus, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("native frontend response timed out")
		}
		if err := bus.Dispatch(); err != nil {
			t.Fatal(err)
		}
		bus.Expire()
		if err := bus.Err(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
}
