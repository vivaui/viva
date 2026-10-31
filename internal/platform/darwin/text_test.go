//go:build darwin

package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestMeasureTextSupportsSystemFallback(t *testing.T) {
	for _, value := range []string{"Viva", "原生界面", "👩‍💻"} {
		metrics, err := MeasureText(value, 16, 0)
		if err != nil {
			t.Fatalf("MeasureText(%q): %v", value, err)
		}
		if metrics.Width <= 0 || metrics.Height <= 0 || metrics.Ascent <= 0 {
			t.Fatalf("metrics for %q = %+v", value, metrics)
		}
	}
}

func TestMeasureTextRejectsInvalidFontSize(t *testing.T) {
	if _, err := MeasureText("Viva", 0, 0); err == nil {
		t.Fatal("zero font size was accepted")
	}
}

func TestUTF16RangeUsesRuneBoundaries(t *testing.T) {
	value := "A中😀B"
	for _, test := range []struct {
		start  int
		length int
		want   text.Range
	}{
		{start: 0, length: 2, want: text.Range{Start: 0, End: 2}},
		{start: 2, length: 2, want: text.Range{Start: 2, End: 3}},
		{start: 3, length: 1, want: text.Range{Start: 2, End: 3}},
		{start: 4, length: 1, want: text.Range{Start: 3, End: 4}},
	} {
		if got := utf16Range(value, test.start, test.length); got != test.want {
			t.Fatalf("utf16Range(%d, %d) = %+v, want %+v", test.start, test.length, got, test.want)
		}
	}
}

func BenchmarkSelectionUTF16Range(b *testing.B) {
	value := "Viva 原生界面 👩‍💻 A中😀B"
	b.ReportAllocs()
	for range b.N {
		selection := utf16Range(value, 4, 14)
		if selection != (text.Range{Start: 4, End: 16}) {
			b.Fatalf("selection = %+v", selection)
		}
	}
}

func TestValidateTextInput(t *testing.T) {
	valid := &TextInput{
		Width:    100,
		Height:   32,
		FontSize: 16,
		State: TextState{
			Value:     "你好",
			Selection: text.Range{Start: 2, End: 2},
		},
		OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
	}
	if err := validateTextInput(valid); err != nil {
		t.Fatal(err)
	}

	invalid := *valid
	invalid.State.Selection = text.Range{Start: 3, End: 3}
	if err := validateTextInput(&invalid); err == nil {
		t.Fatal("out-of-range selection was accepted")
	}
	invalid = *valid
	invalid.OnEvent = nil
	if err := validateTextInput(&invalid); err == nil {
		t.Fatal("missing text handler was accepted")
	}
}

func TestMakeNativeTextLabelsFiltersInvalidDimensions(t *testing.T) {
	labels := []TextLabel{
		{Value: "zero width", Width: 0, Height: 16, FontSize: 14},
		{Value: "negative width", Width: -10, Height: 16, FontSize: 14},
		{Value: "zero height", Width: 100, Height: 0, FontSize: 14},
		{Value: "zero font", Width: 100, Height: 16, FontSize: 0},
		{Value: "valid text", Width: 100, Height: 16, FontSize: 14},
		{Value: "valid bitmap", Width: 20, Height: 20, Bitmap: make([]byte, 20*20*4), BitmapWidth: 20, BitmapHeight: 20, BitmapStride: 20 * 4},
		{Value: "invalid bitmap dims", Width: 0, Height: 20, Bitmap: make([]byte, 20*20*4), BitmapWidth: 20, BitmapHeight: 20, BitmapStride: 20 * 4},
	}
	memory, strings, count, err := makeNativeTextLabels(labels)
	if err != nil {
		t.Fatalf("makeNativeTextLabels failed: %v", err)
	}
	defer freeNativeTextLabels(memory, strings)
	if count != 2 {
		t.Fatalf("valid count = %d, want 2", count)
	}
}

func TestMakeNativeTextLabelsAllInvalidReturnsNil(t *testing.T) {
	labels := []TextLabel{
		{Value: "zero width", Width: 0, Height: 16, FontSize: 14},
		{Value: "zero height", Width: 100, Height: 0, FontSize: 14},
	}
	memory, strings, count, err := makeNativeTextLabels(labels)
	if err != nil {
		t.Fatalf("makeNativeTextLabels failed: %v", err)
	}
	defer freeNativeTextLabels(memory, strings)
	if count != 0 || memory != nil {
		t.Fatalf("count = %d, memory = %p; want count=0 and memory=nil", count, memory)
	}
}

func TestDynamicTextInputRequiresSceneShaderWithoutInitialEditor(t *testing.T) {
	if !needsSceneShader(Config{DynamicTextInput: true}) {
		t.Fatal("dynamic text input did not request the text rendering pipeline")
	}
}

func TestRunStateTextInputOwnerEnableReplaceDisable(t *testing.T) {
	firstEvents := 0
	secondEvents := 0
	first := &TextInput{
		X: 10, Y: 20, Width: 180, Height: 32, FontSize: 14,
		State: TextState{Value: "first", Selection: text.Range{Start: 5, End: 5}},
		OnEvent: func(TextEvent) (TextState, error) {
			firstEvents++
			return TextState{}, nil
		},
	}
	second := *first
	second.X = 30
	second.State = TextState{Value: "second", Selection: text.Range{Start: 6, End: 6}}
	second.OnEvent = func(TextEvent) (TextState, error) {
		secondEvents++
		return TextState{}, nil
	}

	state := &runState{}
	state.setTextInput(first)
	if !state.textInputActive || state.textState != first.State {
		t.Fatalf("first editor state = %+v", state)
	}
	if _, err := state.textHandler(TextEvent{}); err != nil {
		t.Fatal(err)
	}
	state.setTextInput(&second)
	if !state.textInputActive || state.textInput.X != second.X || state.textState != second.State {
		t.Fatalf("replacement editor state = %+v", state)
	}
	if _, err := state.textHandler(TextEvent{}); err != nil {
		t.Fatal(err)
	}
	if firstEvents != 1 || secondEvents != 1 {
		t.Fatalf("handler ownership = first %d second %d", firstEvents, secondEvents)
	}

	state.setTextInput(nil)
	if state.textInputActive || state.textHandler != nil || state.textState != (TextState{}) {
		t.Fatalf("disabled editor state = %+v", state)
	}
}

func TestTextEventRedrawRecognizesAlreadyAppliedNativeState(t *testing.T) {
	input := &TextInput{
		X: 10, Y: 20, Width: 180, Height: 32, FontSize: 14,
		State:   TextState{Value: "你好", Selection: text.Range{Start: 2, End: 2}},
		OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
	}
	state := &runState{}
	state.setTextInput(input)
	if state.textInputAlreadyUpdated(input) {
		t.Fatal("ordinary frame update was mistaken for a text-event synchronization")
	}
	state.textEventRedraw = true
	if !state.textInputAlreadyUpdated(input) {
		t.Fatal("text-event synchronization rebuilt the active native editor")
	}

	moved := *input
	moved.X++
	if state.textInputAlreadyUpdated(&moved) {
		t.Fatal("changed editor geometry was skipped")
	}
	if state.textInputAlreadyUpdated(nil) {
		t.Fatal("disable update was skipped")
	}
}

func TestNativeCaretTimerPolicy(t *testing.T) {
	tests := []struct {
		name           string
		enabled        bool
		selectedLength int
		composing      bool
		terminal       bool
		hideOverlays   bool
		wantCaretTimer bool
	}{
		{name: "disabled"},
		{name: "collapsed selection", enabled: true, wantCaretTimer: true},
		{name: "range selection", enabled: true, selectedLength: 2},
		{name: "composition", enabled: true, composing: true},
		{name: "a terminal keeps its cursor steady", enabled: true, terminal: true},
		{name: "a self-painting component keeps its cursor steady", enabled: true, hideOverlays: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nativeCaretShouldBlink(test.enabled, test.selectedLength, test.composing, test.terminal, test.hideOverlays); got != test.wantCaretTimer {
				t.Fatalf("nativeCaretShouldBlink() = %v, want %v", got, test.wantCaretTimer)
			}
		})
	}
}

func TestNativeTextInputKeyRouting(t *testing.T) {
	for _, key := range []input.Key{input.KeyBackspace, input.KeyDelete, input.KeyLeft, input.KeyRight} {
		if !nativeTextInputShouldInterpretKey(true, false, int(key)) {
			t.Fatalf("editing key %v did not route to AppKit", key)
		}
	}
	for _, key := range []input.Key{input.KeyTab, input.KeyEnter, input.KeyEscape} {
		if nativeTextInputShouldInterpretKey(true, false, int(key)) {
			t.Fatalf("single-line focus key %v was consumed by AppKit", key)
		}
	}
	if !nativeTextInputShouldInterpretKey(true, true, int(input.KeyEnter)) {
		t.Fatal("multiline Enter did not route to AppKit")
	}
	if nativeTextInputShouldInterpretKey(false, false, int(input.KeyBackspace)) {
		t.Fatal("disabled editor consumed Backspace")
	}
}

// A field that lays its rows out itself gets the keys that move between rows as
// raw events, the text system knowing one line only; an input method that is
// composing keeps every key.
func TestNativeRowKeysReachThePlacedFieldRaw(t *testing.T) {
	for _, key := range []input.Key{input.KeyUp, input.KeyDown, input.KeyHome, input.KeyEnd, input.KeyPageUp, input.KeyPageDown} {
		if !nativeTextInputRowKeyIsRaw(true, false, int(key), false) {
			t.Errorf("%v did not reach a placed field raw", key)
		}
		if nativeTextInputRowKeyIsRaw(false, false, int(key), false) {
			t.Errorf("%v was taken from an ordinary field", key)
		}
		if nativeTextInputRowKeyIsRaw(true, true, int(key), false) {
			t.Errorf("%v was taken from an input method that is composing", key)
		}
	}
	if !nativeTextInputRowKeyIsRaw(true, false, int(input.KeyLeft), true) || nativeTextInputRowKeyIsRaw(true, false, int(input.KeyLeft), false) {
		t.Error("only Command+Left is the end of a row; Left alone is the text system's")
	}
}

func TestNativeNavigationKeysReachTheFieldRaw(t *testing.T) {
	for _, key := range []input.Key{input.KeyUp, input.KeyDown, input.KeyPageUp, input.KeyPageDown} {
		if !nativeTextInputNavigationKeyIsRaw(true, false, int(key)) {
			t.Errorf("%v did not reach a field requesting navigation keys raw", key)
		}
		if nativeTextInputNavigationKeyIsRaw(false, false, int(key)) {
			t.Errorf("%v was taken from an ordinary field", key)
		}
		if nativeTextInputNavigationKeyIsRaw(true, true, int(key)) {
			t.Errorf("%v was taken from an input method that is composing", key)
		}
	}
	for _, key := range []input.Key{input.KeyHome, input.KeyEnd, input.KeyLeft, input.KeyRight} {
		if nativeTextInputNavigationKeyIsRaw(true, false, int(key)) {
			t.Errorf("%v must stay with the native text system", key)
		}
	}
}

func BenchmarkMeasureTextMixed(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		if _, err := MeasureText("Viva 原生界面 👩‍💻", 16, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func TestTerminalKeyRoutingSendsNonTextRawAndTextToTheInputSystem(t *testing.T) {
	for _, test := range []struct {
		name                                                         string
		composing, named, control, command, alt, altSendsEscape, raw bool
	}{
		{name: "a printable character", raw: false},
		{name: "a named key such as an arrow", named: true, raw: true},
		{name: "Control+C", control: true, raw: true},
		{name: "a Command chord", command: true, raw: true},
		{name: "Option types text by default", alt: true, raw: false},
		{name: "Option sends Meta when requested", alt: true, altSendsEscape: true, raw: true},
		{name: "a character while composing", composing: true, raw: false},
		{name: "Enter that commits a composition", composing: true, named: true, raw: false},
		{name: "an arrow that picks a candidate", composing: true, named: true, raw: false},
		{name: "Control while composing", composing: true, control: true, raw: false},
		{name: "Option while composing", composing: true, alt: true, altSendsEscape: true, raw: false},
	} {
		if got := nativeTerminalKeyRoutesRaw(test.composing, test.named, test.control, test.command, test.alt, test.altSendsEscape); got != test.raw {
			t.Errorf("%s: raw = %v, want %v", test.name, got, test.raw)
		}
	}
}

func BenchmarkTerminalKeyRouting(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if !nativeTerminalKeyRoutesRaw(false, false, false, false, true, true) {
			b.Fatal("Option+printable did not route raw")
		}
	}
}

func TestNativeTextInputCarriesAltSendsEscape(t *testing.T) {
	native := makeNativeTextInput(TextInput{AltSendsEscape: true}, nil)
	if native.alt_sends_escape == 0 {
		t.Fatal("AltSendsEscape was not passed to AppKit")
	}
}

func TestTerminalKeyCodesNameTheNonTextKeys(t *testing.T) {
	const (
		upArrow, pageUp, forwardDelete, insert, f1, f12 = 0xF700, 0xF72C, 0xF728, 0xF727, 0xF704, 0xF70F
	)
	for _, test := range []struct {
		character uint16
		want      input.Key
		name      string
	}{
		{'\t', input.KeyTab, "tab"},
		{0x19, input.KeyTab, "shift+tab"},
		{'\r', input.KeyEnter, "return"},
		{0x1b, input.KeyEscape, "escape"},
		{upArrow, input.KeyUp, "up"},
		{pageUp, input.KeyPageUp, "page up"},
		{0x7f, input.KeyBackspace, "the Mac delete key is backspace"},
		{forwardDelete, input.KeyDelete, "forward delete"},
		{insert, input.KeyInsert, "insert"},
		{f1, input.KeyF1, "f1"},
		{f12, input.KeyF12, "f12"},
		{'a', input.KeyUnknown, "a letter is text"},
		{' ', input.KeyUnknown, "space is text"},
		{0x4e16, input.KeyUnknown, "a character beyond ASCII is text"},
	} {
		if got := nativeTerminalKeyCode(test.character); got != test.want {
			t.Errorf("%s: key = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestOnlyMovedRecognizesAFollowedTerminalCursor(t *testing.T) {
	base := TextInput{X: 10, Y: 20, Width: 8, Height: 16, FontSize: 13, Terminal: true}
	moved := base
	moved.X, moved.Y = 18, 36
	if !onlyMoved(base, moved) {
		t.Error("a moved terminal cursor was not recognized")
	}
	composing := moved
	composing.State = TextState{Value: "ni", Composing: true}
	if onlyMoved(base, composing) {
		t.Error("new text counted as a move")
	}
	layered := moved
	layered.Layer = 600
	if onlyMoved(base, layered) {
		t.Error("a compositing-layer change counted as a move")
	}
	policy := moved
	policy.AltSendsEscape = true
	if onlyMoved(base, policy) {
		t.Error("a changed Alt policy counted as a move")
	}
	field := base
	field.Terminal = false
	if onlyMoved(field, field) || onlyMoved(base, field) {
		t.Error("an ordinary text field counted as a terminal move")
	}
}

func TestTextEventPlanRedrawsAFieldAlwaysAndATerminalOnlyWhenItsPreeditChanges(t *testing.T) {
	typed := TextState{Value: "a", Selection: text.Range{Start: 1, End: 1}}
	for _, test := range []struct {
		name           string
		terminal       bool
		previous, next TextState
		want           textEventPlan
	}{
		{"typing in a field", false, TextState{}, typed, textEventPlan{update: true, render: true}},
		{"a field's caret move that changes nothing", false, typed, typed, textEventPlan{update: true, render: true}},
		{"plain text going to a program", true, TextState{}, TextState{}, textEventPlan{}},
		{"a paste going to a program", true, TextState{}, TextState{}, textEventPlan{}},
		{"an input method starting a composition", true, TextState{}, TextState{Value: "ni", Composition: text.Range{End: 2}, Composing: true}, textEventPlan{update: true, render: true}},
		{"the composition changing", true, TextState{Value: "ni", Composing: true}, TextState{Value: "nih", Composing: true}, textEventPlan{update: true, render: true}},
		{"the composition being committed", true, TextState{Value: "ni", Composing: true}, TextState{}, textEventPlan{update: true, render: true}},
		{"a composition that did not change", true, TextState{Value: "ni", Composing: true}, TextState{Value: "ni", Composing: true}, textEventPlan{}},
	} {
		if got := planTextEvent(test.terminal, test.previous, test.next); got != test.want {
			t.Errorf("%s: plan = %+v, want %+v", test.name, got, test.want)
		}
	}
}

// A heavier weight is a wider line, and a label fitted at the regular weight
// would clip at the heavier one.
func TestMeasureTextIsWiderAtAHeavierWeight(t *testing.T) {
	regular, err := MeasureText("Import SSH Config", 14, 0)
	if err != nil {
		t.Fatal(err)
	}
	bold, err := MeasureText("Import SSH Config", 14, 0.40)
	if err != nil {
		t.Fatal(err)
	}
	if bold.Width <= regular.Width {
		t.Fatalf("bold %.2f is not wider than regular %.2f", bold.Width, regular.Width)
	}
}

func TestARegisteredFamilyIsMeasuredAndAnUnknownOneFallsBackToTheSystemFace(t *testing.T) {
	system, systemHeight, err := MeasureMono(13, ui.MonoSystem)
	if err != nil || system <= 0 {
		t.Fatalf("system mono: %v %v", system, err)
	}
	courier := ui.MonoFamily("Courier New")
	width, height, err := MeasureMono(13, courier)
	if err != nil || width <= 0 || height <= 0 {
		t.Fatalf("Courier New: %v %v %v", width, height, err)
	}
	if width == system && height == systemHeight {
		t.Fatalf("Courier New measures like the system face (%v x %v): the family was not used", width, height)
	}
	missing := ui.MonoFamily("No Such Family Installed")
	fallbackWidth, fallbackHeight, err := MeasureMono(13, missing)
	if err != nil || fallbackWidth != system || fallbackHeight != systemHeight {
		t.Fatalf("a missing family measured %v x %v (%v), want the system face %v x %v", fallbackWidth, fallbackHeight, err, system, systemHeight)
	}
}

func TestFallbackFamiliesNeverChangeTheCell(t *testing.T) {
	base := ui.MonoFamily("Menlo")
	plainWidth, plainHeight, err := MeasureMono(13, base)
	if err != nil || plainWidth <= 0 {
		t.Fatalf("Menlo: %v %v", plainWidth, err)
	}
	for _, fallbacks := range [][]string{{"Songti SC"}, {"No Such Family Installed", "Songti SC"}, {"Courier New", "Songti SC", "STHeiti"}} {
		width, height, err := MeasureMono(13, ui.MonoStack(base, fallbacks...))
		if err != nil || width != plainWidth || height != plainHeight {
			t.Errorf("Menlo with %q measures %v x %v (%v), want Menlo's own %v x %v", fallbacks, width, height, err, plainWidth, plainHeight)
		}
	}
	system, systemHeight, _ := MeasureMono(13, ui.MonoSystem)
	if width, height, err := MeasureMono(13, ui.MonoStack(ui.MonoSystem, "Songti SC")); err != nil || width != system || height != systemHeight {
		t.Errorf("the system face with a fallback measures %v x %v (%v), want %v x %v", width, height, err, system, systemHeight)
	}
}
