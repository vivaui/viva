package app

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// resolveTextInputAlignment resolves a start- or end-relative alignment into the
// physical edge a native editor places its glyphs with. The decision lives in
// internal/platform so an application that drives a native editor directly gets
// exactly what a viva window gets.
func resolveTextInputAlignment(alignment gfx.Alignment, value string, caret int) gfx.Alignment {
	return platform.ResolveTextAlignment(alignment, value, caret)
}

// TextStateOf snapshots editor into the state native text services mirror.
func TextStateOf(editor *text.Editor) TextState {
	selection := editor.Selection()
	composition, composing := editor.Composition()
	return TextState{
		Value:                editor.String(),
		Selection:            appTextRange(selection),
		SelectionReversed:    selection.Start != selection.End && editor.Caret() == selection.Start,
		Composition:          appTextRange(composition),
		Composing:            composing,
		TentativeComposition: editor.TentativeCompositionEnabled(),
		CompositionIdentity:  editor.TentativeCompositionIdentity(),
	}
}

func platformTextState(state TextState) platform.TextState {
	identity := state.CompositionIdentity
	if !state.TentativeComposition {
		identity = text.CompositionIdentity{}
	}
	return platform.TextState{
		Value:                state.Value,
		Selection:            platformTextRange(state.Selection),
		SelectionReversed:    state.SelectionReversed,
		Composition:          platformTextRange(state.Composition),
		Composing:            state.Composing,
		TentativeComposition: state.TentativeComposition,
		Redraw:               state.Redraw,
		CompositionIdentity:  identity,
	}
}

func appTextEvent(event platform.TextEvent) TextEvent {
	return TextEvent{
		Kind: TextEventKind(event.Kind), Text: event.Text, Rectangular: event.Rectangular,
		Selection:           appTextRange(event.Selection),
		MarkedSelection:     appTextRange(event.MarkedSelection),
		Replacement:         appTextRange(event.Replacement),
		HasReplacement:      event.HasReplacement,
		Command:             TextCommand(event.Command),
		CompositionPhase:    event.CompositionPhase,
		ReplacementUTF16:    event.ReplacementUTF16,
		HasReplacementUTF16: event.HasReplacementUTF16,
	}
}

func platformTextRange(value TextRange) text.Range {
	return text.Range{Start: value.Start, End: value.End}
}

func appTextRange(value text.Range) TextRange {
	return TextRange{Start: value.Start, End: value.End}
}
