package text

import "github.com/vivaui/viva/internal/grapheme"

// Range is a half-open rune range.
type Range struct {
	Start int
	End   int
}

// Span is a half-open range of byte offsets into the UTF-8 text String returns:
// what a component that paints that text, or a search of it, works in. Range is
// the editor's own, counted in runes; Editor.ByteOffset and RuneIndex convert.
type Span struct {
	Start int
	End   int
}

const (
	maxHistoryEntries = 256
	maxHistoryRunes   = 256 * 1024
	// maxSingleEditRunes is the largest edit the history keeps once it is all that
	// is left: a replace-all in a big file is one undo unit however many runes it
	// touches, and losing every undo because it is large would cost more than the
	// memory of keeping it.
	maxSingleEditRunes = 4 * 1024 * 1024
)

type editKind uint8

const (
	editSeparate editKind = iota + 1
	editInsert
	editDeleteBackward
	editDeleteForward
	editComposition
)

type editorSelection struct {
	anchor int
	caret  int
}

type historyEdit struct {
	start             int
	removed, inserted []rune
	before, after     editorSelection
	kind              editKind
}

type textChange struct {
	start             int
	removed, inserted []rune
}

// Editor retains text, selection, and an optional IME composition range. Like
// its mutators, String is not safe for concurrent use: it caches the converted
// text.
type Editor struct {
	runes           []rune
	anchor          int
	caret           int
	composition     Range
	composing       bool
	compositionBase string

	// accept and maxRunes constrain what an edit may leave in the text; see
	// Restrict.
	accept    func(rune) bool
	maxRunes  int
	overwrite bool
	// code and codeSet make the typed-text methods code-aware; see SetCode.
	code    CodeOptions
	codeSet bool
	// lastReplaceStart is where the latest replace began; see replace.
	lastReplaceStart int

	// text caches String's conversion so a paint that reads the value every
	// frame does not allocate; every mutation of runes clears textValid.
	text      string
	textValid bool

	// offsets remembers the last rune/byte pair ByteOffset and RuneIndex
	// resolved, so a caret that moves or types a little is found by walking from
	// there instead of from the start; every mutation of runes trims it.
	offsets offsetMemo

	history      []historyEdit
	historyIndex int
	historyRunes int
	coalescing   editKind

	// visualStarts, visualOrder and visualOdd are the bidi caret walk's scratch,
	// reused by every arrow key on a line that needs reordering.
	visualStarts []int
	visualOrder  []int
	visualOdd    []bool
}

// NewEditor creates an editor with its caret at the end of initial text.
func NewEditor(initial string) *Editor {
	runes := []rune(initial)
	end := len(runes)
	return &Editor{runes: runes, anchor: end, caret: end}
}

// String returns the current UTF-8 text. The conversion is cached until the
// text next changes, so repeated calls between edits allocate nothing.
func (editor *Editor) String() string {
	if !editor.composing {
		editor.compositionBase = ""
	}
	if !editor.textValid {
		editor.text = string(editor.runes)
		editor.textValid = true
	}
	return editor.text
}

// CommittedString returns the text before marked input until the IME commits it.
func (editor *Editor) CommittedString() string {
	if editor.composing {
		return editor.compositionBase
	}
	return editor.String()
}

// Restrict limits what edits may put into the editor: accept reports whether a
// rune may be typed (nil accepts every rune) and maxRunes caps the length of
// the text (zero or less means no cap). It is how a one-time code takes only
// digits, six of them, or a port field only digits.
//
// The limits apply to edits made after the call, whether typed, pasted or
// composed by an IME: unacceptable runes are dropped from the inserted text,
// and what does not fit is cut off the end, so a paste of "12ab3456789" into an
// empty six-digit editor leaves "123456". Text already present is left alone,
// and deleting is never restricted.
func (editor *Editor) Restrict(accept func(rune) bool, maxRunes int) {
	editor.accept = accept
	editor.maxRunes = maxRunes
}

// SetOverwrite switches the editor between inserting and overwriting. When
// overwriting, text typed at a caret replaces the runes after it instead of
// pushing them along, and at the length cap set by Restrict it replaces the last
// rune instead of being refused: the behavior of a fixed-width field such as a
// one-time code, where the caret sits on a slot and the next digit fills it. A
// selection is replaced as always, and runes the restriction rejects delete
// nothing.
func (editor *Editor) SetOverwrite(overwrite bool) {
	editor.overwrite = overwrite
}

// RuneCount returns the number of Unicode code points.
func (editor *Editor) RuneCount() int {
	return len(editor.runes)
}

// Selection returns the normalized selection range.
func (editor *Editor) Selection() Range {
	return ordered(Range{Start: editor.anchor, End: editor.caret})
}

// Caret returns the rune index of the caret: the end of the selection that moves
// when it is extended, which is the Start of a selection made backwards.
func (editor *Editor) Caret() int {
	return editor.caret
}

// SetSelection updates the selection and ends active composition. A range whose
// Start is after its End is a selection made backwards: the caret is at the
// Start, as Caret reports, and Selection still returns it in order.
func (editor *Editor) SetSelection(selection Range) {
	anchor := clamp(selection.Start, 0, len(editor.runes))
	caret := clamp(selection.End, 0, len(editor.runes))
	if editor.anchor != anchor || editor.caret != caret || editor.composing {
		editor.BreakUndoGroup()
	}
	editor.anchor = anchor
	editor.caret = caret
	editor.composing = false
}

// MoveTo puts the caret at rune index, or with extend moves it there from where
// it was and keeps the other end of the selection where it is. It is how an
// editor applies a movement the text itself does not know, such as one line up
// in text a view has wrapped. It ends active composition.
func (editor *Editor) MoveTo(index int, extend bool) {
	editor.BreakUndoGroup()
	index = clamp(index, 0, len(editor.runes))
	if extend {
		editor.caret = index
	} else {
		editor.collapse(index)
	}
	editor.composing = false
}

// Composition returns the active marked-text range.
func (editor *Editor) Composition() (Range, bool) {
	return editor.composition, editor.composing
}

// Insert replaces the current selection or composition and places the caret
// after inserted text.
func (editor *Editor) Insert(value string) {
	target := editor.Selection()
	wasComposing := editor.composing
	if wasComposing {
		target = editor.composition
	}
	overwrite := editor.overwrite && !wasComposing && target.Start == target.End
	before := editor.selectionState()
	change := editor.replace(target, value, overwrite)
	// An overwrite that had to reach back to the last rune (the code is full)
	// left the caret after the rune it replaced.
	caret := editor.lastReplaceStart + len(change.inserted)
	if !overwrite {
		caret = target.Start + len(change.inserted)
	}
	editor.collapse(caret)
	editor.composing = false
	kind := editInsert
	if wasComposing {
		kind = editComposition
	} else if overwrite || target.Start != target.End || hasUndoBoundary(value) {
		kind = editSeparate
	}
	editor.record(change, before, editor.selectionState(), kind)
}

// Paste inserts text as one undo unit, separate from adjacent typing.
func (editor *Editor) Paste(value string) {
	editor.BreakUndoGroup()
	editor.Insert(value)
	editor.BreakUndoGroup()
}

// SetMarkedText replaces the current composition or selection with marked
// text. selected is relative to the marked text, matching native IME APIs.
func (editor *Editor) SetMarkedText(value string, selected Range) {
	if !editor.composing {
		editor.compositionBase = editor.String()
	}
	target := editor.Selection()
	if editor.composing {
		target = editor.composition
	}
	before := editor.selectionState()
	change := editor.replace(target, value, false)
	inserted := len(change.inserted)
	if inserted == 0 && value != "" {
		// Everything the IME proposed is unacceptable here (pinyin in a digit
		// field): there is nothing to mark.
		editor.collapse(target.Start)
		editor.composing = false
		editor.record(change, before, editor.selectionState(), editComposition)
		return
	}
	editor.composition = Range{Start: target.Start, End: target.Start + inserted}
	editor.composing = true
	selected = normalizeRange(selected, inserted)
	editor.anchor = target.Start + selected.Start
	editor.caret = target.Start + selected.End
	editor.record(change, before, editor.selectionState(), editComposition)
}

// Unmark commits the current marked text without changing it.
func (editor *Editor) Unmark() {
	editor.composing = false
	editor.BreakUndoGroup()
}

// DeleteBackward removes the selection or previous grapheme cluster.
func (editor *Editor) DeleteBackward() bool {
	if editor.deleteSelectionOrComposition() {
		return true
	}
	// An IME that has cleared its marked text leaves an empty composition; the
	// edit below shortens the text under it, so it ends here rather than keep a
	// range that no longer exists.
	editor.composing = false
	caret := editor.caret
	if caret == 0 {
		return false
	}
	start := previousBoundary(editor.runes, caret)
	before := editor.selectionState()
	change := editor.replace(Range{Start: start, End: caret}, "", false)
	editor.collapse(start)
	editor.record(change, before, editor.selectionState(), editDeleteBackward)
	return true
}

// DeleteForward removes the selection or next grapheme cluster.
func (editor *Editor) DeleteForward() bool {
	if editor.deleteSelectionOrComposition() {
		return true
	}
	editor.composing = false // see DeleteBackward
	caret := editor.caret
	if caret == len(editor.runes) {
		return false
	}
	end := nextBoundary(editor.runes, caret)
	before := editor.selectionState()
	change := editor.replace(Range{Start: caret, End: end}, "", false)
	editor.record(change, before, editor.selectionState(), editDeleteForward)
	return true
}

// MoveLeft moves or extends selection by one grapheme cluster to the left on the screen:
// the previous one, except in a line with right-to-left text, where the order is the one
// UAX #9 displays it in.
func (editor *Editor) MoveLeft(extend bool) bool {
	editor.BreakUndoGroup()
	editor.composing = false
	selection := editor.Selection()
	if !extend && selection.Start != selection.End {
		editor.collapse(selection.Start)
		return true
	}
	caret := editor.caret
	next := editor.stepLeft(caret)
	if next == caret {
		return false
	}
	if extend {
		editor.caret = next
	} else {
		editor.collapse(next)
	}
	return true
}

// MoveRight moves or extends selection by one grapheme cluster to the right on the screen
// (see MoveLeft).
func (editor *Editor) MoveRight(extend bool) bool {
	editor.BreakUndoGroup()
	editor.composing = false
	selection := editor.Selection()
	if !extend && selection.Start != selection.End {
		editor.collapse(selection.End)
		return true
	}
	caret := editor.caret
	next := editor.stepRight(caret)
	if next == caret {
		return false
	}
	if extend {
		editor.caret = next
	} else {
		editor.collapse(next)
	}
	return true
}

// RuneIndexToUTF16 converts a rune index to a native UTF-16 code-unit index.
func (editor *Editor) RuneIndexToUTF16(index int) int {
	index = clamp(index, 0, len(editor.runes))
	units := 0
	for _, value := range editor.runes[:index] {
		if value > 0xffff {
			units += 2
		} else {
			units++
		}
	}
	return units
}

// UTF16ToRuneIndex converts a native UTF-16 code-unit index to the nearest
// preceding rune boundary.
func (editor *Editor) UTF16ToRuneIndex(units int) int {
	if units <= 0 {
		return 0
	}
	consumed := 0
	for index, value := range editor.runes {
		width := 1
		if value > 0xffff {
			width = 2
		}
		if consumed+width > units {
			return index
		}
		consumed += width
		if consumed == units {
			return index + 1
		}
	}
	return len(editor.runes)
}

func (editor *Editor) deleteSelectionOrComposition() bool {
	target := editor.Selection()
	if editor.composing {
		target = editor.composition
	}
	if target.Start == target.End {
		return false
	}
	before := editor.selectionState()
	change := editor.replace(target, "", false)
	editor.collapse(target.Start)
	editor.composing = false
	editor.record(change, before, editor.selectionState(), editSeparate)
	return true
}

func (editor *Editor) collapse(index int) {
	editor.anchor = index
	editor.caret = index
}

// replace swaps target for value, applying the Restrict limits, and returns the
// number of runes inserted. With overwrite, an empty target is widened to the
// runes the new text lands on (see SetOverwrite); lastReplaceStart records where
// the replacement began, which differs from target.Start only when a full field
// had to replace its last rune.
func (editor *Editor) replace(target Range, value string, overwrite bool) textChange {
	target = normalizeRange(target, len(editor.runes))
	editor.textValid = false
	editor.offsets.trim(target.Start)
	replacement := []rune(value)
	if editor.accept != nil {
		kept := replacement[:0]
		for _, r := range replacement {
			if editor.accept(r) {
				kept = append(kept, r)
			}
		}
		replacement = kept
	}
	if overwrite && len(replacement) > 0 {
		if editor.maxRunes > 0 && len(editor.runes) >= editor.maxRunes && target.Start >= len(editor.runes) {
			target.Start = len(editor.runes) - 1 // a full field: the last rune is replaced
		}
		target.End = min(target.Start+len(replacement), len(editor.runes))
	}
	editor.lastReplaceStart = target.Start
	removed := target.End - target.Start
	if editor.maxRunes > 0 {
		room := max(editor.maxRunes-(len(editor.runes)-removed), 0)
		if len(replacement) > room {
			replacement = replacement[:room]
		}
	}
	change := textChange{
		start:    target.Start,
		removed:  append([]rune(nil), editor.runes[target.Start:target.End]...),
		inserted: replacement,
	}
	delta := len(replacement) - removed
	if delta > 0 {
		editor.runes = append(editor.runes, make([]rune, delta)...)
		copy(editor.runes[target.End+delta:], editor.runes[target.End:len(editor.runes)-delta])
	} else if delta < 0 {
		copy(editor.runes[target.Start+len(replacement):], editor.runes[target.End:])
		editor.runes = editor.runes[:len(editor.runes)+delta]
	}
	copy(editor.runes[target.Start:], replacement)
	return change
}

// CanUndo reports whether Undo would restore an earlier edit.
func (editor *Editor) CanUndo() bool { return editor.historyIndex > 0 }

// CanRedo reports whether Redo would reapply an undone edit.
func (editor *Editor) CanRedo() bool { return editor.historyIndex < len(editor.history) }

// Undo restores the text and selection before the latest undo unit.
func (editor *Editor) Undo() bool {
	if !editor.CanUndo() {
		return false
	}
	editor.historyIndex--
	edit := &editor.history[editor.historyIndex]
	editor.replaceExact(edit.start, len(edit.inserted), edit.removed)
	editor.restoreSelection(edit.before)
	editor.BreakUndoGroup()
	return true
}

// Redo reapplies the next undone edit and restores its resulting selection.
func (editor *Editor) Redo() bool {
	if !editor.CanRedo() {
		return false
	}
	edit := &editor.history[editor.historyIndex]
	editor.replaceExact(edit.start, len(edit.removed), edit.inserted)
	editor.restoreSelection(edit.after)
	editor.historyIndex++
	editor.BreakUndoGroup()
	return true
}

// BreakUndoGroup keeps the next edit from merging with adjacent typing or
// deletion. Movement, selection changes, paste and line breaks call it
// automatically; applications may call it at another semantic boundary.
func (editor *Editor) BreakUndoGroup() { editor.coalescing = 0 }

// ClearHistory forgets every undo and redo unit without changing the text.
func (editor *Editor) ClearHistory() {
	editor.history = nil
	editor.historyIndex = 0
	editor.historyRunes = 0
	editor.coalescing = 0
}

func (editor *Editor) selectionState() editorSelection {
	return editorSelection{anchor: editor.anchor, caret: editor.caret}
}

func (editor *Editor) restoreSelection(selection editorSelection) {
	editor.anchor = clamp(selection.anchor, 0, len(editor.runes))
	editor.caret = clamp(selection.caret, 0, len(editor.runes))
	editor.composing = false
}

func (editor *Editor) record(change textChange, before, after editorSelection, kind editKind) {
	if runesEqual(change.removed, change.inserted) {
		return
	}
	editor.dropRedo()
	if editor.merge(change, before, after, kind) {
		editor.trimHistory()
		return
	}
	edit := historyEdit{
		start: change.start, removed: change.removed, inserted: change.inserted,
		before: before, after: after, kind: kind,
	}
	editor.history = append(editor.history, edit)
	editor.historyIndex = len(editor.history)
	editor.historyRunes += len(edit.removed) + len(edit.inserted)
	if kind == editSeparate {
		editor.coalescing = 0
	} else {
		editor.coalescing = kind
	}
	editor.trimHistory()
}

func (editor *Editor) merge(change textChange, before, after editorSelection, kind editKind) bool {
	if editor.coalescing != kind || editor.historyIndex == 0 || editor.historyIndex != len(editor.history) {
		return false
	}
	previous := &editor.history[editor.historyIndex-1]
	if previous.kind != kind || previous.after != before {
		return false
	}
	switch kind {
	case editInsert:
		if len(previous.removed) != 0 || len(change.removed) != 0 || previous.start+len(previous.inserted) != change.start {
			return false
		}
		previous.inserted = append(previous.inserted, change.inserted...)
		editor.historyRunes += len(change.inserted)
	case editDeleteBackward:
		if len(previous.inserted) != 0 || len(change.inserted) != 0 || change.start+len(change.removed) != previous.start {
			return false
		}
		removed := make([]rune, 0, len(change.removed)+len(previous.removed))
		removed = append(removed, change.removed...)
		removed = append(removed, previous.removed...)
		previous.start = change.start
		previous.removed = removed
		editor.historyRunes += len(change.removed)
	case editDeleteForward:
		if len(previous.inserted) != 0 || len(change.inserted) != 0 || change.start != previous.start {
			return false
		}
		previous.removed = append(previous.removed, change.removed...)
		editor.historyRunes += len(change.removed)
	case editComposition:
		if change.start != previous.start || len(change.removed) != len(previous.inserted) {
			return false
		}
		editor.historyRunes += len(change.inserted) - len(previous.inserted)
		previous.inserted = change.inserted
	default:
		return false
	}
	previous.after = after
	return true
}

func (editor *Editor) dropRedo() {
	if editor.historyIndex == len(editor.history) {
		return
	}
	for index := editor.historyIndex; index < len(editor.history); index++ {
		editor.historyRunes -= len(editor.history[index].removed) + len(editor.history[index].inserted)
	}
	clear(editor.history[editor.historyIndex:])
	editor.history = editor.history[:editor.historyIndex]
}

func (editor *Editor) trimHistory() {
	// The newest unit is always kept, unless it alone is beyond what is worth
	// keeping.
	remove := 0
	for len(editor.history)-remove > 1 && (len(editor.history)-remove > maxHistoryEntries || editor.historyRunes > maxHistoryRunes) {
		edit := editor.history[remove]
		editor.historyRunes -= len(edit.removed) + len(edit.inserted)
		remove++
	}
	if len(editor.history)-remove == 1 && editor.historyRunes > maxSingleEditRunes {
		editor.ClearHistory()
		return
	}
	if remove == 0 {
		return
	}
	copy(editor.history, editor.history[remove:])
	clear(editor.history[len(editor.history)-remove:])
	editor.history = editor.history[:len(editor.history)-remove]
	editor.historyIndex = max(0, editor.historyIndex-remove)
}

func (editor *Editor) replaceExact(start, removed int, replacement []rune) {
	start = clamp(start, 0, len(editor.runes))
	end := clamp(start+removed, start, len(editor.runes))
	delta := len(replacement) - (end - start)
	if delta > 0 {
		editor.runes = append(editor.runes, make([]rune, delta)...)
		copy(editor.runes[end+delta:], editor.runes[end:len(editor.runes)-delta])
	} else if delta < 0 {
		copy(editor.runes[start+len(replacement):], editor.runes[end:])
		editor.runes = editor.runes[:len(editor.runes)+delta]
	}
	copy(editor.runes[start:], replacement)
	editor.lastReplaceStart = start
	editor.textValid = false
	editor.offsets.trim(start)
}

func hasUndoBoundary(value string) bool {
	for _, current := range value {
		if current == '\n' || current == '\r' || current == '\t' {
			return true
		}
	}
	return false
}

func runesEqual(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// nextBoundary and previousBoundary move by one extended grapheme cluster
// (UAX #29), so a flag, a ZWJ family, a Devanagari conjunct, a Hangul jamo
// syllable or CR LF is one caret step. DeleteBackward removes the same whole
// cluster, a base letter with its combining marks included: the editor has
// always deleted by cluster, and a combining mark cannot be re-entered alone
// on most keyboards.
func nextBoundary(values []rune, start int) int { return grapheme.NextRunes(values, start) }

func previousBoundary(values []rune, end int) int { return grapheme.PrevRunes(values, end) }

func normalizeRange(value Range, length int) Range {
	value.Start = clamp(value.Start, 0, length)
	value.End = clamp(value.End, 0, length)
	return ordered(value)
}

func ordered(value Range) Range {
	if value.Start > value.End {
		value.Start, value.End = value.End, value.Start
	}
	return value
}

func clamp(value int, minimum int, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
