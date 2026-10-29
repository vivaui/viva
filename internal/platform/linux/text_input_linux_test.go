package linux

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestLinuxTextInputUsesSharedEditingCommandsForModifiedDeletes(t *testing.T) {
	var got []platform.TextCommand
	state := &linuxTextInput{
		enabled: true,
		input: platform.TextInput{OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			got = append(got, event.Command)
			return platform.TextState{}, nil
		}},
	}
	for _, test := range []struct {
		name string
		key  input.Key
		mods input.Modifiers
		want platform.TextCommand
	}{
		{"control backspace", input.KeyBackspace, input.ModifierControl, platform.TextDeleteWordBackward},
		{"control delete", input.KeyDelete, input.ModifierControl, platform.TextDeleteWordForward},
		{"option left", input.KeyLeft, input.ModifierAlt, platform.TextMoveWordLeft},
		{"option shift right", input.KeyRight, input.ModifierAlt | input.ModifierShift, platform.TextMoveWordRightAndModifySelection},
		{"command backspace", input.KeyBackspace, input.ModifierMeta, platform.TextDeleteLineStart},
		{"command delete", input.KeyDelete, input.ModifierMeta, platform.TextDeleteLineEnd},
	} {
		t.Run(test.name, func(t *testing.T) {
			got = nil
			if _, handled, err := state.key(platform.KeyEvent{Key: test.key, Modifiers: test.mods}); err != nil || !handled {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
			if !reflect.DeepEqual(got, []platform.TextCommand{test.want}) {
				t.Fatalf("commands=%v, want %v", got, []platform.TextCommand{test.want})
			}
		})
	}
}

func TestLinuxTextEventCarriesNativeStateThroughApplicationRedraw(t *testing.T) {
	editor := text.NewEditor("old")
	redraws := 0
	config := platform.Config{
		DynamicTextInput: true,
		OnRedraw: func() (platform.Frame, error) {
			redraws++
			return platform.Frame{Accessibility: []platform.AccessibilityNode{{Value: editor.String()}}}, nil
		},
	}
	state := &linuxTextInput{
		config: &config,
		engine: fixedAdvanceEngine{},
	}
	inputState := platform.TextInput{
		Width: 120, Height: 28, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State: platform.TextState{Value: "old", Selection: text.Range{Start: 3, End: 3}},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(editor)
			return platform.TextState{Value: editor.String(), Selection: editor.Selection()}, nil
		},
	}
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}

	frame, handled, err := state.key(platform.KeyEvent{Text: "x"})
	if err != nil || !handled {
		t.Fatalf("text key: handled=%v err=%v", handled, err)
	}
	if redraws != 1 || !frame.TextInput.Changed || frame.TextInput.Input == nil || frame.TextInput.Input.State.Value != "oldx" {
		t.Fatalf("redraw frame = %+v, redraws=%d", frame, redraws)
	}
	merged, dirty, err := state.prepare(frame)
	if err != nil || !dirty || merged.TextLabels[len(merged.TextLabels)-1].Value != "oldx" {
		t.Fatalf("immediate native visuals = %+v, dirty=%v err=%v", merged.TextLabels, dirty, err)
	}
}

func TestLinuxTerminalTextWithoutPreeditSkipsRedraw(t *testing.T) {
	handled := 0
	redraws := 0
	state := &linuxTextInput{
		enabled: true,
		config: &platform.Config{OnRedraw: func() (platform.Frame, error) {
			redraws++
			return platform.Frame{}, nil
		}},
		input: platform.TextInput{
			Terminal: true,
			OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
				handled++
				return platform.TextState{}, nil
			},
		},
	}
	frame, err := state.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if handled != 1 || redraws != 0 || frame.TextInput.Changed || frame.Rectangles != nil || frame.TextLabels != nil || frame.Accessibility != nil {
		t.Fatalf("terminal text = handled:%d redraws:%d frame:%+v", handled, redraws, frame)
	}
}

func TestLinuxCaretBlinkUpdatesOnlyCaretAndRejectsStaleTicks(t *testing.T) {
	config := platform.Config{DynamicTextInput: true}
	state := &linuxTextInput{
		config: &config,
		engine: fixedAdvanceEngine{},
	}
	inputState := platform.TextInput{
		Width: 120, Height: 28, FontSize: 16,
		Color:   platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State:   platform.TextState{Value: "A", Selection: text.Range{Start: 1, End: 1}},
		OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
	}
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	generation := state.caretGeneration
	frame, changed := state.blinkCaret(generation)
	caretIndex := len(state.baseRectangles) + 1
	if !changed || frame.Update != (render.Update{First: caretIndex, Count: 1}) || frame.Rectangles[caretIndex] != (render.Rectangle{}) {
		t.Fatalf("hidden caret frame = changed:%v update:%+v caret:%+v", changed, frame.Update, frame.Rectangles[caretIndex])
	}
	if _, changed = state.blinkCaret(generation - 1); changed {
		t.Fatal("stale caret tick changed the frame")
	}
	frame, changed = state.blinkCaret(generation)
	if !changed || frame.Rectangles[caretIndex] == (render.Rectangle{}) {
		t.Fatalf("visible caret frame = changed:%v caret:%+v", changed, frame.Rectangles[caretIndex])
	}
}

func TestLinuxTextPointerPlacesCaretAtNearestCharacter(t *testing.T) {
	editor := text.NewEditor("abcd")
	inputState := platform.TextInput{
		X: 10, Y: 20, Width: 100, Height: 30, FontSize: 16,
		State: platform.TextState{Value: "abcd", Selection: text.Range{Start: 4, End: 4}},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(editor)
			return platform.TextState{Value: editor.String(), Selection: editor.Selection()}, nil
		},
	}
	state := &linuxTextInput{
		config: &platform.Config{DynamicTextInput: true},
		engine: fixedAdvanceEngine{advance: 10},
	}
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	frame, handled, err := state.pointer(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 29, Y: 35})
	if err != nil || !handled {
		t.Fatalf("pointer handled=%v err=%v", handled, err)
	}
	if editor.Selection() != (text.Range{Start: 2, End: 2}) || frame.TextInput.Input == nil || frame.TextInput.Input.State.Selection != (text.Range{Start: 2, End: 2}) {
		t.Fatalf("selection=%+v frame=%+v", editor.Selection(), frame.TextInput)
	}
	if _, handled, err = state.pointer(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 111, Y: 35}); err != nil || handled {
		t.Fatalf("outside pointer handled=%v err=%v", handled, err)
	}
}

func TestLinuxTextInputMergesVisualsAndAppliesKeys(t *testing.T) {
	editor := text.NewEditor("A")
	inputState := platform.TextInput{
		X: 10, Y: 20, Width: 120, Height: 28, FontSize: 16, Layer: 600,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State: platform.TextState{Value: "A", Selection: text.Range{Start: 1, End: 1}},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(editor)
			return platform.TextState{Value: editor.String(), Selection: editor.Selection()}, nil
		},
	}
	config := platform.Config{DynamicTextInput: true}
	state := &linuxTextInput{
		config: &config,
		engine: fixedAdvanceEngine{},
	}
	merged, dirty, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}})
	if err != nil || !dirty {
		t.Fatalf("enable input: dirty=%v err=%v", dirty, err)
	}
	if len(merged.Rectangles) != 2 || len(merged.TextLabels) != 1 || merged.TextLabels[0].Value != "A" {
		t.Fatalf("input visuals = %d rectangles, %+v", len(merged.Rectangles), merged.TextLabels)
	}
	if merged.TextLabels[0].Layer != 600 || merged.Rectangles[0].Layer != 600 || merged.Rectangles[1].Layer != 600 {
		t.Fatalf("input visual layers = label %d, rectangles %+v", merged.TextLabels[0].Layer, merged.Rectangles)
	}
	frame, handled, err := state.key(platform.KeyEvent{Key: input.KeyUnknown, Text: "中"})
	if err != nil || !handled {
		t.Fatalf("text key: handled=%v err=%v", handled, err)
	}
	merged, dirty, err = state.prepare(frame)
	if err != nil || !dirty || merged.TextLabels[len(merged.TextLabels)-1].Value != "A中" {
		t.Fatalf("inserted visuals = %+v, dirty=%v err=%v", merged.TextLabels, dirty, err)
	}
	frame, handled, err = state.key(platform.KeyEvent{Key: input.KeyBackspace})
	if err != nil || !handled {
		t.Fatalf("backspace: handled=%v err=%v", handled, err)
	}
	merged, _, err = state.prepare(frame)
	if err != nil || merged.TextLabels[len(merged.TextLabels)-1].Value != "A" {
		t.Fatalf("deleted visuals = %+v err=%v", merged.TextLabels, err)
	}
}

func TestLinuxTextInputPreservesBaseScene(t *testing.T) {
	config := platform.Config{DynamicTextInput: true}
	state := &linuxTextInput{config: &config, engine: fixedAdvanceEngine{}}
	base := []render.Rectangle{{Layer: 3}}
	merged, dirty, err := state.prepare(platform.Frame{Update: render.Update{Count: 1, Rebuild: true}, Rectangles: base})
	if err != nil || !dirty || len(merged.Rectangles) != 1 || merged.Rectangles[0].Layer != 3 {
		t.Fatalf("base scene = %+v dirty=%v err=%v", merged.Rectangles, dirty, err)
	}
}

func TestLinuxTextInputPreservesPartialCanvasUpdate(t *testing.T) {
	config := platform.Config{DynamicTextInput: true}
	state := &linuxTextInput{config: &config, engine: fixedAdvanceEngine{}}
	rectangles := []render.Rectangle{{Layer: 1}, {Layer: 2}, {Layer: 3}}
	if _, _, err := state.prepare(platform.Frame{Update: render.Update{Count: 3, Rebuild: true}, Rectangles: rectangles}); err != nil {
		t.Fatal(err)
	}
	rectangles[1].Layer = 9
	merged, dirty, err := state.prepare(platform.Frame{Update: render.Update{First: 1, Count: 1}, Rectangles: rectangles})
	if err != nil || !dirty {
		t.Fatalf("partial update: dirty=%v err=%v", dirty, err)
	}
	if merged.Update != (render.Update{First: 1, Count: 1}) {
		t.Fatalf("merged update = %+v", merged.Update)
	}
	if merged.Rectangles[0].Layer != 1 || merged.Rectangles[1].Layer != 9 || merged.Rectangles[2].Layer != 3 {
		t.Fatalf("merged rectangles = %+v", merged.Rectangles)
	}
}

// In a multi-line field Enter and Shift+Enter are different commands, so an
// application can submit on one and add a line on the other.
func TestLinuxTextInputTellsEnterFromShiftEnterInAMultilineField(t *testing.T) {
	var commands []platform.TextCommand
	inputState := platform.TextInput{
		X: 10, Y: 20, Width: 120, Height: 56, FontSize: 16, Multiline: true,
		State: platform.TextState{Value: ""},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			commands = append(commands, event.Command)
			return platform.TextState{}, nil
		},
	}
	config := platform.Config{DynamicTextInput: true}
	state := &linuxTextInput{
		config: &config,
		engine: fixedAdvanceEngine{},
	}
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []platform.KeyEvent{
		{Key: input.KeyEnter},
		{Key: input.KeyEnter, Modifiers: input.ModifierShift},
	} {
		if _, handled, err := state.key(event); err != nil || !handled {
			t.Fatalf("%+v: handled=%v err=%v", event, handled, err)
		}
	}
	if len(commands) != 2 || commands[0] != platform.TextInsertNewline || commands[1] != platform.TextInsertLineBreak {
		t.Fatalf("commands = %v, want Newline then LineBreak", commands)
	}

	inputState.Multiline = false
	commands = nil
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	if _, handled, _ := state.key(platform.KeyEvent{Key: input.KeyEnter, Modifiers: input.ModifierShift}); handled || len(commands) != 0 {
		t.Fatalf("a single-line field took Shift+Enter: handled=%v commands=%v", handled, commands)
	}
}

func TestLinuxTextInputOwnsUndoRedoButNotTerminalControlZ(t *testing.T) {
	var commands []platform.TextCommand
	inputState := platform.TextInput{
		State: platform.TextState{Value: "text"},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			commands = append(commands, event.Command)
			return platform.TextState{Value: "text"}, nil
		},
	}
	state := linuxTextInput{enabled: true, config: &platform.Config{}, input: inputState, state: inputState.State}
	for _, event := range []platform.KeyEvent{
		{Modifiers: input.ModifierControl, Text: "z"},
		{Modifiers: input.ModifierControl | input.ModifierShift, Text: "Z"},
		{Modifiers: input.ModifierControl, Text: "y"},
	} {
		if _, handled, err := state.key(event); err != nil || !handled {
			t.Fatalf("%+v: handled=%v err=%v", event, handled, err)
		}
	}
	if want := []platform.TextCommand{platform.TextUndo, platform.TextRedo, platform.TextRedo}; !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %v, want %v", commands, want)
	}
	state.input.Terminal = true
	if _, handled, err := state.key(platform.KeyEvent{Modifiers: input.ModifierControl, Text: "z"}); err != nil || handled {
		t.Fatalf("terminal Control+Z handled=%v err=%v", handled, err)
	}
}

func TestLinuxCaretStaysSolidWhenTheDesktopTurnsBlinkingOff(t *testing.T) {
	config := platform.Config{DynamicTextInput: true}
	blinking := true
	state := &linuxTextInput{
		config: &config,
		engine: fixedAdvanceEngine{},
		settings: func() xsettings {
			return xsettings{hasCursorBlink: true, CursorBlink: map[bool]int32{true: 1, false: 0}[blinking]}
		},
	}
	inputState := platform.TextInput{
		Width: 120, Height: 28, FontSize: 16,
		Color:   platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State:   platform.TextState{Value: "A", Selection: text.Range{Start: 1, End: 1}},
		OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
	}
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	if _, changed := state.blinkCaret(state.caretGeneration); !changed || state.caretVisible {
		t.Fatal("a blinking caret did not hide")
	}
	blinking = false
	frame, changed := state.blinkCaret(state.caretGeneration)
	caretIndex := len(state.baseRectangles) + 1
	if !changed || !state.caretVisible || frame.Rectangles[caretIndex] == (render.Rectangle{}) {
		t.Fatal("a caret hidden when the desktop stopped the blinking did not come back")
	}
	if _, changed = state.blinkCaret(state.caretGeneration); changed {
		t.Fatal("a caret that does not blink changed")
	}
}

func TestPrimaryCandidateIsTheNewlySelectedText(t *testing.T) {
	state := func(value string, start, end int) platform.TextState {
		return platform.TextState{Value: value, Selection: text.Range{Start: start, End: end}}
	}
	for _, test := range []struct {
		name          string
		private       bool
		previous, now platform.TextState
		want          string
		ok            bool
	}{
		{"selected", false, state("hello world", 0, 0), state("hello world", 6, 11), "world", true},
		{"backwards", false, state("hello", 0, 0), state("hello", 4, 1), "ell", true},
		{"multibyte", false, state("输入法测试", 0, 0), state("输入法测试", 1, 3), "入法", true},
		{"collapsed", false, state("hello", 1, 3), state("hello", 2, 2), "", false},
		{"unchanged", false, state("hello", 1, 3), state("hello", 1, 3), "", false},
		{"password", true, state("secret", 0, 0), state("secret", 0, 6), "", false},
		{"out of range", false, state("ab", 0, 0), state("ab", 0, 9), "", false},
	} {
		got, ok := primaryCandidate(test.private, test.previous, test.now)
		if got != test.want || ok != test.ok {
			t.Errorf("%s: got %q,%v want %q,%v", test.name, got, ok, test.want, test.ok)
		}
	}
}
