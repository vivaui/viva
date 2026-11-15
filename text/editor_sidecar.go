package text

import "unsafe"

// The two opt-in capabilities share one journal slot; ordinary byte/line/save
// clients allocate neither this header nor either capability state.
type editorSidecar struct {
	history     *historyJournal
	composition *compositionState
}

const editorSidecarRunes = int((unsafe.Sizeof(editorSidecar{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))

func (journal *editJournal) historyJournal() *historyJournal {
	if journal == nil || journal.optional == nil {
		return nil
	}
	return journal.optional.history
}

func (journal *editJournal) compositionState() *compositionState {
	if journal == nil || journal.optional == nil {
		return nil
	}
	return journal.optional.composition
}

func (journal *editJournal) setHistoryJournal(history *historyJournal) {
	if journal.optional == nil {
		journal.optional = &editorSidecar{}
	}
	journal.optional.history = history
}

func (journal *editJournal) setCompositionState(state *compositionState) {
	if state == nil {
		if journal.optional != nil {
			journal.optional.composition = nil
			if journal.optional.history == nil {
				journal.optional = nil
			}
		}
		return
	}
	if journal.optional == nil {
		journal.optional = &editorSidecar{}
	}
	journal.optional.composition = state
}

func (editor *Editor) compositionBaseCost() int {
	cost := compositionHeaderRunes
	// A retained history header owns the shared slot even after invalidation.
	if editor.journal.historyJournal() == nil {
		cost += editorSidecarRunes
	}
	return cost
}
