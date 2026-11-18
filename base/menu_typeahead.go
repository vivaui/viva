package base

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// menuLabelHasPrefix reports whether label starts with prefix, comparing one
// rune at a time without lowercasing either string. Typeahead runs on a key
// press, so it should not allocate.
func menuLabelHasPrefix(label, prefix string) bool {
	for _, want := range prefix {
		got, size := utf8.DecodeRuneInString(label)
		if size == 0 || unicode.ToLower(got) != unicode.ToLower(want) {
			return false
		}
		label = label[size:]
	}
	return true
}

// MenuTypeaheadReset is how long Base UI keeps a menu's typed prefix before it
// starts a new one.
const MenuTypeaheadReset = 500 * time.Millisecond

// MenuTypeahead is caller-owned keyboard text matching for a menu or a list,
// Base UI's typeahead. Its zero value starts a new session; feed it one
// printable key at a time and it returns the row the query reaches. Time comes
// from the caller as it does for TooltipDelays, so nothing here schedules work.
//
//	var typeahead MenuTypeahead // reset it when the list opens
//	if id, ok := typeahead.Type(event.Text, now, scroll, event.Target); ok {
//		return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}
//	}
//
// Space is part of a query only while a session is running, so Type returns
// false for a lone Space and the caller can use it to activate the focused row,
// as Base UI does. A query that matches nothing ends the session.
type MenuTypeahead struct {
	query string
	start gfx.ID
	last  gfx.ID
	until time.Time
}

// MenuTypeaheadItem is one choice for TypeItems: an identity, the text the user
// may type, and whether it is skipped. It is for a closed control whose rows are
// not painted in a MenuScroll, such as a select trigger.
type MenuTypeaheadItem struct {
	ID       gfx.ID
	Label    string
	Disabled bool
}

// MenuRows is a list MenuTypeahead matches against: a component's own row
// source it can consult for a prefix, and whether a repeated first letter may
// cycle. It lets the typeahead live below the components that own the rows.
type MenuRows interface {
	Match(prefix string, from gfx.ID) (gfx.ID, bool)
	CanCycle() bool
}

// Type feeds one printable key's text to the session and returns the row the
// query now reaches. now is the caller's clock. from is the row the keyboard was
// on when the session began (zero when none was), so a new query starts after
// it; a repeated first letter continues after the previous match, which is how
// Base UI cycles through rows that share an initial. It reports false when the
// key is not typeahead (a lone or trailing Space, a multi-rune key, or no
// match), and leaves the session reset.
func (typeahead *MenuTypeahead) Type(text string, now time.Time, rows MenuRows, from gfx.ID) (gfx.ID, bool) {
	if rows == nil {
		return gfx.ID{}, false
	}
	return typeahead.typeList(text, now, rows, from)
}

// TypeItems is Type for a list that is not painted in a MenuScroll, such as a
// closed select trigger: it matches the same way and returns the option's ID so
// the caller can commit it.
func (typeahead *MenuTypeahead) TypeItems(text string, now time.Time, items []MenuTypeaheadItem, from gfx.ID) (gfx.ID, bool) {
	if items == nil {
		return gfx.ID{}, false
	}
	return typeahead.typeList(text, now, typeaheadItems(items), from)
}

// MenuTypeaheadRows is a list for TypeRows, read a row at a time: the label and
// identity of row i, and whether it is skipped. It is for a virtualized list of a
// million rows, which TypeItems would copy into a million items on every key.
type MenuTypeaheadRows struct {
	N     int
	ID    func(i int) gfx.ID
	Label func(i int) string
	// Disabled may be nil, for a list with no disabled rows.
	Disabled func(i int) bool
}

// TypeRows is TypeItems for rows read lazily: it matches the same way, without
// building the items first.
func (typeahead *MenuTypeahead) TypeRows(text string, now time.Time, rows MenuTypeaheadRows, from gfx.ID) (gfx.ID, bool) {
	if rows.N <= 0 || rows.ID == nil || rows.Label == nil {
		return gfx.ID{}, false
	}
	return typeahead.typeList(text, now, typeaheadRows(rows), from)
}

type typeaheadRows MenuTypeaheadRows

func (rows typeaheadRows) Match(prefix string, from gfx.ID) (gfx.ID, bool) {
	if prefix == "" || rows.N <= 0 {
		return gfx.ID{}, false
	}
	start := 0
	if !from.IsZero() {
		for i := 0; i < rows.N; i++ {
			if rows.ID(i) == from {
				start = i + 1
				break
			}
		}
	}
	for offset := 0; offset < rows.N; offset++ {
		i := (start + offset) % rows.N
		id := rows.ID(i)
		if id.IsZero() || rows.Disabled != nil && rows.Disabled(i) {
			continue
		}
		label := rows.Label(i)
		if label == "" {
			continue
		}
		if menuLabelHasPrefix(label, prefix) {
			return id, true
		}
	}
	return gfx.ID{}, false
}

func (rows typeaheadRows) CanCycle() bool {
	for i := 0; i < rows.N; i++ {
		id := rows.ID(i)
		if id.IsZero() || rows.Disabled != nil && rows.Disabled(i) {
			continue
		}
		label := rows.Label(i)
		first, firstSize := utf8.DecodeRuneInString(label)
		if firstSize == 0 {
			continue
		}
		second, secondSize := utf8.DecodeRuneInString(label[firstSize:])
		if secondSize > 0 && unicode.ToLower(first) == unicode.ToLower(second) {
			return false
		}
	}
	return true
}

func (typeahead *MenuTypeahead) typeList(text string, now time.Time, list MenuRows, from gfx.ID) (gfx.ID, bool) {
	if text == "" || utf8.RuneCountInString(text) != 1 {
		return gfx.ID{}, false
	}
	if rune, _ := utf8.DecodeRuneInString(text); unicode.IsControl(rune) {
		return gfx.ID{}, false
	}
	active := !typeahead.until.IsZero() && now.Before(typeahead.until) && typeahead.query != ""
	if text == " " && !active {
		return gfx.ID{}, false
	}
	if !active {
		typeahead.query = ""
		typeahead.start = from
		typeahead.last = from
	}
	matchFrom := typeahead.start
	if active && typeahead.query == text && list.CanCycle() {
		typeahead.query = ""
		matchFrom = typeahead.last
	}
	typeahead.query += text
	if id, ok := list.Match(typeahead.query, matchFrom); ok {
		typeahead.last = id
		typeahead.until = now.Add(MenuTypeaheadReset)
		return id, true
	}
	typeahead.Reset()
	return gfx.ID{}, false
}

// Typing reports whether a session is running at now, so the caller can tell a
// Space that continues a query from one that activates the focused row.
func (typeahead *MenuTypeahead) Typing(now time.Time) bool {
	return !typeahead.until.IsZero() && now.Before(typeahead.until) && typeahead.query != ""
}

// Reset forgets the session, as closing the list does.
func (typeahead *MenuTypeahead) Reset() { *typeahead = MenuTypeahead{} }

type typeaheadItems []MenuTypeaheadItem

func (items typeaheadItems) Match(prefix string, from gfx.ID) (gfx.ID, bool) {
	if prefix == "" || len(items) == 0 {
		return gfx.ID{}, false
	}
	start := 0
	if !from.IsZero() {
		for index, item := range items {
			if item.ID == from {
				start = index + 1
				break
			}
		}
	}
	for offset := 0; offset < len(items); offset++ {
		item := items[(start+offset)%len(items)]
		if item.ID.IsZero() || item.Disabled || item.Label == "" {
			continue
		}
		if menuLabelHasPrefix(item.Label, prefix) {
			return item.ID, true
		}
	}
	return gfx.ID{}, false
}

func (items typeaheadItems) CanCycle() bool {
	for _, item := range items {
		if item.Disabled || item.ID.IsZero() {
			continue
		}
		first, firstSize := utf8.DecodeRuneInString(item.Label)
		if firstSize == 0 {
			continue
		}
		second, secondSize := utf8.DecodeRuneInString(item.Label[firstSize:])
		if secondSize > 0 && unicode.ToLower(first) == unicode.ToLower(second) {
			return false
		}
	}
	return true
}
