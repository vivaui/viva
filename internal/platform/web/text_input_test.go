package web

import (
	"github.com/vivaui/viva/ui"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestTextInputRuneUTF16Conversions(t *testing.T) {
	value := "A中😀e\u0301"
	tests := []struct {
		runes int
		units int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 5},
		{5, 6},
		{99, 6},
	}
	for _, test := range tests {
		if got := runeIndexToUTF16(value, test.runes); got != test.units {
			t.Fatalf("runeIndexToUTF16(%d) = %d, want %d", test.runes, got, test.units)
		}
		if got := utf16IndexToRune(value, test.units); got != min(test.runes, 5) {
			t.Fatalf("utf16IndexToRune(%d) = %d, want %d", test.units, got, min(test.runes, 5))
		}
	}
	if got := utf16IndexToRune(value, 3); got != 2 {
		t.Fatalf("UTF-16 index inside surrogate pair = %d, want preceding rune 2", got)
	}
}

func TestTextInputCSSAlignment(t *testing.T) {
	for _, testcase := range []struct {
		alignment ui.Alignment
		want      string
	}{
		{ui.AlignLeft, "left"},
		{ui.AlignCenter, "center"},
		{ui.AlignRight, "right"},
	} {
		if got := textAlignCSS(testcase.alignment); got != testcase.want {
			t.Errorf("CSS text alignment = %q, want %q", got, testcase.want)
		}
	}
}

func TestTextInputRangeConversionsClampAndOrder(t *testing.T) {
	value := "Go😀中"
	start, end := runeRangeToUTF16(value, text.Range{Start: 3, End: 1})
	if start != 1 || end != 4 {
		t.Fatalf("rune range = %d:%d, want 1:4 UTF-16 units", start, end)
	}
	if got := utf16RangeToRunes(value, 99, -4); got != (text.Range{Start: 0, End: 4}) {
		t.Fatalf("clamped UTF-16 range = %+v", got)
	}
}

func TestReplaceTextEventReplacesCompleteRuneValue(t *testing.T) {
	event := replaceTextEvent("Go😀", "输入")
	want := platform.TextEvent{
		Kind:           platform.TextInsert,
		Text:           "输入",
		Replacement:    text.Range{End: 3},
		HasReplacement: true,
	}
	if event != want {
		t.Fatalf("replace event = %+v, want %+v", event, want)
	}
}

func TestTextInputHelpersDoNotAllocate(t *testing.T) {
	value := "Go中文😀"
	if allocations := testing.AllocsPerRun(1000, func() {
		runeIndexToUTF16(value, 4)
		utf16IndexToRune(value, 4)
		runeRangeToUTF16(value, text.Range{Start: 1, End: 4})
		utf16RangeToRunes(value, 1, 5)
		replaceTextEvent(value, "Viva")
	}); allocations != 0 {
		t.Fatalf("text input helper allocations = %v", allocations)
	}
}

func TestInputTextEventReportsAnInsertionAtItsPosition(t *testing.T) {
	for _, test := range []struct {
		name      string
		previous  platform.TextState
		next      string
		inputType string
		want      platform.TextEvent
	}{
		{
			"typed at the caret",
			platform.TextState{Value: "482901", Selection: text.Range{Start: 0, End: 0}}, "1482901", "insertText",
			platform.TextEvent{Kind: platform.TextInsert, Text: "1", Replacement: text.Range{Start: 0, End: 0}, HasReplacement: true},
		},
		{
			"typed over a selection",
			platform.TextState{Value: "abcd", Selection: text.Range{Start: 1, End: 3}}, "aXd", "insertText",
			platform.TextEvent{Kind: platform.TextInsert, Text: "X", Replacement: text.Range{Start: 1, End: 3}, HasReplacement: true},
		},
		{
			"an emoji between two characters",
			platform.TextState{Value: "你好", Selection: text.Range{Start: 1, End: 1}}, "你😀好", "insertText",
			platform.TextEvent{Kind: platform.TextInsert, Text: "😀", Replacement: text.Range{Start: 1, End: 1}, HasReplacement: true},
		},
		{
			"pasted text",
			platform.TextState{Value: "ab", Selection: text.Range{Start: 2, End: 2}}, "ab123", "insertFromPaste",
			platform.TextEvent{Kind: platform.TextPaste, Text: "123", Replacement: text.Range{Start: 2, End: 2}, HasReplacement: true},
		},
		{
			"a line break",
			platform.TextState{Value: "ab", Selection: text.Range{Start: 1, End: 1}}, "a\nb", "insertLineBreak",
			platform.TextEvent{Kind: platform.TextInsert, Text: "\n", Replacement: text.Range{Start: 1, End: 1}, HasReplacement: true},
		},
		{
			"a repeated character is placed by the caret, not by the first match",
			platform.TextState{Value: "112", Selection: text.Range{Start: 1, End: 1}}, "1112", "insertText",
			platform.TextEvent{Kind: platform.TextInsert, Text: "1", Replacement: text.Range{Start: 1, End: 1}, HasReplacement: true},
		},
	} {
		if got := inputTextEvent(test.previous, test.next, test.inputType); got != test.want {
			t.Errorf("%s: got %+v, want %+v", test.name, got, test.want)
		}
	}
}

func TestInputTextEventFallsBackToReplacingTheWholeText(t *testing.T) {
	previous := platform.TextState{Value: "abc", Selection: text.Range{Start: 2, End: 2}}
	for _, test := range []struct{ name, next, inputType string }{
		{"a deletion", "ac", "deleteContentBackward"},
		{"a cut", "a", "deleteByCut"},
		{"an unknown edit type", "abXc", ""},
		{"an insertion elsewhere than the mirrored selection", "Xabc", "insertText"},
		{"text that cannot be an insertion at all", "x", "insertText"},
	} {
		if got, want := inputTextEvent(previous, test.next, test.inputType), replaceTextEvent(previous.Value, test.next); got != want {
			t.Errorf("%s: got %+v, want the whole-text replacement %+v", test.name, got, want)
		}
	}
}

func TestBrowserHistoryBecomesEditorUndoAndRedoCommands(t *testing.T) {
	previous := platform.TextState{Value: "abc", Selection: text.Range{Start: 3, End: 3}}
	for _, test := range []struct {
		inputType string
		command   platform.TextCommand
	}{{"historyUndo", platform.TextUndo}, {"historyRedo", platform.TextRedo}} {
		got := inputTextEvent(previous, "browser value is not authoritative", test.inputType)
		want := platform.TextEvent{Kind: platform.TextCommandEvent, Command: test.command}
		if got != want {
			t.Errorf("%s: got %+v, want %+v", test.inputType, got, want)
		}
	}
}

// The browser reports a typed digit as a string that is one longer; fed through
// inputTextEvent the editor still overwrites, restricts and caps like it does
// natively, which a whole-text replacement could not do.
func TestBrowserEditsHonorTheEditorsOverwriteAndRestrictions(t *testing.T) {
	editor := text.NewEditor("482901")
	editor.Restrict(func(r rune) bool { return r >= '0' && r <= '9' }, 6)
	editor.SetOverwrite(true)
	editor.SetSelection(text.Range{Start: 0, End: 0})

	// typeInBrowser mimics the element: the character lands at the mirrored
	// selection, and the edit goes through the same path as a real input event.
	typeInBrowser := func(typed string) {
		state := platform.EditorState(editor)
		runes := []rune(state.Value)
		next := string(runes[:state.Selection.Start]) + typed + string(runes[state.Selection.End:])
		inputTextEvent(state, next, "insertText").Apply(editor)
	}
	for _, step := range []struct {
		typed string
		value string
		caret int
	}{
		{"1", "182901", 1}, // fills the first slot, replacing its digit, and moves on
		{"7", "172901", 2},
		{"x", "172901", 2}, // a letter is dropped and deletes nothing
		{"9", "179901", 3}, // replaces the 2
		{"5", "179501", 4},
		{"5", "179551", 5},
		{"5", "179555", 6},
		{"4", "179554", 6}, // a complete code: the last digit is replaced
	} {
		typeInBrowser(step.typed)
		if got, caret := editor.String(), editor.Selection().End; got != step.value || caret != step.caret {
			t.Fatalf("after typing %q: %q with the caret at %d, want %q at %d", step.typed, got, caret, step.value, step.caret)
		}
	}
}

func TestEditorInputsResolvesOnlyEditableHits(t *testing.T) {
	first, second := text.NewEditor("one"), text.NewEditor("two")
	provider := EditorInputs(func(id ui.ID) (*text.Editor, bool) {
		switch id {
		case ui.Item(1):
			return first, true
		case ui.Item(2):
			return second, true
		}
		return nil, false
	})
	if input := provider(TextInputContext{ID: ui.Item(9)}); input != nil {
		t.Fatal("a hit with no editor was given a text input")
	}

	input := provider(TextInputContext{ID: ui.Item(1)})
	if input == nil || input.State.Value != "one" || input.OnEvent == nil {
		t.Fatalf("input = %+v", input)
	}
	state, err := input.OnEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "!"})
	if err != nil || first.String() != "one!" || state.Value != "one!" || state.Selection != (text.Range{Start: 4, End: 4}) {
		t.Fatalf("event applied to the wrong place: %q, %+v, %v", first.String(), state, err)
	}
	if second.String() != "two" {
		t.Fatal("an event leaked into another editor")
	}

	// Focus moves to the second field: events now go there.
	input = provider(TextInputContext{ID: ui.Item(2)})
	if input.State.Value != "two" {
		t.Fatalf("second input state = %+v", input.State)
	}
	if _, err := input.OnEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "?"}); err != nil || second.String() != "two?" || first.String() != "one!" {
		t.Fatalf("after moving focus: first %q second %q err %v", first.String(), second.String(), err)
	}

	// Focus leaves every editable field: a late event must not edit the last one.
	provider(TextInputContext{ID: ui.Item(9)})
	if _, err := input.OnEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "late"}); err == nil || second.String() != "two?" {
		t.Fatalf("a late event was accepted: err %v, second %q", err, second.String())
	}
}

func TestEditorInputsResolvesWithoutAllocatingOnceWarm(t *testing.T) {
	editor := text.NewEditor("abc")
	provider := EditorInputs(func(ui.ID) (*text.Editor, bool) { return editor, true })
	provider(TextInputContext{ID: ui.Item(1)})
	if allocations := testing.AllocsPerRun(100, func() { provider(TextInputContext{ID: ui.Item(1)}) }); allocations != 0 {
		t.Fatalf("resolving the focused hit allocated %v times per repaint", allocations)
	}
}
