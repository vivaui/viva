//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type gtkFcitxProbe struct {
	p         *gtkTentativeProbe
	engine    string
	trial     int
	ending    string
	want      string
	keys      []rune
	live      []string
	key       int
	contextID func(uintptr) uintptr
}

func TestGTKNativeFcitxComposition(t *testing.T) {
	if os.Getenv("VIVA_IBUS_FRONTEND") != "fcitx5" {
		t.Skip("native Fcitx GTK probe requires the private Fcitx fixture")
	}
	testGTKNativeTentativeProbe(t, 10, func(p *gtkTentativeProbe) {
		probe := &gtkFcitxProbe{p: p, engine: os.Getenv("VIVA_IBUS_ENGINE")}
		p.asynchronous, p.keyIngress = true, true
		backend := typeNameOf(p.w.g, p.w.g.gdkDisplayDefault())
		want := "GdkX11Display"
		if os.Getenv("VIVA_FCITX_BACKEND") == "wayland" {
			want = "GdkWaylandDisplay"
		}
		t.Logf("native GTK backend=%s engine=%s", backend, probe.engine)
		if backend != want || !p.w.g.lookup(&probe.contextID, "gtk_im_multicontext_get_context_id") {
			for range 10 {
				probe.record(fmt.Sprintf("native GTK backend=%s, want=%s, context getter=%t", backend, want, probe.contextID != nil))
			}
			p.w.destroy()
			return
		}
		probe.begin()
	})
}

func TestGTKNativeFcitxWindowClose(t *testing.T) {
	if os.Getenv("VIVA_IBUS_FRONTEND") != "fcitx5" {
		t.Skip("native Fcitx close probe requires the private Fcitx fixture")
	}
	for _, deferred := range []bool{false, true} {
		t.Run(fmt.Sprintf("deferred=%t", deferred), func(t *testing.T) {
			trials := 1
			if deferred {
				trials = 2
			}
			testGTKNativeTentativeProbe(t, trials, func(p *gtkTentativeProbe) {
				p.asynchronous, p.keyIngress, p.tentative, p.composingTrial = true, true, true, true
				editor := text.NewEditor("abc")
				editor.SetSelection(text.Range{Start: 1, End: 2})
				editor.EnableTentativeComposition()
				p.install(editor)
				ime := p.w.ime
				var closeWindow func(uintptr)
				if !p.w.g.lookup(&closeWindow, "gtk_window_close") {
					t.Error("native GTK cannot bind gtk_window_close")
					p.w.destroy()
					return
				}
				var allow func()
				requests, closed, before := 0, 0, 0
				live := ""
				if deferred {
					p.w.config.OnCloseRequest = func(next func()) { requests++; allow = next }
				}
				p.w.config.OnClosed = func() {
					closed++
					var failures []string
					if !p.w.closed || p.w.ime != nil || ime.ctx != 0 || closed != 1 || len(p.events) != before || editor.String() != live {
						failures = append(failures, "native close retained input ownership, ran twice or delivered a closing commit")
					}
					if deferred && requests != 1 {
						failures = append(failures, fmt.Sprintf("close request count=%d, want=1", requests))
					}
					p.record("native WM allowed close", failures...)
				}
				later := func(run func()) {
					a := p.w.app
					a.g.timeoutAdd(5, callbacks().task, a.tasks.add(run))
				}
				deadline := time.Now().Add(2 * time.Second)
				var ready, composed, requested func()
				failed := func(message string) {
					t.Error(message)
					before, live = len(p.events), editor.String()
					p.w.destroy()
				}
				requested = func() {
					if allow == nil {
						if time.Now().After(deadline) {
							failed("native WM did not ask the application before closing")
						} else {
							later(requested)
						}
						return
					}
					var failures []string
					if p.w.closed || p.w.destroying || p.w.ime != ime || !ime.active || requests != 1 || editor.String() != live {
						failures = append(failures, "native WM bypassed deferred close approval")
					}
					p.record("native WM deferred close", failures...)
					allow()
				}
				composed = func() {
					if !p.w.text.state.Composing {
						if time.Now().After(deadline) {
							failed("native close probe did not begin a Fcitx composition")
						} else {
							later(composed)
						}
						return
					}
					before, live = len(p.events), editor.String()
					closeWindow(p.w.toplevel)
					if deferred {
						later(requested)
					}
				}
				ready = func() {
					engine, err := nativeFcitxCommand("--check", "-n")
					if err != nil {
						failed(err.Error())
						return
					}
					if engine == "" {
						if time.Now().After(deadline) {
							failed("native close probe did not focus Fcitx")
						} else {
							later(ready)
						}
						return
					}
					if err := selectNativeFcitxEngine(os.Getenv("VIVA_IBUS_ENGINE")); err != nil {
						failed(err.Error())
						return
					}
					symbol := uint32('n')
					if os.Getenv("VIVA_IBUS_ENGINE") == "hangul" {
						symbol = 'r'
					}
					if err := p.press(ime.ctx, symbol, 0); err != nil {
						failed(err.Error())
						return
					}
					later(composed)
				}
				later(ready)
			})
		})
	}
}

func (probe *gtkFcitxProbe) later(delay time.Duration, run func()) {
	a := probe.p.w.app
	a.g.timeoutAdd(uint32(max(delay.Milliseconds(), 1)), callbacks().task, a.tasks.add(run))
}

func (probe *gtkFcitxProbe) wait(check func() (bool, error), next func()) {
	deadline := time.Now().Add(2 * time.Second)
	var poll func()
	poll = func() {
		ready, err := check()
		if err != nil || !ready && time.Now().After(deadline) {
			probe.finish(fmt.Sprintf("native wait: %v; text=%q composing=%t", err, probe.p.editor.String(), probe.p.w.text.state.Composing))
			return
		}
		if ready {
			next()
		} else {
			probe.later(10*time.Millisecond, poll)
		}
	}
	probe.later(time.Millisecond, poll)
}

func (probe *gtkFcitxProbe) begin() {
	if probe.trial == 10 {
		probe.p.w.destroy()
		return
	}
	p := probe.p
	p.tentative = probe.trial >= 5
	probe.ending = []string{"commit", "escape", "reset", "native-focus-out", "window-focus-out"}[probe.trial%5]
	editor := text.NewEditor("abc")
	editor.SetSelection(text.Range{Start: 1, End: 2})
	if p.tentative && !editor.EnableTentativeComposition() {
		probe.finish("tentative enable refused")
		return
	}
	p.install(editor)
	if module := goString(probe.contextID(p.w.ime.ctx)); module != "fcitx" && module != "fcitx5" {
		probe.finish("native GTK selected module " + module)
		return
	}
	probe.keys, probe.live = []rune("nihao"), []string{"anc", "anic", "ani hc", "ani hac", "ani haoc"}
	if probe.engine == "hangul" {
		probe.keys, probe.live = []rune("rksk"), []string{"a\u3131c", "a\uac00c", "a\uac04c", "a\uac00\ub098c"}
	}
	probe.key = 0
	probe.wait(func() (bool, error) {
		engine, err := nativeFcitxCommand("--check", "-n")
		return engine != "", err
	}, func() {
		if err := selectNativeFcitxEngine(probe.engine); err != nil {
			probe.finish(err.Error())
			return
		}
		probe.pressNext()
	})
}

func (probe *gtkFcitxProbe) pressNext() {
	if probe.key == len(probe.keys) {
		probe.settle()
		return
	}
	index := probe.key
	if err := probe.p.press(probe.p.w.ime.ctx, uint32(probe.keys[index]), 0); err != nil {
		probe.finish(err.Error())
		return
	}
	probe.wait(func() (bool, error) { return probe.p.editor.String() == probe.live[index], nil }, func() {
		probe.key++
		probe.pressNext()
	})
}

func (probe *gtkFcitxProbe) settle() {
	p, context := probe.p, probe.p.w.ime.ctx
	probe.want = "ani haoc"
	if probe.ending == "commit" {
		probe.want = "a\u4f60\u597dc"
	} else if probe.ending == "escape" {
		probe.want = "ac"
	}
	if probe.engine == "hangul" {
		probe.want = "a\uac00\ub098c"
		if probe.ending == "commit" {
			probe.want = "a\uac00\ub098 c"
		}
	}
	switch probe.ending {
	case "commit", "escape":
		symbol := uint32(' ')
		if probe.ending == "escape" {
			symbol = 0xff1b
		}
		if err := p.press(context, symbol, 0); err != nil {
			probe.finish(err.Error())
			return
		}
	case "reset":
		p.w.ime.callContext(context, p.w.g.imReset)
	case "native-focus-out":
		p.w.ime.callContext(context, p.w.g.imFocusOut)
		p.w.ime.active, p.w.ime.focused = false, false
	case "window-focus-out":
		p.w.ime.setFocused(false)
	}
	probe.wait(func() (bool, error) {
		return p.editor.String() == probe.want && !p.w.text.state.Composing, nil
	}, func() { probe.later(50*time.Millisecond, func() { probe.finish("") }) })
}

func (probe *gtkFcitxProbe) finish(failure string) {
	p := probe.p
	var failures []string
	if failure != "" {
		failures = append(failures, failure)
	} else {
		results, cancels := 0, 0
		for _, event := range p.events {
			if !p.tentative && event.CompositionPhase != text.CompositionNone ||
				p.tentative && event.Kind == platform.TextSetMarked && event.CompositionPhase != text.CompositionUpdate {
				failures = append(failures, "event has the wrong composition phase")
			}
			if event.CompositionPhase == text.CompositionResult {
				results++
			}
			if event.CompositionPhase == text.CompositionCancel {
				cancels++
			}
		}
		wantResults, wantCancels := 1, 0
		if probe.engine == "hangul" {
			wantResults = 2
		} else if probe.ending == "escape" {
			wantResults, wantCancels = 0, 1
		}
		if p.tentative && (results != wantResults || cancels != wantCancels) {
			failures = append(failures, fmt.Sprintf("results/cancels=%d/%d, want=%d/%d", results, cancels, wantResults, wantCancels))
		}
		if p.editor.String() != probe.want || p.w.text.state != platform.EditorState(p.editor).WithoutRedraw() || p.w.text.state.Composing {
			failures = append(failures, "settled editor/mirror differ or native tail mutated the result")
		}
		if p.w.ime.endTask != 0 || p.w.ime.endSource != 0 {
			failures = append(failures, "settled native context retained an end task/source")
		}
	}
	probe.record(failures...)
	if failure == "" && p.tentative {
		undos := []string{"ac", "abc"}
		if probe.engine == "hangul" {
			undos = []string{"a\uac00c", "ac", "abc"}
		} else if probe.ending == "escape" {
			undos = []string{"abc"}
		}
		for _, want := range undos {
			if !p.editor.Undo() || p.editor.String() != want {
				last := &p.trials[len(p.trials)-1]
				last.failures = append(last.failures, fmt.Sprintf("Undo=%q, want=%q", p.editor.String(), want))
				break
			}
		}
	}
	probe.trial++
	probe.later(time.Millisecond, probe.begin)
}

func (probe *gtkFcitxProbe) record(failures ...string) {
	p := probe.p
	value := ""
	if p.editor != nil {
		value = p.editor.String()
	}
	trial := gtkTentativeProbeTrial{name: fmt.Sprintf("fcitx/%s/%s/tentative=%t", probe.engine, probe.ending, p.tentative),
		value: value, signals: append([]string(nil), p.signals...), failures: failures}
	for _, event := range p.events {
		trial.events = append(trial.events, fmt.Sprintf("%d/%d:%q", event.Kind, event.CompositionPhase, event.Text))
	}
	p.trials = append(p.trials, trial)
}
