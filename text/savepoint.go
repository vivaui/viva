package text

// SetSavePoint opts into saved-history tracking and marks the current Undo
// boundary without discarding Undo/Redo or enabling byte tracking. In this mode,
// accepted nonempty equal-text edits retain history; default editors are unchanged.
func (editor *Editor) SetSavePoint() {
	if editor.journal == nil {
		editor.journal = &editJournal{}
	}
	if editor.journal.savePoint == nil {
		editor.journal.savePoint = new(int)
	}
	*editor.journal.savePoint = editor.historyIndex
	if history := editor.historyState(); history != nil {
		history.ops.save(history)
	}
	editor.BreakUndoGroup()
}

// AtSavePoint reports whether the marked boundary is reachable and current.
// It is false before SetSavePoint or after that boundary was discarded.
func (editor *Editor) AtSavePoint() bool {
	point := editor.savePointIndex()
	return point != nil && *point >= 0 && *point == editor.historyIndex
}

func (editor *Editor) savePointIndex() *int {
	if editor.journal == nil {
		return nil
	}
	return editor.journal.savePoint
}

// ClearChangeHistory drops Undo/Redo without making unsaved data clean. Unlike
// ClearHistory, a previously unsaved save boundary remains unreachable.
func (editor *Editor) ClearChangeHistory() {
	point := editor.savePointIndex()
	dirty := point != nil && !editor.AtSavePoint()
	if history := editor.historyState(); history != nil {
		dirty = dirty || history.save < 0 || history.current != history.save
	}
	editor.ClearHistory()
	if dirty {
		if point != nil {
			*point = -1
		}
		if history := editor.historyState(); history != nil {
			history.save = -1
		}
	}
}
