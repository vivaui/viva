package app

import "github.com/vivaui/viva/window"

type TextRange = window.TextRange
type TextState = window.TextState
type TextEventKind = window.TextEventKind
type TextCommand = window.TextCommand
type TextEvent = window.TextEvent
type TextInputContext = window.TextInputContext
type TextInputProvider = window.TextInputProvider
type TextInput = window.TextInput

const (
	TextInsert                              = window.TextInsert
	TextSetMarked                           = window.TextSetMarked
	TextUnmark                              = window.TextUnmark
	TextCommandEvent                        = window.TextCommandEvent
	TextSetSelection                        = window.TextSetSelection
	TextPaste                               = window.TextPaste
	TextPasteNoText                         = window.TextPasteNoText
	TextCancelComposition                   = window.TextCancelComposition
	TextDeleteBackward                      = window.TextDeleteBackward
	TextDeleteForward                       = window.TextDeleteForward
	TextMoveLeft                            = window.TextMoveLeft
	TextMoveRight                           = window.TextMoveRight
	TextMoveLeftAndModifySelection          = window.TextMoveLeftAndModifySelection
	TextMoveRightAndModifySelection         = window.TextMoveRightAndModifySelection
	TextInsertNewline                       = window.TextInsertNewline
	TextInsertTab                           = window.TextInsertTab
	TextInsertLineBreak                     = window.TextInsertLineBreak
	TextUndo                                = window.TextUndo
	TextRedo                                = window.TextRedo
	TextSelectAll                           = window.TextSelectAll
	TextMoveLineStart                       = window.TextMoveLineStart
	TextMoveLineEnd                         = window.TextMoveLineEnd
	TextMoveLineStartAndModifySelection     = window.TextMoveLineStartAndModifySelection
	TextMoveLineEndAndModifySelection       = window.TextMoveLineEndAndModifySelection
	TextMoveDocumentStart                   = window.TextMoveDocumentStart
	TextMoveDocumentEnd                     = window.TextMoveDocumentEnd
	TextMoveDocumentStartAndModifySelection = window.TextMoveDocumentStartAndModifySelection
	TextMoveDocumentEndAndModifySelection   = window.TextMoveDocumentEndAndModifySelection
	TextMoveWordLeft                        = window.TextMoveWordLeft
	TextMoveWordRight                       = window.TextMoveWordRight
	TextMoveWordLeftAndModifySelection      = window.TextMoveWordLeftAndModifySelection
	TextMoveWordRightAndModifySelection     = window.TextMoveWordRightAndModifySelection
	TextDeleteWordBackward                  = window.TextDeleteWordBackward
	TextDeleteWordForward                   = window.TextDeleteWordForward
	TextDeleteToLineStart                   = window.TextDeleteToLineStart
	TextDeleteToLineEnd                     = window.TextDeleteToLineEnd
)
