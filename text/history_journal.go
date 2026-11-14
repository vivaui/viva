package text

import (
	"sync/atomic"
	"unsafe"
)

// HistoryCursor identifies one position in one editor's optional history.
// The zero value is invalid and cursors from other sources cannot be replayed.
type HistoryCursor struct {
	source, generation, sequence uint64
}

// HistoryOperation is the operation that accepted a history step.
type HistoryOperation uint8

const (
	HistoryForward HistoryOperation = iota + 1
	HistoryUndo
	HistoryRedo
	HistorySave
)

// HistoryPrimitive is the actual byte insertion/deletion performed by a step.
type HistoryPrimitive uint8

const (
	HistoryNoPrimitive HistoryPrimitive = iota
	HistoryDelete
	HistoryInsert
)

// HistoryContext samples primitive boundaries, separately from Undo units.
// Save and Detach use -1 for an unreachable or absent boundary.
// Forward steps sample after append; Undo/Redo sample before their completion;
// Save samples after SetSavePoint. Unprovable history trimming invalidates the
// producer instead of rebasing these positions. The editor always collects Undo.
type HistoryContext struct {
	Current, Save, Detach    int
	RetainedFrom, RetainedTo int
	Collecting               bool
}

// HistoryStep is an accepted UTF-8 byte primitive or a metadata-only Save.
type HistoryStep struct {
	Operation               HistoryOperation
	Primitive               HistoryPrimitive
	Position, Length        int
	BeforeBytes, AfterBytes int
	Context                 HistoryContext
}

// HistoryFrame is one complete operation, even when its accepted primitives
// overlap or have no net text change. Returned Steps belong to the caller.
type HistoryFrame struct {
	From, To                HistoryCursor
	BeforeBytes, AfterBytes int
	Steps                   []HistoryStep
}

const (
	historyFrameLimit = 256
	historyStepRunes  = int((unsafe.Sizeof(HistoryStep{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))
	historyFrameRunes = int((unsafe.Sizeof(HistoryFrame{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))
)

type historyJournal struct {
	owner                        *Editor
	ops                          historyOps
	source, generation, sequence uint64
	current, save, detach        int
	retainedFrom, retainedTo     int
	frames                       []HistoryFrame
	epoch                        HistoryCursor
	windowStart                  uint64
	bootstrapBytes, bytes        int
	cost, frameCapacity          int
	available                    bool
	oldest                       uint16
}

// A history header owns the shared optional slot even after invalidation.
const historyHeaderRunes = editorSidecarRunes + int((unsafe.Sizeof(historyJournal{})+unsafe.Sizeof(rune(0))-1)/unsafe.Sizeof(rune(0)))

var nextHistorySource atomic.Uint64

// TrackHistory enables immutable primitive-history delivery without enabling
// the byte journal. First enable requires no Undo, Redo or composition. It
// returns the original epoch and UTF-8 length while that prefix is retained;
// expiry refuses a new bootstrap without invalidating a retained consumer.
func (editor *Editor) TrackHistory() (HistoryCursor, int, bool) {
	if editor == nil {
		return HistoryCursor{}, 0, false
	}
	journal := editor.journal
	if state := journal.compositionState(); state != nil && state.owner != editor {
		return HistoryCursor{}, 0, false
	}
	if journal != nil && journal.historyJournal() != nil {
		history := journal.historyJournal()
		if history.owner != editor || !history.available || history.epoch.sequence < history.windowStart {
			return HistoryCursor{}, 0, false
		}
		return history.epoch, history.bootstrapBytes, true
	}
	if journal != nil && journal.private || editor.composing || editor.CanUndo() || editor.CanRedo() {
		return HistoryCursor{}, 0, false
	}
	if journal == nil {
		journal = &editJournal{}
		editor.journal = journal
	}
	source := nextHistorySource.Add(1)
	for source == 0 {
		source = nextHistorySource.Add(1)
	}
	size := len(editor.String())
	saved := 0
	if point := editor.savePointIndex(); point != nil && *point < 0 {
		saved = -1
	}
	history := &historyJournal{
		owner: editor,
		ops: historyOps{
			record:            recordHistoryOwned,
			capturePrimitives: captureHistoryPrimitivesOwned,
			save:              captureHistorySaveOwned,
			replay:            replayHistoryOwned,
			clear:             clearHistoryOwned,
			invalidateFrom:    invalidateHistoryFromOwned,
			enforce:           enforceHistoryBudgetOwned,
		},
		source: source, generation: 1, save: saved, detach: -1,
		epoch:          HistoryCursor{source: source, generation: 1},
		bootstrapBytes: size, bytes: size, cost: historyHeaderRunes, available: true,
	}
	journal.setHistoryJournal(history)
	editor.chargeCompositionCapacity()
	return history.epoch, size, true
}

// HistorySince copies every complete frame after cursor, including nested
// Steps. Repeated reads are independent. Wrong sources/generations, future
// cursors, unavailable history and missing prefixes return no partial replay.
func (editor *Editor) HistorySince(cursor HistoryCursor) ([]HistoryFrame, HistoryCursor, bool) {
	history := editor.historyState()
	if history == nil {
		return nil, HistoryCursor{}, false
	}
	latest := history.cursor()
	if cursor == (HistoryCursor{}) || cursor.source != history.source || cursor.generation != history.generation ||
		cursor.sequence > history.sequence || cursor.sequence < history.windowStart {
		return nil, latest, false
	}
	want := int(history.sequence - cursor.sequence)
	if want == 0 {
		return nil, latest, true
	}
	frames := make([]HistoryFrame, 0, want)
	backing := history.frames[:cap(history.frames)]
	for index := range len(history.frames) {
		frame := backing[(int(history.oldest)+index)%len(backing)]
		if frame.To.sequence <= cursor.sequence {
			continue
		}
		frame.Steps = append([]HistoryStep(nil), frame.Steps...)
		frames = append(frames, frame)
	}
	if len(frames) != want {
		return nil, latest, false
	}
	return frames, latest, true
}

func (history *historyJournal) cursor() HistoryCursor {
	return HistoryCursor{history.source, history.generation, history.sequence}
}

func (history *historyJournal) context(current, tail int) HistoryContext {
	return HistoryContext{Current: current, Save: history.save, Detach: history.detach,
		RetainedFrom: history.retainedFrom, RetainedTo: tail, Collecting: true}
}

func (editor *Editor) historyChangePrimitive(change textChange, kind editKind) (Edit, bool) {
	if kind == editComposition {
		editor.invalidateHistoryFrom(0)
		return Edit{}, false
	}
	if editor.historyState() == nil {
		return Edit{}, false
	}
	position := editor.offsets.offset
	if editor.offsets.index != change.start {
		position = editor.ByteOffset(change.start)
	}
	removed, inserted := historyRuneBytes(change.removed), historyRuneBytes(change.inserted)
	return Edit{Start: position, Removed: removed, Inserted: inserted}, removed != 0 || inserted != 0
}

func historyRuneBytes(value []rune) int {
	bytes := 0
	for _, character := range value {
		bytes += runeBytes(character)
	}
	return bytes
}

func (editor *Editor) captureHistoryPrimitives(primitives []Edit) {
	if event := editor.compositionEvent(); event != nil {
		editor.appendEventPrimitives(event, primitives)
		return
	}
	history := editor.historyState()
	if history == nil || len(primitives) == 0 {
		return
	}
	before, running := history.bytes, history.bytes
	steps := make([]HistoryStep, 0, len(primitives)*2)
	for _, primitive := range primitives {
		if primitive.Removed > 0 {
			steps = editor.appendForwardHistoryStep(history, steps, HistoryDelete, primitive.Start, primitive.Removed, running, running-primitive.Removed)
			running -= primitive.Removed
		}
		if primitive.Inserted > 0 {
			steps = editor.appendForwardHistoryStep(history, steps, HistoryInsert, primitive.Start, primitive.Inserted, running, running+primitive.Inserted)
			running += primitive.Inserted
		}
	}
	if len(steps) > 0 {
		editor.appendHistoryFrame(steps, before, running, steps[len(steps)-1].Context.Current)
	}
}

func (editor *Editor) appendForwardHistoryStep(history *historyJournal, steps []HistoryStep, primitive HistoryPrimitive, position, length, before, after int) []HistoryStep {
	boundary := history.current + len(steps)
	editor.updateHistoryBranch(boundary)
	return append(steps, HistoryStep{HistoryForward, primitive, position, length, before, after, history.context(boundary+1, boundary+1)})
}

func (editor *Editor) updateHistoryBranch(boundary int) {
	history := editor.historyState()
	if history == nil {
		return
	}
	if boundary < history.save {
		history.save = -1
		if history.detach == -1 {
			history.detach = boundary
		}
	} else if history.detach != -1 && history.detach > boundary {
		history.detach = boundary
	}
}

func (editor *Editor) captureHistorySave() {
	history := editor.historyState()
	if history == nil {
		return
	}
	history.save, history.detach = history.current, -1
	size := history.bytes
	step := HistoryStep{Operation: HistorySave, BeforeBytes: size, AfterBytes: size, Context: history.context(history.current, history.retainedTo)}
	editor.appendHistoryFrame([]HistoryStep{step}, size, size, history.current)
}

func (editor *Editor) appendHistoryFrame(steps []HistoryStep, before, after, current int) {
	if len(steps) == 0 {
		return
	}
	editor.appendHistoryFrameWithTail(steps, before, after, current, steps[len(steps)-1].Context.RetainedTo)
}

func (editor *Editor) appendHistoryFrameWithTail(steps []HistoryStep, before, after, current, tail int) {
	history := editor.historyState()
	if history == nil || len(steps) == 0 {
		return
	}
	running := before
	for _, step := range steps {
		valid := step.Position >= 0 && step.Length >= 0 && step.BeforeBytes == running
		switch step.Primitive {
		case HistoryDelete:
			valid = valid && step.AfterBytes == running-step.Length && step.Position <= running-step.Length
		case HistoryInsert:
			valid = valid && step.AfterBytes == running+step.Length && step.Position <= running
		case HistoryNoPrimitive:
			valid = valid && step.AfterBytes == running && step.Position == 0 && step.Length == 0
		default:
			valid = false
		}
		if !valid {
			editor.invalidateHistoryFrom(0)
			return
		}
		running = step.AfterBytes
	}
	if running != after || current < history.retainedFrom || current < 0 || current > tail {
		editor.invalidateHistoryFrom(0)
		return
	}
	from := history.cursor()
	history.sequence++
	if len(history.frames) == historyFrameLimit {
		history.dropOldestFrame()
	}
	if len(history.frames) == cap(history.frames) {
		capacity := min(historyFrameLimit, max(1, cap(history.frames)*2))
		next := make([]HistoryFrame, len(history.frames), capacity)
		old := history.frames[:cap(history.frames)]
		for index := range len(history.frames) {
			next[index] = old[(int(history.oldest)+index)%len(old)]
		}
		history.frames, history.oldest = next, 0
		history.cost += (capacity - history.frameCapacity) * historyFrameRunes
		history.frameCapacity = capacity
	}
	// Logical order can wrap while the slice always starts at its backing base.
	backing := history.frames[:cap(history.frames)]
	backing[(int(history.oldest)+len(history.frames))%len(backing)] = HistoryFrame{from, history.cursor(), before, after, steps}
	history.frames = backing[:len(history.frames)+1]
	history.cost += cap(steps) * historyStepRunes
	history.current, history.retainedTo, history.bytes = current, tail, after
	editor.enforceHistoryBudget()
}

func (history *historyJournal) dropOldestFrame() {
	if len(history.frames) == 0 {
		return
	}
	backing := history.frames[:cap(history.frames)]
	frame := backing[history.oldest]
	history.cost -= cap(frame.Steps) * historyStepRunes
	history.windowStart = frame.To.sequence
	backing[history.oldest] = HistoryFrame{}
	history.oldest = uint16((int(history.oldest) + 1) % len(backing))
	history.frames = backing[:len(history.frames)-1]
}

func (editor *Editor) historyCost() int {
	cost := editor.historyRunes
	if editor.journal != nil && editor.journal.historyJournal() != nil {
		cost += editor.journal.historyJournal().cost
	}
	cost += editor.compositionCost()
	return cost
}

func (editor *Editor) enforceHistoryBudget() {
	history := editor.historyState()
	if history == nil {
		return
	}
	for len(history.frames) > 1 && editor.historyCost() > maxHistoryRunes {
		history.dropOldestFrame()
	}
	if len(history.frames) == 1 && editor.historyCost() > maxSingleEditRunes {
		editor.invalidateHistoryFrom(0)
	}
}

type historyReplayPrimitive struct {
	kind             HistoryPrimitive
	position, length int
}

func (editor *Editor) historyReplayPrimitives(edit *historyEdit) []historyReplayPrimitive {
	primitives := make([]historyReplayPrimitive, 0, len(edit.primitives)*2)
	appendPrimitive := func(position, removed, inserted int) {
		if removed > 0 {
			primitives = append(primitives, historyReplayPrimitive{HistoryDelete, position, removed})
		}
		if inserted > 0 {
			primitives = append(primitives, historyReplayPrimitive{HistoryInsert, position, inserted})
		}
	}
	if len(edit.primitives) > 0 {
		for _, primitive := range edit.primitives {
			appendPrimitive(primitive.Start, primitive.Removed, primitive.Inserted)
		}
	} else {
		appendPrimitive(editor.ByteOffset(edit.start), historyRuneBytes(edit.removed), historyRuneBytes(edit.inserted))
	}
	return primitives
}

func (editor *Editor) captureHistoryReplay(edit *historyEdit, inverse bool) {
	if event := editor.compositionEvent(); event != nil {
		editor.captureEventReplay(event, edit, inverse)
		return
	}
	history := editor.historyState()
	if history == nil {
		return
	}
	primitives := editor.historyReplayPrimitives(edit)
	if len(primitives) == 0 {
		editor.invalidateHistoryFrom(0)
		return
	}
	operation := HistoryRedo
	if inverse {
		operation = HistoryUndo
	}
	before, running, current := history.bytes, history.bytes, history.current
	steps := make([]HistoryStep, 0, len(primitives))
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
		after := running + primitive.length
		if primitive.kind == HistoryDelete {
			after = running - primitive.length
		}
		steps = append(steps, HistoryStep{operation, primitive.kind, primitive.position, primitive.length, running, after, history.context(current, history.retainedTo)})
		running = after
		if inverse {
			current--
		} else {
			current++
		}
	}
	editor.appendHistoryFrame(steps, before, running, current)
}

func (editor *Editor) resetHistoryAfterClear() {
	history := editor.historyState()
	if history == nil {
		return
	}
	history.frames, history.oldest = nil, 0
	history.cost, history.frameCapacity = historyHeaderRunes, 0
	history.generation++
	history.sequence++
	history.current, history.save, history.detach = 0, 0, -1
	history.retainedFrom, history.retainedTo = 0, 0
	history.bytes = len(editor.String())
	history.bootstrapBytes = history.bytes
	history.epoch = history.cursor()
	history.windowStart = history.sequence
}

func invalidateHistoryFromOwned(history *historyJournal, first int) {
	editor := history.owner
	for index := first; index < len(editor.history); index++ {
		edit := &editor.history[index]
		if edit.kind != editSeparate && len(edit.primitives) > 0 {
			editor.historyRunes -= cap(edit.primitives) * historyPrimitiveRunes
			edit.primitives = nil
		}
	}
	history.available = false
	history.frames, history.oldest = nil, 0
	history.cost, history.frameCapacity = historyHeaderRunes, 0
	history.bootstrapBytes, history.bytes = 0, 0
	history.current, history.save, history.detach = 0, 0, -1
	history.retainedFrom, history.retainedTo = 0, 0
	history.generation++
	history.sequence++
	history.epoch = HistoryCursor{}
}
