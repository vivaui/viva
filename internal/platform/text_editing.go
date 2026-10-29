package platform

import (
	"strings"

	"github.com/vivaui/viva/internal/input"
)

// TextClipboardAction recognizes the standard clipboard chords and returns 'c',
// 'x' or 'v', or 0 for any other key. The chords are Copy/Cut/Paste with Control
// or Command alone, Control+Insert, Shift+Insert and Shift+Delete. The application
// keeps ownership of these on terminals; ordinary editors handle them locally.
func TextClipboardAction(event KeyEvent) byte {
	primary := event.Modifiers == input.ModifierControl || event.Modifiers == input.ModifierMeta
	if primary {
		switch strings.ToLower(event.Text) {
		case "c":
			return 'c'
		case "x":
			return 'x'
		case "v":
			return 'v'
		}
		if event.Key == input.KeyInsert {
			return 'c'
		}
	}
	if event.Modifiers == input.ModifierShift {
		if event.Key == input.KeyInsert {
			return 'v'
		}
		if event.Key == input.KeyDelete {
			return 'x'
		}
	}
	return 0
}

// TextEditingCommand maps the conventional editing keys: Control+Command+A, Home,
// End, the word and document chords, and the word and line deletes. Control or
// Command is the primary modifier; Alt/Option also moves by word with Left and
// Right, matching native editors on macOS and browser text controls. Live
// keyboard state is not consulted.
func TextEditingCommand(event KeyEvent) (TextCommand, bool) {
	if command, ok := TextHistoryCommand(event); ok {
		return command, true
	}
	mods := event.Modifiers
	if (mods == input.ModifierControl || mods == input.ModifierMeta) && strings.EqualFold(event.Text, "a") {
		return TextSelectAll, true
	}
	primary := mods & (input.ModifierControl | input.ModifierMeta)
	if primary == (input.ModifierControl|input.ModifierMeta) ||
		(mods&input.ModifierAlt != 0 && (primary != 0 || event.Key != input.KeyLeft && event.Key != input.KeyRight)) {
		return 0, false
	}
	ctrl, command, option, shift := primary == input.ModifierControl, primary == input.ModifierMeta, mods&input.ModifierAlt != 0, mods&input.ModifierShift != 0
	switch event.Key {
	case input.KeyHome:
		if ctrl {
			if shift {
				return TextMoveDocumentStartAndModifySelection, true
			}
			return TextMoveDocumentStart, true
		}
		if shift {
			return TextMoveLineStartAndModifySelection, true
		}
		return TextMoveLineStart, true
	case input.KeyEnd:
		if ctrl {
			if shift {
				return TextMoveDocumentEndAndModifySelection, true
			}
			return TextMoveDocumentEnd, true
		}
		if shift {
			return TextMoveLineEndAndModifySelection, true
		}
		return TextMoveLineEnd, true
	case input.KeyLeft:
		if ctrl || option {
			if shift {
				return TextMoveWordLeftAndModifySelection, true
			}
			return TextMoveWordLeft, true
		}
	case input.KeyRight:
		if ctrl || option {
			if shift {
				return TextMoveWordRightAndModifySelection, true
			}
			return TextMoveWordRight, true
		}
	case input.KeyBackspace:
		if command {
			return TextDeleteLineStart, true
		}
		if ctrl {
			return TextDeleteWordBackward, true
		}
	case input.KeyDelete:
		if command {
			return TextDeleteLineEnd, true
		}
		if ctrl {
			return TextDeleteWordForward, true
		}
	}
	return 0, false
}

// TextClipboardEvent performs clipboard I/O before returning an edit. A failed
// read/write never changes the document, so a field with no selection and a broken
// pasteboard keeps its text; secure fields never copy or cut. A cut is an empty
// TextPaste, which the editor applies as a delete in the same undo unit.
func TextClipboardEvent(state TextState, secure bool, action byte, read func() (string, error), write func(string) error) (TextEvent, bool, error) {
	if action == 'v' {
		value, err := read()
		if err != nil || value == "" {
			return TextEvent{}, false, err
		}
		return TextEvent{Kind: TextPaste, Text: value}, true, nil
	}
	if secure || action != 'c' && action != 'x' {
		return TextEvent{}, false, nil
	}
	runes := []rune(state.Value)
	start := max(0, min(state.Selection.Start, len(runes)))
	end := max(start, min(state.Selection.End, len(runes)))
	if start == end {
		return TextEvent{}, false, nil
	}
	if err := write(string(runes[start:end])); err != nil {
		return TextEvent{}, false, err
	}
	if action == 'x' {
		return TextEvent{Kind: TextPaste}, true, nil
	}
	return TextEvent{}, false, nil
}
