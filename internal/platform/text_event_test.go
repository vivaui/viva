package platform

import (
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/text"
)

func TestTextEventApplyPreservesUnicodeSelectionAndComposition(t *testing.T) {
	editor := text.NewEditor("Go中文")
	TextEvent{Kind: TextSetSelection, Selection: text.Range{Start: 2, End: 4}}.Apply(editor)
	TextEvent{Kind: TextSetMarked, Text: "拼音😀", MarkedSelection: text.Range{Start: 2, End: 3}}.Apply(editor)
	if value := editor.String(); value != "Go拼音😀" {
		t.Fatalf("marked value = %q", value)
	}
	if selection := editor.Selection(); selection != (text.Range{Start: 4, End: 5}) {
		t.Fatalf("marked selection = %+v", selection)
	}
	if composition, composing := editor.Composition(); !composing || composition != (text.Range{Start: 2, End: 5}) {
		t.Fatalf("composition = %+v, %v", composition, composing)
	}
	TextEvent{Kind: TextUnmark}.Apply(editor)
	if _, composing := editor.Composition(); composing {
		t.Fatal("composition remained active after unmark")
	}
	TextEvent{Kind: TextInsert, Text: "Viva", Replacement: text.Range{Start: 0, End: 2}, HasReplacement: true}.Apply(editor)
	if value := editor.String(); value != "Viva拼音😀" {
		t.Fatalf("replacement value = %q", value)
	}
}

func TestTextHistoryCommandUsesPlatformConventionalChords(t *testing.T) {
	for _, test := range []struct {
		name    string
		event   KeyEvent
		command TextCommand
		ok      bool
	}{
		{"control z", KeyEvent{Modifiers: input.ModifierControl, Text: "z"}, TextUndo, true},
		{"command z", KeyEvent{Modifiers: input.ModifierMeta, Text: "Z"}, TextUndo, true},
		{"shift control z", KeyEvent{Modifiers: input.ModifierControl | input.ModifierShift, Text: "z"}, TextRedo, true},
		{"control y", KeyEvent{Modifiers: input.ModifierControl, Text: "y"}, TextRedo, true},
		{"alt control z", KeyEvent{Modifiers: input.ModifierControl | input.ModifierAlt, Text: "z"}, 0, false},
		{"shift control y", KeyEvent{Modifiers: input.ModifierControl | input.ModifierShift, Text: "y"}, 0, false},
		{"both primary modifiers", KeyEvent{Modifiers: input.ModifierControl | input.ModifierMeta, Text: "z"}, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, ok := TextHistoryCommand(test.event)
			if command != test.command || ok != test.ok {
				t.Fatalf("command=%v ok=%v, want %v %v", command, ok, test.command, test.ok)
			}
		})
	}
}

func TestTextEventApplyCommands(t *testing.T) {
	for _, test := range []struct {
		name      string
		value     string
		caret     int
		command   TextCommand
		want      string
		selection text.Range
	}{
		{"delete backward", "abc", 3, TextDeleteBackward, "ab", text.Range{Start: 2, End: 2}},
		{"delete forward", "abc", 1, TextDeleteForward, "ac", text.Range{Start: 1, End: 1}},
		{"move left", "abc", 2, TextMoveLeft, "abc", text.Range{Start: 1, End: 1}},
		{"move right", "abc", 1, TextMoveRight, "abc", text.Range{Start: 2, End: 2}},
		{"extend left", "abc", 2, TextMoveLeftAndModifySelection, "abc", text.Range{Start: 1, End: 2}},
		{"extend right", "abc", 1, TextMoveRightAndModifySelection, "abc", text.Range{Start: 1, End: 2}},
		{"newline", "abc", 2, TextInsertNewline, "ab\nc", text.Range{Start: 3, End: 3}},
		{"line break", "abc", 2, TextInsertLineBreak, "ab\nc", text.Range{Start: 3, End: 3}},
		{"tab", "abc", 2, TextInsertTab, "ab\tc", text.Range{Start: 3, End: 3}},
		{"delete to line start", "abc\ndef", 5, TextDeleteLineStart, "abc\nef", text.Range{Start: 4, End: 4}},
		{"delete to line end", "abc\ndef", 1, TextDeleteLineEnd, "a\ndef", text.Range{Start: 1, End: 1}},
	} {
		editor := text.NewEditor(test.value)
		editor.SetSelection(text.Range{Start: test.caret, End: test.caret})
		TextEvent{Kind: TextCommandEvent, Command: test.command}.Apply(editor)
		if editor.String() != test.want || editor.Selection() != test.selection {
			t.Errorf("%s: got %q with selection %+v, want %q with %+v", test.name, editor.String(), editor.Selection(), test.want, test.selection)
		}
	}
}

func TestTextEventApplyHonorsTheEditorsRestrictions(t *testing.T) {
	editor := text.NewEditor("")
	editor.Restrict(func(r rune) bool { return r >= '0' && r <= '9' }, 4)
	TextEvent{Kind: TextInsert, Text: "1a2b34567"}.Apply(editor)
	if value := editor.String(); value != "1234" {
		t.Fatalf("a restricted editor took %q", value)
	}
}

func TestTextInputPresentationChangesForFieldsAndTerminalPreedit(t *testing.T) {
	empty := TextState{}
	preedit := TextState{Value: "ni", Composition: text.Range{End: 2}, Composing: true}
	if !TextInputPresentationChanged(false, empty, empty) {
		t.Fatal("an ordinary field skipped its native caret redraw")
	}
	if TextInputPresentationChanged(true, empty, empty) {
		t.Fatal("committed terminal text requested a redraw without changing preedit")
	}
	if !TextInputPresentationChanged(true, empty, preedit) {
		t.Fatal("starting terminal preedit did not request a redraw")
	}
	if !TextInputPresentationChanged(true, preedit, empty) {
		t.Fatal("committing terminal preedit did not request a redraw")
	}
}

func TestTextInputRedrawRequestForcesAFrameOnceWithoutChangingTheState(t *testing.T) {
	empty := TextState{}
	request := TextState{Redraw: true}
	if !TextInputPresentationChanged(true, empty, request) {
		t.Fatal("an application redraw request on a terminal was ignored")
	}
	if request.WithoutRedraw() != empty {
		t.Fatalf("clearing the request left %+v", request.WithoutRedraw())
	}
	// The next event compares against the cleared state, so it does not redraw
	// again unless it asks to.
	if TextInputPresentationChanged(true, request.WithoutRedraw(), empty) {
		t.Fatal("the request outlived its event")
	}
}
