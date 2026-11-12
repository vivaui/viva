package text

import "unsafe"

// LineEdit adds the actual source logical line and LF deltas to one UTF-8 byte replacement.
// AtLineStart describes its source, before deletion and insertion. InsertedLF
// reports LF only; it does not classify CR, CRLF or Unicode line separators.
type LineEdit struct {
	Edit
	AtLineStart, InsertedLF           bool
	Line, RemovedLines, InsertedLines int
}

type lineFlags uint8

type lineContext struct {
	flags                   lineFlags
	line, removed, inserted int
}

const lineContextRunes = int((unsafe.Sizeof(lineContext{}) + unsafe.Sizeof(rune(0)) - 1) / unsafe.Sizeof(rune(0)))

type lineDelivery struct {
	first, last uint64
	primitives  []Edit
	contexts    []lineContext
	inverse     bool
}

const (
	lineKnown lineFlags = 1 << iota
	lineStart
	lineInsertedLF
	lineRemovedLF
)

type lineJournal struct {
	entries        []lineContext
	batches        map[*Edit][]lineContext
	position, line int
	delivery       *lineDelivery
}

// TrackLineEdits enables optional LF context on the existing byte journal and
// returns its shared cursor. It does not reset either journal. Earlier byte
// entries and batch history without captured context remain unavailable.
// Editors and history units retain no line context until this is called.
func (editor *Editor) TrackLineEdits() uint64 {
	cursor := editor.TrackEdits()
	if editor.journal.lines == nil {
		editor.journal.lines = &lineJournal{entries: make([]lineContext, len(editor.journal.entries))}
	}
	return cursor
}

// LineEditsSince copies actual contexts using the byte journal's cursor. It also
// delivers the complete most-recent atomic batch when the cursor is exactly
// that batch's start, until another mutation or ClearHistory releases it.
// Other expired/future cursors and missing historical contexts return no partial
// sequence. Undo/Redo preserve original batch/transaction steps, including
// transient text. Only byte counts and LF facts are retained, not text copies.
func (editor *Editor) LineEditsSince(cursor uint64, dst []LineEdit) ([]LineEdit, uint64, bool) {
	dst = dst[:0]
	journal := editor.journal
	if journal == nil || !journal.tracking {
		return dst, 0, false
	}
	if journal.lines == nil || cursor > journal.revision {
		return dst, journal.revision, false
	}
	if journal.revision-cursor > uint64(len(journal.entries)) {
		return journal.lines.copyDelivery(cursor, journal.revision, dst)
	}
	first := len(journal.entries) - int(journal.revision-cursor)
	for index := first; index < len(journal.entries); index++ {
		position := (journal.oldest + index) % len(journal.entries)
		context := journal.lines.entries[position]
		if context.flags&lineKnown == 0 {
			return dst[:0], journal.revision, false
		}
		dst = append(dst, context.edit(journal.entries[position]))
	}
	return dst, journal.revision, true
}

func (context lineContext) edit(edit Edit) LineEdit {
	return LineEdit{Edit: edit, AtLineStart: context.flags&lineStart != 0, InsertedLF: context.flags&lineInsertedLF != 0,
		Line: context.line, RemovedLines: context.removed, InsertedLines: context.inserted}
}

func (lines *lineJournal) copyDelivery(cursor, revision uint64, dst []LineEdit) ([]LineEdit, uint64, bool) {
	delivery := lines.delivery
	if delivery == nil || delivery.first != cursor || delivery.last != revision ||
		delivery.last-delivery.first != uint64(len(delivery.primitives)) || len(delivery.contexts) != len(delivery.primitives) {
		return dst[:0], revision, false
	}
	for index := range delivery.primitives {
		position := index
		if delivery.inverse {
			position = len(delivery.primitives) - index - 1
		}
		edit, context := delivery.primitives[position], delivery.contexts[position]
		if context.flags&lineKnown == 0 {
			return dst[:0], revision, false
		}
		if delivery.inverse {
			edit.Removed, edit.Inserted = edit.Inserted, edit.Removed
			context = context.inverse()
		}
		dst = append(dst, context.edit(edit))
	}
	return dst, revision, true
}

func (editor *Editor) prepareLineContext(start, end int, replacement []rune) lineContext {
	if editor.journal == nil || editor.journal.lines == nil {
		return lineContext{}
	}
	lines := editor.journal.lines
	position, line := lines.position, lines.line
	if position > len(editor.runes) || start < position && start <= position-start {
		position, line = 0, 0
	}
	for position < start {
		if editor.runes[position] == '\n' {
			line++
		}
		position++
	}
	for position > start {
		position--
		if editor.runes[position] == '\n' {
			line--
		}
	}
	lines.position, lines.line = start, line
	return makeLineContext(line, start == 0 || editor.runes[start-1] == '\n', editor.runes[start:end], replacement)
}

func makeLineContext(line int, atLineStart bool, removed, inserted []rune) lineContext {
	context := lineContext{flags: lineKnown, line: line}
	if atLineStart {
		context.flags |= lineStart
	}
	for _, character := range removed {
		if character == '\n' {
			context.removed++
		}
	}
	for _, character := range inserted {
		if character == '\n' {
			context.inserted++
		}
	}
	if context.removed > 0 {
		context.flags |= lineRemovedLF
	}
	if context.inserted > 0 {
		context.flags |= lineInsertedLF
	}
	return context
}

func (editor *Editor) recordLineContext(edit Edit, context lineContext) {
	if edit.Removed == 0 && edit.Inserted == 0 {
		return
	}
	journal := editor.journal
	lines := journal.lines
	lines.delivery = nil
	position := (journal.oldest + len(journal.entries) - 1) % len(journal.entries)
	if position == len(lines.entries) {
		lines.entries = append(lines.entries, context)
	} else {
		lines.entries[position] = context
	}
}

func (context lineContext) inverse() lineContext {
	inverse := context
	inverse.flags &^= lineInsertedLF | lineRemovedLF
	inverse.removed, inverse.inserted = context.inserted, context.removed
	if context.flags&lineInsertedLF != 0 {
		inverse.flags |= lineRemovedLF
	}
	if context.flags&lineRemovedLF != 0 {
		inverse.flags |= lineInsertedLF
	}
	return inverse
}

func (editor *Editor) rememberLineBatch(primitives []Edit, contexts []lineContext) {
	if len(primitives) == 0 || len(contexts) != len(primitives) || editor.journal == nil || editor.journal.lines == nil || editor.historyIndex == 0 {
		return
	}
	retained := editor.history[editor.historyIndex-1].primitives
	if len(retained) == 0 || &retained[0] != &primitives[0] {
		return
	}
	lines := editor.journal.lines
	if lines.batches == nil {
		lines.batches = make(map[*Edit][]lineContext)
	}
	key := &primitives[0]
	previous := len(lines.batches[key])
	lines.batches[key] = contexts
	editor.historyRunes += (len(contexts) - previous) * lineContextRunes
	editor.trimHistory()
}

func (editor *Editor) forgetLineBatch(edit *historyEdit) int {
	if editor.journal != nil && editor.journal.lines != nil && len(edit.primitives) > 0 {
		batches := editor.journal.lines.batches
		key := &edit.primitives[0]
		retained := len(batches[key]) * lineContextRunes
		delete(batches, key)
		return retained
	}
	return 0
}

func (editor *Editor) lineBatchContexts(primitives []Edit) []lineContext {
	if len(primitives) == 0 || editor.journal == nil || editor.journal.lines == nil {
		return nil
	}
	return editor.journal.lines.batches[&primitives[0]]
}
