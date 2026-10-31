package windows

import (
	"errors"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestValidateTextInput(t *testing.T) {
	valid := &platform.TextInput{
		Width: 200, Height: 32, FontSize: 14,
		State:   platform.TextState{Value: "中文", Selection: text.Range{Start: 2, End: 2}},
		OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
	}
	if err := validateTextInput(valid); err != nil {
		t.Fatal(err)
	}
	invalid := *valid
	invalid.State.Selection = text.Range{Start: 0, End: 3}
	if err := validateTextInput(&invalid); err == nil {
		t.Fatal("out-of-range text selection was accepted")
	}
}

func TestTextPointerEventPlacesCaretAtNearestCharacter(t *testing.T) {
	state := windowState{
		textEnabled: true,
		textInput: platform.TextInput{
			X: 10, Y: 20, Width: 100, Height: 30, FontSize: 16,
		},
		textState:  platform.TextState{Value: "abcd", Selection: text.Range{Start: 4, End: 4}},
		textEngine: fixedAdvanceEngine{advance: 10},
	}
	event, handled, err := state.textPointerEvent(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 29, Y: 35})
	if err != nil || !handled || event.Kind != platform.TextSetSelection || event.Selection != (text.Range{Start: 2, End: 2}) {
		t.Fatalf("text pointer = event:%+v handled:%v err:%v", event, handled, err)
	}
	if _, handled, err = state.textPointerEvent(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 111, Y: 35}); err != nil || handled {
		t.Fatalf("outside pointer handled=%v err=%v", handled, err)
	}
}

func TestInitializeTextLabel(t *testing.T) {
	input := platform.TextInput{
		X: 4, Y: 5, Width: 120, Height: 30, FontSize: 16, Layer: 600, Color: platform.Color{Alpha: 1},
		State: platform.TextState{Value: "输入", Selection: text.Range{Start: 1, End: 2}, Composition: text.Range{Start: 0, End: 1}, Composing: true},
	}
	state := windowState{
		textInput: input, textState: input.State, textEnabled: true, caretVisible: true,
		baseTextLabelCount: 1,
	}
	state.textEngine = fixedAdvanceEngine{advance: 8}
	state.textLabels = []platform.TextLabel{{Value: "prefix"}, {}}
	state.textRectangles = make([]render.Rectangle, textRectangleCount)
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	if len(state.textLabels) != 2 {
		t.Fatalf("text label count = %d", len(state.textLabels))
	}
	label := state.textLabels[1]
	if label.Value != "输入" || label.X != 4 || label.Y != 5 || label.Width != 120 || label.FontSize != 16 || label.Layer != 600 {
		t.Fatalf("text input label = %+v", label)
	}
	if caret := state.textRectangles[1]; caret.Bounds.X != 20 || caret.Bounds.Width != textCaretWidth {
		t.Fatalf("caret rectangle = %+v", caret)
	}
	for index, rectangle := range state.textRectangles {
		if rectangle.Bounds.Width != 0 && rectangle.Layer != 600 {
			t.Fatalf("text rectangle %d layer = %d, want 600", index, rectangle.Layer)
		}
	}
}

// TestTerminalPreeditDrawsNoCaretOrSelection guards the contract in
// platform.TextInput.Terminal: a terminal input is a preedit surface, and the
// terminal draws its own cursor and selection. A native caret here would blink
// in the same cell as the terminal's cursor.
func TestTerminalPreeditDrawsNoCaretOrSelection(t *testing.T) {
	state := dynamicTextState(0, 0)
	state.textRectangles = make([]render.Rectangle, textRectangleCount)
	state.textLabels = make([]platform.TextLabel, 1)
	field := testTextInput("ni", 4)
	field.Terminal = true
	field.State.Composing = true
	field.State.Composition = text.Range{End: 2}
	state.textInput, state.textState, state.textEnabled = field, field.State, true
	state.caretVisible = true
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	if label := state.textLabels[0]; label.Value != "ni" {
		t.Fatalf("composition label = %q, want %q", label.Value, "ni")
	}
	for index, name := range []string{"selection", "caret"} {
		if rectangle := state.textRectangles[index]; rectangle.Bounds.Width != 0 || rectangle.Bounds.Height != 0 {
			t.Fatalf("terminal %s rectangle = %+v, want none", name, rectangle)
		}
	}
}

func TestSelfPaintingTextInputCanHideNativeCaretAndSelection(t *testing.T) {
	state := dynamicTextState(0, 0)
	state.textRectangles = make([]render.Rectangle, textRectangleCount)
	state.textLabels = make([]platform.TextLabel, 1)
	field := testTextInput("selected text", 4)
	field.HideNativeOverlays = true
	field.State.Selection = text.Range{Start: 2, End: 7}
	field.State.Composition = text.Range{Start: 2, End: 7}
	field.State.Composing = true
	state.textInput, state.textState, state.textEnabled = field, field.State, true
	state.caretVisible, state.focused = true, true
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	for index, name := range []string{"selection", "caret"} {
		if rectangle := state.textRectangles[index]; rectangle.Bounds.Width != 0 || rectangle.Bounds.Height != 0 {
			t.Fatalf("%s rectangle = %+v, want none", name, rectangle)
		}
	}
	if underline := state.textRectangles[2]; underline.Bounds.Width <= 0 {
		t.Fatalf("composition underline = %+v, want native IME feedback", underline)
	}
	if state.caretBlinks() {
		t.Fatal("a self-painting field still requested a native blinking caret")
	}
}

// TestTerminalPreeditStandsOnTheTerminalGrid guards the composition drawn in the
// fixed-pitch font on the terminal's cells: the label is a grid line, and the
// underline covers the cells of the composed characters (a wide one takes two),
// not the width the system UI font gives them.
func TestTerminalPreeditStandsOnTheTerminalGrid(t *testing.T) {
	state := dynamicTextState(0, 0)
	state.textRectangles = make([]render.Rectangle, textRectangleCount)
	state.textLabels = make([]platform.TextLabel, 1)
	field := testTextInput("ni好", 4)
	field.Terminal = true
	field.MonoFace = ui.MonoClassic
	field.State.Composing = true
	field.State.Composition = text.Range{End: 3}
	state.textInput, state.textState, state.textEnabled = field, field.State, true
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	cell := terminalCellWidth(field.FontSize)
	label := state.textLabels[0]
	if !label.Mono || label.MonoFace != ui.MonoClassic || label.CellWidth != cell {
		t.Fatalf("label mono %v face %v cell %v, want a grid line of %v on the terminal face", label.Mono, label.MonoFace, label.CellWidth, cell)
	}
	if len(label.Runs) != 2 || label.Runs[0].Cells != 2 || label.Runs[1].Column != 2 || label.Runs[1].Cells != 2 {
		t.Fatalf("runs = %+v, want the letters on cells 0-1 and the wide character on cells 2-3", label.Runs)
	}
	if underline := state.textRectangles[2].Bounds; underline.X != field.X || underline.Width != 4*cell {
		t.Fatalf("underline = %+v, want %v wide from the field's left", underline, 4*cell)
	}
}

// TestTerminalPreeditDoesNotBlinkANativeCaret guards the other half of the same
// contract: a terminal input never starts the caret blink timer, which would
// repaint the window every blink for a caret that is not drawn.
func TestTerminalPreeditDoesNotBlinkANativeCaret(t *testing.T) {
	for _, test := range []struct {
		name     string
		terminal bool
		focused  bool
		enabled  bool
		want     bool
	}{
		{"focused field", false, true, true, true},
		{"unfocused field", false, false, true, false},
		{"disabled field", false, true, false, false},
		{"focused terminal", true, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := windowState{textEnabled: test.enabled, focused: test.focused, textInput: platform.TextInput{Terminal: test.terminal}}
			if got := state.caretBlinks(); got != test.want {
				t.Fatalf("caretBlinks() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTextCommandPreservesSingleLineFocusKeys(t *testing.T) {
	state := windowState{textInput: platform.TextInput{}}
	if _, ok := state.textCommand(9); ok {
		t.Fatal("Tab was consumed by the native text editor")
	}
	if _, ok := state.textCommand(13); ok {
		t.Fatal("Enter was consumed by a single-line text editor")
	}
	state.textInput.Multiline = true
	if command, ok := state.textCommand(13); !ok || command != platform.TextInsertNewline {
		t.Fatalf("multiline Enter = %v, %v", command, ok)
	}
	if command, ok := state.textCommandWith(13, true); !ok || command != platform.TextInsertLineBreak {
		t.Fatalf("multiline Shift+Enter = %v, %v", command, ok)
	}
	if command, ok := state.textCommandWith(13, false); !ok || command != platform.TextInsertNewline {
		t.Fatalf("multiline Enter = %v, %v", command, ok)
	}
	state.textInput.Multiline = false
	if _, ok := state.textCommandWith(13, true); ok {
		t.Fatal("Shift+Enter was consumed by a single-line text editor")
	}
	state.textInput.Multiline = true
	if command, ok := state.textCommand(8); !ok || command != platform.TextDeleteBackward {
		t.Fatalf("Backspace = %v, %v", command, ok)
	}
}

func TestDecodeUTF16Character(t *testing.T) {
	if value, pending, ready := decodeUTF16Character(0, '中'); !ready || pending != 0 || value != "中" {
		t.Fatalf("BMP decode = %q, %#x, %v", value, pending, ready)
	}
	_, pending, ready := decodeUTF16Character(0, 0xd83d)
	if ready || pending != 0xd83d {
		t.Fatalf("high surrogate = %#x, %v", pending, ready)
	}
	if value, pending, ready := decodeUTF16Character(pending, 0xde00); !ready || pending != 0 || value != "😀" {
		t.Fatalf("surrogate decode = %q, %#x, %v", value, pending, ready)
	}
}

func TestDynamicTextInputChangedSemantics(t *testing.T) {
	state := dynamicTextState(2, 1)
	ignored := testTextInput("ignored", 12)
	frame, dirty, err := state.prepareFrame(platform.Frame{
		TextInput: platform.TextInputUpdate{Input: &ignored},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if dirty || state.textEnabled || !frame.Update.Empty() || frame.TextLabels != nil {
		t.Fatal("Changed=false modified the native editor")
	}

	first := testTextInput("first", 20)
	frame, dirty, err = state.prepareFrame(platform.Frame{
		TextInput: platform.TextInputUpdate{Changed: true, Input: &first},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || !state.textEnabled || !frame.Update.Rebuild {
		t.Fatalf("enable result = dirty:%v enabled:%v update:%+v", dirty, state.textEnabled, frame.Update)
	}
	if len(frame.Rectangles) != 2+textRectangleCount || len(frame.TextLabels) != 2 {
		t.Fatalf("enabled lengths = %d rectangles, %d labels", len(frame.Rectangles), len(frame.TextLabels))
	}
	if label := frame.TextLabels[1]; label.Value != "first" || label.X != 20 || label.FontSize != 14 {
		t.Fatalf("enabled native label = %+v", label)
	}

	secondCalls := 0
	second := testTextInput("second", 40)
	second.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		secondCalls++
		return second.State, nil
	}
	frame, dirty, err = state.prepareFrame(platform.Frame{
		TextInput: platform.TextInputUpdate{Changed: true, Input: &second},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || frame.Update.Rebuild || frame.Update.First != 2 || frame.Update.Count != textRectangleCount {
		t.Fatalf("replace update = %+v", frame.Update)
	}
	state.textInput.OnEvent(platform.TextEvent{})
	if secondCalls != 1 || state.textInput.X != 40 || state.textState.Value != "second" {
		t.Fatal("replacement geometry, state, or handler was not installed")
	}

	state.caretTimer = 9
	frame, dirty, err = state.prepareFrame(platform.Frame{
		TextInput: platform.TextInputUpdate{Changed: true},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || state.textEnabled || state.caretTimer != 0 || !frame.Update.Rebuild || len(frame.Rectangles) != 2 || len(frame.TextLabels) != 1 {
		t.Fatalf("disable result = dirty:%v enabled:%v update:%+v lengths:%d/%d", dirty, state.textEnabled, frame.Update, len(frame.Rectangles), len(frame.TextLabels))
	}
	for _, rectangle := range state.textRectangles[len(state.textRectangles):cap(state.textRectangles)] {
		if rectangle != (render.Rectangle{}) {
			t.Fatal("disabled native rectangle remained in backing storage")
		}
	}
	for _, label := range state.textLabels[len(state.textLabels):cap(state.textLabels)] {
		if label.Value != "" || label.Width != 0 || label.Runs != nil {
			t.Fatal("disabled native label remained in backing storage")
		}
	}
}

func TestDynamicTextInputMergesCanvasAndNativeRanges(t *testing.T) {
	state := dynamicTextState(4, 0)
	first := testTextInput("one", 10)
	if _, _, err := state.prepareFrame(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &first}}, false); err != nil {
		t.Fatal(err)
	}
	second := testTextInput("two", 30)
	canvas := make([]render.Rectangle, 4)
	canvas[1].Radius = 7
	frame, _, err := state.prepareFrame(platform.Frame{
		Update:     render.Update{First: 1, Count: 1},
		Rectangles: canvas,
		TextInput:  platform.TextInputUpdate{Changed: true, Input: &second},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Update.Rebuild || frame.Update.First != 1 || frame.Update.Count != len(canvas)+textRectangleCount-1 {
		t.Fatalf("merged update = %+v", frame.Update)
	}
	if frame.Rectangles[1].Radius != 7 || len(frame.Rectangles) != len(canvas)+textRectangleCount {
		t.Fatal("canvas prefix or native tail was lost")
	}
}

func TestDynamicTextInputSwitchClearsComposition(t *testing.T) {
	state := dynamicTextState(0, 0)
	first := testTextInput("marked", 0)
	first.State.Composing = true
	first.State.Composition = text.Range{Start: 0, End: 6}
	if _, _, err := state.prepareFrame(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &first}}, false); err != nil {
		t.Fatal(err)
	}
	state.imeComposing = true
	second := testTextInput("plain", 0)
	if _, _, err := state.prepareFrame(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &second}}, false); err != nil {
		t.Fatal(err)
	}
	if state.imeComposing || state.textState.Composing || state.textState.Value != "plain" {
		t.Fatalf("replacement composition state = %+v, ime=%v", state.textState, state.imeComposing)
	}
}

func TestCompositionSurvivesCaretAndLayoutUpdates(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 2.5, 3} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			state := dynamicTextState(0, 0)
			state.scale = scale
			field := testTextInput("", 0)
			field.Placed, field.Multiline = true, true
			field.CaretHeight = 20
			editor := text.NewEditor("")
			field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				event.Apply(editor)
				composition, composing := editor.Composition()
				return platform.TextState{Value: editor.String(), Selection: editor.Selection(), Composition: composition, Composing: composing}, nil
			}
			if _, _, err := state.prepareFrame(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, false); err != nil {
				t.Fatal(err)
			}
			state.config.OnRedraw = func() (platform.Frame, error) {
				next := state.textInput
				next.CaretX += 10
				next.CaretY += 20 // a wrapped line also changes the candidate anchor
				next.Height += 20
				return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &next}}, nil
			}
			for _, value := range []string{"n", "ni", "nih", "nihao"} {
				state.imeComposing, state.imeHasTarget = true, true
				state.imeTarget = text.Range{End: 1}
				frame, changed, err := state.textEventFrame(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: text.Range{Start: len(value), End: len(value)}})
				if err != nil || !changed {
					t.Fatalf("preedit: changed=%v err=%v", changed, err)
				}
				if _, _, err := state.prepareFrame(frame, true); err != nil {
					t.Fatal(err)
				}
				if !state.imeComposing || !state.imeHasTarget || !state.textState.Composing || state.textState.Value != value {
					t.Fatalf("caret update cancelled %q: ime=%v target=%v text=%+v", value, state.imeComposing, state.imeHasTarget, state.textState)
				}
			}
			state.imeComposing, state.imeHasTarget = false, false
			frame, _, err := state.textEventFrame(platform.TextEvent{Kind: platform.TextInsert, Text: "你好"})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := state.prepareFrame(frame, true); err != nil {
				t.Fatal(err)
			}
			if state.textState.Value != "你好" || state.textState.Composing {
				t.Fatalf("commit = %+v", state.textState)
			}
		})
	}
}

func TestTextEventRefreshesCanvasOnceAfterStateUpdate(t *testing.T) {
	handlerCalls, redrawCalls := 0, 0
	state := dynamicTextState(0, 0)
	state.textEnabled = true
	state.caretVisible = true
	state.textInput = testTextInput("old", 0)
	state.textState = state.textInput.State
	state.textInput.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		handlerCalls++
		return platform.TextState{Value: "next", Selection: text.Range{Start: 4, End: 4}}, nil
	}
	state.config.OnRedraw = func() (platform.Frame, error) {
		redrawCalls++
		if state.textState.Value != "next" || state.textInput.State.Value != "next" {
			t.Fatal("OnRedraw ran before native text state was installed")
		}
		return platform.Frame{Accessibility: []platform.AccessibilityNode{{Value: "next"}}}, nil
	}
	frame, changed, err := state.textEventFrame(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !changed || handlerCalls != 1 || redrawCalls != 1 || len(frame.Accessibility) != 1 || frame.Accessibility[0].Value != "next" {
		t.Fatalf("event callbacks = changed:%v handler:%d redraw:%d frame:%+v", changed, handlerCalls, redrawCalls, frame)
	}

	state.textInput.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		return platform.TextState{}, errors.New("rejected")
	}
	if _, _, err := state.textEventFrame(platform.TextEvent{}); err == nil {
		t.Fatal("text handler error was accepted")
	}
	if redrawCalls != 1 {
		t.Fatal("failed text update triggered Canvas redraw")
	}
}

func TestCompositionUpdateLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name     string
		update   func(*platform.TextInput) *platform.TextInput
		preserve bool
	}{
		{"move", func(next *platform.TextInput) *platform.TextInput { next.X++; next.CaretY++; return next }, true},
		{"style", func(next *platform.TextInput) *platform.TextInput { next.FontSize++; next.Color.Red = 1; return next }, true},
		{"target", func(next *platform.TextInput) *platform.TextInput { next.Target = ui.Item(2); return next }, false},
		{"disable", func(next *platform.TextInput) *platform.TextInput { return nil }, false},
		{"value", func(next *platform.TextInput) *platform.TextInput { next.State.Value = "xy"; return next }, false},
		{"selection", func(next *platform.TextInput) *platform.TextInput { next.State.Selection = text.Range{}; return next }, false},
		{"unmark", func(next *platform.TextInput) *platform.TextInput { next.State.Composing = false; return next }, false},
		{"secure", func(next *platform.TextInput) *platform.TextInput { next.Secure = true; return next }, false},
		{"terminal", func(next *platform.TextInput) *platform.TextInput { next.Terminal = true; return next }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := dynamicTextState(0, 0)
			field := testTextInput("ni", 0)
			field.Target = ui.Item(1)
			field.State.Composing = true
			field.State.Composition = text.Range{End: 2}
			state.textInput, state.textState, state.textEnabled = field, field.State, true
			state.imeComposing, state.imeHasTarget = true, true
			state.pendingHighSurrogate = 0xd83d
			if _, _, err := state.applyTextInputUpdate(platform.TextInputUpdate{Changed: true, Input: tc.update(&field)}); err != nil {
				t.Fatal(err)
			}
			if state.imeComposing != tc.preserve || state.imeHasTarget != tc.preserve || (state.pendingHighSurrogate != 0) != tc.preserve {
				t.Fatalf("preserve=%v: ime=%v target=%v surrogate=%x", tc.preserve, state.imeComposing, state.imeHasTarget, state.pendingHighSurrogate)
			}
		})
	}
}

func BenchmarkTextInputCaretUpdate(b *testing.B) {
	state := dynamicTextState(0, 0)
	field := testTextInput("nihao", 0)
	field.State.Composing = true
	field.State.Composition = text.Range{End: 5}
	state.textInput, state.textState, state.textEnabled = field, field.State, true
	state.imeComposing = true
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		field.CaretX++
		if _, _, err := state.applyTextInputUpdate(platform.TextInputUpdate{Changed: true, Input: &field}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestTerminalTextEventWithoutPreeditSkipsApplicationRedraw(t *testing.T) {
	redraws := 0
	state := dynamicTextState(0, 0)
	state.textEnabled = true
	state.textInput = testTextInput("", 0)
	state.textInput.Terminal = true
	state.textInput.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		return platform.TextState{}, nil
	}
	state.config.OnRedraw = func() (platform.Frame, error) {
		redraws++
		return platform.Frame{}, nil
	}
	frame, changed, err := state.textEventFrame(platform.TextEvent{Kind: platform.TextInsert, Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if changed || redraws != 0 || frame.TextInput.Changed || frame.Rectangles != nil || frame.TextLabels != nil || frame.Accessibility != nil {
		t.Fatalf("terminal text = changed:%v redraws:%d frame:%+v", changed, redraws, frame)
	}
}

func TestDynamicTextInputUpdateRequiresOptIn(t *testing.T) {
	state := dynamicTextState(0, 0)
	state.config.DynamicTextInput = false
	input := testTextInput("value", 0)
	if _, _, err := state.prepareFrame(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &input}}, false); err == nil {
		t.Fatal("dynamic update was accepted without DynamicTextInput")
	}
}

func dynamicTextState(rectangleCount, labelCount int) *windowState {
	state := &windowState{
		config:             platform.Config{DynamicTextInput: true},
		baseRectangleCount: rectangleCount,
		baseTextLabelCount: labelCount,
		textRectangles:     make([]render.Rectangle, rectangleCount),
		textLabels:         make([]platform.TextLabel, labelCount),
		textEngine:         fixedAdvanceEngine{advance: 8},
	}
	return state
}

func testTextInput(value string, x float32) platform.TextInput {
	count := len([]rune(value))
	return platform.TextInput{
		X: x, Width: 120, Height: 30, FontSize: 14,
		Color: platform.Color{Alpha: 1},
		State: platform.TextState{Value: value, Selection: text.Range{Start: count, End: count}},
		OnEvent: func(platform.TextEvent) (platform.TextState, error) {
			return platform.TextState{Value: value, Selection: text.Range{Start: count, End: count}}, nil
		},
	}
}

// fixedAdvanceEngine lays every character out the same width.
type fixedAdvanceEngine struct{ advance float32 }

func (engine fixedAdvanceEngine) Width(value string, _, _ float32) float32 {
	return float32(utf8.RuneCountInString(value)) * engine.advance
}

func (engine fixedAdvanceEngine) Caret(value string, _, _ float32, offset int) float32 {
	return engine.Width(value[:max(0, min(offset, len(value)))], 0, 0)
}

func (engine fixedAdvanceEngine) Selection(dst []ui.TextSpan, value string, _, _ float32, start, end int) []ui.TextSpan {
	left, right := engine.Caret(value, 0, 0, start), engine.Caret(value, 0, 0, end)
	return append(dst, ui.TextSpan{X: min(left, right), Width: max(left, right) - min(left, right)})
}

func (engine fixedAdvanceEngine) OffsetAt(value string, _, _, x float32) int {
	best, distance := 0, float32(1e9)
	for offset := range value {
		if d := max(x-engine.Caret(value, 0, 0, offset), engine.Caret(value, 0, 0, offset)-x); d < distance {
			best, distance = offset, d
		}
	}
	if d := max(x-engine.Width(value, 0, 0), engine.Width(value, 0, 0)-x); d < distance {
		best = len(value)
	}
	return best
}
