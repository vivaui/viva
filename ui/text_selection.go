package ui

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/base"
)

// TextSelectionState is the selection of one TextSelectionArea; keep it in the model.
type TextSelectionState = base.TextSelectionState

// TextSelection is a selection that may begin in one painted text and end in
// another, for texts an application paints itself; TextPoint is a place in them and
// TextRange the selected part of one; TextSelectionResult says what a pointer event
// did to it.
type (
	TextSelection       = base.TextSelection
	TextPoint           = base.TextPoint
	TextRange           = base.TextRange
	TextSelectionResult = base.TextSelectionResult
)

// TextSelectionArea declares a column whose texts the user can select and copy;
// see base.TextSelectionArea.
//
//	ui.TextSelectionArea(c, &m.details).Gap(6).Children(func() {
//		ui.Text(c, m.fingerprint)
//	})
func TextSelectionArea(c C, state *TextSelectionState) Box {
	useEditMenu()
	installPrimarySelection.Do(func() { base.SetPrimarySelection = app.SetPrimarySelection })
	return base.TextSelectionArea(c, state)
}
