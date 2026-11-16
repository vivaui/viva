package darwin

import (
	"encoding/json"
	"reflect"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type tentativeBridgeProbe struct {
	editor *text.Editor
	state  *runState
	view   unsafe.Pointer
	events []TextEvent
}

func newTentativeBridgeProbe(t testing.TB, value string, enabled bool) *tentativeBridgeProbe {
	t.Helper()
	probe := &tentativeBridgeProbe{editor: text.NewEditor(value), state: &runState{}}
	probe.editor.MoveTo(probe.editor.RuneCount(), false)
	if enabled && !probe.editor.EnableTentativeComposition() {
		t.Fatal("tentative capability refused")
	}
	view, cleanup := tentativeBridgeView(probe.state)
	probe.view = view
	t.Cleanup(cleanup)
	input := TextInput{Target: gfx.IDFromBits(7), Width: 100, Height: 20, FontSize: 12, State: platform.EditorState(probe.editor)}
	input.OnEvent = func(event TextEvent) (TextState, error) {
		probe.events = append(probe.events, event)
		event.Apply(probe.editor)
		return platform.EditorState(probe.editor), nil
	}
	if err := presentTextInput(probe.state, view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
		t.Fatal(err)
	}
	return probe
}

func (probe *tentativeBridgeProbe) assertState(t *testing.T, want string) {
	t.Helper()
	if probe.editor.String() != want || probe.state.textState != platform.EditorState(probe.editor) || probe.state.textInput.State != probe.state.textState || tentativeBridgeText(probe.view) != want {
		t.Fatalf("native acknowledgment: editor=%q bridge=%+v input=%+v native=%q; want %q", probe.editor.String(), probe.state.textState, probe.state.textInput.State, tentativeBridgeText(probe.view), want)
	}
	if probe.state.err != nil {
		t.Fatal(probe.state.err)
	}
}

func TestDarwinTentativeProbeTerminalPhases(t *testing.T) {
	for _, test := range []struct {
		name  string
		phase text.CompositionPhase
		want  string
	}{
		{"result", text.CompositionResult, "abcZ"},
		{"commit", text.CompositionCommit, "abcx"},
		{"cancel", text.CompositionCancel, "abc"},
		{"empty commit", text.CompositionEmptyCommit, "abc"},
		{"empty update", text.CompositionCancel, "abc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			probe := newTentativeBridgeProbe(t, "abc", true)
			tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
			probe.assertState(t, "abcx")
			if len(probe.events) != 1 || probe.events[0].CompositionPhase != text.CompositionUpdate {
				t.Errorf("real marked update lost phase: %+v", probe.events)
			}
			switch test.name {
			case "result":
				tentativeBridgeInsert(probe.view, "Z", 0, 0, false)
			case "commit":
				tentativeBridgeUnmark(probe.view)
			case "cancel":
				tentativeBridgeCancel(probe.view)
			case "empty commit":
				tentativeBridgeInsert(probe.view, "", 0, 0, false)
			case "empty update":
				tentativeBridgeMark(probe.view, "", 0, 0, 0, 0, false)
			}
			probe.assertState(t, test.want)
			if len(probe.events) != 2 || probe.events[1].CompositionPhase != test.phase {
				t.Fatalf("native terminal phase = %+v; want %v", probe.events, test.phase)
			}
			if _, composing := probe.editor.Composition(); composing {
				t.Fatal("native terminal stranded a live composition")
			}
			if test.phase == text.CompositionResult || test.phase == text.CompositionCommit {
				if !probe.editor.Undo() || probe.editor.String() != "abc" || probe.editor.CanUndo() {
					t.Fatalf("native terminal source Undo = %q", probe.editor.String())
				}
			} else if probe.editor.CanUndo() {
				t.Fatal("native cancellation fabricated ordinary history")
			}
		})
	}
}

func TestDarwinTentativeProbeRawResultAfterRollback(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "A\U0001d11eBC", true)
	probe.editor.MoveTo(0, false)
	input := probe.state.textInput
	input.State = platform.EditorState(probe.editor)
	if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
		t.Fatal(err)
	}
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	probe.assertState(t, "xA\U0001d11eBC")
	// AppKit names the view text, preedit included: index 4 is the "B" the
	// result replaces once the tentative "x" is rolled back.
	tentativeBridgeInsert(probe.view, "Z", 4, 1, true)
	probe.assertState(t, "A\U0001d11eZC")
	result := probe.events[len(probe.events)-1]
	if !result.HasReplacementUTF16 || result.ReplacementUTF16 != (text.UTF16Range{Start: 3, Length: 1}) || result.CompositionPhase != text.CompositionResult {
		t.Fatalf("raw native replacement provenance lost: %+v", result)
	}
	for _, want := range []string{"A\U0001d11eC", "A\U0001d11eBC"} {
		if !probe.editor.Undo() || probe.editor.String() != want {
			t.Fatalf("raw native result Undo = %q; want %q", probe.editor.String(), want)
		}
	}
}

func TestDarwinTentativeProbeMarkedRangeContinuationUpdates(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	probe.editor.MoveTo(1, false)
	input := probe.state.textInput
	input.State = platform.EditorState(probe.editor)
	if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
		t.Fatal(err)
	}
	tentativeBridgeMark(probe.view, "x", 1, 0, 1, 0, false)
	probe.assertState(t, "axbc")
	// AppKit reports the marked text in view coordinates when it continues the
	// composition. That is the same composition, not an out-of-band replacement,
	// so the update must land instead of being refused.
	tentativeBridgeMark(probe.view, "xy", 2, 0, 1, 1, true)
	probe.assertState(t, "axybc")
	if last := probe.events[len(probe.events)-1]; last.CompositionPhase != text.CompositionUpdate || last.HasReplacementUTF16 {
		t.Fatalf("continuation event = %+v; want an update that acts on the composition", last)
	}
}

func TestDarwinTentativeProbeMarkedRangeResultUsesTheComposition(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	probe.editor.MoveTo(1, false)
	input := probe.state.textInput
	input.State = platform.EditorState(probe.editor)
	if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
		t.Fatal(err)
	}
	tentativeBridgeMark(probe.view, "x", 1, 0, 1, 0, false)
	probe.assertState(t, "axbc")
	// Committing with a range that names the preedit replaces the composition,
	// not whatever runes sat at that offset before it was inserted.
	tentativeBridgeInsert(probe.view, "Z", 1, 1, true)
	probe.assertState(t, "aZbc")
	last := probe.events[len(probe.events)-1]
	if last.CompositionPhase != text.CompositionResult || last.HasReplacementUTF16 {
		t.Fatalf("commit event = %+v; want a result that acts on the composition", last)
	}
}

func TestDarwinTentativeProbeLegacyAndOrdinaryInput(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "enabled ordinary"
		}
		t.Run(name, func(t *testing.T) {
			probe := newTentativeBridgeProbe(t, "abc", enabled)
			tentativeBridgeInsert(probe.view, "Z", 0, 0, false)
			probe.assertState(t, "abcZ")
			if len(probe.events) != 1 || probe.events[0].CompositionPhase != text.CompositionNone || probe.events[0].HasReplacementUTF16 {
				t.Fatalf("ordinary input acquired composition facts: %+v", probe.events)
			}
			if !enabled {
				tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
				tentativeBridgeInsert(probe.view, "Y", 0, 0, false)
				probe.assertState(t, "abcZY")
				for _, event := range probe.events {
					if event.CompositionPhase != text.CompositionNone || event.HasReplacementUTF16 {
						t.Fatalf("disabled owner received tentative facts: %+v", event)
					}
				}
			}
		})
	}
}

func TestDarwinTentativeProbePresentationRefreshKeepsContext(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	before := append([]TextEvent(nil), probe.events...)
	resetTentativeBridgeCounts()
	for _, change := range []func(*TextInput){
		func(input *TextInput) { input.X++ },
		func(input *TextInput) { input.Width++ },
		func(input *TextInput) { input.FontSize++ },
	} {
		input := probe.state.textInput
		change(&input)
		if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
			t.Fatal(err)
		}
		probe.assertState(t, "abcx")
	}
	activate, deactivate, discard := tentativeBridgeCounts()
	if activate != 0 || deactivate != 0 || discard != 0 || !reflect.DeepEqual(before, probe.events) {
		t.Fatalf("refresh restarted native context: activate=%d deactivate=%d discard=%d events=%+v", activate, deactivate, discard, probe.events)
	}
}

func TestDarwinTentativeProbeRawRangeBoundaries(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	for _, test := range []struct {
		name          string
		start, length int
		raw           text.UTF16Range
		hasRaw        bool
		want          string
	}{
		{"not found", maximum, 0, text.UTF16Range{}, false, "a\U0001d11ebZ"},
		{"split surrogate", 2, 0, text.UTF16Range{Start: 2}, true, "a\U0001d11ebZ"},
		{"past end", 100, 99, text.UTF16Range{Start: 99, Length: 99}, true, "a\U0001d11ebZ"},
		{"unsigned start", -2, 1, text.UTF16Range{Start: maximum, Length: 1}, true, "a\U0001d11ebZ"},
		{"unsigned length", 1, -2, text.UTF16Range{Start: 1, Length: maximum}, true, "aZ"},
	} {
		t.Run(test.name, func(t *testing.T) {
			probe := newTentativeBridgeProbe(t, "a\U0001d11eb", true)
			tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
			tentativeBridgeInsert(probe.view, "Z", test.start, test.length, true)
			probe.assertState(t, test.want)
			last := probe.events[len(probe.events)-1]
			if last.HasReplacementUTF16 != test.hasRaw || last.ReplacementUTF16 != test.raw {
				t.Fatalf("native raw range = %+v; want %+v available=%v", last, test.raw, test.hasRaw)
			}
		})
	}
}

func TestDarwinTentativeProbeExplicitEmptyCommit(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeInsert(probe.view, "", 1, 1, true)
	probe.assertState(t, "ac")
	if len(probe.events) != 1 || probe.events[0].CompositionPhase != text.CompositionEmptyCommit || !probe.events[0].HasReplacementUTF16 {
		t.Fatalf("explicit native empty commit = %+v", probe.events)
	}
	if !probe.editor.Undo() || probe.editor.String() != "abc" || probe.editor.CanUndo() {
		t.Fatalf("empty commit Undo = %q", probe.editor.String())
	}
}

func TestDarwinTentativeProbeDeferredLossKeepsFinalAcknowledgment(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	probe.state.textHandler = func(event TextEvent) (TextState, error) {
		probe.events = append(probe.events, event)
		if event.CompositionPhase == text.CompositionCancel {
			event.Apply(probe.editor)
			return platform.EditorState(probe.editor), nil
		}
		next := platform.EditorState(probe.editor)
		next.Composing = false
		next.Composition = text.Range{}
		return next, nil
	}
	probe.state.textInput.OnEvent = probe.state.textHandler
	tentativeBridgeCommand(probe.view)
	probe.assertState(t, "abc")
	if len(probe.events) != 3 || probe.events[1].Kind != TextCommandEvent || probe.events[2].CompositionPhase != text.CompositionCancel || probe.editor.CanUndo() {
		t.Fatalf("native state loss did not cancel exactly once: %+v", probe.events)
	}
}

func TestDarwinTentativeProbeSameOwnerPresentationLoss(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	input := probe.state.textInput
	input.X++
	input.State.Composing = false
	input.State.Composition = text.Range{}
	if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
		t.Fatal(err)
	}
	probe.assertState(t, "abc")
	if len(probe.events) != 2 || probe.events[1].CompositionPhase != text.CompositionCancel || probe.editor.CanUndo() {
		t.Fatalf("same-owner presentation loss = %+v", probe.events)
	}
	if probe.state.textInput.X != input.X {
		t.Fatal("resolved cancellation lost the incoming presentation")
	}
}

func TestDarwinTentativeProbeNewOwnerDoesNotReceiveOldCancel(t *testing.T) {
	for _, target := range []uint64{7, 8} {
		t.Run(map[uint64]string{7: "same target", 8: "different target"}[target], func(t *testing.T) {
			probe := newTentativeBridgeProbe(t, "abc", true)
			tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
			probe.editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel})
			nextEditor := text.NewEditor("abc")
			nextEditor.MoveTo(3, false)
			if !nextEditor.EnableTentativeComposition() || !nextEditor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "x", Selected: text.Range{Start: 1, End: 1}}) {
				t.Fatal("new owner refused the real tentative")
			}
			input := probe.state.textInput
			input.Target = gfx.IDFromBits(target)
			input.State = platform.EditorState(nextEditor)
			if input.State.CompositionIdentity == probe.state.textState.CompositionIdentity {
				t.Fatal("distinct editor capability reused the old identity")
			}
			var nextEvents []TextEvent
			input.OnEvent = func(event TextEvent) (TextState, error) {
				nextEvents = append(nextEvents, event)
				event.Apply(nextEditor)
				return platform.EditorState(nextEditor), nil
			}
			resetTentativeBridgeCounts()
			if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
				t.Fatal(err)
			}
			activate, deactivate, discard := tentativeBridgeCounts()
			if activate != 1 || deactivate != 1 || discard != 1 || len(nextEvents) != 0 || probe.editor.String() != "abc" || probe.state.textState != platform.EditorState(nextEditor) || tentativeBridgeText(probe.view) != "abcx" {
				t.Fatalf("new owner inherited old callbacks: events=%+v old=%q state=%+v native=%q context=%d/%d/%d", nextEvents, probe.editor.String(), probe.state.textState, tentativeBridgeText(probe.view), activate, deactivate, discard)
			}
			tentativeBridgeInsert(probe.view, "Z", 0, 0, false)
			if nextEditor.String() != "abcZ" || probe.editor.String() != "abc" || len(nextEvents) != 1 || nextEvents[0].CompositionPhase != text.CompositionResult {
				t.Fatalf("new owner Result = %q old=%q events=%+v", nextEditor.String(), probe.editor.String(), nextEvents)
			}
		})
	}
}

func TestDarwinTentativeProbeNestedAcknowledgmentWins(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	old := probe.state.textState
	probe.state.textHandler = func(event TextEvent) (TextState, error) {
		probe.events = append(probe.events, event)
		event.Apply(probe.editor)
		if event.CompositionPhase == text.CompositionResult {
			tentativeBridgeUnmark(probe.view)
			return old, nil
		}
		return platform.EditorState(probe.editor), nil
	}
	probe.state.textInput.OnEvent = probe.state.textHandler
	tentativeBridgeInsert(probe.view, "Z", 0, 0, false)
	probe.assertState(t, "abcZ")
	if len(probe.events) != 3 || probe.events[2].CompositionPhase != text.CompositionCommit || !probe.editor.Undo() || probe.editor.String() != "abc" || probe.editor.CanUndo() {
		t.Fatalf("nested acknowledgment changed source history: %+v text=%q", probe.events, probe.editor.String())
	}
}

func TestDarwinTentativeProbeRefusedLossRestoresNativeMark(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	probe.state.textHandler = func(event TextEvent) (TextState, error) {
		probe.events = append(probe.events, event)
		next := platform.EditorState(probe.editor)
		if event.CompositionPhase != text.CompositionCancel {
			next.Composing = false
			next.Composition = text.Range{}
		}
		return next, nil
	}
	probe.state.textInput.OnEvent = probe.state.textHandler
	tentativeBridgeCommand(probe.view)
	probe.assertState(t, "abcx")
	if !tentativeBridgeMarked(probe.view) || len(probe.events) != 3 || probe.events[2].CompositionPhase != text.CompositionCancel || !probe.editor.TentativeCompositionActive() {
		t.Fatalf("refused loss desynchronized native preedit: marked=%v events=%+v", tentativeBridgeMarked(probe.view), probe.events)
	}
}

func TestDarwinTentativeProbeFocusLossCancelsOnce(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	tentativeBridgeUnmarkOnDiscard(true)
	t.Cleanup(func() { tentativeBridgeUnmarkOnDiscard(false) })
	tentativeBridgeResign(probe.view)
	tentativeBridgeResign(probe.view)
	probe.assertState(t, "abc")
	if len(probe.events) != 2 || probe.events[1].CompositionPhase != text.CompositionCancel || probe.editor.CanUndo() {
		t.Fatalf("native focus loss committed or repeated preedit: %+v", probe.events)
	}
}

func TestDarwinTentativeProbeSelectionCancelsBeforeCoordinates(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	tentativeBridgeUnmarkOnDiscard(true)
	t.Cleanup(func() { tentativeBridgeUnmarkOnDiscard(false) })
	tentativeBridgeSelect(probe.view, 4, 0)
	probe.assertState(t, "abc")
	if len(probe.events) != 3 || probe.events[1].CompositionPhase != text.CompositionCancel || probe.events[2].Kind != TextSetSelection || probe.events[2].Selection != (text.Range{Start: 3, End: 3}) || probe.editor.CanUndo() {
		t.Fatalf("native selection retained stale coordinates or committed preedit: %+v", probe.events)
	}
}

func TestDarwinTentativeProbeSelectionCannotFollowAReplacementOwner(t *testing.T) {
	for _, name := range []string{"same target", "different target", "passive owner"} {
		t.Run(name, func(t *testing.T) {
			probe := newTentativeBridgeProbe(t, "abc", true)
			tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
			nextEditor := text.NewEditor("replacement")
			nextEditor.SetSelection(text.Range{Start: 1, End: 3})
			if name != "passive owner" && !nextEditor.EnableTentativeComposition() {
				t.Fatal("next owner refused tentative composition")
			}
			want := platform.EditorState(nextEditor)
			var nextEvents []TextEvent
			probe.state.textHandler = func(event TextEvent) (TextState, error) {
				probe.events = append(probe.events, event)
				event.Apply(probe.editor)
				if event.CompositionPhase == text.CompositionCancel {
					input := probe.state.textInput
					if name == "different target" {
						input.Target = gfx.IDFromBits(8)
					}
					input.State = want
					input.OnEvent = func(event TextEvent) (TextState, error) {
						nextEvents = append(nextEvents, event)
						event.Apply(nextEditor)
						return platform.EditorState(nextEditor), nil
					}
					if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
						t.Fatal(err)
					}
				}
				return platform.EditorState(probe.editor), nil
			}
			probe.state.textInput.OnEvent = probe.state.textHandler
			tentativeBridgeSelect(probe.view, 4, 0)
			if len(nextEvents) != 0 || platform.EditorState(nextEditor) != want || probe.state.textState != want || probe.state.textInput.State != want || tentativeBridgeText(probe.view) != want.Value {
				t.Fatalf("old selection reached replacement owner: events=%+v editor=%+v bridge=%+v native=%q", nextEvents, platform.EditorState(nextEditor), probe.state.textState, tentativeBridgeText(probe.view))
			}
			if probe.editor.String() != "abc" || probe.editor.CompositionActive() || probe.editor.CanUndo() || probe.state.err != nil {
				t.Fatalf("old owner was not canceled: text=%q events=%+v error=%v", probe.editor.String(), probe.events, probe.state.err)
			}
		})
	}
}

func TestDarwinTentativeProbeDetachIsSilent(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	tentativeBridgeDetach(probe.view)
	if len(probe.events) != 1 || !probe.editor.TentativeCompositionActive() || tentativeBridgeMarked(probe.view) {
		t.Fatalf("native detach fabricated owner settlement: events=%+v editor=%q marked=%v", probe.events, probe.editor.String(), tentativeBridgeMarked(probe.view))
	}
}

func TestDarwinTentativeProbeLayout(t *testing.T) {
	config, view := tentativeBridgeSizes()
	sidecar := uintptr(0)
	if field, ok := reflect.TypeFor[runState]().FieldByName("textComposition"); ok {
		sidecar = field.Type.Elem().Size()
	}
	data, err := json.Marshal(struct {
		RunState, TextEvent, TextState, TextInput, NativeConfig, NativeView, EnabledSidecar uintptr
	}{unsafe.Sizeof(runState{}), unsafe.Sizeof(TextEvent{}), unsafe.Sizeof(TextState{}), unsafe.Sizeof(TextInput{}), uintptr(config), uintptr(view), sidecar})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("VIVA_LAYOUT %s", data)
}

func TestDarwinTentativeProbeRejectedUpdatePreservesOwner(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	before := platform.EditorState(probe.editor)
	tentativeBridgeMark(probe.view, "bad", 3, 0, 1, 1, true)
	probe.assertState(t, "abcx")
	if platform.EditorState(probe.editor) != before || !tentativeBridgeMarked(probe.view) || !probe.editor.TentativeCompositionActive() {
		t.Fatal("rejected native update changed the tentative boundary")
	}
	input := probe.state.textInput
	input.State.Selection = text.Range{Start: -1, End: 0}
	if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err == nil {
		t.Fatal("invalid native presentation was accepted")
	}
	probe.assertState(t, "abcx")
	if !tentativeBridgeMarked(probe.view) || platform.EditorState(probe.editor) != before {
		t.Fatal("rejected presentation retired the current native owner")
	}
}

func TestDarwinTentativeProbePassiveSnapshotKeepsLegacyInput(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	probe.editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCancel})
	probe.editor.DisableTentativeComposition()
	input := probe.state.textInput
	input.State = platform.EditorState(probe.editor)
	if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
		t.Fatal(err)
	}
	tentativeBridgeMark(probe.view, "y", 1, 0, 0, 0, false)
	tentativeBridgeInsert(probe.view, "Z", 0, 0, false)
	probe.assertState(t, "abcZ")
	for _, event := range probe.events[1:] {
		if event.CompositionPhase != text.CompositionNone || event.HasReplacementUTF16 {
			t.Fatalf("passive snapshot kept a tagged native route: %+v", event)
		}
	}
	if probe.state.textState.TentativeComposition || probe.editor.TentativeCompositionEnabled() {
		t.Fatal("passive native input fabricated a successful capability")
	}
}

func TestDarwinTentativeProbeLossCanReplaceOwner(t *testing.T) {
	probe := newTentativeBridgeProbe(t, "abc", true)
	tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
	nextEditor := text.NewEditor("next")
	nextEditor.MoveTo(4, false)
	if !nextEditor.EnableTentativeComposition() || !nextEditor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "y", Selected: text.Range{Start: 1, End: 1}}) {
		t.Fatal("next owner refused a tentative")
	}
	var nextEvents []TextEvent
	probe.state.textHandler = func(event TextEvent) (TextState, error) {
		probe.events = append(probe.events, event)
		if event.CompositionPhase == text.CompositionCancel {
			event.Apply(probe.editor)
			input := probe.state.textInput
			input.Target = gfx.IDFromBits(8)
			input.State = platform.EditorState(nextEditor)
			input.OnEvent = func(event TextEvent) (TextState, error) {
				nextEvents = append(nextEvents, event)
				event.Apply(nextEditor)
				return platform.EditorState(nextEditor), nil
			}
			if err := presentTextInput(probe.state, probe.view, TextInputUpdate{Changed: true, Input: &input}); err != nil {
				t.Fatal(err)
			}
			return platform.EditorState(probe.editor), nil
		}
		next := platform.EditorState(probe.editor)
		next.Composing = false
		next.Composition = text.Range{}
		return next, nil
	}
	probe.state.textInput.OnEvent = probe.state.textHandler
	tentativeBridgeCommand(probe.view)
	if probe.editor.String() != "abc" || probe.state.textState != platform.EditorState(nextEditor) || probe.state.textInput.Target != gfx.IDFromBits(8) || tentativeBridgeText(probe.view) != "nexty" || len(nextEvents) != 0 {
		t.Fatalf("loss acknowledgment overwrote new owner: old=%q current=%+v native=%q nextEvents=%+v", probe.editor.String(), probe.state.textState, tentativeBridgeText(probe.view), nextEvents)
	}
}

func BenchmarkDarwinTentativeProbeInput(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			probe := newTentativeBridgeProbe(b, "abc", enabled)
			if enabled {
				tentativeBridgeMark(probe.view, "x", 1, 0, 0, 0, false)
			}
			value := probe.editor.String()
			probe.state.textHandler = func(event TextEvent) (TextState, error) {
				if event.Kind != TextInsert || event.Text != "x" {
					b.Fatalf("native benchmark received %+v", event)
				}
				return probe.state.textState, nil
			}
			probe.state.textInput.OnEvent = probe.state.textHandler
			b.ReportAllocs()
			for b.Loop() {
				tentativeBridgeBenchmarkInsert(probe.view)
			}
			if probe.state.err != nil || tentativeBridgeText(probe.view) != value || probe.editor.String() != value {
				b.Fatalf("native benchmark lost its fixed acknowledgment: %v native=%q editor=%q", probe.state.err, tentativeBridgeText(probe.view), probe.editor.String())
			}
		})
	}
}

func BenchmarkDarwinTentativeProbeOwner(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			editor := text.NewEditor("abc")
			if enabled && !editor.EnableTentativeComposition() {
				b.Fatal("capability refused")
			}
			input := TextInput{Target: gfx.IDFromBits(7), State: platform.EditorState(editor)}
			state := &runState{}
			b.ReportAllocs()
			for b.Loop() {
				state.setTextInput(&input)
				state.setTextInput(nil)
			}
		})
	}
}
