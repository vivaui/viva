package text

import "slices"

// LFLabelReplacement describes one actual forward replacement. Labels belongs
// to the callback and contains the labels of Removed's LF characters.
type LFLabelReplacement struct {
	Removed, Inserted string
	Labels            []byte
	Default           byte
}

// LFLabelMapper returns one label for every LF in Inserted. It must not mutate
// or reenter the editor. Returning the wrong number of labels panics before
// text is changed. A nil mapper labels each inserted LF with Default.
type LFLabelMapper func(LFLabelReplacement) []byte

type lfLabelCell struct{ value byte }

type lfLabelEdit struct {
	line              int
	removed, inserted []*lfLabelCell
}

type lfLabelEntry struct {
	edits     []lfLabelEdit
	fragments int
}

type lfLabelOps struct {
	replace func(*lfLabelState, int, []rune, []rune)
	record  func(*lfLabelState, *rune, *rune)
	replay  func(*lfLabelState, *rune, bool)
	forget  func(*lfLabelState, *rune)
	clear   func(*lfLabelState)
	batch   func(*lfLabelState, []lfLabelSpan) (*lfLabelPlan, error)
	clone   func(*lfLabelState, *Editor) *lfLabelState
	finish  func(*lfLabelState, *lfLabelState) *lfLabelPlan
	commit  func(*lfLabelState, *lfLabelPlan)
	hold    func(*lfLabelState, textChange)
	cancel  func(*lfLabelState)
	release func(*lfLabelState)
	discard func(*lfLabelState)
	cost    func(*lfLabelState, *rune) int
	extra   func(*lfLabelState) int
}

type lfLabelState struct {
	ops          lfLabelOps
	owner        *Editor
	current      []*lfLabelCell
	defaultLabel byte
	mapper       LFLabelMapper
	history      map[*rune]lfLabelEntry
	pending      []lfLabelEdit
	replaying    bool
	mapping      bool
	staging      bool
	events       []lfLabelEdit
	revision     uint64
	lastLine     int
	composition  lfLabelComposition
}

// TrackLFLabels enables optional labels before the editor has Undo/Redo or a
// live composition. It copies labels, requiring one per current logical LF.
// An already tracked editor refuses a second source. Labels have no meaning to
// text; applications define their own replacement policy with mapper.
// Private transaction drafts cannot enable or reconfigure labels.
func (editor *Editor) TrackLFLabels(labels []byte, defaultLabel byte, mapper LFLabelMapper) bool {
	if editor == nil || editor.composing || editor.CanUndo() || editor.CanRedo() || editor.lfLabelState() != nil || editor.journal != nil && editor.journal.private {
		return false
	}
	if countLabelLF(editor.runes) != len(labels) {
		return false
	}
	editor.TrackLineEdits()
	state := &lfLabelState{
		owner: editor,
		ops: lfLabelOps{
			replace: replaceLFLabels,
			record:  recordLFLabels,
			replay:  replayLFLabels,
			forget:  forgetLFLabels,
			clear:   clearLFLabelHistory,
			batch:   planLFLabelBatch,
			clone:   cloneLFLabels,
			finish:  finishLFLabelPlan,
			commit:  commitLFLabelPlan,
			hold:    holdLFLabelComposition,
			cancel:  cancelLFLabelComposition,
			release: releaseLFLabelComposition,
			discard: discardLFLabelHeldHistory,
			cost:    lfLabelHistoryCost,
			extra:   lfLabelExtraCost,
		},
		defaultLabel: defaultLabel,
		mapper:       mapper,
		current:      make([]*lfLabelCell, len(labels)),
	}
	for index, value := range labels {
		state.current[index] = &lfLabelCell{value: value}
	}
	editor.journal.lines.labels = state
	return true
}

// LFLabels copies current labels into dst[:0]. The bool distinguishes an enabled
// tracker with no LF characters from an editor that never enabled labels.
func (editor *Editor) LFLabels(dst []byte) ([]byte, bool) {
	dst = dst[:0]
	state := editor.lfLabelState()
	if state == nil {
		return dst, false
	}
	for _, cell := range state.current {
		dst = append(dst, cell.value)
	}
	return dst, true
}

// LFLabelRevision identifies the current label state for an enabled editor.
// It changes on label mutations and replay, including equal-text edits. Cache
// keys must include the editor identity; a revision is local to that source.
func (editor *Editor) LFLabelRevision() (uint64, bool) {
	state := editor.lfLabelState()
	if state == nil {
		return 0, false
	}
	return state.revision, true
}

// SetLFLabels relabels current LF identities and the insertion default without
// adding text history. Those identities retain the labels through Undo/Redo.
// An invalid count, disabled tracker, active composition or private transaction
// draft leaves state alone.
func (editor *Editor) SetLFLabels(labels []byte, defaultLabel byte) bool {
	state := editor.lfLabelState()
	if state == nil || editor.composing || state.mapping || editor.journal.private || len(labels) != len(state.current) {
		return false
	}
	for index, value := range labels {
		state.current[index].value = value
	}
	state.defaultLabel = defaultLabel
	state.revision++
	return true
}

// UnifyLFLabels relabels both current and reachable historical LF identities.
// Old text restored by Undo/Redo therefore retains the new uniform policy.
// Active compositions and private transaction drafts refuse the change.
func (editor *Editor) UnifyLFLabels(label byte) bool {
	state := editor.lfLabelState()
	if state == nil || editor.composing || state.mapping || editor.journal.private {
		return false
	}
	for _, cell := range state.current {
		cell.value = label
	}
	for _, entry := range state.history {
		for _, edit := range entry.edits {
			for _, cells := range [][]*lfLabelCell{edit.removed, edit.inserted} {
				for _, cell := range cells {
					cell.value = label
				}
			}
		}
	}
	state.defaultLabel = label
	state.revision++
	return true
}

func (editor *Editor) lfLabelState() *lfLabelState {
	if editor == nil || editor.journal == nil || editor.journal.lines == nil {
		return nil
	}
	return editor.journal.lines.labels
}

func (editor *Editor) pendingLFLabels() bool {
	state := editor.lfLabelState()
	return state != nil && len(state.pending) != 0
}

func lfLabelHistoryKey(edit *historyEdit) *rune {
	if len(edit.removed) != 0 {
		return &edit.removed[0]
	}
	if len(edit.inserted) != 0 {
		return &edit.inserted[0]
	}
	return nil
}

func countLabelLF(value []rune) int {
	count := 0
	for _, character := range value {
		if character == '\n' {
			count++
		}
	}
	return count
}

func replaceLFLabels(state *lfLabelState, line int, removed, inserted []rune) {
	if state.mapping {
		panic("text: LF label mapper reentered editing")
	}
	if state.replaying {
		return
	}
	state.lastLine = line
	removedLF, insertedLF := countLabelLF(removed), countLabelLF(inserted)
	if removedLF == 0 && insertedLF == 0 {
		return
	}
	previous := state.current[line : line+removedLF]
	labels := make([]byte, insertedLF)
	if state.mapper != nil && insertedLF != 0 {
		oldLabels := make([]byte, len(previous))
		for index, cell := range previous {
			oldLabels[index] = cell.value
		}
		labels = mapLFLabels(state, LFLabelReplacement{Removed: string(removed), Inserted: string(inserted), Labels: oldLabels, Default: state.defaultLabel})
		if len(labels) != insertedLF {
			panic("text: LF label mapper returned the wrong number of labels")
		}
	} else {
		for index := range labels {
			labels[index] = state.defaultLabel
		}
	}
	if state.owner.savePointIndex() == nil && runesEqual(removed, inserted) && len(previous) == len(labels) {
		equal := true
		for index, cell := range previous {
			equal = equal && cell.value == labels[index]
		}
		if equal {
			return
		}
	}
	edit := lfLabelEdit{line: line, removed: slices.Clone(previous), inserted: make([]*lfLabelCell, len(labels))}
	for index, value := range labels {
		edit.inserted[index] = &lfLabelCell{value: value}
	}
	spliceLFLabels(state, line, len(edit.removed), edit.inserted)
	if state.composition.active {
		state.composition.currentLF = insertedLF
	}
	state.pending = append(state.pending, edit)
	if state.staging {
		state.events = append(state.events, edit)
	}
}

func mapLFLabels(state *lfLabelState, change LFLabelReplacement) []byte {
	state.mapping = true
	defer func() { state.mapping = false }()
	return state.mapper(change)
}

func spliceLFLabels(state *lfLabelState, line, removed int, inserted []*lfLabelCell) {
	before := len(state.current)
	delta := len(inserted) - removed
	if delta > 0 {
		state.current = append(state.current, make([]*lfLabelCell, delta)...)
	}
	copy(state.current[line+len(inserted):], state.current[line+removed:before])
	if delta < 0 {
		clear(state.current[before+delta:])
		state.current = state.current[:before+delta]
	}
	copy(state.current[line:], inserted)
	if len(state.current) == 0 {
		state.current = nil
	} else if cap(state.current) > 64 && len(state.current) < cap(state.current)/4 {
		current := make([]*lfLabelCell, len(state.current))
		copy(current, state.current)
		state.current = current
	}
	state.revision++
}

func recordLFLabels(state *lfLabelState, previous, current *rune) {
	var edits []lfLabelEdit
	if previous != nil {
		edits = state.history[previous].edits
		if previous != current {
			if _, exists := state.history[previous]; exists {
				ensureLFLabelHistoryOwned(state)
				delete(state.history, previous)
			}
		}
	}
	if state.composition.active {
		edits = compositionLFLabelEdits(state, previous)
	} else {
		edits = append(edits, state.pending...)
	}
	state.pending = nil
	if len(edits) != 0 {
		ensureLFLabelHistoryOwned(state)
		if state.history == nil {
			state.history = make(map[*rune]lfLabelEntry)
		}
		fragments := 0
		for _, edit := range edits {
			fragments += cap(edit.removed) + cap(edit.inserted)
		}
		state.history[current] = lfLabelEntry{edits: edits, fragments: fragments}
	} else if _, exists := state.history[current]; exists {
		ensureLFLabelHistoryOwned(state)
		delete(state.history, current)
		state.history = cloneLFLabelHistory(state.history)
	}
}

func replayLFLabels(state *lfLabelState, key *rune, inverse bool) {
	if state.mapping {
		panic("text: LF label mapper reentered history")
	}
	edits := state.history[key].edits
	if inverse {
		for index := len(edits) - 1; index >= 0; index-- {
			edit := edits[index]
			spliceLFLabels(state, edit.line, len(edit.inserted), edit.removed)
			if state.staging {
				state.events = append(state.events, lfLabelEdit{line: edit.line, removed: edit.inserted, inserted: edit.removed})
			}
		}
		return
	}
	for _, edit := range edits {
		spliceLFLabels(state, edit.line, len(edit.removed), edit.inserted)
		if state.staging {
			state.events = append(state.events, edit)
		}
	}
}

func forgetLFLabels(state *lfLabelState, key *rune) {
	if _, exists := state.history[key]; !exists {
		return
	}
	ensureLFLabelHistoryOwned(state)
	delete(state.history, key)
	state.history = cloneLFLabelHistory(state.history)
}

func clearLFLabelHistory(state *lfLabelState) {
	state.history = nil
	state.pending = nil
	if state.composition.active {
		state.composition.sharedHistory = false
		state.composition.recorded = false
		state.composition.unitOriginal = nil
		state.composition.prefix = nil
		syncLFLabelExtraCost(state)
	}
	state.revision++
}
