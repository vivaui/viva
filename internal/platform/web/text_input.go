package web

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// TextInputContext describes the focused editable Canvas text-field hit.
type TextInputContext struct {
	ID     ui.ID
	Bounds ui.Rect
	// Layer is the compositing layer captured by the focused hit.
	Layer          uint16
	Multiline      bool
	ReadOnly       bool
	Secure         bool
	NavigationKeys bool
	// Terminal reports a RoleTerminal hit. Plain keys reach the application as
	// text events and the rest as raw key events, as they do natively.
	Terminal bool
	// TypeAhead reports a hit that takes typed text without being a text field.
	TypeAhead bool
	// TextBounds, FontSize, FontWeight and TextColor are the text geometry the component
	// declared; they fill whatever the returned input leaves unset.
	TextBounds         ui.Rect
	FontSize           float32
	FontWeight         float32
	TextColor          ui.Color
	HideNativeOverlays bool
}

func textAlignCSS(alignment ui.Alignment) string {
	switch alignment {
	case ui.AlignCenter:
		return "center"
	case ui.AlignRight:
		return "right"
	default:
		return "left"
	}
}

// TextInputProvider resolves one focused editable hit into an application-owned
// browser editor. Returning nil disables browser text editing.
type TextInputProvider func(TextInputContext) *platform.TextInput

// EditorInputs connects the browser text service straight to Go-owned editors,
// the counterpart of handing the native runtime a TextInput with an Editor.
// lookup names the editor behind a focused hit, or reports false for a hit that
// is not editable; every edit the browser reports is applied to that editor, so
// whatever the editor restricts (accepted runes, length, overwriting) holds
// here as it does natively.
//
// The provider keeps one TextInput and reuses it, so resolving the focused hit
// on every repaint allocates nothing beyond the editor's own snapshot.
func EditorInputs(lookup func(id ui.ID) (*text.Editor, bool)) TextInputProvider {
	var editor *text.Editor
	input := platform.TextInput{
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			if editor == nil {
				return platform.TextState{}, errors.New("web: text event without an active editor")
			}
			event.Apply(editor)
			return platform.EditorState(editor), nil
		},
	}
	return func(context TextInputContext) *platform.TextInput {
		active, ok := lookup(context.ID)
		if !ok {
			editor = nil
			return nil
		}
		editor = active
		input.State = platform.EditorState(editor)
		return &input
	}
}

type TextInput = platform.TextInput
type TextState = platform.TextState
type TextEvent = platform.TextEvent
type TextEventKind = platform.TextEventKind

const (
	TextInsert       = platform.TextInsert
	TextSetMarked    = platform.TextSetMarked
	TextUnmark       = platform.TextUnmark
	TextCommandEvent = platform.TextCommandEvent
	TextSetSelection = platform.TextSetSelection
)

func runeIndexToUTF16(value string, index int) int {
	if index <= 0 {
		return 0
	}
	runes := 0
	units := 0
	for _, current := range value {
		if runes == index {
			break
		}
		if current > 0xffff {
			units += 2
		} else {
			units++
		}
		runes++
	}
	return units
}

func utf16IndexToRune(value string, units int) int {
	if units <= 0 {
		return 0
	}
	consumed := 0
	runes := 0
	for _, current := range value {
		width := 1
		if current > 0xffff {
			width = 2
		}
		if consumed+width > units {
			return runes
		}
		consumed += width
		runes++
		if consumed == units {
			return runes
		}
	}
	return runes
}

func runeRangeToUTF16(value string, selection text.Range) (int, int) {
	selection = normalizedTextRange(selection, utf8.RuneCountInString(value))
	return runeIndexToUTF16(value, selection.Start), runeIndexToUTF16(value, selection.End)
}

func utf16RangeToRunes(value string, start, end int) text.Range {
	return normalizedTextRange(text.Range{
		Start: utf16IndexToRune(value, start),
		End:   utf16IndexToRune(value, end),
	}, utf8.RuneCountInString(value))
}

// inputTextEvent turns an edit the browser made to its input element into the
// event the editor should see.
//
// The page mirrors the editor's selection into the element before the user
// types, so an insertion (typing, pasting, a line break) replaced exactly that
// selection, and it is reported as such: this text over that range. The editor
// then knows where the text landed, which a whole-text replacement cannot tell
// it: that is what lets it overwrite the slot a caret is on, or drop a rejected
// letter without touching its neighbors.
//
// Deletions, cuts, undo, and an insertion that does not line up with the
// mirrored selection are reported as a replacement of the whole text.
func inputTextEvent(previous platform.TextState, next, inputType string) platform.TextEvent {
	switch inputType {
	case "historyUndo":
		return platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextUndo}
	case "historyRedo":
		return platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextRedo}
	}
	if strings.HasPrefix(inputType, "insert") {
		before, after := []rune(previous.Value), []rune(next)
		selection := normalizedTextRange(previous.Selection, len(before))
		tail := len(before) - selection.End
		if len(after) >= selection.Start+tail &&
			runesEqual(after[:selection.Start], before[:selection.Start]) &&
			runesEqual(after[len(after)-tail:], before[selection.End:]) {
			kind := platform.TextInsert
			if inputType == "insertFromPaste" {
				kind = platform.TextPaste
			}
			return platform.TextEvent{
				Kind: kind, Text: string(after[selection.Start : len(after)-tail]),
				Replacement: selection, HasReplacement: true,
			}
		}
	}
	return replaceTextEvent(previous.Value, next)
}

func runesEqual(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func replaceTextEvent(previous, next string) platform.TextEvent {
	return platform.TextEvent{
		Kind:           platform.TextInsert,
		Text:           next,
		Replacement:    text.Range{End: utf8.RuneCountInString(previous)},
		HasReplacement: true,
	}
}

func normalizedTextRange(value text.Range, length int) text.Range {
	value.Start = max(0, min(value.Start, length))
	value.End = max(0, min(value.End, length))
	if value.Start > value.End {
		value.Start, value.End = value.End, value.Start
	}
	return value
}
