package text

import "unsafe"

// One pending event owns the complete mixed primitive stream before mutation.
// Ordinary physical units remain the rollback and Undo source of truth.
type historyEvent struct {
	from       HistoryCursor
	startBytes int
	bytes      int
	current    int
	retainedTo int
	steps      []HistoryStep
	cost       int
	failed     bool
}

const historyEventRunes = int((unsafe.Sizeof(historyEvent{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))

func (editor *Editor) beginCompositionHistory(state *compositionState, plan compositionPlan) *historyEvent {
	history := editor.historyState()
	steps := plan.steps
	if history == nil || steps <= 0 {
		return nil
	}
	if steps > (maxSingleEditRunes-historyFrameRunes-historyEventRunes)/historyStepRunes {
		editor.invalidateHistoryFrom(0)
		return nil
	}
	cost := steps*historyStepRunes + historyFrameRunes + historyEventRunes
	// Active replacements reserve their physical array after rollback. Price
	// that complete generation now, including frame backing eviction cannot free,
	// so an impossible frame cannot force per-scalar array compaction later.
	physical := compositionHeaderRunes + plan.payload + plan.units*historyEditRunes
	backing := historyHeaderRunes + history.frameCapacity*historyFrameRunes
	if physical+backing+cost > maxSingleEditRunes {
		editor.invalidateHistoryFrom(0)
		return nil
	}
	for len(history.frames) > 0 && editor.historyCost()+cost > maxHistoryRunes {
		history.dropOldestFrame()
	}
	if editor.historyCost()+cost > maxSingleEditRunes {
		editor.invalidateHistoryFrom(0)
		return nil
	}
	history.cost += cost
	event := &historyEvent{
		from: history.cursor(), startBytes: history.bytes, bytes: history.bytes,
		current: history.current, retainedTo: history.retainedTo, cost: cost,
		steps: make([]HistoryStep, 0, steps),
	}
	state.event = event
	return event
}

func (editor *Editor) finishCompositionHistory(state *compositionState, event *historyEvent) {
	state.event = nil
	if event == nil {
		return
	}
	history := editor.historyState()
	if history == nil {
		return
	}
	history.cost -= event.cost
	if event.failed || event.from != history.cursor() {
		editor.invalidateHistoryFrom(0)
		return
	}
	if len(event.steps) == 0 {
		history.current, history.bytes, history.retainedTo = event.current, event.bytes, event.retainedTo
		return
	}
	if !validateCompositionHistoryEvent(event) {
		editor.invalidateHistoryFrom(0)
		return
	}
	editor.appendHistoryFrameWithTail(event.steps, event.startBytes, event.bytes, event.current, event.retainedTo)
}

func validateCompositionHistoryEvent(event *historyEvent) bool {
	running := event.startBytes
	for _, step := range event.steps {
		if step.Position < 0 || step.Length < 0 || step.BeforeBytes != running {
			return false
		}
		switch step.Primitive {
		case HistoryDelete:
			if step.AfterBytes != running-step.Length || step.Position > running-step.Length {
				return false
			}
		case HistoryInsert:
			if step.AfterBytes != running+step.Length || step.Position > running {
				return false
			}
		default:
			return false
		}
		switch step.Operation {
		case HistoryForward, HistoryUndo, HistoryRedo:
		default:
			return false
		}
		running = step.AfterBytes
	}
	return running == event.bytes && event.current >= 0 && event.retainedTo >= event.current
}

func (editor *Editor) appendEventForwardPrimitive(event *historyEvent, primitive Edit) {
	if event == nil || primitive.Removed == 0 && primitive.Inserted == 0 {
		return
	}
	history := editor.historyState()
	if history == nil {
		event.failed = true
		return
	}
	running := event.bytes
	if primitive.Removed > 0 {
		after := running - primitive.Removed
		editor.appendEventForwardStep(history, event, HistoryDelete, primitive.Start, primitive.Removed, running, after)
		running = after
	}
	if primitive.Inserted > 0 {
		after := running + primitive.Inserted
		editor.appendEventForwardStep(history, event, HistoryInsert, primitive.Start, primitive.Inserted, running, after)
		running = after
	}
	event.bytes = running
}

func (editor *Editor) appendEventForwardStep(history *historyJournal, event *historyEvent, primitive HistoryPrimitive, position, length, before, after int) {
	if len(event.steps) >= cap(event.steps) {
		event.failed = true
		editor.invalidateHistoryFrom(0)
		return
	}
	boundary := event.current
	editor.updateHistoryBranch(boundary)
	event.steps = append(event.steps, HistoryStep{HistoryForward, primitive, position, length, before, after, history.context(boundary+1, boundary+1)})
	event.current, event.retainedTo, event.bytes = boundary+1, boundary+1, after
}

func (editor *Editor) appendEventReplayStep(history *historyJournal, event *historyEvent, operation HistoryOperation, primitive HistoryPrimitive, position, length, before, after int) {
	if len(event.steps) >= cap(event.steps) {
		event.failed = true
		editor.invalidateHistoryFrom(0)
		return
	}
	event.steps = append(event.steps, HistoryStep{operation, primitive, position, length, before, after, history.context(event.current, event.retainedTo)})
	if operation == HistoryUndo {
		event.current--
	} else {
		event.current++
	}
	if event.current < 0 {
		event.failed = true
		editor.invalidateHistoryFrom(0)
	}
	event.bytes = after
}

func (editor *Editor) appendEventPrimitives(event *historyEvent, primitives []Edit) {
	for _, primitive := range primitives {
		editor.appendEventForwardPrimitive(event, primitive)
	}
}

func (editor *Editor) captureEventReplay(event *historyEvent, edit *historyEdit, inverse bool) {
	history := editor.historyState()
	if history == nil {
		event.failed = true
		return
	}
	primitives := editor.historyReplayPrimitives(edit)
	operation := HistoryRedo
	if inverse {
		operation = HistoryUndo
	}
	for index := range len(primitives) {
		at := index
		if inverse {
			at = len(primitives) - 1 - index
		}
		primitive := primitives[at]
		if inverse {
			if primitive.kind == HistoryInsert {
				primitive.kind = HistoryDelete
			} else {
				primitive.kind = HistoryInsert
			}
		}
		before := event.bytes
		after := before + primitive.length
		if primitive.kind == HistoryDelete {
			after = before - primitive.length
		}
		if after < 0 {
			event.failed = true
			editor.invalidateHistoryFrom(0)
			return
		}
		editor.appendEventReplayStep(history, event, operation, primitive.kind, primitive.position, primitive.length, before, after)
	}
}
