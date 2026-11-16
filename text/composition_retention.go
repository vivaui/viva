package text

import "unsafe"

// historyEditRunes is the actual per-slot backing of the ordinary history
// array. Retention charges real capacity in rune-sized budget units rather than
// only the used lengths; the capability descriptor and producer metadata are
// charged separately by compositionHeaderRunes/historyJournal.cost.
const historyEditRunes = int((unsafe.Sizeof(historyEdit{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))

// retentionActive reports whether this capability currently owns a live,
// pending or completed physical boundary. An enabled idle sidecar retains no
// history capacity charge.
func (state *compositionState) retentionActive() bool {
	return state != nil && (state.live || state.completed || state.pending || state.active)
}

// ownRunes returns an exact-capacity copy of values. Composition units use it
// only when their current backing is larger than the used length, so actual
// retained rune capacity equals the charged length instead of hiding allocator
// slack or sharing an oversized event buffer across units.
func ownRunes(values []rune) []rune {
	if len(values) == 0 {
		return nil
	}
	owned := make([]rune, len(values))
	copy(owned, values)
	return owned
}

// ownCompositionEventText makes every unit created by the current event own
// exactly its used removed/inserted rune slots. The event delivery boundary
// includes the ordinary pre-start/explicit deletion, so it is compacted too.
// Unit indices, text, primitives and producer frames are unchanged.
func (editor *Editor) ownCompositionEventText(state *compositionState) {
	if state == nil {
		return
	}
	from := state.eventStart
	if from < 0 {
		from = state.unitStart
	}
	if from < 0 || from > len(editor.history) {
		return
	}
	for index := from; index < len(editor.history); index++ {
		editor.ownCompositionUnitText(&editor.history[index])
	}
}

func (editor *Editor) ownCompositionUnitText(edit *historyEdit) {
	labels := editor.lfLabelState()
	var previousKey *rune
	if labels != nil {
		previousKey = lfLabelHistoryKey(edit)
	}
	if cap(edit.removed) != len(edit.removed) {
		edit.removed = ownRunes(edit.removed)
	}
	if cap(edit.inserted) != len(edit.inserted) {
		edit.inserted = ownRunes(edit.inserted)
	}
	if labels != nil {
		if currentKey := lfLabelHistoryKey(edit); currentKey != previousKey {
			labels.ops.rekey(labels, previousKey, currentKey)
		}
	}
}

// reserveCompositionHistory gives the planned event exact backing capacity so
// the physical preflight charge is the capacity actually retained, not a
// post-hoc growth guess. It never shrinks or mutates units.
func (editor *Editor) reserveCompositionHistory(need int) {
	if need <= 0 || need <= cap(editor.history) {
		return
	}
	next := make([]historyEdit, len(editor.history), need)
	copy(next, editor.history)
	editor.history = next
	editor.chargeCompositionCapacity()
}

// compactCompositionHistory releases prefix capacity after rollback/trim. Only
// composition-owned history is compacted; default editors keep their exact
// legacy growth behavior.
func (editor *Editor) compactCompositionHistory() {
	if editor == nil || editor.journal == nil || editor.journal.compositionState() == nil {
		return
	}
	if cap(editor.history) == len(editor.history) {
		return
	}
	next := make([]historyEdit, len(editor.history))
	copy(next, editor.history)
	editor.history = next
	editor.chargeCompositionCapacity()
}

// chargeCompositionCapacity recomputes the descriptor header charge from the
// actual history array capacity. It is called only from composition-owned
// reserve/compact/boundary code, never from the default typing path.
func (editor *Editor) chargeCompositionCapacity() {
	if editor == nil || editor.journal == nil || editor.journal.compositionState() == nil {
		return
	}
	state := editor.journal.compositionState()
	state.cost = editor.compositionBaseCost()
	if state.retentionActive() {
		state.cost += cap(editor.history) * historyEditRunes
		return
	}
	// After protection retires, the first maxHistoryEntries slots are the
	// accepted ordinary baseline. Larger composition-owned backing is still real
	// until compacted, so a nonprotecting owner keeps charging its extra slots.
	if extra := cap(editor.history) - maxHistoryEntries; extra > 0 {
		state.cost += extra * historyEditRunes
	}
}

// compositionTrimCost is the eviction predicate cost for a retired
// nonprotecting owner. It models the backing that will exist after the trim
// settles (capacity compacted to the surviving length) instead of letting stale
// oversized capacity force eviction below the ordinary entry limit. The actual
// charge is restored by settleCompositionStorage before the trim returns.
func (editor *Editor) compositionTrimCost(state *compositionState, remaining int) int {
	cost := editor.historyCost()
	if state == nil || state.retentionActive() {
		return cost
	}
	excess := cap(editor.history) - max(remaining, maxHistoryEntries)
	if excess > 0 {
		cost -= excess * historyEditRunes
	}
	return cost
}

func compositionTrimCostOwned(state *compositionState, remaining int) int {
	if state == nil || state.owner == nil {
		return 0
	}
	return state.owner.compositionTrimCost(state, remaining)
}

// settleCompositionStorage is the post-protection owner. It compacts excess
// array capacity when no live/completed boundary protects the units, keeps the
// actual extra slots charged while a large used array remains for Undo/Redo,
// and releases a disabled nonprotecting owner once the backing is ordinary.
// It is called from trim after units have been processed, never with an
// in-flight historyEdit pointer.
func settleCompositionStorage(state *compositionState) {
	if state == nil || state.owner == nil {
		return
	}
	editor := state.owner
	if state.retentionActive() {
		editor.chargeCompositionCapacity()
		return
	}
	if cap(editor.history) > maxHistoryEntries && cap(editor.history) != len(editor.history) {
		next := make([]historyEdit, len(editor.history))
		copy(next, editor.history)
		editor.history = next
	}
	// Ordinary units that survive alongside the retired composition suffix can
	// also carry allocator slack (for example a one-rune append with cap 2).
	// With a live composition-owned storage owner, own exact text capacity for
	// the whole retained array so the charge equals the actual cap.
	for index := range editor.history {
		editor.ownCompositionUnitText(&editor.history[index])
	}
	editor.chargeCompositionCapacity()
	if !state.enabled && !state.live && !state.active && !state.completed && cap(editor.history) <= maxHistoryEntries {
		editor.journal.setCompositionState(nil)
	}
}

// retireCompletedRetention releases the previous completed action before an
// accepted mutation. It is descriptor-only and never touches text, units,
// frames or cursors. A passive disabled owner is released entirely.
func (editor *Editor) retireCompletedRetention() {
	if editor == nil || editor.journal == nil {
		return
	}
	state := editor.journal.compositionState()
	if state == nil || !state.completed {
		return
	}
	state.completed = false
	state.completedStart = -1
	state.passive = false
	editor.chargeCompositionCapacity()
	if !state.enabled && !state.live && !state.active && cap(editor.history) <= maxHistoryEntries {
		editor.journal.setCompositionState(nil)
	}
}

// recordRetiresCompleted is the ordinary-mutation hook. It runs only when a
// real ordinary change/unit is about to be accepted, so refusals, valid no-ops,
// reads, getters and selection-only operations keep completed retention.
func (editor *Editor) recordRetiresCompleted() {
	editor.retireCompletedRetention()
}

func trimCompositionHistoryOwned(state *compositionState) {
	if state != nil && state.owner != nil {
		state.owner.trimCompositionHistory()
	}
}

// Only the enable-time dispatch links retention-aware trimming. Ordinary
// clients keep the existing trim path and never acquire physical protection.
func (editor *Editor) trimCompositionHistory() {
	journal := editor.journal
	state := editor.compositionState()
	if state != nil && state.completed {
		state.completed, state.passive = false, false
		state.completedStart = -1
		editor.chargeCompositionCapacity()
		if !state.enabled && !state.live && !state.active && cap(editor.history) <= maxHistoryEntries {
			journal.setCompositionState(nil)
			state = nil
		}
	}
	if history := editor.historyState(); history != nil {
		history.ops.enforce(history)
	}
	protected := -1
	if state != nil {
		if state.live || state.active || state.pending {
			protected = state.eventStart
			if (state.live || state.active) && (protected < 0 || state.unitStart >= 0 && state.unitStart < protected) {
				protected = state.unitStart
			}
		}
		if state.completed && (protected < 0 || state.completedStart >= 0 && state.completedStart < protected) {
			protected = state.completedStart
		}
	}
	remove, limit := 0, len(editor.history)-1
	if protected >= 0 && protected < limit {
		limit = protected
	}
	for remove < limit {
		remaining := len(editor.history) - remove
		if remaining <= maxHistoryEntries {
			cost := editor.historyCost()
			if state != nil && state.ops.trimCost != nil {
				cost = state.ops.trimCost(state, remaining)
			}
			if cost <= maxHistoryRunes {
				break
			}
		}
		edit := editor.history[remove]
		editor.historyRunes -= editor.historyUnitRunes(&edit) + editor.forgetLineBatch(&editor.history[remove])
		remove++
	}
	if remove > 0 {
		editor.invalidateHistoryFrom(remove)
		if point := editor.savePointIndex(); point != nil && *point >= 0 {
			*point -= remove
			if *point < 0 {
				*point = -1
			}
		}
		copy(editor.history, editor.history[remove:])
		clear(editor.history[len(editor.history)-remove:])
		editor.history = editor.history[:len(editor.history)-remove]
		editor.historyIndex = max(0, editor.historyIndex-remove)
		if state != nil {
			if state.eventStart >= 0 {
				state.eventStart = max(0, state.eventStart-remove)
			}
			if state.unitStart >= 0 {
				state.unitStart = max(0, state.unitStart-remove)
			}
			if state.completedStart >= 0 {
				state.completedStart = max(0, state.completedStart-remove)
			}
		}
	}
	if state != nil && state.ops.settle != nil {
		state.ops.settle(state)
		if journal.compositionState() != state {
			state = nil
		}
	}
	if editor.historyCost() <= maxSingleEditRunes {
		return
	}
	if state != nil && state.retentionActive() {
		capacity := len(editor.history)
		if state.pending && state.eventStart >= 0 {
			// Prefix eviction must preserve the accepted event's future slots.
			// Shrinking to the first scalar would restart slice growth near the
			// hard limit and repeatedly allocate/compact the same large array.
			capacity = max(capacity, state.eventStart+state.pendingUnits)
		}
		if cap(editor.history) != capacity {
			next := make([]historyEdit, len(editor.history), capacity)
			copy(next, editor.history)
			editor.history = next
			editor.chargeCompositionCapacity()
		}
		if editor.historyCost() > maxSingleEditRunes {
			editor.invalidateHistoryFrom(0)
		}
		return
	}
	editor.clearHistory(false)
	if point := editor.savePointIndex(); point != nil {
		*point = -1
	}
}
