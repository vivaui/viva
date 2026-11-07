package app

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/semantic"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestTextSelectionAndIMEUseRuneRanges(t *testing.T) {
	editor := text.NewEditor("Go中文")
	TextEvent{Kind: TextSetSelection, Selection: TextRange{Start: 2, End: 4}}.Apply(editor)
	TextEvent{Kind: TextSetMarked, Text: "拼音", MarkedSelection: TextRange{Start: 1, End: 2}}.Apply(editor)
	state := TextStateOf(editor)
	if state.Value != "Go拼音" || !state.Composing || state.Selection != (TextRange{Start: 3, End: 4}) || state.Composition != (TextRange{Start: 2, End: 4}) {
		t.Fatalf("marked state = %+v", state)
	}
	TextEvent{Kind: TextInsert, Text: "汉字"}.Apply(editor)
	if state = TextStateOf(editor); state.Value != "Go汉字" || state.Composing || state.Selection != (TextRange{Start: 4, End: 4}) {
		t.Fatalf("committed state = %+v", state)
	}
	TextEvent{Kind: TextInsert, HasReplacement: true, Replacement: TextRange{Start: 0, End: 2}, Text: "Viva"}.Apply(editor)
	if editor.String() != "Viva汉字" {
		t.Fatalf("replacement = %q", editor.String())
	}
	TextEvent{Kind: TextSetMarked, Text: "测"}.Apply(editor)
	TextEvent{Kind: TextUnmark}.Apply(editor)
	if TextStateOf(editor).Composing {
		t.Fatal("unmark left active composition")
	}
}

func TestTextCommandsMoveAndEditByGrapheme(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		command       TextCommand
		selection     text.Range
		wantValue     string
		wantSelection TextRange
	}{
		{"delete previous grapheme", TextDeleteBackward, text.Range{Start: 4, End: 4}, "A中B", TextRange{Start: 1, End: 1}},
		{"delete next grapheme", TextDeleteForward, text.Range{Start: 1, End: 1}, "A中B", TextRange{Start: 1, End: 1}},
		{"delete selection backward", TextDeleteBackward, text.Range{Start: 1, End: 5}, "AB", TextRange{Start: 1, End: 1}},
		{"delete selection forward", TextDeleteForward, text.Range{Start: 1, End: 5}, "AB", TextRange{Start: 1, End: 1}},
		{"move left across grapheme", TextMoveLeft, text.Range{Start: 4, End: 4}, "A👩‍💻中B", TextRange{Start: 1, End: 1}},
		{"move right across grapheme", TextMoveRight, text.Range{Start: 1, End: 1}, "A👩‍💻中B", TextRange{Start: 4, End: 4}},
		{"collapse selection left", TextMoveLeft, text.Range{Start: 1, End: 5}, "A👩‍💻中B", TextRange{Start: 1, End: 1}},
		{"collapse selection right", TextMoveRight, text.Range{Start: 1, End: 5}, "A👩‍💻中B", TextRange{Start: 5, End: 5}},
		{"extend selection left", TextMoveLeftAndModifySelection, text.Range{Start: 4, End: 4}, "A👩‍💻中B", TextRange{Start: 1, End: 4}},
		{"extend selection right", TextMoveRightAndModifySelection, text.Range{Start: 1, End: 1}, "A👩‍💻中B", TextRange{Start: 1, End: 4}},
		{"replace selection with newline", TextInsertNewline, text.Range{Start: 1, End: 5}, "A\nB", TextRange{Start: 2, End: 2}},
		{"replace selection with a line break", TextInsertLineBreak, text.Range{Start: 1, End: 5}, "A\nB", TextRange{Start: 2, End: 2}},
		{"replace selection with tab", TextInsertTab, text.Range{Start: 1, End: 5}, "A\tB", TextRange{Start: 2, End: 2}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			editor := text.NewEditor("A👩‍💻中B")
			editor.SetSelection(testCase.selection)
			TextEvent{Kind: TextCommandEvent, Command: testCase.command}.Apply(editor)
			want := TextState{Value: testCase.wantValue, Selection: testCase.wantSelection}
			want.SelectionReversed = testCase.command == TextMoveLeftAndModifySelection
			if got := TextStateOf(editor); got != want {
				t.Fatalf("text state = %+v, want %+v", got, want)
			}
		})
	}
}

func TestTextCommandsUndoRedoAndKeepPasteSeparate(t *testing.T) {
	editor := text.NewEditor("")
	TextEvent{Kind: TextInsert, Text: "ab"}.Apply(editor)
	TextEvent{Kind: TextPaste, Text: " pasted"}.Apply(editor)
	TextEvent{Kind: TextCommandEvent, Command: TextUndo}.Apply(editor)
	if got := TextStateOf(editor); got.Value != "ab" || got.Selection != (TextRange{Start: 2, End: 2}) {
		t.Fatalf("undo paste = %+v", got)
	}
	TextEvent{Kind: TextCommandEvent, Command: TextUndo}.Apply(editor)
	if got := TextStateOf(editor); got.Value != "" || got.Selection != (TextRange{}) {
		t.Fatalf("undo typing = %+v", got)
	}
	TextEvent{Kind: TextCommandEvent, Command: TextRedo}.Apply(editor)
	TextEvent{Kind: TextCommandEvent, Command: TextRedo}.Apply(editor)
	if got := TextStateOf(editor); got.Value != "ab pasted" || got.Selection != (TextRange{Start: 9, End: 9}) {
		t.Fatalf("redo = %+v", got)
	}
}

func TestSelectionExtensionPreservesAnchor(t *testing.T) {
	editor := text.NewEditor("A👩‍💻中B")
	for index, step := range []struct {
		command   TextCommand
		selection TextRange
		reversed  bool
	}{
		{TextMoveLeftAndModifySelection, TextRange{Start: 5, End: 6}, true},
		{TextMoveLeftAndModifySelection, TextRange{Start: 4, End: 6}, true},
		{TextMoveLeftAndModifySelection, TextRange{Start: 1, End: 6}, true},
		{TextMoveRightAndModifySelection, TextRange{Start: 4, End: 6}, true},
		{TextMoveRightAndModifySelection, TextRange{Start: 5, End: 6}, true},
		{TextMoveRightAndModifySelection, TextRange{Start: 6, End: 6}, false},
	} {
		TextEvent{Kind: TextCommandEvent, Command: step.command}.Apply(editor)
		want := TextState{Value: "A👩‍💻中B", Selection: step.selection, SelectionReversed: step.reversed}
		if got := TextStateOf(editor); got != want {
			t.Fatalf("step %d text state = %+v, want %+v", index, got, want)
		}
	}
}

func TestTextStateOfPreservesRuneIndexedComposition(t *testing.T) {
	editor := text.NewEditor("A😀Z")
	editor.SetSelection(text.Range{Start: 2, End: 2})
	editor.SetMarkedText("中文", text.Range{Start: 1, End: 2})
	want := TextState{
		Value: "A😀中文Z", Selection: TextRange{Start: 3, End: 4},
		Composition: TextRange{Start: 2, End: 4}, Composing: true,
	}
	if got := TextStateOf(editor); got != want {
		t.Fatalf("composing text state = %+v, want %+v", got, want)
	}
	TextEvent{Kind: TextCommandEvent, Command: TextMoveLeft}.Apply(editor)
	want.Selection = TextRange{Start: 3, End: 3}
	want.Composing = false
	if got := TextStateOf(editor); got != want {
		t.Fatalf("committed text state = %+v, want %+v", got, want)
	}
}

// An editor-backed input needs no hand-written state or event glue: the
// runtime mirrors the editor into the platform and applies native events to it.
func TestEditorBackedTextInputMirrorsEditorAndAppliesNativeEvents(t *testing.T) {
	const field, fieldLayer = 7, 600
	editor := text.NewEditor("Go")
	var canvas ui.Canvas
	config := Config{Title: "editor", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.PushLayer(fieldLayer)
		canvas.HitWithOptions(ui.Item(int(field)), ui.Rect{Width: 120, Height: 24}, "Name", ui.HitOptions{Role: ui.RoleTextField})
		canvas.PopLayer()
	}
	var input TextInput
	var context TextInputContext
	config.TextInput = func(got TextInputContext) *TextInput {
		context = got
		input = TextInput{Bounds: got.Bounds, FontSize: 13, Editor: editor}
		return &input
	}
	painter := newTestPainter(t, &canvas, config)

	// Pointer-down lands focus and enables native editing; later frames of the
	// same click carry no text-input change.
	var enabled *platform.TextInput
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		frame, err := painter.pointerEvent(platform.PointerEvent{Kind: kind, X: 10, Y: 10})
		if err != nil {
			t.Fatal(err)
		}
		if frame.TextInput.Changed && frame.TextInput.Input != nil {
			enabled = frame.TextInput.Input
		}
	}
	if enabled == nil || enabled.State.Value != "Go" || enabled.Target != ui.Item(field) {
		t.Fatalf("native input was not mirrored from the editor: %+v", enabled)
	}
	if context.Layer != fieldLayer || enabled.Layer != fieldLayer {
		t.Fatalf("text input layers = context %d, native %d; want %d", context.Layer, enabled.Layer, fieldLayer)
	}

	state, err := painter.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if editor.String() != "Gop" || state.Value != "Gop" || painter.nativeText.State.Value != "Gop" {
		t.Fatalf("editor=%q state=%q native=%q, want all Gop", editor.String(), state.Value, painter.nativeText.State.Value)
	}
	if state.Selection != (text.Range{Start: 3, End: 3}) {
		t.Fatalf("caret = %+v, want it after the inserted rune", state.Selection)
	}
	// A single-line field takes no line break from a native command: while an
	// input method is composing, AppKit answers Return with insertNewline:.
	for _, command := range []platform.TextCommand{platform.TextInsertNewline, platform.TextInsertLineBreak} {
		state, err = painter.applyTextEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
		if err != nil {
			t.Fatal(err)
		}
		if editor.String() != "Gop" || state.Value != "Gop" {
			t.Fatalf("single-line field took %v: %q", command, editor.String())
		}
	}
}

func TestTextInputTargetChangeWithIdenticalPresentation(t *testing.T) {
	var canvas ui.Canvas
	editor := text.NewEditor("same")
	input := &TextInput{Editor: editor, Bounds: ui.Rect{Width: 100, Height: 24}, FontSize: 13}
	painter := newTestPainter(t, &canvas, Config{
		Title: "targets", Width: 200, Height: 100,
		Paint: func(c *ui.Canvas, _ ui.Size) {
			for i := 1; i <= 2; i++ {
				c.HitWithOptions(ui.Item(i), ui.Rect{X: float32((i - 1) * 100), Width: 100, Height: 24}, "field", ui.HitOptions{Role: ui.RoleTextField})
			}
		},
		TextInput: func(TextInputContext) *TextInput { return input },
	})
	for i := 1; i <= 2; i++ {
		changed := false
		for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
			frame, err := painter.pointerEvent(platform.PointerEvent{Kind: kind, X: float32((i-1)*100 + 10), Y: 10})
			if err != nil {
				t.Fatal(err)
			}
			if frame.TextInput.Changed && frame.TextInput.Input != nil {
				changed = frame.TextInput.Input.Target == ui.Item(i)
			}
		}
		if !changed {
			t.Fatalf("target %d was not sent to the native editor", i)
		}
	}
}

func TestTextInputWithoutEditorOrHandlerRejectsNativeEvents(t *testing.T) {
	var canvas ui.Canvas
	painter := newTestPainter(t, &canvas, Config{Title: "none", Width: 100, Height: 100})
	if _, err := painter.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"}); err == nil {
		t.Fatal("a native text event without an active input must fail")
	}
}

// A password field reaches the platform masked, and no accessibility path -
// semantic tree, native nodes, or the provider context - carries its text.
func TestSecureFieldIsMaskedForThePlatformAndSilentForAccessibility(t *testing.T) {
	const field = 9
	editor := text.NewEditor("hunter2")
	var canvas ui.Canvas
	config := Config{Title: "secure", Width: 200, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(int(field)), ui.Rect{Width: 120, Height: 24}, "Password", ui.HitOptions{
			Role: ui.RoleTextField, Value: editor.String(), Secure: true,
		})
	}
	var context TextInputContext
	config.TextInput = func(got TextInputContext) *TextInput {
		context = got
		return &TextInput{Bounds: got.Bounds, FontSize: 13, Editor: editor}
	}
	painter := newTestPainter(t, &canvas, config)

	var enabled *platform.TextInput
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		frame, err := painter.pointerEvent(platform.PointerEvent{Kind: kind, X: 10, Y: 10})
		if err != nil {
			t.Fatal(err)
		}
		if frame.TextInput.Changed && frame.TextInput.Input != nil {
			enabled = frame.TextInput.Input
		}
	}
	if !context.Secure {
		t.Fatal("the provider was not told the field is secure")
	}
	if enabled == nil || !enabled.Secure {
		t.Fatalf("the platform was not told to mask the field: %+v", enabled)
	}
	for _, node := range painter.accessibility {
		if node.Value != "" || strings.Contains(node.Label, "hunter2") {
			t.Fatalf("native accessibility node exposes the password: %+v", node)
		}
		if node.Role == uint8(semantic.RoleTextField) && !node.Secure {
			t.Fatalf("native accessibility node is not marked secure: %+v", node)
		}
	}
	checked := 0
	for _, id := range painter.semanticIDs {
		node, ok := painter.semantics.Get(id)
		if !ok || node.Role != semantic.RoleTextField {
			continue
		}
		checked++
		if node.Value != "" || !node.Secure {
			t.Fatalf("semantic node = %+v, want a secure field without a value", node)
		}
	}
	if checked != 1 {
		t.Fatalf("checked %d semantic text fields, want 1", checked)
	}
}

// A focused text field is on the keystroke path: repainting it must not
// allocate. The handler closure and the provider's TextInput are both cached
// across frames; returning a fresh one per paint used to cost two allocations.
func TestFocusedTextFieldRepaintDoesNotAllocate(t *testing.T) {
	var canvas ui.Canvas
	editor := text.NewEditor("hello")
	input := &TextInput{Editor: editor}
	config := Config{Title: "text", Width: 400, Height: 100}
	config.TextInput = func(TextInputContext) *TextInput { return input }
	config.Paint = func(c *ui.Canvas, _ ui.Size) {
		c.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 200, Height: 30}, "field", ui.HitOptions{
			Role: ui.RoleTextField, Autofocus: true,
		})
	}
	painter := newTestPainter(t, &canvas, config)
	if !painter.textEnabled {
		t.Fatal("the text field did not take focus")
	}
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := painter.repaint(); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("a focused text field repaint allocates %v times, want 0", allocs)
	}
}
