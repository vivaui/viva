package linux

import (
	"errors"
	"os"
	"testing"

	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestIBusTentativeUnsupportedResetSettlesEditorBeforeRetirement(t *testing.T) {
	for _, mode := range []string{"missing", "unknown"} {
		for _, visibility := range []string{"visible", "hidden", "empty"} {
			t.Run(mode+"/"+visibility, func(t *testing.T) {
				h := newTentativeIMEHarness(t)
				event := ibus.PreeditEvent{Text: "ni", Cursor: 2, Visible: true}
				if mode == "unknown" {
					event.HasMode, event.Mode = true, 99
				}
				h.preedit(t, event)
				want := "anic"
				if visibility == "hidden" {
					h.preedit(t, ibus.PreeditEvent{Kind: ibus.PreeditHide})
					want = "ac"
				} else if visibility == "empty" {
					event.Text, event.Cursor = "", 0
					h.preedit(t, event)
					want = "ac"
				}
				if !h.ime.composing() {
					t.Fatal("native session did not start")
				}
				settled := len(h.events)
				h.ime.setFocused(false)
				if len(h.events) != settled+1 {
					t.Fatalf("retirement settled %d events, want exactly one", len(h.events)-settled)
				}
				if h.ime.app.firstErr != nil || h.ime.context != nil || !h.context.closed {
					t.Fatal("unsupported mode did not retire the context without disabling input")
				}
				if h.editor.CompositionActive() || h.editor.TentativeCompositionActive() || h.editor.String() != want || h.editor.CommittedString() != want || h.input.state != platform.EditorState(h.editor) {
					t.Fatalf("retired context left unsettled editor: value=%q committed=%q active=%t tentative=%t", h.editor.String(), h.editor.CommittedString(), h.editor.CompositionActive(), h.editor.TentativeCompositionActive())
				}
				last := h.events[len(h.events)-1]
				if last.Kind != platform.TextUnmark || last.CompositionPhase != text.CompositionCommit {
					t.Fatalf("retirement guessed an engine outcome: %+v", last)
				}
				count := len(h.events)
				h.preedit(t, nativeMode("stale", ibus.PreeditModeClear))
				h.ime.commit("stale")
				if h.editor.String() != want || len(h.events) != count {
					t.Fatal("retired context delivered a stale tail")
				}
				if visibility == "visible" && (!h.editor.Undo() || h.editor.String() != "ac") {
					t.Fatalf("committed preedit Undo = %q, want ac", h.editor.String())
				}
				if !h.editor.Undo() || h.editor.String() != "abc" {
					t.Fatalf("ordinary selection deletion Undo = %q, want abc", h.editor.String())
				}
			})
		}
	}
}

// A Reset that crosses an owner change must not settle the retired session's
// text into whatever now owns the field: sync re-checks the context, epoch and
// owner after a failed reset.
func TestRetirementKeepsSettlementWhenResetCrossesAnOwnerChange(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("ni", ibus.PreeditModeClear))
	before := len(h.events)
	h.context.err = errors.New("reset failed")
	h.context.onCall = func(name string) {
		if name == "Reset" {
			h.ime.epoch++
		}
	}
	h.ime.setFocused(false)
	if len(h.events) != before || h.ime.context == nil {
		t.Fatalf("a settlement crossed an owner change: events %d -> %d, context live=%t", before, len(h.events), h.ime.context != nil)
	}
}

func TestNativeIBusUnsupportedModeRetirement(t *testing.T) {
	if os.Getenv("VIVA_IBUS_NATIVE") != "1" {
		t.Skip("native retirement requires a private input-method daemon")
	}
	for _, mode := range []string{"missing", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			bus, err := ibus.Dial()
			if err != nil {
				t.Fatal(err)
			}
			defer bus.Close()
			h := newTentativeIMEHarness(t)
			app := &linuxApplication{ibus: bus, ibusTried: true}
			router := h.ime.router
			router.ime = nil
			ime := app.newIME(&window{}, h.input, router)
			if ime == nil || ime.phase == nil {
				t.Fatal("native phase context missing")
			}
			defer ime.close()
			ime.origin = h.ime.origin
			h.ime = ime
			h.focus(t)
			selectNativeIBusEngine(t, bus, ime.context, os.Getenv("VIVA_IBUS_ENGINE"))
			original := ime.context
			// Inject only the unsupported signal; context creation, retirement
			// and reopening use the real daemon and production factory.
			event := ibus.PreeditEvent{Text: "ni", Cursor: 2, Visible: true}
			if mode == "unknown" {
				event.HasMode, event.Mode = true, 99
			}
			ime.nativePreedit(event)
			ime.setFocused(false)
			if h.editor.String() != "anic" || h.editor.CompositionActive() || h.editor.TentativeCompositionActive() || h.input.state != platform.EditorState(h.editor) {
				t.Fatalf("unsupported native retirement left unsettled editor: %+v", platform.EditorState(h.editor))
			}
			ime.sync()
			if ime.dead || ime.context == nil || ime.context == original || ime.phase == nil || ime.phase.session || app.firstErr != nil {
				t.Fatal("retirement did not retain a fresh native context")
			}
			ime.setFocused(true)
			selectNativeIBusEngine(t, bus, ime.context, os.Getenv("VIVA_IBUS_ENGINE"))
			ime.nativePreedit(nativeMode("x", ibus.PreeditModeClear))
			ime.commit("z")
			if h.editor.String() != "anizc" || h.editor.CompositionActive() || !ime.active || app.firstErr != nil {
				t.Fatalf("fresh context did not accept subsequent composition: %q", h.editor.String())
			}
		})
	}
}
