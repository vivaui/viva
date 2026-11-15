package text

// Only TrackHistory installs these functions, so ordinary editor callers do
// not cause the complete producer implementation to be linked.
type historyOps struct {
	record            func(*historyJournal, textChange, editKind) (Edit, bool)
	capturePrimitives func(*historyJournal, []Edit)
	save              func(*historyJournal)
	replay            func(*historyJournal, int, bool)
	clear             func(*historyJournal, bool)
	invalidateFrom    func(*historyJournal, int)
	enforce           func(*historyJournal)
}

func (editor *Editor) historyState() *historyJournal {
	if editor == nil || editor.journal == nil {
		return nil
	}
	history := editor.journal.history
	if history == nil || history.owner != editor || !history.available {
		return nil
	}
	return history
}

func (editor *Editor) retainEqualEdits() bool {
	return editor.savePointIndex() != nil || editor.journal != nil && (editor.journal.retainEqual || editor.historyState() != nil)
}

func (editor *Editor) invalidateHistoryFrom(first int) {
	if history := editor.historyState(); history != nil {
		history.ops.invalidateFrom(history, first)
	}
}

func recordHistoryOwned(history *historyJournal, change textChange, kind editKind) (Edit, bool) {
	primitive, ok := history.owner.historyChangePrimitive(change, kind)
	if ok {
		history.owner.captureHistoryPrimitives([]Edit{primitive})
	}
	return primitive, ok && history.available
}

func captureHistoryPrimitivesOwned(history *historyJournal, primitives []Edit) {
	history.owner.captureHistoryPrimitives(primitives)
}

func captureHistorySaveOwned(history *historyJournal) {
	history.owner.captureHistorySave()
}

func replayHistoryOwned(history *historyJournal, index int, inverse bool) {
	history.owner.captureHistoryReplay(&history.owner.history[index], inverse)
}

func clearHistoryOwned(history *historyJournal, manual bool) {
	if manual {
		history.owner.resetHistoryAfterClear()
	} else {
		invalidateHistoryFromOwned(history, 0)
	}
}

func enforceHistoryBudgetOwned(history *historyJournal) {
	history.owner.enforceHistoryBudget()
}
