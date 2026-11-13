package linux

import (
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
				}, nil
			}
			frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
			if err != nil {
				t.Fatal(err)
			}
			if !frame.Update.Empty() || frame.Rectangles != nil || frame.TextLabels != nil || frame.Accessibility != nil {
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
