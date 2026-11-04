// Package find searches text for the find and replace bar of an editor: plain
// text or a regular expression, with or without regard to case, and as whole
// words or not. It works on UTF-8 strings and reports byte offsets, the unit the
// editing components paint with; an editor converts them to rune indexes where
// it needs those (text.Editor.RuneIndex).
//
// It is a package of its own because regular expressions are not free to link:
// an application that never searches does not carry them.
package find

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
)

// DefaultLimit is how many matches Matches collects when asked for none in
// particular: more than a person can step through, and a bound on what an
// accidental pattern such as a single space can allocate in a megabyte of text.
const DefaultLimit = 10_000

// Options says how a pattern is read.
type Options struct {
	// CaseSensitive makes "a" not match "A".
	CaseSensitive bool
	// WholeWord keeps only matches that are not part of a longer word: not
	// preceded or followed by a letter, digit or underscore, unless the match
	// itself begins or ends with something that is not one.
	WholeWord bool
	// Regexp reads the pattern as a regular expression (the syntax of package
	// regexp, with ^ and $ matching at each line).
	Regexp bool
}

// Match is the byte range [Start, End) of one match, a text.Span: the matches of
// a search are what a code editor paints marks at without conversion. A regular
// expression may match nothing at all, a range with Start equal to End.
type Match = text.Span

// Query is a compiled pattern. It is immutable and safe for concurrent use.
type Query struct {
	pattern string
	options Options
	// re is nil for a plain case-sensitive search, which strings.Index does
	// faster than any expression could.
	re *regexp.Regexp
}

// Compile prepares pattern for searching. An empty pattern is valid and has no
// matches; a regular expression that does not parse returns its error.
func Compile(pattern string, options Options) (*Query, error) {
	query := &Query{pattern: pattern, options: options}
	if pattern == "" {
		return query, nil
	}
	if !options.Regexp && options.CaseSensitive {
		return query, nil
	}
	expression := pattern
	if !options.Regexp {
		expression = regexp.QuoteMeta(pattern)
	}
	flags := "(?m"
	if !options.CaseSensitive {
		flags += "i"
	}
	re, err := regexp.Compile(flags + ")" + expression)
	if err != nil {
		return nil, err
	}
	query.re = re
	return query, nil
}

// Pattern is the text the query was compiled from.
func (query *Query) Pattern() string { return query.pattern }

// Options is how the query reads its pattern.
func (query *Query) Options() Options { return query.options }

// Empty reports whether the query has nothing to look for.
func (query *Query) Empty() bool { return query.pattern == "" }

// Matches appends to dst the matches of the query in value, in order and without
// overlap, at most limit of them (DefaultLimit when limit is zero or less), and
// reports whether there were more. Appending reuses dst's capacity, so a bar that
// searches again after every keystroke allocates only when its text grows.
func (query *Query) Matches(dst []Match, value string, limit int) (matches []Match, truncated bool) {
	if query.Empty() {
		return dst, false
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	if query.options.Regexp {
		return query.regexpMatches(dst, value, limit)
	}
	for from := 0; from <= len(value); {
		start, end, ok := query.literal(value, from)
		if !ok {
			break
		}
		if query.options.WholeWord && !wholeWord(value, start, end) {
			from = start + runeLength(value, start) // a longer match may start inside this one
			continue
		}
		if len(dst) >= limit {
			return dst, true
		}
		dst = append(dst, Match{Start: start, End: end})
		from = end
	}
	return dst, false
}

func runeLength(value string, at int) int {
	if at >= len(value) {
		return 1
	}
	_, size := utf8.DecodeRuneInString(value[at:])
	return size
}

// literal finds the first plain match at or after from.
func (query *Query) literal(value string, from int) (start, end int, ok bool) {
	if query.re == nil {
		index := strings.Index(value[from:], query.pattern)
		if index < 0 {
			return 0, 0, false
		}
		return from + index, from + index + len(query.pattern), true
	}
	// The pattern is quoted text, so it has no anchors or boundaries to lose by
	// searching from a cut.
	found := query.re.FindStringIndex(value[from:])
	if found == nil {
		return 0, 0, false
	}
	return from + found[0], from + found[1], true
}

func (query *Query) regexpMatches(dst []Match, value string, limit int) ([]Match, bool) {
	found := query.re.FindAllStringIndex(value, -1)
	for _, span := range found {
		if query.options.WholeWord && !wholeWord(value, span[0], span[1]) {
			continue
		}
		if len(dst) >= limit {
			return dst, true
		}
		dst = append(dst, Match{Start: span[0], End: span[1]})
	}
	return dst, false
}

// Replacement is the text that replaces match when the query is applied with
// template. For a regular expression the template may refer to groups as $1 or
// ${name}, and $$ is a dollar; for plain text it is used as it is.
func (query *Query) Replacement(value string, match Match, template string) string {
	if !query.options.Regexp || query.re == nil || !strings.Contains(template, "$") {
		return template
	}
	for _, submatch := range query.re.FindAllStringSubmatchIndex(value, -1) {
		if submatch[0] == match.Start && submatch[1] == match.End {
			return string(query.re.ExpandString(nil, template, value, submatch))
		}
	}
	return template
}

// each calls visit with the bytes [start, end) of every match, in order, and the
// text that replaces it, and returns how many there were.
func (query *Query) each(value, template string, visit func(start, end int, replacement string)) int {
	if query.Empty() {
		return 0
	}
	count := 0
	if query.options.Regexp {
		expandsGroups := strings.Contains(template, "$")
		for _, submatch := range query.re.FindAllStringSubmatchIndex(value, -1) {
			if query.options.WholeWord && !wholeWord(value, submatch[0], submatch[1]) {
				continue
			}
			replacement := template
			if expandsGroups {
				replacement = string(query.re.ExpandString(nil, template, value, submatch))
			}
			visit(submatch[0], submatch[1], replacement)
			count++
		}
		return count
	}
	matches, _ := query.Matches(nil, value, len(value)+1)
	for _, match := range matches {
		visit(match.Start, match.End, template)
	}
	return len(matches)
}

// ReplaceAll returns value with every match of the query replaced by template
// (see Replacement), and the number of replacements. It is one pass: a
// replacement is never searched again.
func (query *Query) ReplaceAll(value, template string) (string, int) {
	var out strings.Builder
	last := 0
	count := query.each(value, template, func(start, end int, replacement string) {
		out.WriteString(value[last:start])
		out.WriteString(replacement)
		last = end
	})
	if count == 0 {
		return value, 0
	}
	out.WriteString(value[last:])
	return out.String(), count
}

// Edit is a replacement of the bytes [Start, End) of a text by Text.
type Edit struct {
	Start, End int
	Text       string
}

// ReplaceAllEdit is ReplaceAll as the one edit that makes it: from the start of
// the first match to the end of the last, which an editor applies as a single
// undo unit without rewriting the text around it. It returns the number of
// replacements, and an empty Edit when there are none.
func (query *Query) ReplaceAllEdit(value, template string) (Edit, int) {
	var out strings.Builder
	edit := Edit{Start: -1}
	count := query.each(value, template, func(start, end int, replacement string) {
		if edit.Start < 0 {
			edit.Start = start
		} else {
			out.WriteString(value[edit.End:start])
		}
		out.WriteString(replacement)
		edit.End = end
	})
	if count == 0 {
		return Edit{}, 0
	}
	edit.Text = out.String()
	return edit, count
}

// Next is the index of the first match that starts at or after offset, wrapping
// to the first match when there is none, and -1 when there are no matches.
func Next(matches []Match, offset int) int {
	if len(matches) == 0 {
		return -1
	}
	low, high := 0, len(matches)
	for low < high {
		middle := (low + high) / 2
		if matches[middle].Start < offset {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low == len(matches) {
		return 0
	}
	return low
}

// Previous is the index of the last match that starts before offset, wrapping to
// the last match when there is none, and -1 when there are no matches.
func Previous(matches []Match, offset int) int {
	if len(matches) == 0 {
		return -1
	}
	index := Next(matches, offset)
	if index <= 0 {
		return len(matches) - 1
	}
	return index - 1
}

// Index is the index of the match that is exactly the range [start, end), or -1:
// which match the current selection is, for a bar that shows "3 of 12".
func Index(matches []Match, start, end int) int {
	low, high := 0, len(matches)
	for low < high {
		middle := (low + high) / 2
		if matches[middle].Start < start {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low < len(matches) && matches[low].Start == start && matches[low].End == end {
		return low
	}
	return -1
}

// wholeWord reports whether [start, end) of value stands as a word of its own.
func wholeWord(value string, start, end int) bool {
	if start == end {
		return true
	}
	first, _ := utf8.DecodeRuneInString(value[start:end])
	last, _ := utf8.DecodeLastRuneInString(value[start:end])
	if start > 0 && isWordRune(first) {
		if previous, _ := utf8.DecodeLastRuneInString(value[:start]); isWordRune(previous) {
			return false
		}
	}
	if end < len(value) && isWordRune(last) {
		if next, _ := utf8.DecodeRuneInString(value[end:]); isWordRune(next) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
