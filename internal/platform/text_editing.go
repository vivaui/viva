package platform

import (
	"strings"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/text"
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
// End, the word and document chords, and the word deletes. The primary modifier is
// Control or Command, matching the platform that delivers the event; the headless
// driver uses Control so its tests stay faithful. Live keyboard state is not
// consulted.
func TextEditingCommand(event KeyEvent) (TextCommand, bool) {
	if command, ok := TextHistoryCommand(event); ok {
		return command, true
	}
	mods := event.Modifiers
	if (mods == input.ModifierControl || mods == input.ModifierMeta) && strings.EqualFold(event.Text, "a") {
		return TextSelectAll, true
	}
	if mods & ^(input.ModifierControl|input.ModifierShift) != 0 {
		return 0, false
	}
	ctrl, shift := mods&input.ModifierControl != 0, mods&input.ModifierShift != 0
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
		if ctrl {
			if shift {
				return TextMoveWordLeftAndModifySelection, true
			}
			return TextMoveWordLeft, true
		}
	case input.KeyRight:
		if ctrl {
			if shift {
				return TextMoveWordRightAndModifySelection, true
			}
			return TextMoveWordRight, true
		}
	case input.KeyBackspace:
		if ctrl {
			return TextDeleteWordBackward, true
		}
	case input.KeyDelete:
		if ctrl {
			return TextDeleteWordForward, true
		}
	}
	return 0, false
}

// AppleTextEditingCommand maps the macOS navigation chords AppKit's text system
// turns into commands: Option with Left and Right moves by word and with Backspace
// and Delete removes one; Command with Left and Right goes to the ends of the line,
// with Up and Down to the ends of the text, and with Backspace removes to the line
// start. Shift extends a move. Command with Forward Delete is bound to nothing. A line is the text up to a newline; a
// field that wraps and lays its own rows out takes Command with an arrow as a key
// event instead. The native macOS backend gets the same commands from AppKit's
// selectors, so this is for the headless driver, which has no AppKit; it is not
// TextEditingCommand, whose Control chords are what Windows and Linux use.
func AppleTextEditingCommand(event KeyEvent) (TextCommand, bool) {
	shift := event.Modifiers&input.ModifierShift != 0
	switch event.Modifiers &^ input.ModifierShift {
	case input.ModifierAlt:
		switch event.Key {
		case input.KeyLeft:
			if shift {
				return TextMoveWordLeftAndModifySelection, true
			}
			return TextMoveWordLeft, true
		case input.KeyRight:
			if shift {
				return TextMoveWordRightAndModifySelection, true
			}
			return TextMoveWordRight, true
		case input.KeyBackspace:
			if !shift {
				return TextDeleteWordBackward, true
			}
		case input.KeyDelete:
			if !shift {
				return TextDeleteWordForward, true
			}
		}
	case input.ModifierMeta:
		switch event.Key {
		case input.KeyLeft:
			if shift {
				return TextMoveLineStartAndModifySelection, true
			}
			return TextMoveLineStart, true
		case input.KeyRight:
			if shift {
				return TextMoveLineEndAndModifySelection, true
			}
			return TextMoveLineEnd, true
		case input.KeyUp:
			if shift {
				return TextMoveDocumentStartAndModifySelection, true
			}
			return TextMoveDocumentStart, true
		case input.KeyDown:
			if shift {
				return TextMoveDocumentEndAndModifySelection, true
			}
			return TextMoveDocumentEnd, true
		case input.KeyBackspace:
			if !shift {
				return TextDeleteToLineStart, true
			}
		}
	}
	return 0, false
}

// TextClipboardEvent performs clipboard I/O before returning an edit. A failed
// read/write never changes the document, so a field with no selection and a broken
// pasteboard keeps its text; secure fields never copy or cut. A cut is an empty
// TextPaste, which the editor applies as a delete in the same undo unit. A paste
// with no text on the clipboard is a TextPasteNoText, so that a field which also
// takes pictures can look for one.
func TextClipboardEvent(state TextState, secure bool, action byte, read func() (string, error), write func(string) error) (TextEvent, bool, error) {
	if action == 'v' {
		value, err := read()
		if err != nil {
			return TextEvent{}, false, err
		}
		if value == "" {
			return TextEvent{Kind: TextPasteNoText}, true, nil
		}
		return TextEvent{Kind: TextPaste, Text: value}, true, nil
	}
	if secure || action != 'c' && action != 'x' {
		return TextEvent{}, false, nil
	}
	start, end := text.ByteRange(state.Value, text.Range{Start: max(state.Selection.Start, 0), End: max(state.Selection.Start, state.Selection.End)})
	if start == end {
		return TextEvent{}, false, nil
	}
	if err := write(state.Value[start:end]); err != nil {
		return TextEvent{}, false, err
	}
	if action == 'x' {
		return TextEvent{Kind: TextPaste}, true, nil
	}
	return TextEvent{}, false, nil
}
