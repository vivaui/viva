package text

// This file is the thin private boundary between ordinary editor paths and the
// optional composition subgraph. Default editors keep a nil sidecar and never
// reach the per-scalar transaction or producer-accumulator implementation.

func (editor *Editor) compositionState() *compositionState {
	if editor == nil || editor.journal == nil {
		return nil
	}
	state := editor.journal.compositionState()
	if state == nil || state.owner != editor {
		return nil
	}
	return state
}

func (editor *Editor) tentativeActive() bool {
	state := editor.compositionState()
	return state != nil && state.active
}

func (editor *Editor) compositionEvent() *historyEvent {
	state := editor.compositionState()
	if state == nil {
		return nil
	}
	return state.event
}

func (editor *Editor) compositionRetainEqual() bool {
	state := editor.compositionState()
	return state != nil && state.retainEqual
}

// syncCompositionTail reflects a private no-callback redo truncation in the
// producer's retained tail without publishing a frame or advancing sequence.
func (editor *Editor) syncCompositionTail() {
	if history := editor.historyState(); history != nil {
		history.retainedTo = history.current
	}
}

func (editor *Editor) compositionCost() int {
	state := editor.compositionState()
	if state == nil {
		return 0
	}
	return state.cost
}

// sealTentativeHistory is the explicit legacy compatibility escape. It ends
// only the tentative history boundary, keeps the visible text/mark/selection
// unchanged, truncates redo and invalidates producer metadata before the legacy
// operation runs with its original UI composition targeting.
func (editor *Editor) sealTentativeHistory() {
	state := editor.compositionState()
	if state == nil || !state.active {
		return
	}
	editor.sealTentativeHistorySlow()
}

// sealTentativeHistorySlow keeps the ordinary-path seal inlinable. It only runs
// for an owned active tentative, so the invalidation/redo work is never a
// dormant default-path call.
func (editor *Editor) sealTentativeHistorySlow() {
	state := editor.journal.compositionState()
	state.active = false
	state.live = false
	state.completed = false
	state.completedStart = -1
	state.eventStart = -1
	state.unitStart = -1
	editor.chargeCompositionCapacity()
	editor.invalidateHistoryFrom(0)
	editor.dropRedo()
}

// disableTentativeForConfiguration seals an active tentative, releases only the
// capability sidecar and leaves visible/UI marking untouched. The caller then
// performs its normal restriction or overwrite change.
func (editor *Editor) disableTentativeForConfiguration() {
	state := editor.compositionState()
	if state == nil {
		return
	}
	if state.active {
		// The visible units stay Undoable, so they become one passive completed
		// physical boundary rather than unprotected history that the next trim
		// could cut. Producer metadata still invalidates on this escape.
		state.active = false
		state.live = false
		editor.compositionComplete(state)
		editor.invalidateHistoryFrom(0)
		editor.dropRedo()
	}
	state.event = nil
	state.enabled = false
	editor.chargeCompositionCapacity()
	if state.completed {
		state.passive = true
		return
	}
	if cap(editor.history) > maxHistoryEntries {
		// The backing is large enough to remain a charged nonprotecting owner.
		state.passive = false
		return
	}
	editor.journal.setCompositionState(nil)
}
