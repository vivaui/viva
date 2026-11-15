package text

import "slices"

type lfLabelComposition struct {
	active, sharedHistory, recorded bool
	line, currentLF                 int
	original, unitOriginal          []*lfLabelCell
	prefix                          []lfLabelEdit
	history                         map[*rune]lfLabelEntry
	heldRunes, extraRunes           int
}

func holdLFLabelComposition(state *lfLabelState, change textChange) {
	composition := lfLabelComposition{
		active:        true,
		sharedHistory: true,
		line:          state.lastLine,
		currentLF:     countLabelLF(change.inserted),
		history:       state.history,
	}
	for _, entry := range composition.history {
		composition.heldRunes += entry.retainedRunes()
	}
	if len(state.pending) != 0 {
		composition.original = state.pending[0].removed
	} else if count := countLabelLF(change.removed); count != 0 {
		composition.original = slices.Clone(state.current[composition.line : composition.line+count])
	}
	state.composition = composition
	syncLFLabelExtraCost(state)
}

func ensureLFLabelHistoryOwned(state *lfLabelState) {
	if state.composition.active && state.composition.sharedHistory {
		state.history = cloneLFLabelHistory(state.history)
		state.composition.sharedHistory = false
		syncLFLabelExtraCost(state)
	}
}

func compositionLFLabelEdits(state *lfLabelState, previous *rune) []lfLabelEdit {
	composition := &state.composition
	if !composition.recorded {
		composition.prefix = composition.history[previous].edits
		if len(state.pending) != 0 {
			composition.unitOriginal = state.pending[0].removed
		} else if composition.currentLF != 0 {
			composition.unitOriginal = slices.Clone(state.current[composition.line : composition.line+composition.currentLF])
		}
		composition.recorded = true
	}
	edits := slices.Clone(composition.prefix)
	if len(composition.unitOriginal) != 0 || composition.currentLF != 0 {
		edits = append(edits, lfLabelEdit{
			line:     composition.line,
			removed:  composition.unitOriginal,
			inserted: slices.Clone(state.current[composition.line : composition.line+composition.currentLF]),
		})
	}
	return edits
}

func cancelLFLabelComposition(state *lfLabelState) {
	composition := state.composition
	if !composition.active {
		return
	}
	if state.staging && (composition.currentLF != 0 || len(composition.original) != 0) {
		state.events = append(state.events, lfLabelEdit{
			line:     composition.line,
			removed:  slices.Clone(state.current[composition.line : composition.line+composition.currentLF]),
			inserted: composition.original,
		})
	}
	spliceLFLabels(state, composition.line, composition.currentLF, composition.original)
	state.history = composition.history
	state.pending = nil
	state.composition = lfLabelComposition{}
}

func releaseLFLabelComposition(state *lfLabelState) {
	if !state.owner.composing {
		state.owner.historyRunes -= state.composition.extraRunes
		state.composition = lfLabelComposition{}
	}
}

func discardLFLabelHeldHistory(state *lfLabelState) {
	if state.composition.active {
		state.composition.history = nil
		state.composition.heldRunes = 0
		state.composition.sharedHistory = false
		state.composition.recorded = false
		state.composition.unitOriginal = nil
		state.composition.prefix = nil
		syncLFLabelExtraCost(state)
	}
}
