package windows

import (
	"errors"
	"runtime"
	"slices"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
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

// Losing the keyboard settles the live composition the way WM_IME_ENDCOMPOSITION
// does, and a native end that arrives afterwards delivers nothing more.
func TestCompositionKillFocusSettlesTheLiveComposition(t *testing.T) {
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
			state := &windowState{ready: true, textEnabled: true, imeComposing: true, imeHasTarget: true, textState: platform.EditorState(editor)}
			var events []platform.TextEvent
			stop := errors.New("stop before renderer submission")
			state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				events = append(events, event)
				event.Apply(editor)
				return platform.EditorState(editor), stop
			}
			const handle = ^uintptr(0)
			previous := active.Load()
			windowStates.Store(handle, state)
			t.Cleanup(func() { windowStates.Delete(handle); active.Store(previous) })
			windowProc(handle, wmKillFocus, 0, 0)
			if len(events) != 1 || state.imeComposing || state.imeHasTarget {
				t.Fatalf("focus loss delivery: events=%+v composing=%v target=%v", events, state.imeComposing, state.imeHasTarget)
			}
			wantPhase, wantText := text.CompositionNone, "axc"
			if enabled {
				wantPhase, wantText = text.CompositionCancel, "ac"
			}
			if events[0].CompositionPhase != wantPhase || editor.String() != wantText {
				t.Fatalf("focus loss phase=%v text=%q, want phase=%v text=%q", events[0].CompositionPhase, editor.String(), wantPhase, wantText)
			}
			windowProc(handle, wmIMEEnd, 0, 0)
			if len(events) != 1 {
				t.Fatal("a native end after focus loss delivered another edit")
			}
		})
	}
}

func compositionMessageWindow(t *testing.T, editor *text.Editor) (*windowState, *[]platform.TextEvent) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	state := dynamicTextState(0, 0)
	state.window, state.scale, state.ready = staticWindow(t), 1, true
	if err := state.renderer.initialize(state.window, platform.Color{Alpha: 1}); err != nil {
		t.Skipf("no graphics device for the native message surface: %v", err)
	}
	t.Cleanup(func() {
		state.stopCaretTimer()
		state.renderer.close()
	})
	state.textEnabled = true
	state.textInput = testTextInput(editor.String(), 0)
	state.textInput.Target = gfx.Item(1)
	state.textState = platform.EditorState(editor)
	state.textInput.State = state.textState
	events := new([]platform.TextEvent)
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		*events = append(*events, event)
		event.Apply(editor)
		return platform.EditorState(editor), nil
	}
	previous := active.Load()
	windowStates.Store(state.window, state)
	t.Cleanup(func() { windowStates.Delete(state.window); active.Store(previous) })
	return state, events
}

func TestCompositionMessageResultAndFollowingPreedit(t *testing.T) {
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
			state, events := compositionMessageWindow(t, editor)
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x", selection: text.Range{Start: 1, End: 1}})
			if state.err != nil || !state.textState.Composing || editor.String() != "axc" {
				t.Fatalf("preedit acknowledgement: %+v err=%v", state.textState, state.err)
			}
			// IMM may finish one conversion and begin the next in the same message.
			state.applyIMEComposition(imeCompositionMessage{
				hasResult: true, result: "r", hasPreedit: true, preedit: "y",
				selection: text.Range{Start: 1, End: 1}, target: text.Range{End: 1}, hasTarget: true,
			})
			if state.err != nil || len(*events) != 3 || editor.String() != "aryc" || !state.imeComposing || !state.imeHasTarget {
				t.Fatalf("combined result/preedit: text=%q events=%+v state=%+v err=%v", editor.String(), *events, state.textState, state.err)
			}
			want := []text.CompositionPhase{text.CompositionNone, text.CompositionNone, text.CompositionNone}
			if enabled {
				want = []text.CompositionPhase{text.CompositionUpdate, text.CompositionResult, text.CompositionUpdate}
			}
			phases := make([]text.CompositionPhase, len(*events))
			for index, event := range *events {
				phases[index] = event.CompositionPhase
			}
			if !slices.Equal(phases, want) {
				t.Fatalf("native phases=%v, want %v", phases, want)
			}
			windowProc(state.window, wmIMEEnd, 0, 0)
			wantText := "aryc"
			if enabled {
				wantText = "arc"
			}
			if state.err != nil || editor.String() != wantText || state.textState.Composing {
				t.Fatalf("end after combined message: text=%q state=%+v err=%v", editor.String(), state.textState, state.err)
			}
		})
	}
}

func TestCompositionMessageEmptyPreeditRemainsAnUpdate(t *testing.T) {
	editor := text.NewEditor("abc")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.SetSelection(text.Range{Start: 1, End: 2})
	state, events := compositionMessageWindow(t, editor)
	for _, value := range []string{"x", ""} {
		state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: value})
	}
	if state.err != nil || len(*events) != 2 || editor.String() != "ac" {
		t.Fatalf("empty preedit: text=%q events=%+v err=%v", editor.String(), *events, state.err)
	}
	for _, event := range *events {
		if event.CompositionPhase != text.CompositionUpdate {
			t.Fatalf("empty preedit fabricated a terminal phase: %+v", event)
		}
	}
}

func TestCompositionResultDoesNotSuppressAnIdenticalOrdinaryCharacter(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r"})
	windowProc(state.window, wmIMEEnd, 0, 0)
	windowProc(state.window, wmChar, 'r', 0)
	if state.err != nil || editor.String() != "arr" || len(*events) != 3 || (*events)[2].CompositionPhase != text.CompositionNone {
		t.Fatalf("ordinary character after result: text=%q events=%+v err=%v", editor.String(), *events, state.err)
	}
	windowProc(state.window, wmChar, 0xd801, 0)
	windowProc(state.window, wmChar, 0xdc37, 0)
	if state.err != nil || editor.String() != "arr\U00010437" || len(*events) != 4 || state.pendingHighSurrogate != 0 {
		t.Fatalf("UTF-16 character after result: text=%q events=%+v err=%v", editor.String(), *events, state.err)
	}
}

func TestCompositionMessageDoesNotContinueIntoAReplacementInput(t *testing.T) {
	for _, name := range []string{"replacement", "same target", "disabled", "multiline", "placed"} {
		t.Run(name, func(t *testing.T) {
			editor := text.NewEditor("a")
			if !editor.EnableTentativeComposition() {
				t.Fatal("could not enable fixture capability")
			}
			editor.MoveTo(1, false)
			state, events := compositionMessageWindow(t, editor)
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
			foreign := 0
			state.config.OnRedraw = func() (platform.Frame, error) {
				var input *platform.TextInput
				if name != "disabled" {
					next := testTextInput("other", 0)
					next.Target = gfx.Item(2)
					if name == "same target" {
						next.Target = state.textInput.Target
						next.State.TentativeComposition = true
					}
					if name == "multiline" || name == "placed" {
						next = state.textInput
						next.Multiline = name == "multiline"
						next.Placed = name == "placed"
					}
					next.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
						foreign++
						return next.State, nil
					}
					input = &next
				}
				return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: input}}, nil
			}
			state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
			if state.err != nil || editor.String() != "ar" || len(*events) != 2 || foreign != 0 {
				t.Fatalf("message reached a new owner: text=%q events=%+v foreign=%d err=%v", editor.String(), *events, foreign, state.err)
			}
		})
	}
}

func TestCompositionMessageContinuesAfterGeometryOnlyRedraw(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	state.config.OnRedraw = func() (platform.Frame, error) {
		next := state.textInput
		next.CaretX += 10
		next.Width += 10
		return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &next}}, nil
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 3 || editor.String() != "ary" || !state.textState.Composing {
		t.Fatalf("geometry redraw lost next preedit: text=%q events=%+v err=%v", editor.String(), *events, state.err)
	}
}

func TestCompositionMessageDoesNotEnterAnIdenticalNewEditor(t *testing.T) {
	makeEditor := func() *text.Editor {
		editor := text.NewEditor("a")
		if !editor.EnableTentativeComposition() {
			t.Fatal("could not enable fixture capability")
		}
		editor.MoveTo(1, false)
		return editor
	}
	editor, other := makeEditor(), makeEditor()
	other.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "x"})
	other.ApplyComposition(text.CompositionEvent{Phase: text.CompositionResult, Text: "r"})
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	foreign := 0
	state.config.OnRedraw = func() (platform.Frame, error) {
		next := state.textInput
		next.State = platform.EditorState(other)
		oldValue := state.textState
		oldValue.CompositionIdentity = next.State.CompositionIdentity
		if oldValue != next.State {
			t.Fatal("replacement fixture differs by more than Editor identity")
		}
		next.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
			foreign++
			event.Apply(other)
			return platform.EditorState(other), nil
		}
		return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &next}}, nil
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || foreign != 0 || len(*events) != 2 || editor.String() != "ar" || other.String() != "ar" {
		t.Fatalf("old message entered identical replacement: old=%q new=%q foreign=%d err=%v", editor.String(), other.String(), foreign, state.err)
	}
}

func TestCompositionMessageDoesNotContinueAfterReentrantEnd(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	state.config.OnRedraw = func() (platform.Frame, error) {
		windowProc(state.window, wmIMEEnd, 0, 0)
		return platform.Frame{}, nil
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 2 || editor.String() != "ar" || state.textState.Composing {
		t.Fatalf("old message resumed after End: text=%q events=%+v state=%+v err=%v", editor.String(), *events, state.textState, state.err)
	}
	if state.textState != platform.EditorState(editor) || state.textLabels[state.baseTextLabelCount].Value != state.textState.Value {
		t.Fatalf("End discarded the only Result frame: editor=%q state=%+v label=%q", editor.String(), state.textState, state.textLabels[state.baseTextLabelCount].Value)
	}
}

func TestCompositionResultAcknowledgmentSurvivesReentrantEnd(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	apply := state.textInput.OnEvent
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		ack, err := apply(event)
		windowProc(state.window, wmIMEEnd, 0, 0)
		return ack, err
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 2 || editor.String() != "ar" || state.textState != platform.EditorState(editor) ||
		state.textLabels[state.baseTextLabelCount].Value != "ar" {
		t.Fatalf("End discarded Result acknowledgment: editor=%q state=%+v events=%+v err=%v", editor.String(), state.textState, *events, state.err)
	}
}

func TestCompositionInputReplacementDoesNotOverwriteNativeCancelReentry(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	state, _ := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	stale := testTextInput("stale replacement", 0)
	latest := testTextInput("latest replacement", 0)
	latest.Target = gfx.Item(3)
	calls := 0
	_, _, err := state.applyTextInputUpdateWithCancel(platform.TextInputUpdate{Changed: true, Input: &stale}, func() {
		calls++
		// Model the synchronous window callback at the IMM cancellation boundary.
		if _, _, err := state.applyTextInputUpdate(platform.TextInputUpdate{Changed: true, Input: &latest}); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || calls != 1 || state.textInput.Target != latest.Target || state.textState != latest.State {
		t.Fatalf("cancel reentry overwritten: state=%+v target=%+v calls=%d err=%v", state.textState, state.textInput.Target, calls, err)
	}
}

func TestCompositionSupersededFrameDoesNotPublishOldNativeViews(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	state, _ := compositionMessageWindow(t, editor)
	base := render.Rectangle{Bounds: scene.Rect{X: 1, Width: 10, Height: 10}}
	if err := state.frameWithText(platform.Frame{Rectangles: []render.Rectangle{base}, Update: render.Update{Count: 1, Rebuild: true}}, nil, false); err != nil {
		t.Fatal(err)
	}
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	state.web = &webHost{state: state}
	stale, latest := testTextInput("stale", 0), testTextInput("latest", 0)
	staleView := platform.NativeView{ID: 2, Bounds: gfx.Rect{Width: 10, Height: 10}}
	latestView := platform.NativeView{ID: 3, Bounds: gfx.Rect{Width: 20, Height: 20}}
	presented := 0
	state.config.FramePresented = func() { presented++ }
	err := state.frameWithTextAndCancel(platform.Frame{
		TextInput: platform.TextInputUpdate{Changed: true, Input: &stale}, NativeViews: []platform.NativeView{staleView},
		Rectangles: []render.Rectangle{{Bounds: scene.Rect{X: 2, Width: 10, Height: 10}}}, Update: render.Update{Count: 1},
	}, nil, false, func() {
		if err := state.frameWithText(platform.Frame{
			TextInput: platform.TextInputUpdate{Changed: true, Input: &latest}, NativeViews: []platform.NativeView{latestView},
		}, nil, false); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || state.err != nil || state.textState != latest.State ||
		len(state.web.rects) != 1 || state.web.rects[0].ID != latestView.ID || presented == 0 || state.textRectangles[0] != base {
		t.Fatalf("superseded frame published: state=%+v views=%+v rectangle=%+v presented=%d err=%v", state.textState, state.web.rects, state.textRectangles[0], presented, err)
	}
}

func TestCompositionMessageDoesNotOverwriteAReentrantUpdate(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	reentered := false
	state.config.OnRedraw = func() (platform.Frame, error) {
		if !reentered {
			reentered = true
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "z"})
		}
		return platform.Frame{}, nil
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 3 || editor.String() != "arz" || state.textState.Value != "arz" {
		t.Fatalf("older tail overwrote reentrant update: text=%q state=%+v events=%+v err=%v", editor.String(), state.textState, *events, state.err)
	}
}

func TestCompositionMessageDoesNotInstallAReentrantEventStaleAck(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	apply := state.textInput.OnEvent
	reentered := false
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		ack, err := apply(event)
		if !reentered && event.CompositionPhase == text.CompositionResult {
			reentered = true
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "z"})
		}
		return ack, err
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 3 || editor.String() != "arz" || state.textState != platform.EditorState(editor) {
		t.Fatalf("stale event acknowledgment: text=%q state=%+v events=%+v err=%v", editor.String(), state.textState, *events, state.err)
	}
}

func TestCompositionEmptyResultDoesNotOverwriteSameValueReentrantPreedit(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	apply := state.textInput.OnEvent
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		ack, err := apply(event)
		if event.CompositionPhase == text.CompositionResult {
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
		}
		return ack, err
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 3 || editor.String() != "ax" || state.textState != platform.EditorState(editor) ||
		state.textLabels[state.baseTextLabelCount].Value != "ax" {
		t.Fatalf("same-value acknowledgment overwritten: editor=%q state=%+v events=%+v err=%v", editor.String(), state.textState, *events, state.err)
	}
}

func TestCompositionResultFrameSurvivesReentrantNoopAcknowledgment(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, _ := compositionMessageWindow(t, editor)
	state.textInput.Terminal, state.textInput.CellWidth, state.textInput.Baseline = true, 8, 12
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	redraws := 0
	state.config.OnRedraw = func() (platform.Frame, error) {
		redraws++
		if redraws > 1 {
			t.Fatal("unchanged terminal acknowledgment requested another redraw")
		}
		state.handleTextEvent(platform.TextEvent{Kind: platform.TextSetSelection, Selection: editor.Selection()})
		return platform.Frame{}, nil
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r"})
	if state.err != nil || redraws != 1 || editor.String() != "ar" || state.textState != platform.EditorState(editor) ||
		state.textLabels[state.baseTextLabelCount].Value != "ar" {
		t.Fatalf("noop acknowledgment lost Result frame: editor=%q state=%+v label=%q err=%v", editor.String(), state.textState, state.textLabels[state.baseTextLabelCount].Value, state.err)
	}
}

func TestCompositionMessageDoesNotInstallAReentrantRedrawStaleFrame(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	reentered := false
	state.config.OnRedraw = func() (platform.Frame, error) {
		stale := state.textInput
		if !reentered {
			reentered = true
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "z"})
		}
		return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &stale}}, nil
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 3 || editor.String() != "arz" || state.textState != platform.EditorState(editor) {
		t.Fatalf("stale redraw acknowledgment: text=%q state=%+v events=%+v err=%v", editor.String(), state.textState, *events, state.err)
	}
}

func TestCompositionMessageDoesNotContinueAfterAckChangesOwner(t *testing.T) {
	editor, other := text.NewEditor("a"), text.NewEditor("other")
	if !editor.EnableTentativeComposition() || !other.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	foreign := 0
	apply := state.textInput.OnEvent
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		_, err := apply(event)
		state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
			foreign++
			event.Apply(other)
			return platform.EditorState(other), nil
		}
		return platform.EditorState(other), err
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != nil || len(*events) != 2 || foreign != 0 || editor.String() != "ar" || other.String() != "other" || state.textState != platform.EditorState(other) {
		t.Fatalf("message entered an acknowledgment owner: old=%q new=%q foreign=%d state=%+v err=%v", editor.String(), other.String(), foreign, state.textState, state.err)
	}
}

func TestCompositionFailedResultReadStillAllowsEndCancellation(t *testing.T) {
	for _, failure := range []string{"negative length", "odd length", "negative write", "odd write", "oversized write"} {
		t.Run(failure, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				t.Fatal("could not enable fixture capability")
			}
			editor.SetSelection(text.Range{Start: 1, End: 2})
			state, events := compositionMessageWindow(t, editor)
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
			read := func(args ...uintptr) (uintptr, uintptr, error) {
				if args[2] == 0 {
					switch failure {
					case "negative length":
						return ^uintptr(0), 0, nil
					case "odd length":
						return 3, 0, nil
					}
					return 4, 0, nil
				}
				switch failure {
				case "negative write":
					return ^uintptr(0), 0, nil
				case "odd write":
					return 3, 0, nil
				default:
					return 6, 0, nil
				}
			}
			state.handleCompositionData(1, resultString, read)
			if len(*events) != 1 || editor.String() != "axc" || state.err != nil {
				t.Fatal("a failed IMM read fabricated a result or damaged the pending state")
			}
			windowProc(state.window, wmIMEEnd, 0, 0)
			if len(*events) != 2 || editor.String() != "ac" || state.textState.Composing || (*events)[1].CompositionPhase != text.CompositionCancel {
				t.Fatalf("failed read stranded composition: text=%q events=%+v state=%+v", editor.String(), *events, state.textState)
			}
			editor.Undo()
			if editor.String() != "abc" {
				t.Fatal("failed sampling lost the ordinary deletion's Undo boundary")
			}
		})
	}
}

func TestCompositionSamplesTheWholeMessageBeforeCallbacks(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, events := compositionMessageWindow(t, editor)
	sampledCursor := false
	apply := state.textInput.OnEvent
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		if !sampledCursor {
			t.Error("application callback ran before native sampling finished")
		}
		return apply(event)
	}
	read := func(args ...uintptr) (uintptr, uintptr, error) {
		switch args[1] {
		case compositionCursor:
			sampledCursor = true
			return 2, 0, nil
		case compositionAttr:
			return 0, 0, nil
		}
		value := "r"
		if args[1] == compositionString {
			value = "\U00010437x"
		}
		units := utf16.Encode([]rune(value))
		if args[2] != 0 {
			copy(unsafe.Slice((*uint16)(pointer(args[2])), len(units)), units)
		}
		return uintptr(len(units) * 2), 0, nil
	}
	state.handleCompositionData(1, resultString|compositionString|compositionCursor, read)
	if state.err != nil || len(*events) != 2 || editor.String() != "ar\U00010437x" || (*events)[1].MarkedSelection != (text.Range{Start: 1, End: 1}) {
		t.Fatalf("sampled message/UTF-16 cursor: text=%q events=%+v err=%v", editor.String(), *events, state.err)
	}
}

func TestCompositionMessageStopsAfterAHandlerError(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	editor.MoveTo(1, false)
	state, _ := compositionMessageWindow(t, editor)
	stop, calls := errors.New("rejected native event"), 0
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		calls++
		event.Apply(editor)
		return platform.EditorState(editor), stop
	}
	state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
	if state.err != stop || calls != 1 || editor.String() != "ar" {
		t.Fatalf("continued a failed message: calls=%d text=%q err=%v", calls, editor.String(), state.err)
	}
}

func BenchmarkCompositionMessageDispatch(b *testing.B) {
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
			message := imeCompositionMessage{hasPreedit: true, preedit: "x"}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				state.applyIMEComposition(message)
			}
			b.ReportMetric(float64(unsafe.Sizeof(windowState{})), "window-B")
		})
	}
}

func TestCompositionNativeCancelDoesNotReenableDestroyedWindow(t *testing.T) {
	editor := text.NewEditor("a")
	if !editor.EnableTentativeComposition() {
		t.Fatal("could not enable fixture capability")
	}
	state, _ := compositionMessageWindow(t, editor)
	state.application = &windowsApplication{windows: 1}
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
	window := state.window
	closed, staleCalls := 0, 0
	state.config.OnClosed = func() { closed++ }
	stale := testTextInput("stale replacement", 0)
	stale.Target = gfx.Item(2)
	stale.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		staleCalls++
		return stale.State, nil
	}
	_, _, err := state.applyTextInputUpdateWithCancel(platform.TextInputUpdate{Changed: true, Input: &stale}, func() {
		windowProc(window, wmDestroy, 0, 0)
	})
	if state.window != 0 || state.application.windows != 0 || closed != 1 || state.err != nil {
		t.Fatalf("window destruction fixture: window=%d count=%d closed=%d err=%v", state.window, state.application.windows, closed, state.err)
	}
	if err != nil || state.textEnabled || state.textInput.Target == stale.Target {
		t.Fatalf("native cancellation revived destroyed input: enabled=%v target=%v err=%v", state.textEnabled, state.textInput.Target, err)
	}
	state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "late"})
	if staleCalls != 0 {
		t.Fatalf("destroyed window received %d stale callbacks", staleCalls)
	}
}

func TestCompositionDestroyedCallbacksDoNotReviveFrames(t *testing.T) {
	for _, callback := range []string{"event", "redraw"} {
		t.Run(callback, func(t *testing.T) {
			editor := text.NewEditor("a")
			if !editor.EnableTentativeComposition() {
				t.Fatal("could not enable fixture capability")
			}
			editor.MoveTo(1, false)
			state, events := compositionMessageWindow(t, editor)
			state.application = &windowsApplication{windows: 1}
			state.web = &webHost{state: state}
			state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
			window, closed := state.window, 0
			state.config.OnClosed = func() { closed++ }
			if callback == "event" {
				apply := state.textInput.OnEvent
				state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
					ack, err := apply(event)
					windowProc(window, wmDestroy, 0, 0)
					return ack, err
				}
			} else {
				state.config.OnRedraw = func() (platform.Frame, error) {
					stale := state.textInput
					windowProc(window, wmDestroy, 0, 0)
					return platform.Frame{
						TextInput:   platform.TextInputUpdate{Changed: true, Input: &stale},
						NativeViews: []platform.NativeView{{ID: 9, Bounds: gfx.Rect{Width: 10, Height: 10}}},
					}, nil
				}
			}
			var trapped any
			func() {
				defer func() { trapped = recover() }()
				state.applyIMEComposition(imeCompositionMessage{hasResult: true, result: "r", hasPreedit: true, preedit: "y"})
			}()
			if state.window != 0 || state.application.windows != 0 || closed != 1 {
				t.Fatalf("window destruction fixture: window=%d count=%d closed=%d", state.window, state.application.windows, closed)
			}
			if trapped != nil {
				t.Fatalf("closed callback used released native resources: %v", trapped)
			}
			if state.err != nil || state.textEnabled || editor.String() != "ar" || len(*events) != 2 || len(state.web.rects) != 0 {
				t.Fatalf("closed callback resumed input/frame: enabled=%v text=%q events=%d views=%d err=%v", state.textEnabled, editor.String(), len(*events), len(state.web.rects), state.err)
			}
		})
	}
}

func TestCompositionCancellationKeepsRetiredIMEOffTheReplacement(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		mode := "legacy"
		if enabled {
			mode = "tentative"
		}
		for _, message := range []string{"retired preedit", "ordinary character"} {
			t.Run(mode+"/"+message, func(t *testing.T) {
				outgoing, incoming := text.NewEditor("a"), text.NewEditor("abc")
				if enabled && (!outgoing.EnableTentativeComposition() || !incoming.EnableTentativeComposition()) {
					t.Fatal("could not enable fixture capability")
				}
				outgoing.MoveTo(1, false)
				state, _ := compositionMessageWindow(t, outgoing)
				state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
				if enabled {
					outgoing.ApplyComposition(text.CompositionEvent{Phase: text.CompositionCommit})
				} else {
					outgoing.Unmark()
				}
				incoming.SetSelection(text.Range{Start: 1, End: 2})
				foreign := 0
				state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
					foreign++
					event.Apply(incoming)
					return platform.EditorState(incoming), nil
				}
				next := state.textInput
				next.Target, next.State = gfx.Item(2), platform.EditorState(incoming)
				state.config.OnRedraw = func() (platform.Frame, error) {
					live := next
					live.State = platform.EditorState(incoming)
					return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &live}}, nil
				}
				_, _, err := state.applyTextInputUpdateWithCancel(platform.TextInputUpdate{Changed: true, Input: &next}, func() {
					if message == "ordinary character" {
						windowProc(state.window, wmChar, 'q', 0)
					} else {
						state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: ""})
						windowProc(state.window, wmIMEEnd, 0, 0)
					}
				})
				wantText, wantCalls := "abc", 0
				if message == "ordinary character" {
					wantText, wantCalls = "aqc", 1
				}
				if err != nil || state.err != nil || incoming.String() != wantText || foreign != wantCalls ||
					!state.textEnabled || state.textInput.Target != next.Target || state.textState != platform.EditorState(incoming) {
					t.Fatalf("cancellation crossed input ownership: message=%s text=%q calls=%d enabled=%v target=%v state=%+v err=%v/%v", message, incoming.String(), foreign, state.textEnabled, state.textInput.Target, state.textState, err, state.err)
				}
				if message == "retired preedit" {
					state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "y"})
					if state.err != nil || foreign != 1 || incoming.String() != "ayc" || !state.textState.Composing {
						t.Fatalf("fresh owner could not compose: text=%q calls=%d state=%+v err=%v", incoming.String(), foreign, state.textState, state.err)
					}
				}
			})
		}
	}
}

func TestCompositionAcknowledgmentCancellationPreservesTheCurrentOwner(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		mode := "legacy"
		if enabled {
			mode = "tentative"
		}
		for _, reentry := range []string{"retired preedit", "ordinary character", "new input", "destroyed window"} {
			t.Run(mode+"/"+reentry, func(t *testing.T) {
				outgoing, incoming := text.NewEditor("a"), text.NewEditor("abc")
				if !outgoing.EnableTentativeComposition() || enabled && !incoming.EnableTentativeComposition() {
					t.Fatal("could not enable fixture capability")
				}
				outgoing.MoveTo(1, false)
				incoming.SetSelection(text.Range{Start: 1, End: 2})
				state, events := compositionMessageWindow(t, outgoing)
				state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "x"})
				state.application = &windowsApplication{windows: 1}
				foreign, redraws, closed := 0, 0, 0
				apply := state.textInput.OnEvent
				state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
					_, err := apply(event)
					state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
						foreign++
						event.Apply(incoming)
						return platform.EditorState(incoming), nil
					}
					return platform.EditorState(incoming), err
				}
				state.config.OnRedraw = func() (platform.Frame, error) {
					redraws++
					return platform.Frame{}, nil
				}
				state.config.OnClosed = func() { closed++ }
				latest := testTextInput("latest", 0)
				latest.Target = gfx.Item(3)
				cancelCalls := 0
				_, changed, err := state.textEventFrameWithCancel(platform.TextEvent{
					Kind: platform.TextInsert, Text: "r", CompositionPhase: text.CompositionResult,
				}, func() {
					cancelCalls++
					switch reentry {
					case "retired preedit":
						state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: ""})
						windowProc(state.window, wmIMEStart, 0, 0)
						windowProc(state.window, wmIMEEnd, 0, 0)
					case "ordinary character":
						windowProc(state.window, wmChar, 'q', 0)
					case "new input":
						_, _, updateErr := state.applyTextInputUpdate(platform.TextInputUpdate{Changed: true, Input: &latest})
						if updateErr != nil {
							t.Fatalf("new input during cancellation: %v", updateErr)
						}
					case "destroyed window":
						windowProc(state.window, wmDestroy, 0, 0)
					}
				})
				if err != nil || state.err != nil || cancelCalls != 1 || outgoing.String() != "ar" || len(*events) != 2 {
					t.Fatalf("acknowledgment fixture: cancels=%d text=%q events=%d err=%v/%v", cancelCalls, outgoing.String(), len(*events), err, state.err)
				}
				wantText, wantCalls, wantRedraws := "abc", 0, 1
				switch reentry {
				case "ordinary character":
					wantText, wantCalls = "aqc", 1
				case "new input":
					wantRedraws = 0
					if !state.textEnabled || state.textInput.Target != latest.Target || state.textState.Value != "latest" {
						t.Fatalf("cancellation lost the newest input: enabled=%v target=%v state=%+v", state.textEnabled, state.textInput.Target, state.textState)
					}
				case "destroyed window":
					wantRedraws = 0
					if state.window != 0 || state.textEnabled || state.application.windows != 0 || closed != 1 || changed {
						t.Fatalf("cancellation revived a destroyed window: window=%d enabled=%v count=%d closed=%d changed=%v", state.window, state.textEnabled, state.application.windows, closed, changed)
					}
				}
				if incoming.String() != wantText || foreign != wantCalls || redraws != wantRedraws || state.imeCancelling {
					t.Fatalf("acknowledgment cancellation crossed owners: text=%q calls=%d redraws=%d cancelling=%v", incoming.String(), foreign, redraws, state.imeCancelling)
				}
				if reentry == "retired preedit" {
					state.applyIMEComposition(imeCompositionMessage{hasPreedit: true, preedit: "y"})
					if state.err != nil || incoming.String() != "ayc" || foreign != 1 || !state.textState.Composing {
						t.Fatalf("replacement could not begin a fresh composition: text=%q calls=%d state=%+v err=%v", incoming.String(), foreign, state.textState, state.err)
					}
				}
			})
		}
	}
}
