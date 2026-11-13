package linux

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
)

func callbackTextField(editor *text.Editor) platform.TextInput {
	return platform.TextInput{
		Target: gfx.Scope(1).At(1), Width: 120, Height: 28, FontSize: 16,
		State: platform.EditorState(editor),
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(editor)
			return platform.EditorState(editor), nil
		},
	}
}

func prepareCallbackFrame(t *testing.T, state *linuxTextInput, frame platform.Frame) platform.Frame {
	t.Helper()
	merged, _, err := state.prepare(frame)
	if err != nil {
		t.Fatal(err)
	}
	return merged
}

func TestLinuxTextCallbackKeepsInputInstalledDuringEvent(t *testing.T) {
	for _, scenario := range []struct {
		name, replacement string
		disable           bool
	}{
		{name: "different value", replacement: "new"},
		{name: "same value", replacement: "old"},
		{name: "disabled", disable: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := &linuxTextInput{
				config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{},
			}
			t.Cleanup(state.close)
			old := callbackTextField(text.NewEditor("old"))
			next := callbackTextField(text.NewEditor(scenario.replacement))
			next.Target, next.Width = gfx.Scope(1).At(2), 240
			old.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
				var replacement *platform.TextInput
				if !scenario.disable {
					replacement = &next
				}
				prepareCallbackFrame(t, state, platform.Frame{
					TextInput: platform.TextInputUpdate{Changed: true, Input: replacement},
				})
				// A stale acknowledgment is not valid for the replacement owner.
				stale := old.State
				stale.Selection.End = 100
				return stale, nil
			}
			prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &old}})
			frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
			if err != nil {
				t.Fatalf("superseded acknowledgment was validated: %v", err)
			}
			merged := prepareCallbackFrame(t, state, frame)
			if scenario.disable {
				if state.enabled || state.state != (platform.TextState{}) || len(merged.TextLabels) != 0 {
					t.Fatalf("disabled input was resurrected: enabled=%t state=%+v labels=%v", state.enabled, state.state, merged.TextLabels)
				}
				return
			}
			if !state.enabled || state.input.Target != next.Target || state.input.Width != 240 || state.state != next.State {
				t.Fatalf("replacement input was overwritten: input=%+v state=%+v", state.input, state.state)
			}
			if len(merged.TextLabels) != 1 || merged.TextLabels[0].Value != scenario.replacement {
				t.Fatalf("replacement visuals = %v", merged.TextLabels)
			}
		})
	}
}

func TestLinuxTextCallbackKeepsNestedEventAcknowledgment(t *testing.T) {
	editor := text.NewEditor("abc")
	state := &linuxTextInput{
		config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{},
	}
	t.Cleanup(state.close)
	field := callbackTextField(editor)
	nested := false
	field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		event.Apply(editor)
		next := platform.EditorState(editor)
		if !nested {
			nested = true
			if _, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "n"}); err != nil {
				t.Fatal(err)
			}
		}
		return next, nil
	}
	prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}})
	frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	merged := prepareCallbackFrame(t, state, frame)
	if editor.String() != "abcxn" || state.state.Value != "abcxn" || state.input.State.Value != "abcxn" ||
		len(merged.TextLabels) != 1 || merged.TextLabels[0].Value != "abcxn" {
		t.Fatalf("nested edit lost: editor=%q state=%+v labels=%v", editor.String(), state.state, merged.TextLabels)
	}
}

func TestLinuxTextCallbackAcceptsEventAcrossGeometryUpdate(t *testing.T) {
	for _, beforeEdit := range []bool{true, false} {
		t.Run(map[bool]string{true: "before edit", false: "after edit"}[beforeEdit], func(t *testing.T) {
			editor := text.NewEditor("abc")
			redraws := 0
			state := &linuxTextInput{
				config: &platform.Config{DynamicTextInput: true, OnRedraw: func() (platform.Frame, error) {
					redraws++
					return platform.Frame{}, nil
				}},
				engine: fixedAdvanceEngine{},
			}
			t.Cleanup(state.close)
			field := callbackTextField(editor)
			field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				if beforeEdit {
					geometry := state.input
					geometry.Width = 240
					prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &geometry}})
				}
				event.Apply(editor)
				if !beforeEdit {
					geometry := state.input
					geometry.Width = 240
					prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &geometry}})
				}
				return platform.EditorState(editor), nil
			}
			prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}})
			frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
			if err != nil {
				t.Fatal(err)
			}
			merged := prepareCallbackFrame(t, state, frame)
			if editor.String() != "abcx" || state.state.Value != "abcx" || state.input.Width != 240 || redraws != 1 ||
				len(merged.TextLabels) != 1 || merged.TextLabels[0].Value != "abcx" {
				t.Fatalf("geometry update lost valid edit: editor=%q state=%+v width=%v redraws=%d labels=%v", editor.String(), state.state, state.input.Width, redraws, merged.TextLabels)
			}
		})
	}
}

func TestLinuxTextCallbackDiscardsSupersededRedraw(t *testing.T) {
	for _, scenario := range []string{"replacement", "same-value replacement", "disabled", "nested edit", "same-value acknowledgment"} {
		t.Run(scenario, func(t *testing.T) {
			editor := text.NewEditor("abc")
			config := &platform.Config{DynamicTextInput: true}
			state := &linuxTextInput{config: config, engine: fixedAdvanceEngine{}}
			t.Cleanup(state.close)
			field := callbackTextField(editor)
			field.Terminal = scenario == "same-value acknowledgment"
			prepareCallbackFrame(t, state, platform.Frame{
				Update: render.Update{Count: 1, Rebuild: true}, Rectangles: []render.Rectangle{{Layer: 7}},
				TextLabels: []platform.TextLabel{{Value: "current scene"}},
				TextInput:  platform.TextInputUpdate{Changed: true, Input: &field},
			})
			redraws := 0
			config.OnRedraw = func() (platform.Frame, error) {
				redraws++
				if redraws > 1 {
					return platform.Frame{}, nil
				}
				oldInput := state.input
				switch scenario {
				case "replacement", "same-value replacement":
					value := "new"
					if scenario == "same-value replacement" {
						value = editor.String()
					}
					replacement := callbackTextField(text.NewEditor(value))
					replacement.Width = 240
					prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &replacement}})
				case "disabled":
					prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true}})
				case "nested edit":
					if _, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "n"}); err != nil {
						t.Fatal(err)
					}
				case "same-value acknowledgment":
					if _, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert}); err != nil {
						t.Fatal(err)
					}
				}
				return platform.Frame{
					Update: render.Update{Count: 1, Rebuild: true}, Rectangles: []render.Rectangle{{Layer: 99}},
					TextLabels:    []platform.TextLabel{{Value: "superseded scene"}},
					Accessibility: []platform.AccessibilityNode{{Value: "superseded input"}},
					TextInput:     platform.TextInputUpdate{Changed: true, Input: &oldInput},
					NativeViews:   []platform.NativeView{{}},
				}, nil
			}
			frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
			if err != nil {
				t.Fatal(err)
			}
			if !frame.Update.Empty() || frame.Rectangles != nil || frame.TextLabels != nil || frame.Accessibility != nil || frame.NativeViews != nil {
				t.Fatalf("superseded application scene survived: %+v", frame)
			}
			prepareCallbackFrame(t, state, frame)
			if state.baseRectangles[0].Layer != 7 || state.baseLabels[0].Value != "current scene" {
				t.Fatalf("retained scene overwritten: rectangles=%v labels=%v", state.baseRectangles, state.baseLabels)
			}
			switch scenario {
			case "replacement":
				if state.state.Value != "new" || state.input.Width != 240 {
					t.Fatalf("replacement overwritten: %+v", state.input)
				}
			case "same-value replacement":
				if state.state.Value != "abcx" || state.input.Width != 240 {
					t.Fatalf("same-value input overwritten: %+v", state.input)
				}
			case "disabled":
				if state.enabled {
					t.Fatal("disabled input resurrected")
				}
			case "nested edit":
				if state.state.Value != "abcxn" {
					t.Fatalf("nested edit overwritten: %+v", state.state)
				}
			case "same-value acknowledgment":
				if !state.input.Terminal || redraws != 1 {
					t.Fatalf("terminal acknowledgment was overwritten or redrawn: terminal=%t redraws=%d", state.input.Terminal, redraws)
				}
			}
		})
	}
}

func TestLinuxTextCallbackRejectsEventAfterInputModeChange(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*platform.TextInput)
	}{
		{"secure", func(input *platform.TextInput) { input.Secure = true }},
		{"terminal", func(input *platform.TextInput) { input.Terminal = true }},
		{"placed", func(input *platform.TextInput) { input.Placed = true }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{}}
			t.Cleanup(state.close)
			field := callbackTextField(editor)
			field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				next := state.input
				scenario.change(&next)
				prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &next}})
				event.Apply(editor)
				return platform.EditorState(editor), nil
			}
			prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}})
			frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
			if err != nil {
				t.Fatal(err)
			}
			prepareCallbackFrame(t, state, frame)
			if editor.String() != "abcx" || state.state.Value != "abc" || state.input.State.Value != "abc" {
				t.Fatalf("old-mode acknowledgment crossed into new mode: editor=%q state=%+v", editor.String(), state.state)
			}
		})
	}
}

func TestLinuxTextCallbackCannotReviveClosedInput(t *testing.T) {
	for _, duringRedraw := range []bool{false, true} {
		t.Run(map[bool]string{true: "during redraw", false: "during event"}[duringRedraw], func(t *testing.T) {
			editor := text.NewEditor("abc")
			state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{}}
			state.postCaret = func(uint32) {}
			t.Cleanup(state.close)
			field := callbackTextField(editor)
			if !duringRedraw {
				field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
					event.Apply(editor)
					state.close()
					return platform.EditorState(editor), nil
				}
			}
			redraws := 0
			state.config.OnRedraw = func() (platform.Frame, error) {
				redraws++
				state.close()
				return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil
			}
			prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}})
			if state.caretTimer == nil {
				t.Fatal("fixture did not arm the caret timer")
			}
			frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
			if err != nil {
				t.Fatal(err)
			}
			if state.enabled || state.state != (platform.TextState{}) || state.caretTimer != nil || frame.TextInput.Changed ||
				redraws != map[bool]int{true: 1, false: 0}[duringRedraw] {
				t.Fatalf("closed input revived: enabled=%t state=%+v timer=%v redraws=%d frame=%+v", state.enabled, state.state, state.caretTimer, redraws, frame)
			}
			if _, changed, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil || changed || state.enabled {
				t.Fatalf("late frame revived closed input: changed=%t enabled=%t err=%v", changed, state.enabled, err)
			}
			if frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "late"}); err != nil || frame.TextInput.Changed || editor.String() != "abcx" {
				t.Fatalf("closed input accepted a late event: editor=%q frame=%+v err=%v", editor.String(), frame, err)
			}
		})
	}
}

func TestLinuxTextCallbackPreservesErrorsAfterReentry(t *testing.T) {
	for _, duringRedraw := range []bool{false, true} {
		t.Run(map[bool]string{true: "redraw error", false: "event error"}[duringRedraw], func(t *testing.T) {
			sentinel := errors.New("callback failed")
			state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{}}
			t.Cleanup(state.close)
			field := callbackTextField(text.NewEditor("abc"))
			replacement := callbackTextField(text.NewEditor("new"))
			replacement.Target = gfx.Scope(1).At(2)
			replace := func() {
				prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &replacement}})
			}
			if duringRedraw {
				state.config.OnRedraw = func() (platform.Frame, error) {
					replace()
					return platform.Frame{}, sentinel
				}
			} else {
				field.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
					replace()
					return field.State, sentinel
				}
			}
			prepareCallbackFrame(t, state, platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}})
			if _, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"}); err != sentinel || state.state.Value != "new" || state.input.Target != replacement.Target {
				t.Fatalf("callback error or newer input lost: err=%v state=%+v input=%+v", err, state.state, state.input)
			}
		})
	}
}
