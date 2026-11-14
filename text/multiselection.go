package text

import (
	"sort"
	"unicode/utf8"
)

// Multi-selection, the model CodeMirror uses: one primary selection, which
// Selection and Caret report and every single-selection method acts on, plus
// any number of secondary ones. Selections never overlap: adding or setting a
// range that overlaps another merges the two, so an edit can be applied to
// every selection back to front without the ranges crossing.
//
// The secondary selections are editing state, not history. Typing or deleting
// keeps them, each becoming a caret after what it replaced, as in CodeMirror;
// moving the primary or editing the text another way ends with the primary
// alone, and Undo and Redo restore one selection. An editor that never calls
// AddSelection or SetSelections behaves exactly as it did before they existed.

// SelectionCount returns how many selections there are, the primary included,
// so an editor without secondary selections reports one.
func (editor *Editor) SelectionCount() int {
	return 1 + len(editor.secondary)
}

// SelectionsInto copies every selection, the primary included, into dst,
// ordered by start and then by end, and returns the result. Pass a reused slice
// as dst to avoid an allocation: a paint calls it every frame. Selection
// reports which range is the primary one.
func (editor *Editor) SelectionsInto(dst []Range) []Range {
	total := 1 + len(editor.secondary)
	dst = dst[:0]
	if cap(dst) < total {
		dst = make([]Range, 0, total)
	}
	dst = append(dst, ordered(Range{Start: editor.anchor, End: editor.caret}))
	for _, selection := range editor.secondary {
		dst = append(dst, selection.rangeValue())
	}
	// A handful of ranges: an insertion sort beats sort.Slice, and the common
	// one-selection call only runs the outer loop once.
	for i := 1; i < len(dst); i++ {
		for j := i; j > 0; j-- {
			left, right := dst[j-1], dst[j]
			if left.Start < right.Start || left.Start == right.Start && left.End <= right.End {
				break
			}
			dst[j-1], dst[j] = right, left
		}
	}
	return dst
}

// Selections returns every selection, the primary included, ordered by
// position. Selection reports the primary. It allocates; a paint uses
// SelectionsInto with its own scratch.
func (editor *Editor) Selections() []Range {
	return editor.SelectionsInto(make([]Range, 0, 1+len(editor.secondary)))
}

// SelectionSpansInto is SelectionsInto in the byte offsets String and
// components.CodeEditorSnapshot.SelectionRanges use, so a view paints every
// selection without converting them itself. The spans are ordered; a collapsed
// selection contributes an empty span, which paints nothing. The editor never
// lets selections overlap, so painting them in order is enough. Pass a reused
// dst to avoid an allocation.
func (editor *Editor) SelectionSpansInto(dst []Span) []Span {
	dst = dst[:0]
	if cap(dst) < 1+len(editor.secondary) {
		dst = make([]Span, 0, 1+len(editor.secondary))
	}
	dst = append(dst, editor.byteSpan(editor.Selection()))
	for _, selection := range editor.secondary {
		dst = append(dst, editor.byteSpan(selection.rangeValue()))
	}
	// Selections() is already ordered; keep its order, which byte offsets share.
	for i := 1; i < len(dst); i++ {
		for j := i; j > 0 && dst[j-1].Start > dst[j].Start; j-- {
			dst[j-1], dst[j] = dst[j], dst[j-1]
		}
	}
	return dst
}

// SelectionSpans returns SelectionSpansInto's slice in a new allocation.
func (editor *Editor) SelectionSpans() []Span {
	return editor.SelectionSpansInto(nil)
}

func (editor *Editor) byteSpan(selection Range) Span {
	return Span{Start: editor.ByteOffset(selection.Start), End: editor.ByteOffset(selection.End)}
}

// AddSelection adds a selection and makes it the primary, as CodeMirror's add
// selection does, so the caret and the next movement are on the range just
// added. A range that overlaps an existing one merges with it and with every
// range it touches, and adding the selection the editor already has changes
// nothing. It reports whether the selection set changed. It ends an active
// composition, and does nothing while one is active: the input method owns the
// text until it commits.
func (editor *Editor) AddSelection(selection Range) bool {
	if editor.composing {
		return false
	}
	states := editor.selectionStates()
	states = append(states, editorSelection{
		anchor: clamp(selection.Start, 0, len(editor.runes)),
		caret:  clamp(selection.End, 0, len(editor.runes)),
	})
	merged, main := normalizeSelections(states, len(states)-1)
	previous, _ := normalizeSelections(editor.selectionStates(), 0)
	if sameRangeSet(merged, previous) {
		return false
	}
	editor.installSelections(merged, main, true)
	return true
}

// ClearSecondarySelections drops every selection but the primary and reports
// whether there was one. It is what Escape does, and what anything that moves
// the primary or edits the text as one selection does implicitly.
func (editor *Editor) ClearSecondarySelections() bool {
	if len(editor.secondary) == 0 {
		return false
	}
	editor.clearSecondary()
	editor.BreakUndoGroup()
	return true
}

// SetSelections replaces every selection. selections are ranges in any order
// and direction, and main is the index in that slice of the one that becomes
// primary; overlapping ranges merge, and the primary is the merged range its
// own became. Two identical ranges become one. An empty selections clears the
// secondary selections and leaves the primary where it is; a main outside
// selections changes nothing. Both report false. It ends an active composition,
// and does nothing while one is active.
func (editor *Editor) SetSelections(selections []Range, main int) bool {
	if len(selections) == 0 {
		editor.ClearSecondarySelections()
		return false
	}
	if editor.composing || main < 0 || main >= len(selections) {
		return false
	}
	states := make([]editorSelection, len(selections))
	for index, selection := range selections {
		states[index] = editorSelection{
			anchor: clamp(selection.Start, 0, len(editor.runes)),
			caret:  clamp(selection.End, 0, len(editor.runes)),
		}
	}
	merged, mergedMain := normalizeSelections(states, main)
	editor.installSelections(merged, mergedMain, true)
	return true
}

// selectionStates is the whole selection set, primary first, as the editor
// keeps it: ordered and disjoint.
func (editor *Editor) selectionStates() []editorSelection {
	states := make([]editorSelection, 0, 1+len(editor.secondary))
	states = append(states, editor.selectionState())
	return append(states, editor.secondary...)
}

// installSelections installs a normalized set: states[main] is the primary and
// the rest are secondary, in the order normalizeSelections left them, which is
// by position.
func (editor *Editor) installSelections(states []editorSelection, main int, breakUndo bool) {
	secondary := make([]editorSelection, 0, len(states)-1)
	primary := states[main]
	for index, selection := range states {
		if index != main {
			secondary = append(secondary, selection)
		}
	}
	changed := editor.anchor != primary.anchor || editor.caret != primary.caret || !sameSelectionStates(editor.secondary, secondary)
	if changed && breakUndo {
		editor.BreakUndoGroup()
	}
	editor.anchor, editor.caret = primary.anchor, primary.caret
	editor.secondary = append(editor.secondary[:0], secondary...)
	editor.composing = false
	editor.releaseCompositionHistory()
}

// clearSecondary is the hot path's ClearSecondarySelections: no undo boundary,
// for the movements and edits that already break the group.
func (editor *Editor) clearSecondary() {
	if len(editor.secondary) > 0 {
		clear(editor.secondary)
		editor.secondary = editor.secondary[:0]
	}
}

func (selection editorSelection) from() int {
	if selection.anchor < selection.caret {
		return selection.anchor
	}
	return selection.caret
}

func (selection editorSelection) to() int {
	if selection.anchor > selection.caret {
		return selection.anchor
	}
	return selection.caret
}

func (selection editorSelection) rangeValue() Range {
	return Range{Start: selection.from(), End: selection.to()}
}

func sameSelectionStates(left, right []editorSelection) bool {
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

// sameRangeSet reports whether two normalized sets cover the same ranges,
// whatever their order in the slice and whichever is primary.
func sameRangeSet(left, right []editorSelection) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].from() != right[index].from() || left[index].to() != right[index].to() {
			return false
		}
	}
	return true
}

// normalizeSelections orders and merges states, and reports where the main
// selection ended up in the returned slice. Ranges that overlap or start at the
// same place become one union; a range that merged with nothing keeps the
// direction it was given, while a union is ordered. states is reused as the
// output's storage.
func normalizeSelections(states []editorSelection, main int) ([]editorSelection, int) {
	type entry struct {
		selection editorSelection
		main      bool
	}
	entries := make([]entry, len(states))
	for index, selection := range states {
		entries[index] = entry{selection: selection, main: index == main}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].selection.from() != entries[j].selection.from() {
			return entries[i].selection.from() < entries[j].selection.from()
		}
		return entries[i].selection.to() < entries[j].selection.to()
	})
	merged := entries[:0]
	mergedMain := 0
	for _, current := range entries {
		if count := len(merged); count > 0 {
			last := &merged[count-1]
			if current.selection.from() < last.selection.to() || current.selection.from() == last.selection.from() {
				if current.selection.to() > last.selection.to() {
					last.selection.anchor, last.selection.caret = last.selection.from(), current.selection.to()
				} else {
					last.selection.anchor, last.selection.caret = last.selection.from(), last.selection.to()
				}
				last.main = last.main || current.main
				if last.main {
					mergedMain = count - 1
				}
				continue
			}
		}
		merged = append(merged, current)
		if current.main {
			mergedMain = len(merged) - 1
		}
	}
	out := states[:0]
	for _, current := range merged {
		out = append(out, current.selection)
	}
	return out, mergedMain
}

// multiEdit is one replacement at one selection, in the original text's rune
// indices.
type multiEdit struct {
	start, end int
	text       string
}

// commitMultiEdits applies edits, sorted and disjoint, as one undo unit and
// leaves a caret after each inserted text, with edits[main] the primary. Going
// through ReplaceRangesWith means the text changes in one buffer replacement:
// no selection's offset is ever read after an earlier selection moved it.
func (editor *Editor) commitMultiEdits(edits []multiEdit, main int) bool {
	replacements := make([]Replacement, len(edits))
	carets := make([]editorSelection, len(edits))
	shift := 0
	for index, edit := range edits {
		inserted := utf8.RuneCountInString(edit.text)
		replacements[index] = Replacement{Range: Range{Start: edit.start, End: edit.end}, Text: edit.text}
		at := edit.start + shift + inserted
		carets[index] = editorSelection{anchor: at, caret: at}
		shift += inserted - (edit.end - edit.start)
	}
	after := carets[main].rangeValue()
	if err := editor.ReplaceRangesWith(replacements, ReplaceRangesOptions{AfterSelection: &after}); err != nil {
		return false
	}
	// ReplaceRangesWith is not a multi-selection caller: the secondary selection
	// the batch cleared is restored here, normalized for merged carets.
	states, main := normalizeSelections(carets, main)
	editor.installSelections(states, main, false)
	return true
}

// multiInsert replaces every selection with value as one undo unit and leaves a
// caret after each copy of it. It reports whether it handled the edit: an
// overwriting or restricted editor keeps the primary alone, and a composition
// owns the text.
func (editor *Editor) multiInsert(value string) bool {
	if len(editor.secondary) == 0 || editor.composing || editor.overwrite || editor.accept != nil || editor.maxRunes > 0 {
		return false
	}
	ordered, main := orderedSelections(editor)
	edits := make([]multiEdit, len(ordered))
	for index, selection := range ordered {
		edits[index] = multiEdit{start: selection.from(), end: selection.to(), text: value}
	}
	return editor.commitMultiEdits(edits, main)
}

// multiDelete removes what every selection covers, or the grapheme cluster
// before or after each collapsed selection, as one undo unit, and leaves one
// caret where each selection began. A caret never deletes through its
// neighbour. It reports whether it handled the edit.
func (editor *Editor) multiDelete(forward bool) bool {
	// A restricted editor (a one-time code, a port) is single-line by
	// construction and ReplaceRangesWith refuses it; the primary path deletes.
	if len(editor.secondary) == 0 || editor.composing || editor.accept != nil || editor.maxRunes > 0 {
		return false
	}
	ordered, main := orderedSelections(editor)
	edits := make([]multiEdit, len(ordered))
	positions := make([]int, len(ordered))
	for index, selection := range ordered {
		start, end := selection.from(), selection.to()
		if start == end {
			if forward {
				limit := len(editor.runes)
				if index+1 < len(ordered) {
					limit = ordered[index+1].from()
				}
				end = min(nextBoundary(editor.runes, end), limit)
			} else {
				limit := 0
				if index > 0 {
					limit = ordered[index-1].to()
				}
				start = max(previousBoundary(editor.runes, start), limit)
			}
		}
		edits[index] = multiEdit{start: start, end: end}
		positions[index] = start
	}
	// A caret with nothing left to delete keeps its place; ReplaceRangesWith
	// takes only the real, ordered changes. positions maps every selection's
	// start to where it lands once the deletions before it are applied.
	kept := make([]multiEdit, 0, len(edits))
	caretPositions := make([]int, len(positions))
	shift := 0
	for index, edit := range edits {
		caretPositions[index] = edit.start + shift
		if edit.end > edit.start {
			kept = append(kept, edit)
			shift -= edit.end - edit.start
		}
	}
	if len(kept) == 0 {
		return false
	}
	replacements := make([]Replacement, len(kept))
	for index, edit := range kept {
		replacements[index] = Replacement{Range: Range{Start: edit.start, End: edit.end}}
	}
	after := caretPositions[main]
	selection := Range{Start: after, End: after}
	if err := editor.ReplaceRangesWith(replacements, ReplaceRangesOptions{AfterSelection: &selection}); err != nil {
		return false
	}
	carets := make([]editorSelection, len(caretPositions))
	for index, position := range caretPositions {
		carets[index] = editorSelection{anchor: position, caret: position}
	}
	carets, main = normalizeSelections(carets, main)
	editor.installSelections(carets, main, false)
	return true
}

// orderedSelections is the selection set ordered by position, with the index
// the primary ends up at.
func orderedSelections(editor *Editor) ([]editorSelection, int) {
	states := editor.selectionStates()
	primary := editor.selectionState()
	ordered := make([]editorSelection, len(states))
	copy(ordered, states)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].from() != ordered[j].from() {
			return ordered[i].from() < ordered[j].from()
		}
		return ordered[i].to() < ordered[j].to()
	})
	main := 0
	for index, selection := range ordered {
		if selection == primary {
			main = index
			break
		}
	}
	return ordered, main
}
