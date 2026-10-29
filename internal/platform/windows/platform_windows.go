// Package windows implements the native Windows application and D3D11 backend.
package windows

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type Color = platform.Color
type Config = platform.Config
type TextInput = platform.TextInput
type TextState = platform.TextState
type TextEventKind = platform.TextEventKind
type TextCommand = platform.TextCommand
type TextEvent = platform.TextEvent
type TextHandler = platform.TextHandler

const (
	TextInsert                      = platform.TextInsert
	TextSetMarked                   = platform.TextSetMarked
	TextUnmark                      = platform.TextUnmark
	TextCommandEvent                = platform.TextCommandEvent
	TextSetSelection                = platform.TextSetSelection
	TextDeleteBackward              = platform.TextDeleteBackward
	TextDeleteForward               = platform.TextDeleteForward
	TextMoveLeft                    = platform.TextMoveLeft
	TextMoveRight                   = platform.TextMoveRight
	TextMoveLeftAndModifySelection  = platform.TextMoveLeftAndModifySelection
	TextMoveRightAndModifySelection = platform.TextMoveRightAndModifySelection
	TextInsertNewline               = platform.TextInsertNewline
	TextInsertTab                   = platform.TextInsertTab
	TextInsertLineBreak             = platform.TextInsertLineBreak
	TextUndo                        = platform.TextUndo
	TextRedo                        = platform.TextRedo
	TextDeleteLineStart             = platform.TextDeleteLineStart
	TextDeleteLineEnd               = platform.TextDeleteLineEnd
)

func EditorState(editor *text.Editor) TextState { return platform.EditorState(editor) }
