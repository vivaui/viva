package text

import "unicode/utf8"

// Edit is one actual replacement in UTF-8 bytes, before it was coalesced into
// an undo unit. Apply Removed at Start, then Inserted at that same position.
// It retains byte counts only, not the user's removed or inserted text.
type Edit struct {
	Start, Removed, Inserted int
}

// EditJournalLimit is the number of primitive replacements a tracked editor retains.
const EditJournalLimit = 1024

type editJournal struct {
	revision  uint64
	entries   []Edit
	oldest    int
	unlimited bool
	tracking  bool
	savePoint *int
}

// TrackEdits enables a bounded mutation journal and returns its current cursor.
// Call it before attaching ranges to text. Calling it again does not reset the
// journal. Editors that never call TrackEdits retain no mutation journal.
func (editor *Editor) TrackEdits() uint64 {
	if editor.journal == nil {
		editor.journal = &editJournal{}
	}
	editor.journal.tracking = true
	return editor.journal.revision
}

// EditsSince copies subsequent primitive edits into dst and returns a new cursor.
// The boolean is false when tracking was not enabled, the cursor is in the
// future, or older edits expired. Consumers must not apply a partial sequence
// in that case. Typing, accepted restrictions, overwrite, IME and Undo/Redo use
// this same byte stream; moving a selection or clearing Undo does not enter it.
// ReplaceRanges reports disjoint ranges from last to first, and its Undo reports
// their inverses from first to last, without treating untouched gaps as edits.
// ReplaceRangesWith can opt into forward target order with reverse inverse order.
func (editor *Editor) EditsSince(cursor uint64, dst []Edit) ([]Edit, uint64, bool) {
	dst = dst[:0]
	journal := editor.journal
	if journal == nil || !journal.tracking {
		return dst, 0, false
	}
	if cursor > journal.revision || journal.revision-cursor > uint64(len(journal.entries)) {
		return dst, journal.revision, false
	}
	first := len(journal.entries) - int(journal.revision-cursor)
	for index := first; index < len(journal.entries); index++ {
		position := (journal.oldest + index) % len(journal.entries)
		dst = append(dst, journal.entries[position])
	}
	return dst, journal.revision, true
}

func (editor *Editor) prepareEdit(start, end int, replacement []rune) Edit {
	if editor.journal == nil || !editor.journal.tracking {
		return Edit{}
	}
	at, offset := editor.offsets.index, editor.offsets.offset
	if start < at && start <= at-start {
		at, offset = 0, 0
	} else if start >= at && editor.textValid && len(editor.runes)-start < start-at {
		at, offset = len(editor.runes), len(editor.text)
	}
	for at < start {
		offset += runeBytes(editor.runes[at])
		at++
	}
	for at > start {
		at--
		offset -= runeBytes(editor.runes[at])
	}
	editor.offsets = offsetMemo{index: start, offset: offset}
	edit := Edit{Start: offset}
	for _, character := range editor.runes[start:end] {
		edit.Removed += runeBytes(character)
	}
	for _, character := range replacement {
		edit.Inserted += runeBytes(character)
	}
	return edit
}

func runeBytes(character rune) int {
	width := utf8.RuneLen(character)
	if width < 0 {
		width = utf8.RuneLen(utf8.RuneError)
	}
	return width
}

func (editor *Editor) recordEdit(edit Edit) {
	journal := editor.journal
	if journal == nil || !journal.tracking || edit.Removed == 0 && edit.Inserted == 0 {
		return
	}
	if journal.unlimited || len(journal.entries) < EditJournalLimit {
		journal.entries = append(journal.entries, edit)
	} else {
		journal.entries[journal.oldest] = edit
		journal.oldest = (journal.oldest + 1) % EditJournalLimit
	}
	journal.revision++
}

func (editor *Editor) recordBatchEdits(primitives []Edit, inverse bool) {
	if editor.journal == nil || !editor.journal.tracking {
		return
	}
	if inverse {
		for index := len(primitives) - 1; index >= 0; index-- {
			edit := primitives[index]
			editor.recordEdit(Edit{Start: edit.Start, Removed: edit.Inserted, Inserted: edit.Removed})
		}
	} else {
		for _, edit := range primitives {
			editor.recordEdit(edit)
		}
	}
}

func (editor *Editor) replaceHistory(edit *historyEdit, inverse bool) {
	journal := editor.journal
	if len(edit.primitives) > 0 {
		editor.journal = nil
	}
	if inverse {
		editor.replaceExact(edit.start, len(edit.inserted), edit.removed)
	} else {
		editor.replaceExact(edit.start, len(edit.removed), edit.inserted)
	}
	editor.journal = journal
	editor.recordBatchEdits(edit.primitives, inverse)
}
