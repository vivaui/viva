package text

import (
	"sort"
)

// CodeOptions is how code-aware editing behaves. The zero value indents with
// four spaces and does nothing the caller did not switch on, so a plain editor
// keeps plain behavior. The operations that take it are Editor methods, each of
// which is one undo unit whatever number of lines it touches.
type CodeOptions struct {
	// TabWidth is the width of a tab stop in columns, and the number of spaces in
	// one indent when HardTabs is off. Zero or less is four; it is capped at 16.
	TabWidth int
	// HardTabs makes one indent a tab character.
	HardTabs bool
	// AutoIndent makes a new line start at the indent of the line above, one
	// level deeper after a character of IndentAfter, and makes Backspace in
	// indentation remove back to the previous tab stop.
	AutoIndent bool
	// AutoClose pairs brackets and quotes: typing an opener inserts its closer,
	// typing the closer in front of one steps over it, Backspace between a pair
	// removes both, and typing an opener over a selection wraps it.
	AutoClose bool
	// IndentAfter lists the characters that open a block: when the text before
	// the caret ends with one, the new line is indented one level deeper. "{[("
	// suits C-like languages; YAML and Python add ':'.
	IndentAfter string
	// LineComment is the prefix ToggleLineComment adds and removes, such as "//"
	// or "#". Empty disables the operation.
	LineComment string
}

func (options CodeOptions) tabWidth() int {
	switch {
	case options.TabWidth <= 0:
		return 4
	case options.TabWidth > 16:
		return 16
	}
	return options.TabWidth
}

// unit is one indent.
func (options CodeOptions) unit() []rune {
	if options.HardTabs {
		return []rune{'\t'}
	}
	unit := make([]rune, options.tabWidth())
	for index := range unit {
		unit[index] = ' '
	}
	return unit
}

func isIndentRune(r rune) bool { return r == ' ' || r == '\t' }

// lineStartAt is the index of the first rune of the line that contains index.
func (editor *Editor) lineStartAt(index int) int {
	for index > 0 && editor.runes[index-1] != '\n' {
		index--
	}
	return index
}

// lineEndAt is the index of the newline that ends the line containing index, or
// the length of the text.
func (editor *Editor) lineEndAt(index int) int {
	for index < len(editor.runes) && editor.runes[index] != '\n' {
		index++
	}
	return index
}

// indentEnd is the index of the first rune of [start, end) that is not a space
// or a tab: where the text of the line begins.
func (editor *Editor) indentEnd(start, end int) int {
	for start < end && isIndentRune(editor.runes[start]) {
		start++
	}
	return start
}

// cellsOf is the width in columns of one rune at column, a tab stopping at the
// next multiple of tabWidth and a wide character taking two.
func cellsOf(r rune, column, tabWidth int) int {
	if r == '\t' {
		return tabWidth - column%tabWidth
	}
	return CellWidth(r)
}

// column is the display column of index on its line.
func (editor *Editor) column(index, tabWidth int) int {
	column := 0
	for at := editor.lineStartAt(index); at < index; at++ {
		column += cellsOf(editor.runes[at], column, tabWidth)
	}
	return column
}

// editBlock replaces target with replacement as one undo unit, and leaves the
// selection at anchor and caret in the new text.
func (editor *Editor) editBlock(target Range, replacement []rune, anchor, caret int) {
	// The block commands are line commands: they act on the primary selection,
	// and the set of carets they leave is the one they compute here.
	editor.clearSecondary()
	before := editor.selectionState()
	change := editor.replace(target, string(replacement), false)
	editor.anchor = clamp(anchor, 0, len(editor.runes))
	editor.caret = clamp(caret, 0, len(editor.runes))
	editor.composing = false
	editor.record(change, before, editor.selectionState(), editSeparate)
}

// selectedLines is the run of whole lines the selection touches, as the rune
// range [first, last): first is the start of the first line and last the end of
// the last, before its newline. A selection that ends at the very start of a
// line, just after a newline, does not include that line.
func (editor *Editor) selectedLines() (first, last int) {
	selection := editor.Selection()
	first = editor.lineStartAt(selection.Start)
	end := selection.End
	if end > selection.Start && editor.runes[end-1] == '\n' {
		end--
	}
	return first, editor.lineEndAt(end)
}

// lineChange describes what a line transform did to its line: it inserted
// delta runes at column at, or, when delta is negative, removed -delta runes
// from there.
type lineChange struct{ at, delta int }

// transformLines rewrites the lines of [first, last) one by one as a single
// edit, and moves the selection with the text. A position at the column of an
// insertion goes after the inserted runes, as a caret stays with the text it
// was in front of, except the start of a selection, which stays before them so
// that the lines it selected are still selected whole.
func (editor *Editor) transformLines(first, last int, transform func(line []rune) ([]rune, lineChange)) {
	selection := editor.Selection()
	var (
		block     []rune
		oldStarts []int
		newStarts []int
		changes   []lineChange
	)
	for position := first; ; {
		end := editor.lineEndAt(position)
		out, change := transform(editor.runes[position:end])
		oldStarts = append(oldStarts, position)
		newStarts = append(newStarts, first+len(block))
		changes = append(changes, change)
		block = append(block, out...)
		if end >= last {
			break
		}
		block = append(block, '\n')
		position = end + 1
	}
	total := len(block) - (last - first)
	mapIndex := func(index int) int {
		before := selection.Start != selection.End && index == selection.Start
		if index > last {
			return index + total
		}
		if index < first {
			return index
		}
		line := sort.Search(len(oldStarts), func(i int) bool { return oldStarts[i] > index }) - 1
		relative, change := index-oldStarts[line], changes[line]
		switch {
		case change.delta > 0 && (relative > change.at || relative == change.at && !before):
			relative += change.delta
		case change.delta < 0 && relative > change.at:
			relative = max(change.at, relative+change.delta)
		}
		return newStarts[line] + relative
	}
	editor.editBlock(Range{Start: first, End: last}, block, mapIndex(editor.anchor), mapIndex(editor.caret))
}

// InsertNewlineCode replaces the selection with a line break. With AutoIndent
// the new line starts at the indent of the line it was split from, one level
// deeper when the text before the caret ends with a character of IndentAfter,
// and a caret between a bracket and its closer opens a block: the closer moves
// to a line of its own and the caret to an indented one between. A line that
// held only indentation up to the caret is emptied rather than left with
// trailing spaces. An active composition is left to the input method.
func (editor *Editor) InsertNewlineCode(options CodeOptions) {
	if editor.composing || !options.AutoIndent {
		editor.Insert("\n")
		return
	}
	selection := editor.Selection()
	start := editor.lineStartAt(selection.Start)
	indentEnd := min(editor.indentEnd(start, selection.Start), selection.Start)
	indent := append([]rune(nil), editor.runes[start:indentEnd]...)
	blank := indentEnd == selection.Start // nothing but indentation before the caret
	extra := false
	for at := selection.Start; at > start; at-- {
		if r := editor.runes[at-1]; !isIndentRune(r) {
			extra = containsRune(options.IndentAfter, r)
			break
		}
	}
	hasCloser := false
	if extra && selection.End < len(editor.runes) {
		if previous, ok := editor.previousCodeRune(selection.Start, start); ok {
			hasCloser = isBracket(previous) && closerOf(previous) == editor.runes[selection.End]
		}
	}
	text := append([]rune{'\n'}, indent...)
	if extra {
		text = append(text, options.unit()...)
	}
	caret := len(text)
	if hasCloser {
		text = append(text, '\n')
		text = append(text, indent...)
	}
	target := selection
	if blank && selection.Start == selection.End {
		target.Start = start // the blank line keeps no trailing indentation
	}
	editor.editBlock(target, text, target.Start+caret, target.Start+caret)
}

// previousCodeRune is the last rune before index on the line starting at
// lineStart that is not a space or a tab.
func (editor *Editor) previousCodeRune(index, lineStart int) (rune, bool) {
	for at := index; at > lineStart; at-- {
		if r := editor.runes[at-1]; !isIndentRune(r) {
			return r, true
		}
	}
	return 0, false
}

// InsertTabCode answers the Tab key. A selection across lines indents each of
// its lines; otherwise it replaces the selection with a tab, or with the spaces
// that reach the next tab stop.
func (editor *Editor) InsertTabCode(options CodeOptions) {
	if editor.composing {
		editor.Insert("\t")
		return
	}
	selection := editor.Selection()
	if selection.Start != selection.End && containsRuneIn(editor.runes[selection.Start:selection.End], '\n') {
		editor.indentLines(options)
		return
	}
	if options.HardTabs {
		editor.Insert("\t")
		return
	}
	width := options.tabWidth()
	column := editor.column(selection.Start, width)
	editor.Insert(string(spaces(width - column%width)))
}

func spaces(count int) []rune {
	out := make([]rune, count)
	for index := range out {
		out[index] = ' '
	}
	return out
}

// IndentLinesCode adds one indent to every non-empty line the selection touches,
// or to the caret's line when nothing is selected.
func (editor *Editor) IndentLinesCode(options CodeOptions) {
	if editor.composing {
		return
	}
	editor.indentLines(options)
}

func (editor *Editor) indentLines(options CodeOptions) {
	first, last := editor.selectedLines()
	unit := options.unit()
	editor.transformLines(first, last, func(line []rune) ([]rune, lineChange) {
		if len(line) == 0 {
			return line, lineChange{}
		}
		return append(append([]rune(nil), unit...), line...), lineChange{at: 0, delta: len(unit)}
	})
}

// OutdentCode answers Shift+Tab: it removes one indent, a tab or up to a tab
// width of spaces, from every line the selection touches, or from the caret's
// line when nothing is selected.
func (editor *Editor) OutdentCode(options CodeOptions) {
	if editor.composing {
		return
	}
	width := options.tabWidth()
	first, last := editor.selectedLines()
	editor.transformLines(first, last, func(line []rune) ([]rune, lineChange) {
		remove := 0
		switch {
		case len(line) > 0 && line[0] == '\t':
			remove = 1
		default:
			for remove < len(line) && remove < width && line[remove] == ' ' {
				remove++
			}
		}
		return line[remove:], lineChange{at: 0, delta: -remove}
	})
}

// ToggleLineComment comments out every line the selection touches with
// options.LineComment, or, when each non-empty one is already commented,
// removes the comment. The prefix goes at the shallowest indent of the lines so
// the block stays aligned, and empty lines are left alone.
func (editor *Editor) ToggleLineComment(options CodeOptions) {
	if options.LineComment == "" || editor.composing {
		return
	}
	prefix := []rune(options.LineComment)
	first, last := editor.selectedLines()
	shallowest, anyText, allCommented := -1, false, true
	for position := first; ; {
		end := editor.lineEndAt(position)
		text := editor.indentEnd(position, end)
		if text < end {
			anyText = true
			if depth := text - position; shallowest < 0 || depth < shallowest {
				shallowest = depth
			}
			if !hasRunePrefix(editor.runes[text:end], prefix) {
				allCommented = false
			}
		}
		if end >= last {
			break
		}
		position = end + 1
	}
	if !anyText {
		return
	}
	if allCommented {
		editor.transformLines(first, last, func(line []rune) ([]rune, lineChange) {
			text := 0
			for text < len(line) && isIndentRune(line[text]) {
				text++
			}
			if !hasRunePrefix(line[text:], prefix) {
				return line, lineChange{}
			}
			remove := len(prefix)
			if text+remove < len(line) && line[text+remove] == ' ' {
				remove++
			}
			out := append(append([]rune(nil), line[:text]...), line[text+remove:]...)
			return out, lineChange{at: text, delta: -remove}
		})
		return
	}
	insert := append(append([]rune(nil), prefix...), ' ')
	editor.transformLines(first, last, func(line []rune) ([]rune, lineChange) {
		if blankRunes(line) {
			return line, lineChange{}
		}
		out := make([]rune, 0, len(line)+len(insert))
		out = append(out, line[:shallowest]...)
		out = append(out, insert...)
		out = append(out, line[shallowest:]...)
		return out, lineChange{at: shallowest, delta: len(insert)}
	})
}

func blankRunes(line []rune) bool {
	for _, r := range line {
		if !isIndentRune(r) {
			return false
		}
	}
	return true
}

func onlySpaces(values []rune) bool {
	for _, r := range values {
		if r != ' ' {
			return false
		}
	}
	return true
}

// DuplicateLines copies the lines the selection touches below themselves, or
// above them when up is set. The selection stays where it was in the text, which
// puts it on the copy below, and on the new line above for up: as in the editors
// that have Copy Line Up and Down.
func (editor *Editor) DuplicateLines(up bool) {
	if editor.composing {
		return
	}
	first, last := editor.selectedLines()
	if up {
		block := append(append([]rune(nil), editor.runes[first:last]...), '\n')
		editor.editBlock(Range{Start: first, End: first}, block, editor.anchor, editor.caret)
		return
	}
	block := append([]rune{'\n'}, editor.runes[first:last]...)
	shift := len(block)
	editor.editBlock(Range{Start: last, End: last}, block, editor.anchor+shift, editor.caret+shift)
}

// MoveLines swaps the lines the selection touches with the line above them, or
// below, keeping the selection on the moved text. It does nothing at the first
// or last line.
func (editor *Editor) MoveLines(up bool) {
	if editor.composing {
		return
	}
	first, last := editor.selectedLines()
	block := editor.runes[first:last]
	if up {
		if first == 0 {
			return
		}
		previousStart := editor.lineStartAt(first - 1)
		previous := editor.runes[previousStart : first-1]
		moved := append(append(append([]rune(nil), block...), '\n'), previous...)
		shift := -(len(previous) + 1)
		editor.editBlock(Range{Start: previousStart, End: last}, moved, editor.anchor+shift, editor.caret+shift)
		return
	}
	if last >= len(editor.runes) {
		return
	}
	nextEnd := editor.lineEndAt(last + 1)
	next := editor.runes[last+1 : nextEnd]
	moved := append(append(append([]rune(nil), next...), '\n'), block...)
	shift := len(next) + 1
	editor.editBlock(Range{Start: first, End: nextEnd}, moved, editor.anchor+shift, editor.caret+shift)
}

// DeleteLines removes the lines the selection touches, and puts the caret at
// the start of the line that takes their place.
func (editor *Editor) DeleteLines() {
	if editor.composing {
		return
	}
	first, last := editor.selectedLines()
	start, end, caret := first, last, first
	switch {
	case last < len(editor.runes):
		end++ // the line's own newline goes with it
	case first > 0:
		start-- // the last line has none: the one before it does
		caret = start
	}
	editor.editBlock(Range{Start: start, End: end}, nil, caret, caret)
}

// TypeCode inserts typed text. With AutoClose, a single typed bracket or quote
// pairs itself, steps over an identical closer in front of the caret, or wraps
// the selection; everything else is an ordinary Insert. An opener pairs only
// where that is unlikely to be unwanted: before whitespace, a closer, a
// separator or the end of the text, and a quote also not after a letter, digit
// or the same quote, so an apostrophe in a word stays alone.
func (editor *Editor) TypeCode(value string, options CodeOptions) {
	runes := []rune(value)
	// Pairing is decided against the primary selection's neighbours; with several
	// selections the plain multi-selection insert is what every one of them gets.
	if !options.AutoClose || editor.composing || len(runes) != 1 || editor.SelectionCount() > 1 {
		editor.Insert(value)
		return
	}
	typed := runes[0]
	selection := editor.Selection()
	closer, opens := pairCloser(typed)
	switch {
	case opens && selection.Start != selection.End:
		wrapped := make([]rune, 0, selection.End-selection.Start+2)
		wrapped = append(wrapped, typed)
		wrapped = append(wrapped, editor.runes[selection.Start:selection.End]...)
		wrapped = append(wrapped, closer)
		editor.editBlock(selection, wrapped, selection.Start+1, selection.End+1)
	case selection.Start == selection.End && isCloserOrQuote(typed) && editor.caret < len(editor.runes) && editor.runes[editor.caret] == typed:
		editor.MoveTo(editor.caret+1, false)
	case opens && selection.Start == selection.End && editor.mayPair(typed, editor.caret):
		editor.editBlock(selection, []rune{typed, closer}, selection.Start+1, selection.Start+1)
	default:
		editor.Insert(value)
	}
}

// mayPair reports whether an opener typed at index should insert its closer.
func (editor *Editor) mayPair(typed rune, index int) bool {
	if index < len(editor.runes) {
		next := editor.runes[index]
		if !isIndentRune(next) && next != '\n' && !containsRune(";:.,=}])> ", next) {
			return false
		}
	}
	if isQuoteRune(typed) && index > 0 {
		previous := editor.runes[index-1]
		if previous == typed || previous == '\\' || isWordRune(previous) {
			return false
		}
	}
	return true
}

// DeleteBackwardCode answers Backspace. With AutoClose, a caret between an
// opener and its closer removes both. With AutoIndent, a caret inside the
// indentation of a line that is all spaces to its left removes back to the
// previous tab stop. Anything else is DeleteBackward.
func (editor *Editor) DeleteBackwardCode(options CodeOptions) {
	selection := editor.Selection()
	// Bracket and indent pairing look at the primary selection; every selection
	// of a multi-selection deletes by the plain rule instead.
	if editor.composing || editor.SelectionCount() > 1 || selection.Start != selection.End || selection.Start == 0 {
		editor.DeleteBackward()
		return
	}
	caret := selection.Start
	previous := editor.runes[caret-1]
	if options.AutoClose && caret < len(editor.runes) {
		if closer, ok := pairCloser(previous); ok && editor.runes[caret] == closer {
			before := editor.selectionState()
			change := editor.replace(Range{Start: caret - 1, End: caret + 1}, "", false)
			editor.collapse(caret - 1)
			editor.record(change, before, editor.selectionState(), editSeparate)
			return
		}
	}
	if options.AutoIndent && !options.HardTabs && previous == ' ' {
		start := editor.lineStartAt(caret)
		if onlySpaces(editor.runes[start:caret]) {
			width := options.tabWidth()
			remove := (caret - start) % width
			if remove == 0 {
				remove = width
			}
			remove = min(remove, caret-start)
			before := editor.selectionState()
			change := editor.replace(Range{Start: caret - remove, End: caret}, "", false)
			editor.collapse(caret - remove)
			editor.record(change, before, editor.selectionState(), editDeleteBackward)
			return
		}
	}
	editor.DeleteBackward()
}

// MoveLineStartSmart moves to the first character of the caret's line, or, when
// it is already there, to the start of the line: Home as code editors have it.
func (editor *Editor) MoveLineStartSmart(extend bool) {
	start := editor.lineStartAt(editor.caret)
	target := editor.indentEnd(start, editor.lineEndAt(start))
	if editor.caret == target {
		target = start
	}
	editor.MoveTo(target, extend)
}

func pairCloser(r rune) (rune, bool) {
	switch r {
	case '(':
		return ')', true
	case '[':
		return ']', true
	case '{':
		return '}', true
	case '"', '\'', '`':
		return r, true
	}
	return 0, false
}

// closerOf is the closing bracket that matches the opening one r, or zero.
func closerOf(r rune) rune {
	switch r {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	}
	return 0
}

func isBracket(r rune) bool { return r == '(' || r == '[' || r == '{' }

func isQuoteRune(r rune) bool { return r == '"' || r == '\'' || r == '`' }

func isCloserOrQuote(r rune) bool {
	return r == ')' || r == ']' || r == '}' || isQuoteRune(r)
}

func containsRune(set string, target rune) bool {
	for _, r := range set {
		if r == target {
			return true
		}
	}
	return false
}

func containsRuneIn(values []rune, target rune) bool {
	for _, r := range values {
		if r == target {
			return true
		}
	}
	return false
}

func hasRunePrefix(values, prefix []rune) bool {
	if len(values) < len(prefix) {
		return false
	}
	for index, r := range prefix {
		if values[index] != r {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool { return wordClass(r) == 1 }

// SetCode makes the editor edit code: from now on Type, NewLine, Tab and
// Backspace, which a platform text system's events are applied through (see
// internal/platform.TextEvent.Apply), follow options, and the commands of a code
// editor (components.CodeEditorCommand) use it too. An editor that was never
// given options edits plain text, as before.
func (editor *Editor) SetCode(options CodeOptions) {
	editor.code, editor.codeSet = options, true
}

// Code returns the options set by SetCode, and false if there are none.
func (editor *Editor) Code() (CodeOptions, bool) {
	return editor.code, editor.codeSet
}

// Type inserts text the user typed or an input method committed: Insert, or with
// SetCode, TypeCode.
func (editor *Editor) Type(value string) {
	if editor.codeSet {
		editor.TypeCode(value, editor.code)
		return
	}
	editor.Insert(value)
}

// NewLine answers Enter: a line break, indented as the code around it with
// SetCode, and plain without.
func (editor *Editor) NewLine() {
	if editor.codeSet {
		editor.InsertNewlineCode(editor.code)
		return
	}
	editor.Insert("\n")
}

// Tab answers the Tab key: a tab character, or with SetCode InsertTabCode.
func (editor *Editor) Tab() {
	if editor.codeSet {
		editor.InsertTabCode(editor.code)
		return
	}
	editor.Insert("\t")
}

// Backspace answers the Backspace key: DeleteBackward, or with SetCode
// DeleteBackwardCode.
func (editor *Editor) Backspace() {
	if editor.codeSet {
		editor.DeleteBackwardCode(editor.code)
		return
	}
	editor.DeleteBackward()
}

// ReplaceRange replaces the runes of target with value as one undo unit and puts
// the caret after what it inserted: how a find bar replaces a match, or all of
// them. It ends an active composition, which the replaced text may include.
func (editor *Editor) ReplaceRange(target Range, value string) {
	editor.clearSecondary() // one range replaced, one caret left
	target = normalizeRange(target, len(editor.runes))
	before := editor.selectionState()
	change := editor.replace(target, value, false)
	editor.collapse(target.Start + len(change.inserted))
	editor.composing = false
	editor.record(change, before, editor.selectionState(), editSeparate)
}
