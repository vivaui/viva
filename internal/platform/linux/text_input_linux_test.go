package linux

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

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

func TestLinuxSelfPaintingInputHidesSelectionAndCaretButKeepsComposition(t *testing.T) {
	state := &linuxTextInput{
		config: &platform.Config{DynamicTextInput: true},
		engine: fixedAdvanceEngine{advance: 10},
	}
	inputState := platform.TextInput{
		X: 10, Y: 20, Width: 120, Height: 28, FontSize: 16,
		HideNativeOverlays: true,
		Color:              platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State: platform.TextState{
			Value: "selected", Selection: text.Range{Start: 2, End: 7},
			Composing: true, Composition: text.Range{Start: 2, End: 7},
		},
		OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
	}
	frame, changed, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}})
	if err != nil || !changed {
		t.Fatalf("prepare: changed=%t err=%v", changed, err)
	}
	base := len(state.baseRectangles)
	if frame.Rectangles[base] != (render.Rectangle{}) || frame.Rectangles[base+1] != (render.Rectangle{}) {
		t.Fatalf("native selection/caret = %+v / %+v, want none", frame.Rectangles[base], frame.Rectangles[base+1])
	}
	if frame.Rectangles[base+2].Bounds.Width <= 0 {
		t.Fatalf("composition underline = %+v, want native IME feedback", frame.Rectangles[base+2])
	}
	if state.caretShown() {
		t.Fatal("self-painting input still requested a native caret")
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

func TestLinuxTextInputAlignmentMatchesGlyphCaretUnderlineAndPointer(t *testing.T) {
	for _, testcase := range []struct {
		alignment ui.Alignment
		origin    float32
	}{
		{ui.AlignCenter, 55},
		{ui.AlignRight, 100},
	} {
		t.Run(fmt.Sprint(testcase.alignment), func(t *testing.T) {
			editor := text.NewEditor("abc")
			inputState := platform.TextInput{
				X: 10, Y: 20, Width: 120, Height: 30, FontSize: 16, Alignment: testcase.alignment,
				State: platform.TextState{Value: "abc", Selection: text.Range{Start: 3, End: 3}, Composing: true, Composition: text.Range{Start: 0, End: 3}},
				OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
					event.Apply(editor)
					return platform.TextState{Value: editor.String(), Selection: editor.Selection()}, nil
				},
			}
			state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{advance: 10}}
			frame, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}})
			if err != nil {
				t.Fatal(err)
			}
			if label := frame.TextLabels[len(frame.TextLabels)-1]; label.X != testcase.origin {
				t.Fatalf("glyph origin = %v, want %v", label.X, testcase.origin)
			}
			wantCaret := platform.TextCaretInsideField(testcase.alignment, inputState.X, inputState.Width, testcase.origin+30, linuxTextCaretWidth)
			if caret := state.caret.Bounds.X; caret != wantCaret {
				t.Fatalf("caret = %v, want %v", caret, wantCaret)
			}
			if underline := frame.Rectangles[len(frame.Rectangles)-1].Bounds.X; underline != testcase.origin {
				t.Fatalf("composition underline = %v, want %v", underline, testcase.origin)
			}
			state.state.Composing = false
			state.input.State.Composing = false
			_, handled, err := state.pointer(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: testcase.origin + 12, Y: 35})
			if err != nil || !handled || editor.Caret() != 1 {
				t.Fatalf("aligned click: caret=%d handled=%v err=%v", editor.Caret(), handled, err)
			}
		})
	}
}

func TestLinuxRightToLeftAlignmentUsesLineWidthNotLogicalEndCaret(t *testing.T) {
	const value = "אבג"
	engine := ui.ReferenceTextEngine()
	width := engine.Width(value, 20, 0)
	if caret := engine.Caret(value, 20, 0, len(value)); caret >= width/2 {
		t.Fatalf("fixture is not an RTL end-caret: caret=%v width=%v", caret, width)
	}
	for _, alignment := range []ui.Alignment{ui.AlignCenter, ui.AlignRight} {
		inputState := platform.TextInput{
			X: 10, Y: 20, Width: 120, Height: 32, FontSize: 20, Alignment: alignment,
			State:   platform.TextState{Value: value, Selection: text.Range{Start: 3, End: 3}},
			OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
		}
		state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: engine}
		frame, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}})
		if err != nil {
			t.Fatal(err)
		}
		want := inputState.X + platform.TextAlignmentInset(alignment, inputState.Width, width, false, false)
		if got := frame.TextLabels[len(frame.TextLabels)-1].X; got < want-0.1 || got > want+0.1 {
			t.Fatalf("RTL glyph origin = %v, want %v (line width %v)", got, want, width)
		}
	}
}

func TestLinuxTextInputMergesVisualsAndAppliesKeys(t *testing.T) {
	editor := text.NewEditor("A")
	inputState := platform.TextInput{
		X: 10, Y: 20, Width: 120, Height: 28, FontSize: 16, Weight: ui.WeightSemibold, Layer: 600,
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
	if len(merged.Rectangles) != 2 || len(merged.TextLabels) != 1 || merged.TextLabels[0].Value != "A" || merged.TextLabels[0].Weight != ui.WeightSemibold {
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

// A terminal's composition stands on the terminal's cells, as its grid draws the text once it is
// committed: with the font's own advances a wide character would be narrower than its two cells
// and the text would jump.
func TestLinuxTerminalPreeditIsLaidOnTheCellsAndUnderlined(t *testing.T) {
	state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{}}
	input := platform.TextInput{
		X: 20, Y: 40, Width: 300, Height: 16, FontSize: 13, Terminal: true, CellWidth: 7.5,
		Color:   platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State:   platform.TextState{Value: "ni你好", Composing: true, Composition: text.Range{End: 4}},
		OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
	}
	merged, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &input}})
	if err != nil {
		t.Fatal(err)
	}
	label := merged.TextLabels[len(merged.TextLabels)-1]
	cell := input.CellWidth
	if !label.Mono || label.CellWidth != cell {
		t.Fatalf("label = mono:%v cell:%v, want the terminal's cell %v", label.Mono, label.CellWidth, cell)
	}
	want := []ui.TextRun{
		{Column: 0, Bytes: 2, Cells: 2, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{Column: 2, Bytes: 3, Cells: 2, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{Column: 4, Bytes: 3, Cells: 2, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
	}
	if !reflect.DeepEqual(label.Runs, want) {
		t.Fatalf("runs = %+v, want %+v", label.Runs, want)
	}
	underline := merged.Rectangles[len(merged.Rectangles)-1]
	if underline.Bounds.X != 20 || underline.Bounds.Width != 6*cell || underline.Bounds.Height <= 0 {
		t.Fatalf("underline = %+v, want it across the six cells of the composition", underline.Bounds)
	}
}

func newEditableLinuxInput(t *testing.T, value string, width float32) (*linuxTextInput, *text.Editor) {
	t.Helper()
	editor := text.NewEditor(value)
	state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{advance: 10}}
	inputState := platform.TextInput{
		X: 20, Y: 10, Width: width, Height: 28, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State: platform.TextState{Value: value, Selection: editor.Selection()},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(editor)
			return platform.TextState{Value: editor.String(), Selection: editor.Selection()}, nil
		},
	}
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	return state, editor
}

// Dragging across a field selects from where the press landed to where the pointer is, also
// when it has left the field; word and line selection come from repeated presses; nothing
// moves once the button is up.
func TestLinuxTextFieldSelectsWithThePointer(t *testing.T) {
	state, editor := newEditableLinuxInput(t, "hello brave world", 400)
	pointer := func(kind platform.PointerEventKind, x float32, clicks int) {
		t.Helper()
		if _, handled, err := state.pointer(platform.PointerEvent{Kind: kind, Button: ui.PointerButtonPrimary, X: x, Y: 20, ClickCount: clicks}); err != nil {
			t.Fatal(err)
		} else if kind == platform.PointerDown && !handled {
			t.Fatal("a press in the field was not taken")
		}
	}
	pointer(platform.PointerDown, 20+20, 1)
	pointer(platform.PointerMove, 20+80, 1)
	if got := editor.Selection(); got != (text.Range{Start: 2, End: 8}) {
		t.Fatalf("drag selection = %+v", got)
	}
	pointer(platform.PointerMove, 900, 1) // far past the field's right edge
	if got := editor.Selection(); got != (text.Range{Start: 2, End: 17}) {
		t.Fatalf("drag past the field = %+v, want to the end of the text", got)
	}
	pointer(platform.PointerUp, 900, 1)
	pointer(platform.PointerMove, 20, 1)
	if got := editor.Selection(); got != (text.Range{Start: 2, End: 17}) {
		t.Fatalf("a move after release changed the selection to %+v", got)
	}
	pointer(platform.PointerDown, 20+80, 2)
	if got := editor.Selection(); got != (text.Range{Start: 6, End: 11}) {
		t.Fatalf("double press selected %+v, want the word", got)
	}
	pointer(platform.PointerUp, 20+80, 2)
	pointer(platform.PointerDown, 20+80, 3)
	if got := editor.Selection(); got != (text.Range{Start: 0, End: 17}) {
		t.Fatalf("triple press selected %+v, want everything", got)
	}
}

// A field takes the editing chords itself, ahead of the application's shortcuts.
func TestLinuxTextFieldTakesEditingChords(t *testing.T) {
	state, editor := newEditableLinuxInput(t, "hello brave world", 400)
	selectAll := platform.KeyEvent{Modifiers: input.ModifierControl, Text: "a"}
	if !state.ownsChord(selectAll) {
		t.Fatal("Ctrl+A is left to the application's shortcuts")
	}
	if _, handled, err := state.key(selectAll); err != nil || !handled {
		t.Fatalf("Ctrl+A: handled=%v err=%v", handled, err)
	}
	if got := editor.Selection(); got != (text.Range{Start: 0, End: 17}) {
		t.Fatalf("Ctrl+A selected %+v", got)
	}
	if _, handled, err := state.key(platform.KeyEvent{Key: input.KeyHome}); err != nil || !handled {
		t.Fatalf("Home: handled=%v err=%v", handled, err)
	}
	if got := editor.Selection(); got != (text.Range{}) {
		t.Fatalf("Home left the selection at %+v", got)
	}
	state.state.Composing = true
	if state.ownsChord(selectAll) {
		t.Fatal("a field composing text kept Ctrl+A from the input method")
	}
}

// A selection longer than the field is painted only inside it, and the text shifts so the
// caret stays in view.
func TestLinuxLongTextScrollsAndKeepsTheSelectionInsideTheField(t *testing.T) {
	long := "example-text-0123456789 example-text-0123456789 example-text-0123456789"
	state, _ := newEditableLinuxInput(t, long, 120)
	if _, handled, err := state.key(platform.KeyEvent{Modifiers: input.ModifierControl, Text: "a"}); err != nil || !handled {
		t.Fatalf("select all: handled=%v err=%v", handled, err)
	}
	if err := state.rebuild(); err != nil {
		t.Fatal(err)
	}
	field := state.input
	label := state.labels[len(state.baseLabels)]
	if !label.Clipped || label.Clip != (ui.Rect{X: field.X, Y: field.Y, Width: field.Width, Height: field.Height}) {
		t.Fatalf("the text is not clipped to the field: %+v", label)
	}
	if state.scroll <= 0 || label.X != field.X-state.scroll {
		t.Fatalf("the end of the text is out of view: scroll %v label at %v", state.scroll, label.X)
	}
	selection := state.rectangles[len(state.baseRectangles)].Bounds
	if selection.Width <= 0 || selection.X < field.X || selection.X+selection.Width > field.X+field.Width {
		t.Fatalf("the selection %+v leaves the field %v..%v", selection, field.X, field.X+field.Width)
	}
	caret := state.caret.Bounds
	if caret.X < field.X || caret.X+caret.Width > field.X+field.Width {
		t.Fatalf("the caret %+v is outside the field", caret)
	}
}

// A multi-line field's value is a document, and the engine shapes one line: the label, the
// caret and a click are the line the caret is on, counted in the document.
func TestLinuxMultilineFieldLaysOutTheCaretsLine(t *testing.T) {
	value := strings.Repeat("first line\n", 3000) + "the caret's line\n" + strings.Repeat("last line\n", 3000)
	state, editor := newEditableLinuxInput(t, value, 400)
	first := utf8.RuneCountInString(strings.Repeat("first line\n", 3000))
	editor.SetSelection(text.Range{Start: first + 4, End: first + 4})
	inputState := state.input
	inputState.State = platform.EditorState(editor)
	if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
		t.Fatal(err)
	}
	if err := state.rebuild(); err != nil {
		t.Fatal(err)
	}
	label := state.labels[len(state.baseLabels)]
	if label.Value != "the caret's line" {
		t.Fatalf("label = %q, want the caret's line", label.Value)
	}
	if caret := state.caret.Bounds; caret.X != 20+4*10 {
		t.Fatalf("caret x = %v, want the fourth character of the line", caret.X)
	}
	index, err := state.nearestTextIndex(7 * 10)
	if err != nil || index != first+7 {
		t.Fatalf("click on the seventh character = %d (err %v), want %d", index, err, first+7)
	}
}

// A field that paints its own caret still needs the engine's candidate window to
// sit where the glyphs are, not at the field's left edge.
func TestLinuxCandidateWindowFollowsAlignmentWhenNativeOverlaysAreHidden(t *testing.T) {
	for _, alignment := range []ui.Alignment{ui.AlignCenter, ui.AlignRight} {
		t.Run(fmt.Sprint(alignment), func(t *testing.T) {
			inputState := platform.TextInput{
				X: 10, Y: 20, Width: 120, Height: 30, FontSize: 16, Alignment: alignment,
				HideNativeOverlays: true,
				State:              platform.TextState{Value: "abc", Selection: text.Range{Start: 3, End: 3}},
				OnEvent:            func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
			}
			state := &linuxTextInput{config: &platform.Config{DynamicTextInput: true}, engine: fixedAdvanceEngine{advance: 10}}
			if _, _, err := state.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &inputState}}); err != nil {
				t.Fatal(err)
			}
			x, _, _, height := (&linuxIME{text: state}).caretRectangle()
			if height == 0 {
				t.Fatal("candidate rectangle fell back to the field's origin box")
			}
			if x <= inputState.X || x > inputState.X+inputState.Width {
				t.Fatalf("candidate x = %v, want inside the field and right of its left edge %v", x, inputState.X)
			}
		})
	}
}
