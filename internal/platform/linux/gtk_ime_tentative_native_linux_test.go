//go:build linux && vivagtk

package linux

import (
	"errors"
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

type gtkTentativeProbeKeys struct {
	newEvent  func(int32) uintptr
	freeEvent func(uintptr)
	setDevice func(uintptr, uintptr)
	keyboard  func(uintptr) uintptr
	ref       func(uintptr) uintptr
	emit      func(uintptr, *byte, *byte)
	entries   func(uintptr, uint32, *uintptr, *int32) int32
	translate func(uintptr, uint32, uint32, int32, *uint32, *int32, *int32, *uint32) int32
}

type gtkTentativeProbeKey struct {
	Code         uint32
	Group, Level int32
}

type gtkTentativeProbeTrial struct {
	name, value string
	signals     []string
	events      []string
	failures    []string
}

type gtkTentativeProbe struct {
	w                         *gtkWindow
	keys                      gtkTentativeProbeKeys
	editor                    *text.Editor
	context                   uintptr
	signals                   []string
	events                    []platform.TextEvent
	hook                      func(platform.TextEvent)
	reject                    func(platform.TextEvent) error
	expectedErr               error
	trials                    []gtkTentativeProbeTrial
	tentative, composingTrial bool
	asynchronous              bool
	keyIngress                bool
}

var gtkTentativeProbeActive *gtkTentativeProbe
var gtkTentativeProbeOnce sync.Once
var gtkTentativeProbeSignals struct{ start, changed, end, commit uintptr }

func (p *gtkTentativeProbe) observe() {
	gtkTentativeProbeOnce.Do(func() {
		gtkTentativeProbeSignals.start = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkTentativeProbeActive; p != nil && p.context == ctx {
				p.signals = append(p.signals, "start")
			}
			return 0
		})
		gtkTentativeProbeSignals.changed = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkTentativeProbeActive; p != nil && p.context == ctx {
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
		gtkTentativeProbeSignals.end = purego.NewCallback(func(ctx, _ uintptr) uintptr {
			if p := gtkTentativeProbeActive; p != nil && p.context == ctx {
				p.signals = append(p.signals, "end")
			}
			return 0
		})
		gtkTentativeProbeSignals.commit = purego.NewCallback(func(ctx, value, _ uintptr) uintptr {
			if p := gtkTentativeProbeActive; p != nil && p.context == ctx {
				p.signals = append(p.signals, "commit:"+goString(value))
			}
			return 0
		})
	})
	ctx := p.w.ime.ctx
	if ctx != p.context {
		p.w.g.connect(ctx, "preedit-start", gtkTentativeProbeSignals.start, 0)
		p.w.g.connect(ctx, "preedit-changed", gtkTentativeProbeSignals.changed, 0)
		p.w.g.connect(ctx, "preedit-end", gtkTentativeProbeSignals.end, 0)
		p.w.g.connect(ctx, "commit", gtkTentativeProbeSignals.commit, 0)
		p.context = ctx
	}
}

func (p *gtkTentativeProbe) install(editor *text.Editor) {
	p.editor = editor
	field := platform.TextInput{
		Target: gfx.Item(1), X: 20, Y: 20, Width: 300, Height: 100, FontSize: 16,
		Multiline: true, Color: platform.Color{Alpha: 1}, State: platform.EditorState(editor),
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			p.events = append(p.events, event)
			if p.reject != nil {
				if err := p.reject(event); err != nil {
					return platform.EditorState(editor), err
				}
			}
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

func (p *gtkTentativeProbe) press(ctx uintptr, symbol, state uint32) error {
	return p.keyEvent(ctx, symbol, state, gdkKeyPress)
}

func (p *gtkTentativeProbe) release(symbol, state uint32) error {
	return p.keyEvent(p.w.ime.ctx, symbol, state, gdkKeyRelease)
}

func (p *gtkTentativeProbe) keyEvent(ctx uintptr, symbol, state uint32, kind gdkEventType) error {
	g := p.w.g
	keymap := g.gdkKeymapDefault()
	var entries uintptr
	var count int32
	if p.keys.entries(keymap, symbol, &entries, &count) == 0 || entries == 0 || count == 0 {
		return fmt.Errorf("no native keymap entry for %#x", symbol)
	}
	key := *(*gtkTentativeProbeKey)(unsafePointer(entries))
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
	event := p.keys.newEvent(int32(kind))
	defer p.keys.freeEvent(event)
	record := (*gdkEventKey)(unsafePointer(event))
	record.Window, record.SendEvent = p.keys.ref(window), 1
	record.State, record.Keyval = state, value
	record.HardwareKeycode, record.Group = uint16(key.Code), uint8(group)
	p.keys.setDevice(event, device)
	handled := false
	if p.keyIngress && ctx == p.w.ime.ctx {
		handled = p.w.keyEvent(event) != 0
	} else if ctx == p.w.ime.ctx {
		handled = p.w.ime.filter(event)
	} else {
		handled = g.imFilterKeypress(ctx, event) != 0
	}
	if !handled {
		return fmt.Errorf("native GTK did not consume key %#x (translated %#x)", symbol, value)
	}
	return nil
}

func (p *gtkTentativeProbe) compose() error {
	for _, symbol := range []uint32{'U', '3', 'A', '9'} {
		if err := p.press(p.w.ime.ctx, symbol, (1<<2)|(1<<0)); err != nil {
			return err
		}
	}
	return nil
}

func (p *gtkTentativeProbe) record(name string, failures ...string) {
	trial := gtkTentativeProbeTrial{name: name, value: p.editor.String(), signals: append([]string(nil), p.signals...), failures: failures}
	for _, event := range p.events {
		trial.events = append(trial.events, fmt.Sprintf("%d/%d:%q", event.Kind, event.CompositionPhase, event.Text))
		want := text.CompositionNone
		if p.tentative {
			switch event.Kind {
			case platform.TextSetMarked:
				want = text.CompositionUpdate
			case platform.TextInsert:
				if p.composingTrial {
					want = text.CompositionResult
				}
			case platform.TextCancelComposition:
				want = text.CompositionCancel
			default:
				trial.failures = append(trial.failures, fmt.Sprintf("unexpected tentative event kind %d", event.Kind))
			}
		}
		if event.CompositionPhase != want {
			trial.failures = append(trial.failures, fmt.Sprintf("event %d phase=%d, want %d", event.Kind, event.CompositionPhase, want))
		}
	}
	if p.tentative && !p.editor.TentativeCompositionEnabled() {
		trial.failures = append(trial.failures, "native ingress disabled the requested tentative capability")
	}
	if mirror := p.w.text.state; !p.w.closed && mirror != platform.EditorState(p.editor).WithoutRedraw() {
		trial.failures = append(trial.failures, fmt.Sprintf("model/mirror differ: model=%+v mirror=%+v", platform.EditorState(p.editor), mirror))
	}
	p.trials = append(p.trials, trial)
}

func (p *gtkTentativeProbe) run() {
	for _, tentative := range []bool{false, true} {
		p.tentative, p.composingTrial = tentative, true
		for _, ending := range []string{"commit", "escape", "reset", "focus-out", "window-focus-out"} {
			editor := text.NewEditor("abc")
			editor.SetSelection(text.Range{Start: 1, End: 2})
			if tentative && !editor.EnableTentativeComposition() {
				p.trials = append(p.trials, gtkTentativeProbeTrial{name: ending, failures: []string{"enable refused"}})
				return
			}
			p.install(editor)
			if err := p.compose(); err != nil {
				p.record(fmt.Sprintf("tentative=%t/%s", tentative, ending), err.Error())
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
				p.w.ime.callContext(p.w.ime.ctx, p.w.g.imReset)
			case "focus-out":
				p.w.g.imFocusOut(p.w.ime.ctx)
				if !slices.Equal(p.signals, prefix) || editor.String() != "au3A9c" {
					failures = append(failures, "GtkIMContextSimple focus-out unexpectedly settled preedit")
				}
				p.w.ime.callContext(p.w.ime.ctx, p.w.g.imReset)
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
			if tentative {
				terminals := 0
				wantPhase := text.CompositionCancel
				if ending == "commit" {
					wantPhase = text.CompositionResult
				}
				for _, event := range p.events {
					if event.CompositionPhase == text.CompositionResult || event.CompositionPhase == text.CompositionCancel {
						terminals++
						if event.CompositionPhase != wantPhase {
							failures = append(failures, fmt.Sprintf("terminal phase=%d, want %d", event.CompositionPhase, wantPhase))
						}
					}
				}
				if terminals != 1 {
					failures = append(failures, fmt.Sprintf("terminal count=%d, want exactly one", terminals))
				}
			}
			p.record(fmt.Sprintf("tentative=%t/%s", tentative, ending), failures...)
			if !editor.Undo() || ending == "commit" && editor.String() != "ac" || ending != "commit" && editor.String() != "abc" {
				p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, fmt.Sprintf("first Undo=%q", editor.String()))
			}
			if ending == "commit" && (!editor.Undo() || editor.String() != "abc") {
				p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, fmt.Sprintf("second Undo=%q", editor.String()))
			}
		}
		p.composingTrial = false
		plain := text.NewEditor("abc")
		if tentative {
			plain.EnableTentativeComposition()
		}
		p.install(plain)
		var failures []string
		for _, symbol := range []uint32{'r', 's'} {
			if err := p.press(p.w.ime.ctx, symbol, 0); err != nil {
				failures = append(failures, err.Error())
			}
		}
		if plain.String() != "abcrs" || !slices.Equal(p.signals, []string{"commit:r", "commit:s"}) {
			failures = append(failures, "ordinary native commits changed text or signal order")
		}
		p.record(fmt.Sprintf("tentative=%t/plain", tentative), failures...)
		if !plain.Undo() || plain.String() != "abc" {
			p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, "ordinary GTK commits stopped coalescing")
		}
	}

}

func (p *gtkTentativeProbe) ownership() {
	p.tentative, p.composingTrial = true, false
	for _, duringFrame := range []bool{false, true} {
		p.replaceOwner(duringFrame)
	}
	editor := text.NewEditor("abc")
	editor.EnableTentativeComposition()
	p.install(editor)
	context := p.w.ime.ctx
	field := p.w.text.input
	field.Width += 20
	p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil)
	var failures []string
	if context != p.w.ime.ctx {
		failures = append(failures, "geometry-only frame replaced the current native context")
	}
	if err := p.press(p.w.ime.ctx, 'r', 0); err != nil {
		failures = append(failures, err.Error())
	}
	if editor.String() != "abcr" {
		failures = append(failures, fmt.Sprintf("geometry-only frame dropped the native key: %q", editor.String()))
	}
	p.record("same-owner geometry", failures...)
}

func (p *gtkTentativeProbe) replaceOwner(duringFrame bool) {
	old := text.NewEditor("abc")
	old.EnableTentativeComposition()
	p.install(old)
	oldContext := p.keys.ref(p.w.ime.ctx)
	defer p.w.g.objectUnref(oldContext)
	next := text.NewEditor("abc")
	next.EnableTentativeComposition()
	var failures []string
	if duringFrame {
		p.w.config.FramePresented = func() {
			p.w.config.FramePresented = nil
			if err := p.press(oldContext, 'z', 0); err != nil {
				failures = append(failures, err.Error())
			}
		}
	}
	p.install(next)
	if !duringFrame {
		p.w.g.imReset(oldContext)
		if err := p.press(oldContext, 'z', 0); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if p.w.ime.ctx == oldContext || next.String() != "abc" || len(p.events) != 0 {
		failures = append(failures, fmt.Sprintf("old native context reached replacement input: reused=%t text=%q events=%d", p.w.ime.ctx == oldContext, next.String(), len(p.events)))
	}
	name := "same-target replacement"
	if duringFrame {
		name = "same-target frame handoff"
	}
	p.record(name, failures...)
}

func (p *gtkTentativeProbe) emit(signal, value string) {
	name, data := cString(signal), cString(value)
	p.keys.emit(p.w.ime.ctx, &name[0], &data[0])
}

func (p *gtkTentativeProbe) later(check func()) {
	a := p.w.app
	a.g.idleAddFull(250, callbacks().task, a.tasks.add(check), 0)
}

func (p *gtkTentativeProbe) beginEndTrial() error {
	p.tentative, p.composingTrial = true, true
	editor := text.NewEditor("abc")
	editor.SetSelection(text.Range{Start: 1, End: 2})
	if !editor.EnableTentativeComposition() {
		return fmt.Errorf("enable refused")
	}
	p.install(editor)
	return p.compose()
}

func (p *gtkTentativeProbe) endOrdering() {
	if err := p.beginEndTrial(); err != nil {
		p.record("idle end cancellation", err.Error())
		return
	}
	p.emit("preedit-end", "")
	p.asynchronous = true
	var failures []string
	if p.editor.String() != "au3A9c" || !p.w.text.state.Composing {
		failures = append(failures, "end canceled before the native signal batch returned")
	}
	p.later(func() {
		if p.editor.String() != "ac" || p.w.text.state.Composing || p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
			failures = append(failures, "idle end did not settle and release its task")
		}
		p.record("idle end cancellation", failures...)
		if !p.editor.Undo() || p.editor.String() != "abc" {
			p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, "cancel Undo did not restore selected source")
		}
		p.endThenResult()
	})
}

func (p *gtkTentativeProbe) endThenResult() {
	if err := p.beginEndTrial(); err != nil {
		p.record("end before result", err.Error())
		p.w.destroy()
		return
	}
	p.emit("preedit-end", "")
	p.emit("commit", "\u03a9")
	var failures []string
	results, cancels := 0, 0
	for _, event := range p.events {
		if event.CompositionPhase == text.CompositionResult {
			results++
		}
		if event.CompositionPhase == text.CompositionCancel {
			cancels++
		}
	}
	if p.editor.String() != "a\u03a9c" || p.w.text.state.Composing || results != 1 || cancels != 0 || p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
		failures = append(failures, fmt.Sprintf("result text=%q results=%d cancels=%d", p.editor.String(), results, cancels))
	}
	p.record("end before result", failures...)
	p.events, p.signals, p.composingTrial = nil, nil, false
	p.emit("commit", "\u03a9")
	failures = nil
	if p.editor.String() != "a\u03a9\u03a9c" || len(p.events) != 1 || p.events[0].CompositionPhase != text.CompositionNone {
		failures = append(failures, "ordinary same-value commit after Result was lost or tagged")
	}
	p.record("ordinary after result", failures...)
	if err := p.beginEndTrial(); err != nil {
		p.record("idle end owner handoff", err.Error())
		p.w.destroy()
		return
	}
	p.emit("preedit-end", "")
	old := p.w.ime
	next := text.NewEditor("abc")
	next.EnableTentativeComposition()
	p.install(next)
	p.composingTrial = false
	p.later(func() {
		var failures []string
		if next.String() != "abc" || len(p.events) != 0 || old.endTask != 0 || old.endSource != 0 {
			failures = append(failures, "deferred end entered the replacement owner or retained its task")
		}
		p.record("idle end owner handoff", failures...)
		p.closePendingEnd()
	})
}

func (p *gtkTentativeProbe) closePendingEnd() {
	if err := p.beginEndTrial(); err != nil {
		p.record("pending end close", err.Error())
		p.w.destroy()
		return
	}
	p.emit("preedit-end", "")
	ime := p.w.ime
	task, source, events := ime.endTask, ime.endSource, len(p.events)
	p.w.destroy()
	p.w.app.tasks.mu.Lock()
	retained := p.w.app.tasks.waiting[task] != nil
	p.w.app.tasks.mu.Unlock()
	var failures []string
	if !p.w.closed || p.w.ime != nil || ime.endTask != 0 || ime.endSource != 0 || retained || len(p.events) != events || p.w.g.sourceFind(0, source) != 0 {
		failures = append(failures, "closing retained a pending task/source or delivered a late event")
	}
	p.record("pending end close", failures...)
}

func (p *gtkTentativeProbe) callbackHandoff() {
	for _, phase := range []text.CompositionPhase{text.CompositionUpdate, text.CompositionResult, text.CompositionCancel} {
		if err := p.beginEndTrial(); err != nil {
			p.record(fmt.Sprintf("phase %d callback handoff", phase), err.Error())
			return
		}
		old := p.editor
		next := text.NewEditor("abc")
		next.EnableTentativeComposition()
		called := false
		p.hook = func(event platform.TextEvent) {
			if event.CompositionPhase != phase || phase == text.CompositionUpdate && event.Text != "" {
				return
			}
			p.hook = nil
			called = true
			p.install(next)
		}
		symbol := uint32(' ')
		if phase == text.CompositionCancel {
			symbol = 0xff1b
		}
		var failures []string
		if err := p.press(p.w.ime.ctx, symbol, (1<<2)|(1<<0)); err != nil {
			failures = append(failures, err.Error())
		}
		p.hook, p.composingTrial = nil, false
		wantOld := "ac"
		if phase == text.CompositionResult {
			wantOld = "a\u03a9c"
		}
		if !called || p.editor != next || next.String() != "abc" || old.String() != wantOld || len(p.events) != 0 {
			failures = append(failures, fmt.Sprintf("called=%t old=%q next=%q events=%d", called, old.String(), next.String(), len(p.events)))
		}
		p.record(fmt.Sprintf("phase %d callback handoff", phase), failures...)
	}
}

func (p *gtkTentativeProbe) provenance() {
	p.tentative, p.composingTrial = true, false
	editor := text.NewEditor("abc")
	editor.SetSelection(text.Range{Start: 1, End: 2})
	editor.EnableTentativeComposition()
	editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "M"})
	p.install(editor)
	var failures []string
	p.emit("preedit-changed", "")
	if len(p.events) != 0 || editor.String() != "aMc" {
		failures = append(failures, "idle native empty preedit changed a model-only composition")
	}
	if err := p.press(p.w.ime.ctx, 'r', 0); err != nil {
		failures = append(failures, err.Error())
	}
	if len(p.events) != 1 || p.events[0].CompositionPhase != text.CompositionNone {
		failures = append(failures, "ordinary native commit inferred Result from model-only preedit")
	}
	p.record("model-only preedit ordinary commit", failures...)
	editor = text.NewEditor("abc")
	editor.SetSelection(text.Range{Start: 1, End: 2})
	editor.EnableTentativeComposition()
	p.install(editor)
	p.emit("preedit-changed", "")
	failures = nil
	if len(p.events) != 0 || editor.String() != "abc" || editor.Selection() != (text.Range{Start: 1, End: 2}) || editor.CanUndo() {
		failures = append(failures, "an idle empty preedit changed source or history")
	}
	p.record("idle empty preedit", failures...)
	p.events, p.signals, p.composingTrial = nil, nil, true
	p.emit("preedit-start", "")
	p.emit("preedit-changed", "")
	failures = nil
	if len(p.events) != 1 || p.events[0].Kind != platform.TextSetMarked || p.events[0].CompositionPhase != text.CompositionUpdate {
		failures = append(failures, "active empty preedit was not an Update")
	}
	p.emit("preedit-end", "")
	p.asynchronous = true
	p.later(func() {
		if len(p.events) != 2 || p.events[1].CompositionPhase != text.CompositionCancel || editor.String() != "abc" || editor.CanUndo() {
			failures = append(failures, "empty session did not cancel without editing source")
		}
		p.record("active empty preedit", failures...)
		p.w.destroy()
	})
}

func (p *gtkTentativeProbe) keyBoundaries() {
	for _, shortcut := range []bool{false, true} {
		if err := p.beginEndTrial(); err != nil {
			p.record("key before end idle", err.Error())
			return
		}
		// Reset GTK's compose buffer without unwinding a bridge-owned native call.
		p.w.g.imReset(p.w.ime.ctx)
		p.events, p.composingTrial = nil, false
		shortcuts := 0
		if shortcut {
			p.w.config.OnShortcut = func(platform.KeyEvent) (platform.Frame, bool, error) {
				shortcuts++
				return platform.Frame{}, true, nil
			}
		}
		p.keyIngress = true
		var failures []string
		for _, symbol := range []uint32{'r', 's'} {
			if err := p.press(p.w.ime.ctx, symbol, 0); err != nil {
				failures = append(failures, err.Error())
			}
		}
		p.keyIngress, p.w.config.OnShortcut = false, nil
		want := "arsc"
		wantEvents, wantShortcuts := 3, 0
		if shortcut {
			want, wantEvents, wantShortcuts = "ac", 1, 2
		}
		if p.editor.String() != want || len(p.events) != wantEvents || p.events[0].CompositionPhase != text.CompositionCancel || shortcuts != wantShortcuts || p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
			failures = append(failures, fmt.Sprintf("text=%q events=%d shortcuts=%d pending=%t", p.editor.String(), len(p.events), shortcuts, p.w.ime.ending))
		}
		p.record(fmt.Sprintf("key before end idle shortcut=%t", shortcut), failures...)
		if !shortcut && (!p.editor.Undo() || p.editor.String() != "ac") {
			p.trials[len(p.trials)-1].failures = append(p.trials[len(p.trials)-1].failures, "ordinary keys after Cancel did not coalesce")
		}
	}
	for _, state := range []uint32{gdkControlMask, gdkMetaMask} {
		p.tentative, p.composingTrial = true, true
		editor := text.NewEditor("abc")
		editor.EnableTentativeComposition()
		p.install(editor)
		p.emit("preedit-start", "")
		p.emit("preedit-changed", "")
		shortcuts, filtered := 0, 0
		p.w.config.OnShortcut = func(platform.KeyEvent) (platform.Frame, bool, error) {
			shortcuts++
			return platform.Frame{}, true, nil
		}
		filter := p.w.g.imFilterKeypress
		p.w.g.imFilterKeypress = func(ctx, event uintptr) int32 {
			filtered++
			return filter(ctx, event)
		}
		p.keyIngress = true
		var failures []string
		if err := p.press(p.w.ime.ctx, 'q', state); err != nil {
			failures = append(failures, err.Error())
		}
		p.keyIngress, p.w.config.OnShortcut, p.w.g.imFilterKeypress = false, nil, filter
		if shortcuts != 0 || filtered != 1 {
			failures = append(failures, fmt.Sprintf("active empty session shortcuts=%d native filters=%d", shortcuts, filtered))
		}
		p.record(fmt.Sprintf("active empty session modifier=%#x", state), failures...)
	}
}

func (p *gtkTentativeProbe) keyCancelHandoff(closeWindow bool) {
	if err := p.beginEndTrial(); err != nil {
		p.record("key Cancel handoff", err.Error())
		return
	}
	p.w.g.imReset(p.w.ime.ctx)
	p.events, p.composingTrial = nil, false
	old := p.editor
	next := text.NewEditor("abc")
	next.EnableTentativeComposition()
	called := false
	p.hook = func(event platform.TextEvent) {
		if event.CompositionPhase == text.CompositionCancel {
			called = true
			p.hook = nil
			if closeWindow {
				p.w.destroy()
			} else {
				p.install(next)
			}
		}
	}
	p.keyIngress = true
	var failures []string
	if err := p.press(p.w.ime.ctx, 'r', 0); err != nil {
		failures = append(failures, err.Error())
	}
	p.keyIngress, p.hook = false, nil
	if !called || old.String() != "ac" || !closeWindow && (next.String() != "abc" || len(p.events) != 0) || closeWindow && (!p.w.closed || len(p.events) != 1) {
		failures = append(failures, fmt.Sprintf("called=%t old=%q next=%q callbacks=%d closed=%t", called, old.String(), next.String(), len(p.events), p.w.closed))
	}
	p.record(fmt.Sprintf("key Cancel handoff close=%t", closeWindow), failures...)
}

func (p *gtkTentativeProbe) releaseHandoff(modifier bool) {
	p.tentative = true
	editor := text.NewEditor("abc")
	editor.EnableTentativeComposition()
	p.install(editor)
	var keys []platform.KeyEvent
	p.w.config.ModifierTaps = true
	p.w.config.OnKey = func(event platform.KeyEvent) (platform.Frame, error) {
		keys = append(keys, event)
		return platform.Frame{}, nil
	}
	p.w.config.OnShortcut = func(event platform.KeyEvent) (platform.Frame, bool, error) {
		keys = append(keys, event)
		return platform.Frame{}, true, nil
	}
	symbol := uint32('r')
	if modifier {
		symbol = 0xffe1 // Shift_L
	}
	p.keyIngress = true
	if err := p.press(p.w.ime.ctx, symbol, 0); err != nil {
		p.record("release owner handoff", err.Error())
		return
	}
	p.keyIngress = false
	var failures []string
	var code uint16
	if len(p.w.keys.down) != 1 || len(keys) != 1 || keys[0].Repeat || keys[0].Release {
		failures = append(failures, fmt.Sprintf("initial down=%v keys=%+v", p.w.keys.down, keys))
	}
	for down := range p.w.keys.down {
		code = down
	}
	if err := p.beginEndTrial(); err != nil {
		p.record("release owner handoff", err.Error())
		return
	}
	p.w.g.imReset(p.w.ime.ctx)
	p.events, p.composingTrial = nil, false
	old := p.editor
	next := text.NewEditor("abc")
	next.EnableTentativeComposition()
	cancels := 0
	p.hook = func(event platform.TextEvent) {
		if event.CompositionPhase == text.CompositionCancel {
			cancels++
			p.hook = nil
			p.install(next)
		}
	}
	if !p.w.keys.down[code] || !p.w.ime.ending {
		failures = append(failures, "press or pending native end missing before release")
	}
	p.keyIngress = true
	state := uint32(0)
	if modifier {
		state = gdkShiftMask
	}
	if err := p.release(symbol, state); err != nil {
		failures = append(failures, err.Error())
	}
	if p.w.keys.down[code] {
		failures = append(failures, "release retained hardware key down")
	}
	p.asynchronous = true
	p.later(func() {
		if cancels != 1 || old.String() != "ac" || next.String() != "abc" || len(p.events) != 0 || p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
			failures = append(failures, fmt.Sprintf("cancels=%d old=%q next=%q callbacks=%d pending=%t", cancels, old.String(), next.String(), len(p.events), p.w.ime.ending))
		}
		if err := p.press(p.w.ime.ctx, symbol, 0); err != nil {
			failures = append(failures, err.Error())
		}
		want := 2
		if modifier {
			want = 3
			if len(keys) < 2 || !keys[1].Release || keys[1].Repeat || keys[1].Code != uint32(code) {
				failures = append(failures, fmt.Sprintf("modifier release missing or mismatched: %+v", keys))
			}
		}
		if len(keys) != want || keys[len(keys)-1].Repeat || keys[len(keys)-1].Release {
			failures = append(failures, fmt.Sprintf("next independent press keys=%+v, want %d events without Repeat", keys, want))
		}
		if next.String() != "abc" || len(p.events) != 0 {
			failures = append(failures, "old key reached replacement text input")
		}
		p.record(fmt.Sprintf("release owner handoff modifier=%t", modifier), failures...)
		p.w.destroy()
	})
}

func (p *gtkTentativeProbe) errorTail(symbol uint32) {
	if err := p.beginEndTrial(); err != nil {
		p.record("failed update tail", err.Error())
		return
	}
	p.expectedErr = errors.New("reject native empty update")
	p.events = nil
	p.reject = func(event platform.TextEvent) error {
		if event.CompositionPhase == text.CompositionUpdate && event.Text == "" {
			return p.expectedErr
		}
		return nil
	}
	var failures []string
	if err := p.press(p.w.ime.ctx, symbol, gdkControlMask|gdkShiftMask); err != nil {
		failures = append(failures, err.Error())
	}
	if !p.w.app.stopping || len(p.events) != 1 || p.events[0].CompositionPhase != text.CompositionUpdate || p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
		failures = append(failures, fmt.Sprintf("stopping=%t callbacks=%d pending=%t", p.w.app.stopping, len(p.events), p.w.ime.ending))
	}
	p.record("failed update tail", failures...)
}

func (p *gtkTentativeProbe) pendingEndError() {
	if err := p.beginEndTrial(); err != nil {
		p.record("pending end after error", err.Error())
		return
	}
	p.w.g.imReset(p.w.ime.ctx)
	p.events = nil
	p.expectedErr = errors.New("stop before pending end")
	p.w.present(platform.Frame{}, p.expectedErr)
	p.w.ime.finishPreedit(p.w.ime.ctx)
	var failures []string
	if len(p.events) != 0 || p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
		failures = append(failures, "pending end delivered after error or retained its task/source")
	}
	p.record("pending end after error", failures...)
}

func TestGTKNativeTentativeComposition(t *testing.T) {
	testGTKNativeTentativeProbe(t, 12, (*gtkTentativeProbe).run)
}

func TestGTKNativeSameTargetOwnership(t *testing.T) {
	testGTKNativeTentativeProbe(t, 3, (*gtkTentativeProbe).ownership)
}

func TestGTKNativeTentativeEndOrdering(t *testing.T) {
	testGTKNativeTentativeProbe(t, 5, (*gtkTentativeProbe).endOrdering)
}

func TestGTKNativeTentativeCallbackHandoff(t *testing.T) {
	testGTKNativeTentativeProbe(t, 3, (*gtkTentativeProbe).callbackHandoff)
}

func TestGTKNativeTentativeProvenance(t *testing.T) {
	testGTKNativeTentativeProbe(t, 3, (*gtkTentativeProbe).provenance)
}

func TestGTKNativeTentativeKeyBoundaries(t *testing.T) {
	testGTKNativeTentativeProbe(t, 4, (*gtkTentativeProbe).keyBoundaries)
}

func TestGTKNativeTentativeCancelErrorTail(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, func(p *gtkTentativeProbe) { p.errorTail(0xff1b) })
}

func TestGTKNativeTentativeResultErrorTail(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, func(p *gtkTentativeProbe) { p.errorTail(' ') })
}

func TestGTKNativeTentativePendingEndError(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, (*gtkTentativeProbe).pendingEndError)
}

func TestGTKNativeTentativeKeyHandoff(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, func(p *gtkTentativeProbe) { p.keyCancelHandoff(false) })
}

func TestGTKNativeTentativeKeyClose(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, func(p *gtkTentativeProbe) { p.keyCancelHandoff(true) })
}

func TestGTKNativeTentativeKeyReleaseHandoff(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, func(p *gtkTentativeProbe) { p.releaseHandoff(false) })
}

func TestGTKNativeTentativeModifierReleaseHandoff(t *testing.T) {
	testGTKNativeTentativeProbe(t, 1, func(p *gtkTentativeProbe) { p.releaseHandoff(true) })
}

func testGTKNativeTentativeProbe(t *testing.T, trials int, run func(*gtkTentativeProbe)) {
	t.Helper()
	if os.Getenv("VIVA_GTK_IME_NATIVE") != "1" {
		t.Skip("native GTK IME probe requires VIVA_GTK_IME_NATIVE=1 and a private display")
	}
	p := &gtkTentativeProbe{}
	gtkTentativeProbeActive = p
	defer func() { gtkTentativeProbeActive = nil }()
	err := RunGTKApplication(func(application platform.Application) error {
		a := application.(*gtkApplication)
		for _, binding := range []struct {
			function any
			name     string
		}{
			{&p.keys.newEvent, "gdk_event_new"}, {&p.keys.freeEvent, "gdk_event_free"},
			{&p.keys.setDevice, "gdk_event_set_device"}, {&p.keys.keyboard, "gdk_seat_get_keyboard"},
			{&p.keys.ref, "g_object_ref"}, {&p.keys.entries, "gdk_keymap_get_entries_for_keyval"},
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
			run(p)
			if !p.asynchronous {
				p.w.destroy()
			}
		})
		return nil
	})
	if !errors.Is(err, p.expectedErr) {
		t.Fatalf("application error=%v, want %v", err, p.expectedErr)
	}
	if len(p.trials) != trials {
		t.Fatalf("native trials=%d, want %d", len(p.trials), trials)
	}
	for _, trial := range p.trials {
		t.Logf("%s signals=%q text=%q events=%q", trial.name, trial.signals, trial.value, trial.events)
		for _, failure := range trial.failures {
			t.Errorf("%s: %s", trial.name, failure)
		}
	}
}
