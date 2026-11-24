//go:build linux && vivagtk

package linux

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestGTKFocusOutDiscardsNativeResetCommit(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		tentative bool
	}{{"legacy", false}, {"tentative", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			library := &gtkLibrary{}
			window := &gtkWindow{app: &gtkApplication{g: library}, g: library}
			state := platform.TextState{
				Value: "anic", Selection: text.Range{Start: 3, End: 3},
				Composition: text.Range{Start: 1, End: 3}, Composing: true,
				TentativeComposition: scenario.tentative,
			}
			var events []platform.TextEventKind
			window.text = &linuxTextInput{config: &window.config, enabled: true, state: state, input: platform.TextInput{
				State: state, HideNativeOverlays: true,
				OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
					events = append(events, event.Kind)
					return state, nil
				},
			}}
			ime := &gtkIME{w: window, g: library, ctx: 23, active: true, focused: true,
				session: true, engineComposing: true, tentative: scenario.tentative}
			window.ime = ime
			library.objectRef = func(context uintptr) uintptr { return context }
			library.objectUnref = func(uintptr) {}
			var nativeCalls []string
			library.imReset = func(context uintptr) {
				nativeCalls = append(nativeCalls, "reset")
				ime.preeditEnded()
				ime.commit("ni")
			}
			library.imFocusOut = func(context uintptr) { nativeCalls = append(nativeCalls, "focus-out") }
			ime.setFocused(false)
			if !slices.Equal(nativeCalls, []string{"reset", "focus-out"}) || ime.active || ime.session {
				t.Fatalf("unexpected blur teardown: calls=%v active=%t session=%t", nativeCalls, ime.active, ime.session)
			}
			if !slices.Equal(events, []platform.TextEventKind{platform.TextCancelComposition}) {
				t.Fatalf("blur events = %v, want a single cancellation", events)
			}
		})
	}
}
