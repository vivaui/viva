package windows

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestCompositionIMEEndPreservesOptInAndLegacyBoundaries(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		t.Run(name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if enabled && !editor.EnableTentativeComposition() {
				t.Fatal("could not enable fixture capability")
			}
			editor.SetSelection(text.Range{Start: 1, End: 2})
			preedit := platform.TextEvent{Kind: platform.TextSetMarked, Text: "x", MarkedSelection: text.Range{Start: 1, End: 1}}
			if enabled {
				preedit.CompositionPhase = text.CompositionUpdate
			}
			preedit.Apply(editor)
			if editor.String() != "axc" {
				t.Fatal("fixture did not begin selected preedit")
			}
			state := &windowState{ready: true, textEnabled: true, imeComposing: true, textState: platform.EditorState(editor)}
			var events []platform.TextEvent
			stop := errors.New("stop before renderer submission")
			state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				events = append(events, event)
				event.Apply(editor)
				return platform.EditorState(editor), stop
			}
			// Exercise the actual dispatcher without a graphics device. The handler
			// error stops rendering only after the event has reached the real Editor.
			const handle = ^uintptr(0)
			previous := active.Load()
			windowStates.Store(handle, state)
			t.Cleanup(func() { windowStates.Delete(handle); active.Store(previous) })
			windowProc(handle, wmIMEEnd, 0, 0)
			if len(events) != 1 || state.err != stop || state.imeComposing {
				t.Fatalf("end delivery: events=%+v err=%v composing=%v", events, state.err, state.imeComposing)
			}
			wantPhase, wantText := text.CompositionNone, "axc"
			if enabled {
				wantPhase, wantText = text.CompositionCancel, "ac"
			}
			if events[0].CompositionPhase != wantPhase || editor.String() != wantText {
				t.Fatalf("end phase=%v text=%q, want phase=%v text=%q", events[0].CompositionPhase, editor.String(), wantPhase, wantText)
			}
			windowProc(handle, wmIMEEnd, 0, 0)
			if len(events) != 1 {
				t.Fatal("duplicate end delivered another edit")
			}
			if enabled {
				editor.Undo()
				if editor.String() != "abc" {
					t.Fatalf("ordinary selection deletion did not remain undoable: %q", editor.String())
				}
			}
		})
	}
}

func BenchmarkCompositionTextEventAcknowledgment(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			state := &windowState{textEnabled: true, textState: platform.TextState{TentativeComposition: enabled}}
			state.textInput.Terminal = true
			state.textInput.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
				return state.textState, nil
			}
			event := platform.TextEvent{Kind: platform.TextSetMarked, Text: "x"}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				state.handleTextEvent(event)
			}
			b.ReportMetric(float64(unsafe.Sizeof(windowState{})), "window-B")
		})
	}
}
