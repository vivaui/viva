package platform

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestTextEventApplyTaggedCompositionRoutesOnlyWithCapability(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		enabled bool
		phase   text.CompositionPhase
	}{
		{name: "disabled-tagged", phase: text.CompositionResult},
		{name: "disabled-zero"},
		{name: "enabled-zero", enabled: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if scenario.enabled && !editor.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			TextEvent{
				Kind: TextInsert, Text: "X", CompositionPhase: scenario.phase,
				HasReplacement: true, Replacement: text.Range{Start: 1, End: 2},
				HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: 99, Length: 99},
			}.Apply(editor)
			if got := editor.String(); got != "aXc" {
				t.Fatalf("legacy rune targeting = %q, want aXc", got)
			}
			if editor.TentativeCompositionEnabled() != scenario.enabled || EditorState(editor).TentativeComposition != scenario.enabled {
				t.Fatal("event routing changed the opt-in capability")
			}
		})
	}
}

func TestTextEventApplyTaggedCompositionForwardsUpdateAndResult(t *testing.T) {
	editor := text.NewEditor("abcd")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	cursor, _, ok := editor.TrackHistory()
	if !ok {
		t.Fatal("history setup refused")
	}
	editor.SetSelection(text.Range{Start: 1, End: 2})
	TextEvent{
		Kind: TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "\u00e9\U0001d11e",
		MarkedSelection: text.Range{Start: 2, End: 2},
	}.Apply(editor)
	if got := editor.String(); got != "a\u00e9\U0001d11ecd" {
		t.Fatalf("tagged Update = %q", got)
	}
	if state := EditorState(editor); !state.Composing || !state.TentativeComposition {
		t.Fatalf("tagged Update state = %+v", state)
	}
	frames, cursor, ok := editor.HistorySince(cursor)
	if !ok || len(frames) != 1 || len(frames[0].Steps) != 3 {
		t.Fatalf("tagged Update frames = %+v, want one complete three-scalar frame", frames)
	}
	TextEvent{
		Kind: TextInsert, CompositionPhase: text.CompositionResult, Text: "r",
		HasReplacement: true, Replacement: text.Range{Start: 0, End: 1},
		HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: 5, Length: 1},
	}.Apply(editor)
	if got := editor.String(); got != "acdr" {
		t.Fatalf("tagged raw Result = %q, want acdr", got)
	}
	if state := EditorState(editor); state.Composing || !state.TentativeComposition {
		t.Fatalf("tagged Result state = %+v", state)
	}
	frames, _, ok = editor.HistorySince(cursor)
	if !ok || len(frames) != 1 || len(frames[0].Steps) != 3 || frames[0].BeforeBytes != 9 || frames[0].AfterBytes != 4 {
		t.Fatalf("tagged Result frames = %+v, want one atomic post-rollback frame", frames)
	}
}

func TestTextEventApplyTaggedCompositionTerminalPhases(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		phase text.CompositionPhase
		want  string
	}{
		{name: "commit", phase: text.CompositionCommit, want: "abcx"},
		{name: "cancel", phase: text.CompositionCancel, want: "abc"},
		{name: "empty-commit", phase: text.CompositionEmptyCommit, want: "abc"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			TextEvent{Kind: TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "x"}.Apply(editor)
			TextEvent{Kind: TextInsert, CompositionPhase: scenario.phase, Text: "ignored"}.Apply(editor)
			if got := editor.String(); got != scenario.want {
				t.Fatalf("%v = %q, want %q", scenario.phase, got, scenario.want)
			}
			if state := EditorState(editor); state.Composing || !state.TentativeComposition {
				t.Fatalf("%v state = %+v", scenario.phase, state)
			}
		})
	}
}

func TestTextEventApplyTaggedCompositionRejectsWithoutFallback(t *testing.T) {
	editor := text.NewEditor("abc")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	editor.SetSelection(text.Range{Start: 1, End: 1})
	before := EditorState(editor)
	cursor, _, ok := editor.TrackHistory()
	if !ok {
		t.Fatal("history setup refused")
	}
	for name, event := range map[string]TextEvent{
		"invalid phase": {Kind: TextInsert, Text: "BAD", CompositionPhase: text.CompositionPhase(255), HasReplacement: true, Replacement: text.Range{Start: 0, End: 3}},
		"negative raw":  {Kind: TextInsert, Text: "BAD", CompositionPhase: text.CompositionResult, HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: -1}},
		"capacity":      {Kind: TextInsert, Text: strings.Repeat("x", 4*1024*1024+1), CompositionPhase: text.CompositionUpdate},
	} {
		t.Run(name, func(t *testing.T) {
			event.Apply(editor)
			if editor.String() != "abc" || EditorState(editor) != before || editor.CanUndo() {
				t.Fatal("rejected tagged event mutated editing state")
			}
			if frames, latest, ok := editor.HistorySince(cursor); !ok || len(frames) != 0 || latest != cursor {
				t.Fatal("rejected event changed history metadata")
			}
		})
	}
}

func TestEditorStateTentativeCompositionSnapshot(t *testing.T) {
	editor := text.NewEditor("abc")
	if EditorState(editor).TentativeComposition || EditorState(editor).CompositionIdentity != (text.CompositionIdentity{}) {
		t.Fatal("default editor reported opt-in ownership")
	}
	if !editor.EnableTentativeComposition() || !EditorState(editor).TentativeComposition {
		t.Fatal("capability snapshot did not follow opt-in")
	}
	identity := editor.TentativeCompositionIdentity()
	TextEvent{Kind: TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "x"}.Apply(editor)
	if state := EditorState(editor); !state.Composing || !state.TentativeComposition || state.CompositionIdentity != identity {
		t.Fatalf("live state = %+v", state)
	}
	TextEvent{Kind: TextInsert, CompositionPhase: text.CompositionCommit}.Apply(editor)
	if state := EditorState(editor); state.Composing || !state.TentativeComposition || state.CompositionIdentity != identity {
		t.Fatalf("idle state = %+v", state)
	}
	if !editor.DisableTentativeComposition() {
		t.Fatal("disable refused")
	}
	if state := EditorState(editor); state.Composing || state.TentativeComposition || state.CompositionIdentity != (text.CompositionIdentity{}) {
		t.Fatalf("passive disabled state = %+v", state)
	}
}

func TestTextEventLegacyCancellationPreservesSelectedSource(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		editor := text.NewEditor("abc")
		if enabled && !editor.EnableTentativeComposition() {
			t.Fatal("capability setup refused")
		}
		editor.SetSelection(text.Range{Start: 1, End: 2})
		TextEvent{Kind: TextSetMarked, Text: "x"}.Apply(editor)
		TextEvent{Kind: TextCancelComposition}.Apply(editor)
		if editor.String() != "abc" || editor.Selection() != (text.Range{Start: 1, End: 2}) || editor.CanUndo() || editor.CanRedo() || editor.CompositionActive() {
			t.Fatalf("zero-phase legacy Cancel changed selected-source policy: enabled=%t state=%+v", enabled, EditorState(editor))
		}
	}
}
