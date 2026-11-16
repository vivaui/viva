//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type gtkIMEProbeKeys struct {
	newEvent  func(int32) uintptr
	freeEvent func(uintptr)
	setDevice func(uintptr, uintptr)
	keyboard  func(uintptr) uintptr
	ref       func(uintptr) uintptr
	weakRef   func(uintptr, uintptr, uintptr)
	emit      func(uintptr, *byte, *byte)
	entries   func(uintptr, uint32, *uintptr, *int32) int32
	translate func(uintptr, uint32, uint32, int32, *uint32, *int32, *int32, *uint32) int32
}

type gtkIMEProbeKey struct {
	Code         uint32
	Group, Level int32
}

type gtkIMEProbeTrial struct {
	name, value string
	signals     []string
	events      []string
	failures    []string
}

type gtkIMEProbe struct {
	w        *gtkWindow
	keys     gtkIMEProbeKeys
	editor   *text.Editor
	context  uintptr
	signals  []string
	events   []platform.TextEvent
	hook     func(platform.TextEvent)
	shortcut bool
	trials   []gtkIMEProbeTrial
	freed    map[uintptr]bool
}

var gtkIMEProbeActive *gtkIMEProbe
var gtkIMEProbeOnce sync.Once
var gtkIMEProbeSignals struct{ start, changed, end, commit, finalized uintptr }

func (p *gtkIMEProbe) observe() {
	gtkIMEProbeOnce.Do(func() {
		gtkIMEProbeSignals.start = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkIMEProbeActive; p != nil && p.context == ctx {
				p.signals = append(p.signals, "start")
			}
			return 0
		})
		gtkIMEProbeSignals.changed = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkIMEProbeActive; p != nil && p.context == ctx {
				var value, attrs uintptr
				var cursor int32
				p.w.g.imGetPreedit(ctx, &value, &attrs, &cursor)
				p.signals = append(p.signals, "preedit:"+goString(value))
				if value != 0 {
					p.w.g.gFree(value)
				}
				if attrs != 0 {
					p.w.g.attrListUnref(attrs)
				}
			}
			return 0
		})
		gtkIMEProbeSignals.end = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkIMEProbeActive; p != nil && p.context == ctx {
				p.signals = append(p.signals, "end")
			}
			return 0
		})
		gtkIMEProbeSignals.commit = purego.NewCallback(func(ctx, value, _ uintptr) uintptr {
			if p := gtkIMEProbeActive; p != nil && p.context == ctx {
				p.signals = append(p.signals, "commit:"+goString(value))
			}
			return 0
		})
		gtkIMEProbeSignals.finalized = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkIMEProbeActive; p != nil {
				p.freed[ctx] = true
			}
			return 0
		})
	})
	ctx := p.w.ime.ctx
	if ctx != p.context {
		p.w.g.connect(ctx, "preedit-start", gtkIMEProbeSignals.start, 0)
		p.w.g.connect(ctx, "preedit-changed", gtkIMEProbeSignals.changed, 0)
		p.w.g.connect(ctx, "preedit-end", gtkIMEProbeSignals.end, 0)
		p.w.g.connect(ctx, "commit", gtkIMEProbeSignals.commit, 0)
		p.keys.weakRef(ctx, gtkIMEProbeSignals.finalized, ctx)
		p.freed[ctx] = false
		p.context = ctx
	}
}

func (p *gtkIMEProbe) install(editor *text.Editor, target gfx.ID) {
	p.editor = editor
	field := platform.TextInput{
		Target: target, X: 20, Y: 20, Width: 300, Height: 100, FontSize: 16,
		Multiline: true, Color: platform.Color{Alpha: 1}, State: platform.EditorState(editor),
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			p.events = append(p.events, event)
			event.Apply(editor)
			next := platform.EditorState(editor)
			if p.hook != nil {
				p.hook(event)
			}
			return next, nil
		},
	}
	p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil)
	p.w.ime.setFocused(true)
	p.observe()
	p.signals, p.events = nil, nil
}

func (p *gtkIMEProbe) press(ctx uintptr, symbol, state uint32) error {
	g := p.w.g
	keymap := g.gdkKeymapDefault()
	var entries uintptr
	var count int32
	if p.keys.entries(keymap, symbol, &entries, &count) == 0 || entries == 0 || count == 0 {
		return fmt.Errorf("no native keymap entry for %#x", symbol)
	}
	key := *(*gtkIMEProbeKey)(unsafePointer(entries))
	g.gFree(entries)
	var value, consumed uint32
	var group, level int32
	if p.keys.translate(keymap, key.Code, state, key.Group, &value, &group, &level, &consumed) == 0 {
		return fmt.Errorf("native keymap did not translate %#x", symbol)
	}
	window := g.widgetGetWindow(p.w.toplevel)
	device := p.keys.keyboard(g.gdkDisplaySeat(g.gdkDisplayDefault()))
	if window == 0 || device == 0 || ctx == 0 {
		return fmt.Errorf("native window/keyboard/context unavailable")
	}
	event := p.keys.newEvent(int32(gdkKeyPress))
	defer p.keys.freeEvent(event)
	record := (*gdkEventKey)(unsafePointer(event))
	record.Window, record.SendEvent = p.keys.ref(window), 1
	record.State, record.Keyval = state, value
	record.HardwareKeycode, record.Group = uint16(key.Code), uint8(group)
	p.keys.setDevice(event, device)
	handled := false
	if ctx == p.w.ime.ctx {
		if p.shortcut {
			handled = p.w.ime.filterShortcut(event, platform.KeyEvent{}, string(rune(value)))
		} else {
			handled = p.w.ime.filter(event)
		}
	} else {
		handled = g.imFilterKeypress(ctx, event) != 0
	}
	if !handled {
		return fmt.Errorf("native GTK did not consume key %#x (translated %#x)", symbol, value)
	}
	return nil
}

func (p *gtkIMEProbe) compose() error {
	for _, symbol := range []uint32{'U', '3', 'A', '9'} {
		if err := p.press(p.w.ime.ctx, symbol, (1<<2)|(1<<0)); err != nil {
			return err
		}
	}
	return nil
}

func (p *gtkIMEProbe) record(name string, failures ...string) {
	trial := gtkIMEProbeTrial{name: name, value: p.editor.String(), signals: append([]string(nil), p.signals...), failures: failures}
	for _, event := range p.events {
		trial.events = append(trial.events, fmt.Sprintf("%d:%q", event.Kind, event.Text))
	}
	if mirror := p.w.text.state; !p.w.closed && mirror != platform.EditorState(p.editor).WithoutRedraw() {
		trial.failures = append(trial.failures, fmt.Sprintf("model/mirror differ: model=%+v mirror=%+v", platform.EditorState(p.editor), mirror))
	}
	p.trials = append(p.trials, trial)
}

func (p *gtkIMEProbe) run() {
	for _, ending := range []string{"commit", "escape", "reset", "focus-out", "window-focus-out"} {
		editor := text.NewEditor("abc")
		editor.SetSelection(text.Range{Start: 1, End: 2})
		p.install(editor, gfx.Item(1))
		if err := p.compose(); err != nil {
			p.record(ending, err.Error())
			continue
		}
		var failures []string
		prefix := []string{"start", "preedit:u", "preedit:u3", "preedit:u3A", "preedit:u3A9"}
		if !slices.Equal(p.signals, prefix) || editor.String() != "au3A9c" {
			failures = append(failures, fmt.Sprintf("preedit signals=%q text=%q", p.signals, editor.String()))
		}
		switch ending {
		case "commit":
			if err := p.press(p.w.ime.ctx, ' ', (1<<2)|(1<<0)); err != nil {
				failures = append(failures, err.Error())
			}
		case "escape":
			if err := p.press(p.w.ime.ctx, 0xff1b, (1<<2)|(1<<0)); err != nil {
				failures = append(failures, err.Error())
			}
		case "reset":
			p.w.g.imReset(p.w.ime.ctx)
		case "focus-out":
			p.w.g.imFocusOut(p.w.ime.ctx)
			if !slices.Equal(p.signals, prefix) || editor.String() != "au3A9c" {
				failures = append(failures, "GtkIMContextSimple focus-out unexpectedly settled preedit")
			}
			p.w.g.imReset(p.w.ime.ctx)
		case "window-focus-out":
			p.w.ime.setFocused(false)
		}
		wantSignals := append(prefix, "preedit:", "end")
		wantText := "ac"
		if ending == "commit" {
			wantSignals = append(wantSignals, "commit:\u03a9")
			wantText = "a\u03a9c"
		}
		if !slices.Equal(p.signals, wantSignals) || editor.String() != wantText || p.w.text.state.Composing {
			failures = append(failures, fmt.Sprintf("ending signals=%q text=%q composing=%t", p.signals, editor.String(), p.w.text.state.Composing))
		}
		p.record(ending, failures...)
		if !editor.Undo() || ending == "commit" && editor.String() != "ac" || ending != "commit" && editor.String() != "abc" {
			p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, fmt.Sprintf("first Undo=%q", editor.String()))
		}
		if ending == "commit" && (!editor.Undo() || editor.String() != "abc") {
			p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, fmt.Sprintf("second Undo=%q", editor.String()))
		}
	}
	plain := text.NewEditor("abc")
	p.install(plain, gfx.Item(1))
	var failures []string
	for _, symbol := range []uint32{'r', 's'} {
		if err := p.press(p.w.ime.ctx, symbol, 0); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if plain.String() != "abcrs" || !slices.Equal(p.signals, []string{"commit:r", "commit:s"}) {
		failures = append(failures, "ordinary native commits changed text or signal order")
	}
	p.record("plain", failures...)
	if !plain.Undo() || plain.String() != "abc" {
		p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, "ordinary GTK commits stopped coalescing")
	}
	old := text.NewEditor("abc")
	p.install(old, gfx.Item(1))
	oldContext := p.keys.ref(p.w.ime.ctx)
	defer p.w.g.objectUnref(oldContext)
	next := text.NewEditor("abc")
	p.install(next, gfx.Item(2))
	p.w.g.imReset(oldContext)
	failures = nil
	signal, value := cString("commit"), cString("z")
	p.keys.emit(oldContext, &signal[0], &value[0])
	if p.w.ime.ctx == oldContext || next.String() != "abc" || len(p.events) != 0 {
		failures = append(failures, fmt.Sprintf("old native context reached replacement input: reused=%t text=%q events=%d", p.w.ime.ctx == oldContext, next.String(), len(p.events)))
	}
	p.record("target replacement", failures...)

	p.install(text.NewEditor("abc"), gfx.Item(1))
	p.editor.SetSelection(text.Range{Start: 1, End: 2})
	field := p.w.text.input
	field.State = platform.EditorState(p.editor)
	p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil)
	if err := p.compose(); err != nil {
		p.record("native callback handoff", err.Error())
		return
	}
	p.hook = func(event platform.TextEvent) {
		if event.Kind == platform.TextSetMarked && event.Text == "" {
			p.hook = nil
			p.install(text.NewEditor("next"), gfx.Item(2))
		}
	}
	failures = nil
	if err := p.press(p.w.ime.ctx, ' ', (1<<2)|(1<<0)); err != nil {
		failures = append(failures, err.Error())
	}
	if p.hook != nil || p.editor.String() != "next" || len(p.events) != 0 {
		failures = append(failures, fmt.Sprintf("native clear/end/commit tail entered replacement: hook=%t text=%q events=%d", p.hook != nil, p.editor.String(), len(p.events)))
	}
	p.hook = nil
	p.record("native callback handoff", failures...)
	for _, disabled := range []bool{false, true} {
		p.frameHandoff(disabled)
	}
	for _, geometry := range []bool{false, true} {
		p.shortcutHandoff(geometry)
	}
}

func (p *gtkIMEProbe) frameHandoff(disabled bool) {
	p.install(text.NewEditor("abc"), gfx.Item(1))
	context := p.keys.ref(p.w.ime.ctx)
	defer p.w.g.objectUnref(context)
	called := false
	p.w.config.FramePresented = func() {
		p.w.config.FramePresented = nil
		called = true
		signal, value := cString("commit"), cString("z")
		p.keys.emit(context, &signal[0], &value[0])
	}
	name := "frame presentation handoff"
	if disabled {
		name = "disabled frame presentation"
		p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true}}, nil)
	} else {
		p.install(text.NewEditor("next"), gfx.Item(2))
	}
	var failures []string
	if !called || !disabled && p.editor.String() != "next" || disabled && p.editor.String() != "abc" {
		failures = append(failures, fmt.Sprintf("retired context entered pending frame: called=%t text=%q", called, p.editor.String()))
	}
	if disabled && p.w.text.enabled {
		failures = append(failures, "late native signal re-enabled disabled input")
	}
	if disabled {
		p.install(p.editor, gfx.Item(1))
	}
	p.record(name, failures...)
}

func (p *gtkIMEProbe) shortcutHandoff(geometry bool) {
	p.install(text.NewEditor("abc"), gfx.Item(1))
	old := p.w.text.input
	called := false
	p.w.config.OnShortcut = func(platform.KeyEvent) (platform.Frame, bool, error) {
		called = true
		if geometry {
			current := p.w.text.input
			current.Width = old.Width + 20
			p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &current}}, nil)
		} else {
			p.install(text.NewEditor("next"), gfx.Item(2))
		}
		return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &old}}, false, nil
	}
	p.shortcut = true
	var failures []string
	if err := p.press(p.w.ime.ctx, 'r', 0); err != nil {
		failures = append(failures, err.Error())
	}
	p.shortcut, p.w.config.OnShortcut = false, nil
	name, want := "shortcut target handoff", "next"
	if geometry {
		name, want = "shortcut geometry handoff", "abcr"
		if p.w.text.input.Width != old.Width+20 {
			failures = append(failures, "stale shortcut frame overwrote newer input geometry")
		}
	}
	if !called || p.editor.String() != want {
		failures = append(failures, fmt.Sprintf("shortcut handoff: called=%t text=%q, want %q", called, p.editor.String(), want))
	}
	p.record(name, failures...)
}

func (p *gtkIMEProbe) closeDuringCommit() {
	p.install(text.NewEditor("abc"), gfx.Item(1))
	p.editor.SetSelection(text.Range{Start: 1, End: 2})
	field := p.w.text.input
	field.State = platform.EditorState(p.editor)
	p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil)
	if err := p.compose(); err != nil {
		p.record("close during native commit", err.Error())
		p.w.destroy()
		return
	}
	p.hook = func(event platform.TextEvent) {
		if event.Kind == platform.TextSetMarked && event.Text == "" {
			p.hook = nil
			p.w.destroy()
		}
	}
	p.events = nil
	var failures []string
	if err := p.press(p.w.ime.ctx, ' ', (1<<2)|(1<<0)); err != nil {
		failures = append(failures, err.Error())
	}
	if !p.w.closed || p.w.ime != nil || p.w.text.enabled || p.editor.String() != "ac" || len(p.events) != 1 {
		failures = append(failures, fmt.Sprintf("close in native callback: closed=%t ime=%t enabled=%t text=%q events=%d", p.w.closed, p.w.ime != nil, p.w.text.enabled, p.editor.String(), len(p.events)))
	}
	p.hook = nil
	p.record("close during native commit", failures...)
	p.w.destroy()
}

func TestGTKNativeComposition(t *testing.T) {
	if os.Getenv("VIVA_GTK_IME_NATIVE") != "1" {
		t.Skip("native GTK IME probe requires VIVA_GTK_IME_NATIVE=1 and a private display")
	}
	p := &gtkIMEProbe{freed: make(map[uintptr]bool)}
	gtkIMEProbeActive = p
	defer func() { gtkIMEProbeActive = nil }()
	err := RunGTKApplication(func(application platform.Application) error {
		a := application.(*gtkApplication)
		for _, binding := range []struct {
			function any
			name     string
		}{
			{&p.keys.newEvent, "gdk_event_new"}, {&p.keys.freeEvent, "gdk_event_free"},
			{&p.keys.setDevice, "gdk_event_set_device"}, {&p.keys.keyboard, "gdk_seat_get_keyboard"},
			{&p.keys.ref, "g_object_ref"}, {&p.keys.entries, "gdk_keymap_get_entries_for_keyval"},
			{&p.keys.weakRef, "g_object_weak_ref"},
			{&p.keys.emit, "g_signal_emit_by_name"},
			{&p.keys.translate, "gdk_keymap_translate_keyboard_state"},
		} {
			if !a.g.lookup(binding.function, binding.name) {
				return fmt.Errorf("native probe cannot bind %s", binding.name)
			}
		}
		window, err := application.OpenWindow(platform.Config{
			Title: "Viva native IME probe", Width: 360, Height: 160,
			ClearColor:       platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			DynamicTextInput: true, OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
		})
		if err != nil {
			return err
		}
		p.w = window.(*gtkWindow)
		a.enqueue(func() {
			p.run()
			// Run after default-priority idle releases, while the main loop is alive.
			a.g.idleAddFull(300, callbacks().task, a.tasks.add(func() {
				var failures []string
				for context, freed := range p.freed {
					if context != p.w.ime.ctx && !freed {
						failures = append(failures, fmt.Sprintf("retired context %#x retained before shutdown", context))
					}
				}
				p.record("idle retirement", failures...)
				p.closeDuringCommit()
			}), 0)
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.trials) != 14 {
		t.Fatalf("native trials=%d, want 14", len(p.trials))
	}
	for _, trial := range p.trials {
		t.Logf("%s signals=%q text=%q events=%q", trial.name, trial.signals, trial.value, trial.events)
		for _, failure := range trial.failures {
			t.Errorf("%s: %s", trial.name, failure)
		}
	}
	for ctx, freed := range p.freed {
		if !freed {
			t.Errorf("native context %#x was not finalized after application shutdown", ctx)
		}
	}
}
