package app

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// TextRange is a half-open rune-indexed range.
type TextRange struct {
	Start int
	End   int
}

// TextState is the complete application-owned native editor state.
type TextState struct {
	// SelectionReversed preserves the moving end of a backwards selection.
	SelectionReversed bool
	Value             string
	Selection         TextRange
	Composition       TextRange
	Composing         bool
	// Redraw asks for a frame after the event although the editor looks the same,
	// because the application changed something else in response. A terminal
	// uses it when typed text appears ahead of the server's echo; ordinary
	// terminal input leaves it false and costs no frame.
	Redraw bool
}

// TextEventKind identifies one native text-system mutation.
type TextEventKind uint8

const (
	TextInsert TextEventKind = iota + 1
	TextSetMarked
	TextUnmark
	TextCommandEvent
	TextSetSelection
	// TextPaste is one pasteboard edit. It is a separate undo unit in an
	// ordinary editor; a terminal receives it as is, without line breaks
	// flattened, and decides how to forward it.
	TextPaste
	// TextPasteNoText is a paste asked for while the clipboard holds no text. It
	// changes nothing in an editor (TextEvent.Apply ignores it). A field that
	// takes more than text, such as a chat box that accepts a screenshot, uses it
	// as the cue to look at the clipboard's other formats (ClipboardImagePNG).
	// Terminals never receive it.
	TextPasteNoText
)

// TextCommand identifies one platform-normalized editing command.
type TextCommand uint8

const (
	TextDeleteBackward TextCommand = iota + 1
	TextDeleteForward
	TextMoveLeft
	TextMoveRight
	TextMoveLeftAndModifySelection
	TextMoveRightAndModifySelection
	TextInsertNewline
	TextInsertTab
	// TextInsertLineBreak is a line break asked for without ending the entry:
	// Shift+Enter (and Option or Control+Enter on macOS). A chat box submits on
	// TextInsertNewline and adds a line on this one.
	TextInsertLineBreak
	TextUndo
	TextRedo
	TextSelectAll
	TextMoveLineStart
	TextMoveLineEnd
	TextMoveLineStartAndModifySelection
	TextMoveLineEndAndModifySelection
	TextMoveDocumentStart
	TextMoveDocumentEnd
	TextMoveDocumentStartAndModifySelection
	TextMoveDocumentEndAndModifySelection
	TextMoveWordLeft
	TextMoveWordRight
	TextMoveWordLeftAndModifySelection
	TextMoveWordRightAndModifySelection
	TextDeleteWordBackward
	TextDeleteWordForward
	// TextDeleteToLineStart and TextDeleteToLineEnd remove the text between the
	// caret and the start or end of its line (Command+Backspace and
	// Command+Delete on macOS).
	TextDeleteToLineStart
	TextDeleteToLineEnd
)

// TextEvent contains rune-indexed selection and replacement ranges.
type TextEvent struct {
	Kind            TextEventKind
	Text            string
	Selection       TextRange
	MarkedSelection TextRange
	Replacement     TextRange
	HasReplacement  bool
	Command         TextCommand
}

// TextInputContext describes the focused Canvas text-field hit.
type TextInputContext struct {
	ID     ui.ID
	Bounds ui.Rect
	// Layer is the compositing layer captured by the focused hit.
	Layer     uint16
	Multiline bool
	ReadOnly  bool
	Secure    bool
	// NavigationKeys reports a field that asked for Up, Down, PageUp and
	// PageDown to be delivered to the application as raw ui.KeyEvents.
	NavigationKeys bool
	// Terminal reports a RoleTerminal hit. The native editor then is a
	// preedit surface in the fixed-pitch font: it draws only the text being
	// composed, with no caret or selection, and Bounds should be the cursor
	// cell. Plain keys reach the application as TextInsert events and the
	// rest as raw ui.KeyEvents (see ui.KeyEvent).
	Terminal bool
	// TextBounds, FontSize and TextColor are the text geometry the component
	// declared for this field. They fill whatever the returned TextInput leaves
	// unset, so a bare &TextInput{Editor: editor} draws its text exactly where
	// the field painted it; set them only to deliberately override.
	TextBounds ui.Rect
	FontSize   float32
	TextColor  ui.Color
	// HideNativeOverlays requests that the platform editor keep its own caret
	// and selection invisible. Editing, IME and clipboard behavior are
	// unchanged. Components that paint those overlays themselves set it.
	HideNativeOverlays bool
}

// TextInputProvider resolves one focused text-field hit. Returning nil disables
// native editing without retaining a hidden first responder.
type TextInputProvider func(TextInputContext) *TextInput

// TextInput describes one application-owned native editor surface. Bounds are
// logical window pixels. Bounds, FontSize and Color default to the text
// geometry the focused component declared (see TextInputContext), which is
// what keeps the text from jumping when a field gains focus.
type TextInput struct {
	Bounds   ui.Rect
	FontSize float32
	Color    ui.Color
	MonoFace ui.MonoFace
	// AltSendsEscape asks printable Alt/Option chords on a terminal to arrive as
	// raw key events with the unmodified character in KeyEvent.Text. Terminal
	// encoders can then send the conventional Escape-prefixed Meta sequence.
	// It is ignored for ordinary text fields, and input-method composition keeps
	// ownership of every key until it commits. The native policy is currently
	// implemented on macOS; app.Driver implements it for headless verification.
	AltSendsEscape bool
	// KeyReleases asks a terminal for the whole keyboard as presses and
	// releases: the modifier keys, printable keys (as raw KeyEvents with Text,
	// not text events, unless an input method consumed them) and a KeyEvent
	// with Release set when each key goes up. It is for a terminal that shows
	// another machine's desktop, where a held key or modifier means something
	// there, and for a terminal implementing a keyboard protocol (kitty's
	// progressive enhancement or win32-input-mode). Releases are never delivered to a handler that did not ask, every
	// release matches a press the handler saw, and keys still down when the
	// terminal loses the keyboard or the window loses focus are released
	// first. Committed text from an input method still arrives as text events.
	// Implemented on macOS, Windows and X11, and by app.Driver.
	KeyReleases bool
	// NavigationKeys asks Up, Down, PageUp and PageDown to reach the
	// application as raw ui.KeyEvents while the field is focused, instead
	// of being consumed by the text system or moving its caret. Use it
	// for a search field, combobox or command palette with a dropdown.
	// Cursor placement, selection, Home/End, word navigation and
	// input-method composition remain native. Defaults to the component's
	// declaration (see TextInputContext.NavigationKeys).
	// Honored by the macOS text system and app.Driver; the other platform
	// backends do not consult it yet.
	NavigationKeys bool
	// Editor connects the native text service straight to a text.Editor: the
	// runtime mirrors the editor into the platform and applies every native
	// event to it, so State and OnEvent are ignored. Leave it nil to own the
	// event handling yourself, for example to filter or transform edits.
	Editor  *text.Editor
	State   TextState
	OnEvent func(TextEvent) (TextState, error)
	// HideNativeOverlays keeps the platform's caret and selection invisible
	// while native editing continues. It combines with the declaration on the
	// focused Canvas hit.
	HideNativeOverlays bool
}

// TextStateOf snapshots editor into the state native text services mirror.
func TextStateOf(editor *text.Editor) TextState {
	selection := editor.Selection()
	composition, composing := editor.Composition()
	return TextState{
		Value:             editor.String(),
		Selection:         appTextRange(selection),
		SelectionReversed: selection.Start != selection.End && editor.Caret() == selection.Start,
		Composition:       appTextRange(composition),
		Composing:         composing,
	}
}

// Apply routes one native text event into editor. The event kinds and commands are
// the platform's, value for value, so the one switch that maps them lives in
// internal/platform rather than in a second copy here.
func (event TextEvent) Apply(editor *text.Editor) {
	platform.TextEvent{
		Kind:            platform.TextEventKind(event.Kind),
		Text:            event.Text,
		Selection:       platformTextRange(event.Selection),
		MarkedSelection: platformTextRange(event.MarkedSelection),
		Replacement:     platformTextRange(event.Replacement),
		HasReplacement:  event.HasReplacement,
		Command:         platform.TextCommand(event.Command),
	}.Apply(editor)
}

func platformTextRange(value TextRange) text.Range {
	return text.Range{Start: value.Start, End: value.End}
}

func appTextRange(value text.Range) TextRange {
	return TextRange{Start: value.Start, End: value.End}
}
