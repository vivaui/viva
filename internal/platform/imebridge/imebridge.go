// Package imebridge turns what a phone's input method does into the text events
// the application's editor takes. Android's InputConnection and iOS's UITextInput
// both speak in UTF-16 ranges of a copy of the text, and commit, mark, replace and
// select in ways that differ from a desktop's; the bridge reads each operation
// against the editor's state as it stands, so an operation that follows another sees
// what the first did. It has no system call in it and is tested on the host.
package imebridge

import (
	"unicode/utf16"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// Session is one focused editor as the input method edits it: the state the
// application last gave, and the handler that applies an event and answers with the
// state after it.
type Session struct {
	Handler platform.TextHandler
	State   platform.TextState
}

// Mirror is the text and the ranges an input method reads, in UTF-16 units.
type Mirror struct {
	Text                 string
	SelStart, SelEnd     int
	CompStart, CompEnd   int
	Composing            bool
	SelectionEndIsCaret  bool
	TentativeComposition bool
}

// Mirror is the state as the input method counts it.
func (session *Session) Mirror() Mirror { return MirrorOf(session.State) }

// MirrorOf is state counted in UTF-16 units.
func MirrorOf(state platform.TextState) Mirror {
	mirror := Mirror{
		Text:                state.Value,
		SelStart:            text.UTF16Offset(state.Value, state.Selection.Start),
		SelEnd:              text.UTF16Offset(state.Value, state.Selection.End),
		SelectionEndIsCaret: !state.SelectionReversed,
		Composing:           state.Composing,
	}
	if state.Composing {
		mirror.CompStart = text.UTF16Offset(state.Value, state.Composition.Start)
		mirror.CompEnd = text.UTF16Offset(state.Value, state.Composition.End)
	} else {
		mirror.CompStart, mirror.CompEnd = -1, -1
	}
	return mirror
}

// apply sends one event and takes the state after it.
func (session *Session) apply(event platform.TextEvent) error {
	next, err := session.Handler(event)
	if err != nil {
		return err
	}
	session.State = next
	return nil
}

// units is the number of UTF-16 units of value.
func units(value string) int { return len(utf16.Encode([]rune(value))) }

// runeRange is the rune range of the UTF-16 range [start, end) of value, clamped to the
// text and ordered.
func runeRange(value string, start, end int) text.Range {
	length := units(value)
	start, end = min(max(start, 0), length), min(max(end, 0), length)
	if end < start {
		start, end = end, start
	}
	return text.Range{Start: text.RuneIndexAtUTF16(value, start), End: text.RuneIndexAtUTF16(value, end)}
}

// Commit puts value in place of the composing text, or of the selection when nothing is
// composing, and ends the composition: InputConnection.commitText and UIKeyInput's
// insertText. newCursor places the caret as commitText does: above zero, after the text
// counted from its end (1 is just after); zero or below, before it counted from its start.
func (session *Session) Commit(value string, newCursor int) error {
	event := platform.TextEvent{Kind: platform.TextInsert, Text: value}
	if session.State.Composing {
		event.Replacement, event.HasReplacement = session.State.Composition, true
	}
	if err := session.apply(event); err != nil {
		return err
	}
	if newCursor == 1 {
		return nil
	}
	after := MirrorOf(session.State)
	end := after.SelEnd
	target := end + newCursor - 1
	if newCursor <= 0 {
		target = end - units(value) + newCursor
	}
	target = min(max(target, 0), units(after.Text))
	if target == end {
		return nil
	}
	return session.apply(platform.TextEvent{Kind: platform.TextSetSelection, Selection: runeRange(after.Text, target, target)})
}

// SetComposing replaces the composing text, or the selection when nothing is composing,
// with value, which stays composing: InputConnection.setComposingText. The caret is
// placed inside it as commitText's newCursor does.
func (session *Session) SetComposing(value string, newCursor int) error {
	length := units(value)
	caret := newCursor
	if newCursor > 0 {
		caret = length + newCursor - 1
	}
	return session.SetMarked(value, caret, caret)
}

// SetMarked replaces the composing text, or the selection, with value, composing, and
// selects [start, end) of it in UTF-16 units: UITextInput's setMarkedText.
func (session *Session) SetMarked(value string, start, end int) error {
	event := platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: runeRange(value, start, end)}
	if session.State.Composing {
		event.Replacement, event.HasReplacement = session.State.Composition, true
	}
	return session.apply(event)
}

// Finish ends the composition, keeping what is composed: finishComposingText and
// unmarkText.
func (session *Session) Finish() error {
	if !session.State.Composing {
		return nil
	}
	return session.apply(platform.TextEvent{Kind: platform.TextUnmark})
}

// DeleteSurrounding deletes before units before the selection and after units after it,
// counting UTF-16 units, or code points when codePoints says so.
func (session *Session) DeleteSurrounding(before, after int, codePoints bool) error {
	mirror := MirrorOf(session.State)
	start, end := mirror.SelStart, mirror.SelEnd
	if start > end {
		start, end = end, start
	}
	if codePoints {
		runes := []rune(mirror.Text)
		first := text.RuneIndexAtUTF16(mirror.Text, start)
		last := text.RuneIndexAtUTF16(mirror.Text, end)
		first = max(first-before, 0)
		last = min(last+after, len(runes))
		return session.replace(text.Range{Start: first, End: last}, "")
	}
	return session.replace(runeRange(mirror.Text, start-before, end+after), "")
}

// Replace puts value in place of the UTF-16 range [start, end): UITextInput's
// replaceRange, an autocorrection or a suggestion taken.
func (session *Session) Replace(start, end int, value string) error {
	return session.replace(runeRange(session.State.Value, start, end), value)
}

func (session *Session) replace(replacement text.Range, value string) error {
	if replacement.Start == replacement.End && value == "" {
		return nil
	}
	return session.apply(platform.TextEvent{Kind: platform.TextInsert, Text: value, Replacement: replacement, HasReplacement: true})
}

// Select selects [start, end) in UTF-16 units: setSelection and selectedTextRange.
func (session *Session) Select(start, end int) error {
	return session.apply(platform.TextEvent{Kind: platform.TextSetSelection, Selection: runeRange(session.State.Value, start, end)})
}

// MarkRange makes text that is already there the composing text: setComposingRegion,
// which a keyboard uses to reopen a word it is about to correct.
func (session *Session) MarkRange(start, end int) error {
	value := session.State.Value
	rng := runeRange(value, start, end)
	if rng.Start == rng.End {
		return nil
	}
	marked := string([]rune(value)[rng.Start:rng.End])
	size := rng.End - rng.Start
	return session.apply(platform.TextEvent{
		Kind: platform.TextSetMarked, Text: marked, MarkedSelection: text.Range{Start: size, End: size},
		Replacement: rng, HasReplacement: true,
	})
}

// Command applies an editing command: deleteBackward of a hardware or software key.
func (session *Session) Command(command platform.TextCommand) error {
	return session.apply(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
}

// KeyEdit is what a key of a hardware keyboard does in a focused text input: type its
// character, or edit as the key says, which is how the desktops' native text services
// take the keys of a text field. A key it does not answer for, such as Enter in a field of
// one line, is for the application, and so are all the keys of a terminal.
func KeyEdit(key gfx.Key, modifiers input.Modifiers, character string, field *platform.TextInput) (KeyAction, bool) {
	if field.Terminal {
		return KeyAction{}, false
	}
	word := modifiers&(input.ModifierControl|input.ModifierAlt) != 0
	shift := modifiers&input.ModifierShift != 0
	choose := func(plain, selecting, byWord, selectingByWord platform.TextCommand) (KeyAction, bool) {
		command := plain
		switch {
		case shift && word:
			command = selectingByWord
		case word:
			command = byWord
		case shift:
			command = selecting
		}
		return KeyAction{Command: command}, command != 0
	}
	switch key {
	case gfx.KeyBackspace:
		return choose(platform.TextDeleteBackward, platform.TextDeleteBackward, platform.TextDeleteWordBackward, platform.TextDeleteWordBackward)
	case gfx.KeyDelete:
		return choose(platform.TextDeleteForward, platform.TextDeleteForward, platform.TextDeleteWordForward, platform.TextDeleteWordForward)
	case gfx.KeyLeft:
		return choose(platform.TextMoveLeft, platform.TextMoveLeftAndModifySelection, platform.TextMoveWordLeft, platform.TextMoveWordLeftAndModifySelection)
	case gfx.KeyRight:
		return choose(platform.TextMoveRight, platform.TextMoveRightAndModifySelection, platform.TextMoveWordRight, platform.TextMoveWordRightAndModifySelection)
	case gfx.KeyHome:
		return choose(platform.TextMoveLineStart, platform.TextMoveLineStartAndModifySelection, platform.TextMoveLineStart, platform.TextMoveLineStartAndModifySelection)
	case gfx.KeyEnd:
		return choose(platform.TextMoveLineEnd, platform.TextMoveLineEndAndModifySelection, platform.TextMoveLineEnd, platform.TextMoveLineEndAndModifySelection)
	case gfx.KeyEnter:
		if field.Multiline {
			return KeyAction{Command: platform.TextInsertNewline}, true
		}
	case gfx.KeySpace:
		return KeyAction{Text: " "}, true
	case gfx.KeyUnknown:
		if character != "" {
			return KeyAction{Text: character}, true
		}
	}
	return KeyAction{}, false
}

// KeyAction is what KeyEdit decided: a command to apply, or text to commit.
type KeyAction struct {
	Command platform.TextCommand
	Text    string
}

// Apply does the action to the session.
func (action KeyAction) Apply(session *Session) error {
	if action.Command != 0 {
		return session.Command(action.Command)
	}
	return session.Commit(action.Text, 1)
}
