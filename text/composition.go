package text

import (
	"sync/atomic"
	"unicode/utf8"
	"unsafe"
)

// CompositionPhase is the explicit opt-in tentative-composition event phase.
// CompositionNone is the zero value used by disabled clients.
type CompositionPhase uint8

const (
	CompositionNone CompositionPhase = iota
	CompositionUpdate
	CompositionResult
	CompositionCommit
	CompositionCancel
	CompositionEmptyCommit
)

// UTF16Range is a raw native UTF-16 start/length pair, converted after any
// tentative rollback exactly like the source PositionsFromCharacters path.
type UTF16Range struct {
	Start, Length int
}

// CompositionEvent is the opt-in editor ingress for tentative composition.
// Text is UTF-8 for Update/Result; Selected is relative to that text. Raw
// UTF-16 replacement coordinates take precedence over the rune Replacement.
type CompositionEvent struct {
	Phase               CompositionPhase
	Text                string
	Selected            Range
	Replacement         Range
	HasReplacement      bool
	ReplacementUTF16    UTF16Range
	HasReplacementUTF16 bool
}

// CompositionIdentity is the comparable, opaque owner of one enabled
// composition capability. The zero value denotes an unowned snapshot.
type CompositionIdentity struct {
	source uint64
}

var compositionIdentities atomic.Uint64

func nextCompositionIdentity() CompositionIdentity {
	source := compositionIdentities.Add(1)
	for source == 0 {
		source = compositionIdentities.Add(1)
	}
	return CompositionIdentity{source: source}
}

// compositionState is the optional sidecar. It owns the physical tentative
// boundary, the one pending producer frame and the bounded retention
// descriptor; it never copies source text.
type compositionState struct {
	owner    *Editor
	identity CompositionIdentity
	ops      compositionOps
	event    *historyEvent
	cost     int
	// unitStart is the rollback boundary: the first tentative unit after any
	// ordinary pre-start deletion. eventStart is the atomic delivery boundary:
	// it also includes that ordinary deletion. They are deliberately distinct.
	unitStart      int
	eventStart     int
	pendingUnits   int
	completedStart int
	start          int
	retainEqual    bool
	pending        bool
	active         bool
	enabled        bool
	live           bool
	completed      bool
	passive        bool
}

type compositionOps struct {
	apply     func(*compositionState, CompositionEvent) bool
	committed func(*compositionState) (string, bool)
	settle    func(*compositionState)
	trimCost  func(*compositionState, int) int
	trim      func(*compositionState)
}

const compositionHeaderRunes = int((unsafe.Sizeof(compositionState{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))

// EnableTentativeComposition opts an unrestricted editor into source-scalar
// insertion/overwrite composition without changing its current editing state.
// It never edits text, selection, journal revisions, source identity/generation
// or ordinary Undo/Redo/save state.
func (editor *Editor) EnableTentativeComposition() (ok bool) {
	if editor == nil {
		return false
	}
	journal := editor.journal
	if journal != nil && journal.private {
		return false
	}
	if history := journal.historyJournal(); history != nil && history.owner != editor {
		return false
	}
	if journal != nil && journal.compositionState() != nil {
		state := journal.compositionState()
		if state.owner != editor {
			return false
		}
		if state.enabled {
			return true
		}
		// A passive descriptor survives an inactive Disable while it protects a
		// complete physical action. Re-enable reuses that owner; it does not
		// allocate a second sidecar or change text/history/frames/cursors.
		if editor.composing || editor.accept != nil || editor.maxRunes > 0 {
			return false
		}
		state.enabled = true
		state.passive = false
		state.identity = nextCompositionIdentity()
		return true
	}
	if editor.composing || editor.accept != nil || editor.maxRunes > 0 {
		return false
	}
	if journal == nil {
		journal = &editJournal{}
	}
	cost := editor.compositionBaseCost()
	total := editor.historyCost() + cost
	if total > maxHistoryRunes {
		// Charging the sidecar must not silently force ordinary unit or frame
		// pruning. A lone oversized action may still use its single-action room.
		if journal.historyJournal() != nil && len(journal.historyJournal().frames) > 0 {
			return false
		}
		if len(editor.history) > 1 || total > maxSingleEditRunes {
			return false
		}
	}
	editor.journal = journal
	journal.setCompositionState(&compositionState{
		owner:      editor,
		identity:   nextCompositionIdentity(),
		ops:        compositionOps{apply: applyCompositionOwned, committed: compositionCommittedText, settle: settleCompositionStorage, trimCost: compositionTrimCostOwned, trim: trimCompositionHistoryOwned},
		cost:       cost,
		unitStart:  -1,
		eventStart: -1,
		enabled:    true,
	})
	return true
}

// DisableTentativeComposition disables explicit input without changing ordinary
// units, producer frames/cursors or visible marking. Completed protection or
// oversized retired storage keeps a charged disabled descriptor until a later
// mutation or Clear can release it.
func (editor *Editor) DisableTentativeComposition() (ok bool) {
	if editor == nil {
		return false
	}
	journal := editor.journal
	if journal != nil && journal.private {
		return false
	}
	if journal == nil {
		return true
	}
	state := journal.compositionState()
	if state == nil {
		return true
	}
	if state.owner != editor {
		return false
	}
	if editor.composing || state.active {
		return false
	}
	if !state.enabled {
		return true
	}
	if state.completed {
		// Inactive Disable leaves text/history/frames/cursors intact. Keep one
		// passive charged descriptor so the next trim cannot cut the completed
		// action; the enabled getter reports false meanwhile.
		state.enabled = false
		state.passive = true
		return true
	}
	state.enabled = false
	if state.ops.settle != nil {
		state.ops.settle(state)
	}
	return true
}

// CompositionActive reports whether a pre-edit currently owns rollback state:
// either the legacy composing flag or a live tentative composition. Callers use
// it to refuse operations that would strand an in-flight composition.
func (editor *Editor) CompositionActive() bool {
	if editor == nil {
		return false
	}
	if editor.composing {
		return true
	}
	state := editor.compositionState()
	return state != nil && state.active
}

// TentativeCompositionEnabled is a nil-safe, allocation-free capability read.
func (editor *Editor) TentativeCompositionEnabled() bool {
	state := editor.compositionState()
	return state != nil && state.enabled
}

// TentativeCompositionActive reports an owned explicit rollback boundary. A
// legacy marked string alone does not establish that boundary. The read is
// nil-safe and does not allocate or enable optional state.
func (editor *Editor) TentativeCompositionActive() bool {
	return editor.tentativeActive()
}

// TentativeCompositionIdentity snapshots ownership without allocating,
// enabling optional state or retaining the editor. Disabled owners are zero.
func (editor *Editor) TentativeCompositionIdentity() CompositionIdentity {
	state := editor.compositionState()
	if state != nil && state.enabled {
		return state.identity
	}
	return CompositionIdentity{}
}

// ApplyComposition handles one explicit event. True includes valid no-ops;
// false rejects without source mutation.
func (editor *Editor) ApplyComposition(event CompositionEvent) (handled bool) {
	if editor == nil || editor.journal == nil || editor.journal.compositionState() == nil {
		return false
	}
	state := editor.compositionState()
	if state == nil || !state.enabled {
		return false
	}
	apply := state.ops.apply
	if apply == nil {
		return false
	}
	return apply(state, event)
}

func applyCompositionOwned(state *compositionState, event CompositionEvent) bool {
	if state == nil || state.owner == nil {
		return false
	}
	editor := state.owner
	state.retainEqual = true
	defer func() { state.retainEqual = false }()
	switch event.Phase {
	case CompositionUpdate, CompositionResult, CompositionCommit, CompositionCancel, CompositionEmptyCommit:
	default:
		return false
	}
	if editor.composing && !state.active {
		// A legacy/Clear residue has no owned tentative boundary; fail closed
		// until the legacy Unmark path ends it.
		return false
	}
	var text []rune
	if event.Phase == CompositionUpdate || event.Phase == CompositionResult {
		if !utf8.ValidString(event.Text) {
			return false
		}
		text = []rune(event.Text)
	}
	usesReplacement := event.Phase == CompositionResult ||
		event.Phase == CompositionUpdate && len(text) > 0 ||
		event.Phase == CompositionEmptyCommit && !state.active
	if event.HasReplacementUTF16 && usesReplacement {
		if event.ReplacementUTF16.Start < 0 || event.ReplacementUTF16.Length < 0 {
			return false
		}
	}
	if event.Phase == CompositionUpdate && state.active && (event.HasReplacement || event.HasReplacementUTF16) {
		return false
	}
	plan, ok := editor.planCompositionEvent(state, event, text)
	if !ok {
		return false
	}
	if plan.mutate {
		// Retiring completed protection is a descriptor-only action. It happens
		// after a successful preflight, so a refused event never retires it, and
		// before any byte/history mutation.
		editor.retireCompletedRetention()
		// Redo belongs to the old branch. Remove it before recording physical
		// tentative indices, after preflight has guaranteed this event is handled.
		editor.dropRedo()
	}
	newUnits := plan.units
	if plan.joinsPrevious {
		newUnits--
	}
	if newUnits > 0 && !state.active {
		// Active update/result events roll back first and reserve their exact
		// post-rollback suffix capacity inside the apply step.
		editor.reserveCompositionHistory(len(editor.history) + newUnits)
	}
	state.pending = true
	state.pendingUnits = plan.units
	editor.chargeCompositionCapacity()
	pending := editor.beginCompositionHistory(state, plan)
	switch event.Phase {
	case CompositionUpdate:
		editor.applyCompositionUpdate(state, event, text, pending, plan.units)
	case CompositionResult:
		editor.applyCompositionResult(state, event, text, pending, plan)
	case CompositionCommit:
		editor.applyCompositionCommit(state)
	case CompositionCancel:
		editor.applyCompositionCancel(state)
	case CompositionEmptyCommit:
		editor.applyCompositionEmptyCommit(state, event)
	}
	if plan.units > 0 {
		editor.ownCompositionEventText(state)
	}
	editor.finishCompositionHistory(state, pending)
	state.pending = false
	state.pendingUnits = 0
	editor.chargeCompositionCapacity()
	return true
}

// compositionPlan is the physical and producer-shape preflight for one explicit
// event. payload counts rune slots retained by ordinary history, units counts
// the ordinary units that must survive, and steps counts source primitives.
type compositionPlan struct {
	steps   int
	units   int
	payload int

	mutate        bool
	joinsPrevious bool
}

// planCompositionEvent separates the editor's hard cancelability preflight from
// metadata reservation. A false result never mutates text, history or selection.
func (editor *Editor) planCompositionEvent(state *compositionState, event CompositionEvent, text []rune) (plan compositionPlan, ok bool) {
	if state.active && (state.unitStart < 0 || state.unitStart > len(editor.history)) {
		return plan, false
	}
	if state.active && event.Phase != CompositionCommit {
		plan.steps += editor.compositionRollbackSteps(state)
	}
	needBase := false
	switch event.Phase {
	case CompositionUpdate:
		needBase = len(text) > 0
	case CompositionResult:
		needBase = len(text) > 0 || event.HasReplacement || event.HasReplacementUTF16
	case CompositionEmptyCommit:
		needBase = !state.active
	}
	base := editor.runes
	if needBase {
		var baseOK bool
		base, baseOK = editor.compositionPlanBase(state)
		if !baseOK {
			return plan, false
		}
	}
	switch event.Phase {
	case CompositionUpdate:
		if len(text) == 0 {
			plan.mutate = state.active // empty update is cancel
			return plan, true
		}
		plan.mutate = true
		if state.active {
			editor.compositionPlanScalars(&plan, base, state.start, text, 0, 0)
			return plan, editor.compositionPlanFits(state, plan)
		}
		target := editor.compositionReplacementRangeFor(base, event)
		deleted := target.End - target.Start
		if deleted > 0 {
			plan.steps++
			plan.units++
			plan.payload += deleted

		}
		editor.compositionPlanScalars(&plan, base, target.Start, text, deleted, 0)
	case CompositionResult:
		hasExplicit := event.HasReplacement || event.HasReplacementUTF16
		if hasExplicit {
			target := editor.compositionReplacementRangeFor(base, event)
			deleted := target.End - target.Start
			if deleted > 0 {
				plan.steps++
				plan.units++
				plan.payload += deleted

			}
			if len(text) > 0 {
				editor.compositionPlanScalars(&plan, base, target.Start, text, deleted, 0)
				if !state.active && deleted == 0 {
					editor.compositionPlanPreviousInsert(&plan, target.Start)
				}
			}
			plan.mutate = state.active || len(text) > 0 || deleted > 0
			return plan, editor.compositionPlanFits(state, plan)
		}
		if state.active {
			if len(text) > 0 {
				editor.compositionPlanScalars(&plan, base, state.start, text, 0, 0)
			}
			plan.mutate = true
			return plan, editor.compositionPlanFits(state, plan)
		}
		selection := editor.Selection()
		grouped := 0
		if len(text) > 0 {
			grouped = selection.End - selection.Start
		}
		if len(text) > 0 {
			editor.compositionPlanScalars(&plan, base, selection.Start, text, 0, grouped)
			if grouped == 0 {
				editor.compositionPlanPreviousInsert(&plan, selection.Start)
			}
		}
		plan.mutate = len(text) > 0
	case CompositionEmptyCommit:
		if state.active {
			plan.mutate = true
			return plan, true
		}
		target := editor.compositionReplacementRangeFor(base, event)
		deleted := target.End - target.Start
		if deleted > 0 {
			plan.steps++
			plan.units++
			plan.payload += deleted

			plan.mutate = true
		}
	case CompositionCommit:
		plan.mutate = state.active
	case CompositionCancel:
		plan.mutate = state.active
	}
	if plan.units == 0 {
		return plan, true
	}
	return plan, editor.compositionPlanFits(state, plan)
}

func (editor *Editor) compositionPlanPreviousInsert(plan *compositionPlan, start int) {
	if editor.overwrite || editor.coalescing != editInsert || editor.historyIndex == 0 || editor.historyIndex != len(editor.history) || editor.AtSavePoint() {
		return
	}
	previous := &editor.history[editor.historyIndex-1]
	if previous.kind != editInsert || previous.after != (editorSelection{anchor: start, caret: start}) || len(previous.removed) != 0 || previous.start+len(previous.inserted) != start {
		return
	}
	// The source scalar loop coalesces into this existing unit. Its text
	// cannot be pruned independently of the accepted new input.
	plan.payload += len(previous.inserted)
	plan.joinsPrevious = true
}

// compositionPlanFits applies the physical single-action bound independently of
// discardable producer metadata and of the replaced generation. An active
// update rolls back its old tentative suffix before inserting the new one, so
// charging state.cost (which includes the old unit-array capacity) in addition
// to the new units would count two mutually exclusive generations. Only the
// post-rollback event's own unit headers and retained rune text are required to
// stay cancelable within the single-action bound.
func (editor *Editor) compositionPlanFits(state *compositionState, plan compositionPlan) bool {
	room := maxSingleEditRunes - compositionHeaderRunes - editorSidecarRunes
	if plan.payload < 0 || plan.units < 0 || plan.payload > room {
		return false
	}
	return plan.units <= (room-plan.payload)/historyEditRunes
}

// compositionPlanScalars simulates the source scalar loop without mutating the
// editor. base is post-rollback text; deleted counts an ordinary pre-start or
// explicit deletion before the scalars; grouped counts an implicit selection
// deletion that belongs to the first scalar's ordinary unit.
func (editor *Editor) compositionPlanScalars(plan *compositionPlan, base []rune, start int, text []rune, deleted, grouped int) {
	if len(text) == 0 {
		return
	}
	plan.payload += len(text)
	overwrite := editor.overwrite

	if overwrite {
		plan.units += len(text)
	} else if len(text) > 1 && grouped > 0 {
		// An implicit selection replacement owns the first scalar's unit.
		// Other scalars coalesce into one new suffix, even at a save point.
		plan.units += 2
	} else {
		plan.units++
	}
	shift := -deleted
	cursor := start
	for index := range text {
		switch {
		case index == 0 && grouped > 0:
			plan.payload += grouped
			plan.steps += 2
			shift -= grouped
		case overwrite:
			position := cursor - shift
			if cursor < len(base)+shift && position >= 0 && position < len(base) && !compositionLineEnd(base[position]) {
				plan.payload++
				plan.steps += 2
			} else {
				plan.steps++
				shift++
			}
		default:
			plan.steps++
			shift++
		}
		cursor++
	}
}

// compositionPlanBase reconstructs the text after rolling back the owned
// tentative suffix, without touching the live editor. Inactive events use the
// current text directly.
func (editor *Editor) compositionPlanBase(state *compositionState) ([]rune, bool) {
	if !state.active {
		return editor.runes, true
	}
	return editor.compositionRollbackRunes(state)
}

// compositionRollbackRunes reconstructs the post-rollback text in one forward
// pass over the final buffer and the owned suffix units. Each unit's recorded
// start is its final rune position; replacing those inserted segments with the
// recorded removed text restores the pre-event buffer without copying the whole
// document once per unit. The returned slice is owned by the caller.
func (editor *Editor) compositionRollbackRunes(state *compositionState) ([]rune, bool) {
	if state == nil || !state.active {
		return editor.runes, true
	}
	if state.unitStart < 0 || state.unitStart > len(editor.history) {
		return nil, false
	}
	final := editor.runes
	capacity := len(final)
	for index := state.unitStart; index < len(editor.history); index++ {
		edit := &editor.history[index]
		capacity += len(edit.removed) - len(edit.inserted)
	}
	if capacity < 0 {
		capacity = 0
	}
	base := make([]rune, 0, capacity)
	previous := 0
	for index := state.unitStart; index < len(editor.history); index++ {
		edit := &editor.history[index]
		start := edit.start
		if start < previous || start < 0 || start > len(final) || start+len(edit.inserted) > len(final) {
			return nil, false
		}
		if !runesEqual(final[start:start+len(edit.inserted)], edit.inserted) {
			return nil, false
		}
		base = append(base, final[previous:start]...)
		base = append(base, edit.removed...)
		previous = start + len(edit.inserted)
	}
	base = append(base, final[previous:]...)
	return base, true
}

// compositionCommittedText is installed only by EnableTentativeComposition so a
// default client that never enables the capability does not link the rollback
// reconstruction merely by calling CommittedString.
func compositionCommittedText(state *compositionState) (string, bool) {
	if state == nil || state.owner == nil {
		return "", false
	}
	committed, ok := state.owner.compositionRollbackRunes(state)
	if !ok {
		return "", false
	}
	return string(committed), true
}

func compositionLineEnd(value rune) bool {
	return value == '\n' || value == '\r'
}

// compositionRollbackSteps counts the inverse source primitives that a private
// rollback will emit. It never treats a unit ordinal as the producer Current.
func (editor *Editor) compositionRollbackSteps(state *compositionState) int {
	count := 0
	for index := state.unitStart; index < len(editor.history); index++ {
		edit := &editor.history[index]
		if len(edit.primitives) == 0 {
			if len(edit.removed) > 0 {
				count++
			}
			if len(edit.inserted) > 0 {
				count++
			}
			continue
		}
		for _, primitive := range edit.primitives {
			if primitive.Removed != 0 {
				count++
			}
			if primitive.Inserted != 0 {
				count++
			}
		}
	}
	return count
}

func (editor *Editor) compositionReplacementRange(event CompositionEvent) Range {
	return editor.compositionReplacementRangeFor(editor.runes, event)
}

func (editor *Editor) compositionReplacementRangeFor(base []rune, event CompositionEvent) Range {
	if event.HasReplacementUTF16 {
		return editor.compositionUTF16RangeFor(base, event.ReplacementUTF16)
	}
	if event.HasReplacement {
		return normalizeRange(event.Replacement, len(base))
	}
	if len(base) == len(editor.runes) {
		return editor.Selection()
	}
	return Range{Start: editor.caret, End: editor.caret}
}

func (editor *Editor) applyCompositionUpdate(state *compositionState, event CompositionEvent, text []rune, pending *historyEvent, units int) {
	if len(text) == 0 {
		editor.applyCompositionCancel(state)
		return
	}
	var start int
	if state.active {
		start = editor.rollbackComposition(state, pending)
		state.eventStart = len(editor.history)
		editor.reserveCompositionHistory(len(editor.history) + units)
	} else {
		state.eventStart = len(editor.history)
		target := editor.compositionReplacementRange(event)
		editor.compositionDeleteOrdinary(target)
		start = target.Start
	}
	editor.BreakUndoGroup()
	insertStart := start
	state.start = insertStart
	state.unitStart = len(editor.history)
	state.live = true
	state.completed = false
	state.completedStart = -1
	start = editor.compositionInsertEventScalars(start, text, true)
	editor.composition = Range{Start: insertStart, End: start}
	editor.composing = true
	selected := normalizeRange(event.Selected, len(text))
	editor.anchor = insertStart + selected.Start
	editor.caret = insertStart + selected.End
	if state.unitStart >= 0 && state.unitStart < len(editor.history) {
		// The merged tentative unit must restore the event's final Selected on
		// Redo, not the transient caret left by the last scalar insertion.
		editor.history[state.unitStart].after = editor.selectionState()
	}
	state.active = true
}

func (editor *Editor) applyCompositionResult(state *compositionState, event CompositionEvent, text []rune, pending *historyEvent, plan compositionPlan) {
	wasActive := state.active
	if wasActive {
		editor.rollbackComposition(state, pending)
		editor.reserveCompositionHistory(len(editor.history) + plan.units)
	}
	var explicit Range
	hasExplicit := false
	switch {
	case event.HasReplacementUTF16:
		explicit = editor.compositionUTF16Range(event.ReplacementUTF16)
		hasExplicit = true
	case event.HasReplacement:
		explicit = normalizeRange(event.Replacement, len(editor.runes))
		hasExplicit = true
	}
	if !wasActive && len(text) == 0 && (!hasExplicit || explicit.Start == explicit.End) {
		// Valid no-op result: never retire completed retention or change bytes.
		editor.composing = false
		editor.composition = Range{}
		return
	}
	state.eventStart = len(editor.history)
	if plan.joinsPrevious {
		state.eventStart--
	}
	state.live = false
	state.completed = false
	state.completedStart = -1
	editor.composing = false
	editor.composition = Range{}
	if hasExplicit {
		deleted := editor.compositionDeleteOrdinary(explicit)
		editor.collapse(explicit.Start)
		if len(text) > 0 {
			editor.compositionInsertEventScalars(explicit.Start, text, deleted)
		}
		editor.compositionComplete(state)
		editor.composing = false
		editor.composition = Range{}
		return
	}
	if len(text) == 0 {
		// Empty result never deletes an implicit selection.
		editor.composing = false
		editor.composition = Range{}
		return
	}
	selection := editor.Selection()
	if selection.Start != selection.End {
		editor.compositionReplaceAndInsert(selection, text)
	} else {
		editor.compositionInsertEventScalars(selection.Start, text, false)
	}
	editor.compositionComplete(state)
	editor.composing = false
	editor.composition = Range{}
}

func (editor *Editor) applyCompositionCommit(state *compositionState) {
	if !state.active {
		return
	}
	state.active = false
	state.live = false
	editor.compositionComplete(state)
	editor.composing = false
	editor.composition = Range{}
	editor.dropRedo()
	editor.syncCompositionTail()
}

func (editor *Editor) applyCompositionCancel(state *compositionState) {
	if !state.active {
		return
	}
	editor.rollbackComposition(state, editor.compositionEvent())
	editor.composing = false
	editor.composition = Range{}
	editor.dropRedo()
	editor.syncCompositionTail()
}

func (editor *Editor) applyCompositionEmptyCommit(state *compositionState, event CompositionEvent) {
	if state.active {
		editor.rollbackComposition(state, editor.compositionEvent())
		editor.composing = false
		editor.composition = Range{}
		editor.dropRedo()
		editor.syncCompositionTail()
		return
	}
	state.eventStart = len(editor.history)
	target := editor.compositionReplacementRange(event)
	editor.compositionDeleteOrdinary(target)
	editor.dropRedo()
	editor.syncCompositionTail()
	editor.compositionComplete(state)
	editor.composing = false
	editor.composition = Range{}
}

// compositionComplete transfers live protection to one newest completed action
// only when the accepted event actually retained ordinary units.
func (editor *Editor) compositionComplete(state *compositionState) {
	if state.eventStart >= 0 && state.eventStart < len(editor.history) {
		state.completed = true
		state.completedStart = state.eventStart
	}
}

// rollbackComposition reverses only the owned tentative suffix, newest unit
// first, and truncates the suffix privately. It never touches the ordinary
// pre-start deletion. The producer frame is assembled by the event capture.
func (editor *Editor) rollbackComposition(state *compositionState, pending *historyEvent) int {
	if !state.active || state.unitStart < 0 || state.unitStart > len(editor.history) {
		state.active = false
		return state.start
	}
	for index := len(editor.history) - 1; index >= state.unitStart; index-- {
		edit := &editor.history[index]
		if pending != nil && editor.historyState() != nil {
			editor.captureHistoryReplay(edit, true)
		}
		editor.replaceHistory(edit, true)
	}
	editor.truncateCompositionUnits(state.unitStart)
	state.active = false
	state.live = false
	state.completed = false
	state.completedStart = -1
	state.eventStart = -1
	editor.collapse(clamp(state.start, 0, len(editor.runes)))
	editor.BreakUndoGroup()
	if pending != nil {
		pending.retainedTo = pending.current
	}
	return editor.caret
}

func (editor *Editor) truncateCompositionUnits(from int) {
	from = clamp(from, 0, len(editor.history))
	for index := from; index < len(editor.history); index++ {
		edit := &editor.history[index]
		editor.historyRunes -= editor.historyUnitRunes(edit) + editor.forgetLineBatch(edit)

	}
	clear(editor.history[from:])
	editor.history = editor.history[:from]
	if editor.historyIndex > from {
		editor.historyIndex = from
	}
	if point := editor.savePointIndex(); point != nil && *point > from {
		*point = -1
	}
	editor.coalescing = 0
	editor.compactCompositionHistory()
}

func (editor *Editor) compositionDeleteOrdinary(target Range) bool {
	target = normalizeRange(target, len(editor.runes))
	if target.Start == target.End {
		return false
	}
	before := editor.selectionState()
	editor.collapse(target.Start)
	change := editor.replace(target, "", false)
	editor.collapse(target.Start)
	editor.record(change, before, editor.selectionState(), editSeparate)
	return true
}

// compositionRecord is the explicit-event record path. Unlike the default
// record it retains a nonempty equal replacement (a valid source action) without
// requiring save-point or producer state, keeping the default retainEqualEdits
// fast path unchanged.
func (editor *Editor) compositionRecord(change textChange, before, after editorSelection, kind editKind) {
	if runesEqual(change.removed, change.inserted) && len(change.removed) == 0 {
		return
	}
	editor.dropRedo()
	var primitive Edit
	historyOwned := false
	if kind == editComposition {
		editor.invalidateHistoryFrom(0)
	} else if history := editor.historyState(); history != nil {
		primitive, historyOwned = history.ops.record(history, change, kind)
	}
	unitOwned := historyOwned && kind != editSeparate
	if editor.merge(change, before, after, kind, primitive, unitOwned) {
		editor.trimHistory()
		return
	}
	edit := historyEdit{
		start: change.start, removed: change.removed, inserted: change.inserted,
		before: before, after: after, kind: kind,
	}
	if unitOwned {
		edit.primitives = []Edit{primitive}
	}
	editor.appendHistory(edit)
}

func (editor *Editor) compositionInsertScalarsWithBreak(start int, text []rune, breakBefore bool) int {
	if len(text) == 0 {
		return start
	}
	if breakBefore {
		editor.BreakUndoGroup()
	}
	start = clamp(start, 0, len(editor.runes))
	editor.collapse(start)
	for index := range text {
		before := editor.selectionState()
		change := editor.replaceRunes(Range{Start: start, End: start}, text[index:index+1], false)
		editor.collapse(start + 1)
		editor.compositionRecord(change, before, editor.selectionState(), editInsert)
		start++
	}
	return start
}

// compositionInsertEventScalars applies the source scalar loop. Overwrite mode
// emits one isolated ordinary Undo unit per scalar and deletes exactly one code
// point outside the line terminator/EOF; insertion mode keeps E1's coalescing.
func (editor *Editor) compositionInsertEventScalars(start int, text []rune, breakFirst bool) int {
	if len(text) == 0 {
		return start
	}
	if !editor.overwrite {
		return editor.compositionInsertScalarsWithBreak(start, text, breakFirst)
	}
	start = clamp(start, 0, len(editor.runes))
	editor.collapse(start)
	for _, value := range text {
		start = editor.compositionOverwriteScalar(start, value)
	}
	return start
}

// compositionOverwriteScalar is the exact source InsertCharacter empty-selection
// overwrite step: delete one code point when the caret is before EOF and not in
// a line terminator, then insert the scalar. It uses one ordinary change/unit,
// so it neither deletes a grapheme cluster nor the whole replacement width.
func (editor *Editor) compositionOverwriteScalar(start int, value rune) int {
	start = clamp(start, 0, len(editor.runes))
	target := Range{Start: start, End: start}
	if start < len(editor.runes) && !compositionLineEnd(editor.runes[start]) {
		target.End = start + 1
	}
	before := editor.selectionState()
	editor.collapse(start)
	var scalar [1]rune
	scalar[0] = value
	change := editor.replaceRunes(target, scalar[:], false)
	editor.collapse(start + 1)
	editor.compositionRecord(change, before, editor.selectionState(), editSeparate)
	return start + 1
}

// compositionReplaceAndInsert is the source Result grouping: an implicit
// selection deletion shares one ordinary unit with the first scalar, while the
// producer still receives the exact ordered delete and per-scalar inserts.
func (editor *Editor) compositionReplaceAndInsert(target Range, text []rune) {
	target = normalizeRange(target, len(editor.runes))
	if target.Start == target.End || len(text) == 0 {
		editor.compositionInsertEventScalars(target.Start, text, false)
		return
	}
	// Source InsertCharacter grouping: the implicit selection deletion shares
	// its ordinary unit with the FIRST scalar. EndUndoAction then makes the
	// following scalars a separate source group.
	before := editor.selectionState()
	editor.collapse(target.Start)
	change := editor.replaceRunes(target, text[:1], false)
	editor.collapse(target.Start + 1)
	editor.compositionRecord(change, before, editor.selectionState(), editSeparate)
	if len(text) > 1 {
		editor.compositionInsertEventScalars(target.Start+1, text[1:], true)
	}
}

// compositionUTF16Range converts raw code units after any rollback. Start is
// measured from zero and Length relative to that start. Valid boundaries follow
// the pinned source start-from-zero/relative-length conversion; out-of-range or
// split-surrogate boundaries fall to the document end as the E1 contract binds.
func (editor *Editor) compositionUTF16Range(raw UTF16Range) Range {
	return editor.compositionUTF16RangeFor(editor.runes, raw)
}

func (editor *Editor) compositionUTF16RangeFor(text []rune, raw UTF16Range) Range {
	total := editor.compositionUTF16Length(text)
	start, startOK := editor.compositionUTF16StartIn(text, raw.Start, total)
	if !startOK {
		// An invalid start maps both ends to the document end.
		end := len(text)
		return Range{Start: end, End: end}
	}
	end, endOK := editor.compositionUTF16RelativeIn(text, start, raw.Length)
	if !endOK {
		end = len(text)
	}
	if end < start {
		end = start
	}
	return Range{Start: start, End: end}
}

// compositionUTF16Relative converts a nonnegative code-unit length relative to
// an already resolved rune start. It never adds the raw start and length, so a
// huge positive length simply falls to the document end instead of overflowing.
func (editor *Editor) compositionUTF16Relative(start, units int) (int, bool) {
	return editor.compositionUTF16RelativeIn(editor.runes, start, units)
}

func (editor *Editor) compositionUTF16RelativeIn(text []rune, start, units int) (int, bool) {
	if units == 0 {
		return start, true
	}
	if start < 0 || start > len(text) {
		return 0, false
	}
	remaining := units
	for index := start; index < len(text); index++ {
		width := 1
		if text[index] > 0xffff {
			width = 2
		}
		if remaining < width {
			return 0, false
		}
		remaining -= width
		if remaining == 0 {
			return index + 1, true
		}
	}
	return 0, false
}

func (editor *Editor) compositionUTF16Start(units, total int) (int, bool) {
	return editor.compositionUTF16StartIn(editor.runes, units, total)
}

func (editor *Editor) compositionUTF16StartIn(text []rune, units, total int) (int, bool) {
	if units < 0 || units > total {
		return 0, false
	}
	consumed := 0
	for index, character := range text {
		width := 1
		if character > 0xffff {
			width = 2
		}
		if consumed == units {
			return index, true
		}
		if consumed+width > units {
			// The raw position splits a surrogate pair.
			return 0, false
		}
		consumed += width
		if consumed == units {
			return index + 1, true
		}
	}
	if consumed == units {
		return len(text), true
	}
	return 0, false
}

func (editor *Editor) compositionUTF16Length(text []rune) int {
	units := 0
	for _, value := range text {
		if value > 0xffff {
			units += 2
		} else {
			units++
		}
	}
	return units
}
