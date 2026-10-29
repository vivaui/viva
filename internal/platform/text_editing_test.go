package platform

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/text"
)

func TestClipboardEditingIsAtomicAndSecure(t *testing.T) {
	for _, action := range []byte{'c', 'x', 'v'} {
		for _, fail := range []bool{false, true} {
			editor := text.NewEditor("前中文后")
			editor.SetSelection(text.Range{Start: 1, End: 3})
			var copied string
			problem := errors.New("busy clipboard")
			event, changed, err := TextClipboardEvent(EditorState(editor), false, action,
				func() (string, error) {
					if fail {
						return "", problem
					}
					return "粘贴", nil
				},
				func(value string) error {
					if fail {
						return problem
					}
					copied = value
					return nil
				})
			if fail {
				if err != problem || changed || editor.String() != "前中文后" {
					t.Fatal("failed I/O produced an edit")
				}
				continue
			}
			if changed {
				event.Apply(editor)
			}
			want := "前中文后"
			if action == 'x' {
				want = "前后"
			}
			if action == 'v' {
				want = "前粘贴后"
			}
			if editor.String() != want {
				t.Fatalf("%c: %q", action, editor.String())
			}
			if action != 'v' && copied != "中文" {
				t.Fatalf("copy=%q", copied)
			}
			if changed && (!editor.Undo() || editor.String() != "前中文后") {
				t.Fatal("edit was not undoable")
			}
		}
	}
	state := TextState{Value: "secret", Selection: text.Range{End: 6}}
	for _, action := range []byte{'c', 'x'} {
		_, changed, err := TextClipboardEvent(state, true, action, nil, func(string) error { t.Fatal("secure field copied"); return nil })
		if changed || err != nil {
			t.Fatal("secure field was changed")
		}
	}
	_, changed, err := TextClipboardEvent(state, false, 'v', func() (string, error) { return "", nil }, nil)
	if changed || err != nil {
		t.Fatal("empty clipboard deleted selection")
	}
}

func TestEditingKeyMapPreservesModifiers(t *testing.T) {
	if TextDeleteLineStart != 27 || TextDeleteLineEnd != 28 {
		t.Fatalf("line-delete command values changed: start=%d end=%d", TextDeleteLineStart, TextDeleteLineEnd)
	}
	for _, tc := range []struct {
		key       input.Key
		mods      input.Modifiers
		character string
		command   TextCommand
	}{
		{input.KeyUnknown, input.ModifierControl, "A", TextSelectAll},
		{input.KeyUnknown, input.ModifierControl, "z", TextUndo},
		{input.KeyUnknown, input.ModifierControl | input.ModifierShift, "z", TextRedo},
		{input.KeyUnknown, input.ModifierControl, "y", TextRedo},
		{input.KeyHome, 0, "", TextMoveLineStart},
		{input.KeyEnd, input.ModifierShift, "", TextMoveLineEndAndModifySelection},
		{input.KeyHome, input.ModifierControl | input.ModifierShift, "", TextMoveDocumentStartAndModifySelection},
		{input.KeyLeft, input.ModifierControl, "", TextMoveWordLeft},
		{input.KeyRight, input.ModifierControl | input.ModifierShift, "", TextMoveWordRightAndModifySelection},
		{input.KeyLeft, input.ModifierAlt, "", TextMoveWordLeft},
		{input.KeyRight, input.ModifierAlt | input.ModifierShift, "", TextMoveWordRightAndModifySelection},
		{input.KeyBackspace, input.ModifierControl, "", TextDeleteWordBackward},
		{input.KeyDelete, input.ModifierControl, "", TextDeleteWordForward},
		{input.KeyBackspace, input.ModifierMeta, "", TextDeleteLineStart},
		{input.KeyDelete, input.ModifierMeta, "", TextDeleteLineEnd},
	} {
		got, ok := TextEditingCommand(KeyEvent{Key: tc.key, Modifiers: tc.mods, Text: tc.character})
		if !ok || got != tc.command {
			t.Fatalf("%+v: %v %v", tc, got, ok)
		}
	}
	for _, mods := range []input.Modifiers{0, input.ModifierAlt, input.ModifierControl | input.ModifierAlt, input.ModifierControl | input.ModifierShift} {
		if TextClipboardAction(KeyEvent{Modifiers: mods, Text: "v"}) != 0 {
			t.Fatal("modified V was treated as paste")
		}
	}
	for _, tc := range []struct {
		key  input.Key
		mods input.Modifiers
		want byte
	}{
		{input.KeyInsert, input.ModifierControl, 'c'}, {input.KeyInsert, input.ModifierShift, 'v'}, {input.KeyDelete, input.ModifierShift, 'x'},
	} {
		if got := TextClipboardAction(KeyEvent{Key: tc.key, Modifiers: tc.mods}); got != tc.want {
			t.Fatalf("clipboard key=%c", got)
		}
	}
}
