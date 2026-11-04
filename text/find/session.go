package find

import "github.com/vivaui/viva/text"

// Session is the state behind a find and replace bar: the two text fields as
// editors, the options, and the matches of the query in the text being searched.
// The application hands Query and Replacement to the platform text service for
// their fields, calls Update when the text may have changed, and acts on the
// bar's buttons with Step, ReplaceCurrent and ReplaceAll.
//
// The zero value is ready. It is not safe for concurrent use.
type Session struct {
	// Query and Replacement are the editors of the bar's two fields.
	Query, Replacement text.Editor
	// Options say how the query is read. Change them freely; the next Update
	// searches again.
	Options Options

	compiled  *Query
	err       error
	matches   []Match
	truncated bool

	seen struct {
		valid   bool
		pattern string
		options Options
		value   string
	}
}

// Update searches value for the query if the query, the options or value are not
// the ones the matches are of, and reports whether it did. It costs a string
// comparison when nothing changed, so a paint can call it.
func (session *Session) Update(value string) bool {
	pattern := session.Query.String()
	if session.seen.valid && session.seen.pattern == pattern && session.seen.options == session.Options && session.seen.value == value {
		return false
	}
	session.seen.valid, session.seen.pattern, session.seen.options, session.seen.value = true, pattern, session.Options, value
	session.compiled, session.err = Compile(pattern, session.Options)
	session.matches, session.truncated = session.matches[:0], false
	if session.err != nil {
		session.compiled = nil
		return true
	}
	session.matches, session.truncated = session.compiled.Matches(session.matches, value, 0)
	return true
}

// Matches are the matches of the query in the text last given to Update, in
// order, valid until the next Update. They are what a code editor paints marks
// at.
func (session *Session) Matches() []Match { return session.matches }

// Truncated reports that there were more matches than Matches holds.
func (session *Session) Truncated() bool { return session.truncated }

// Err is why the query is not a valid regular expression, or nil.
func (session *Session) Err() error { return session.err }

// Empty reports whether the query field is empty.
func (session *Session) Empty() bool { return session.Query.RuneCount() == 0 }

// Position is the number of the match the editor's selection is, counted from
// one, and zero when the selection is not on a match, and how many matches
// there are: the "3 of 12" of the bar.
func (session *Session) Position(editor *text.Editor) (current, total int) {
	selection := editor.Selection()
	return Index(session.matches, editor.ByteOffset(selection.Start), editor.ByteOffset(selection.End)) + 1, len(session.matches)
}

func (session *Session) selectMatch(editor *text.Editor, match Match) {
	editor.SetSelection(text.Range{Start: editor.RuneIndex(match.Start), End: editor.RuneIndex(match.End)})
}

// Seek selects the first match at or after the start of the editor's selection,
// wrapping to the first: what typing in the query field does, so the editor
// follows the search from where it was. It reports whether there was a match.
func (session *Session) Seek(editor *text.Editor) bool {
	index := Next(session.matches, editor.ByteOffset(editor.Selection().Start))
	if index < 0 {
		return false
	}
	session.selectMatch(editor, session.matches[index])
	return true
}

// Step selects the match after the editor's selection, or before it when forward
// is false, wrapping around the ends of the text. It reports whether there was a
// match to go to.
func (session *Session) Step(editor *text.Editor, forward bool) bool {
	selection := editor.Selection()
	var index int
	if forward {
		index = Next(session.matches, editor.ByteOffset(selection.End))
	} else {
		index = Previous(session.matches, editor.ByteOffset(selection.Start))
	}
	if index < 0 {
		return false
	}
	session.selectMatch(editor, session.matches[index])
	return true
}

// ReplaceCurrent replaces the match the editor's selection is on with the
// replacement field's text (with $1 for a regular expression's groups) as one
// undo unit and selects the next match. When the selection is not on a match it
// only goes to the next one, as the bar's button does. It reports whether it
// replaced.
func (session *Session) ReplaceCurrent(editor *text.Editor) bool {
	selection := editor.Selection()
	index := Index(session.matches, editor.ByteOffset(selection.Start), editor.ByteOffset(selection.End))
	if index < 0 || session.compiled == nil {
		session.Step(editor, true)
		return false
	}
	replacement := session.compiled.Replacement(editor.String(), session.matches[index], session.Replacement.String())
	editor.ReplaceRange(selection, replacement)
	session.Update(editor.String())
	if next := Next(session.matches, editor.ByteOffset(editor.Caret())); next >= 0 {
		session.selectMatch(editor, session.matches[next])
	}
	return true
}

// ReplaceAll replaces every match with the replacement field's text as one undo
// unit, and returns how many it replaced.
func (session *Session) ReplaceAll(editor *text.Editor) int {
	if session.compiled == nil {
		return 0
	}
	edit, count := session.compiled.ReplaceAllEdit(editor.String(), session.Replacement.String())
	if count == 0 {
		return 0
	}
	editor.ReplaceRange(text.Range{Start: editor.RuneIndex(edit.Start), End: editor.RuneIndex(edit.End)}, edit.Text)
	session.Update(editor.String())
	return count
}
