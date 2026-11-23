package base

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

// bound is a Field holding value with the selection made from anchor to caret (rune
// indexes), as a field bound to a string holds it.
func bound(value string, anchor, caret int) *Field {
	var field Field
	field.SetText(value)
	field.ensure().SetSelection(text.Range{Start: anchor, End: caret})
	return &field
}

func selectionOf(field *Field) (anchor, caret int) {
	editor := field.ensure()
	selection := editor.Selection()
	caret = editor.Caret()
	anchor = selection.Start
	if caret == selection.Start {
		anchor = selection.End
	}
	return anchor, caret
}

// A string the program changed is put in its field without taking the user's selection
// away: the selection keeps to the text it was on.
func TestAProgramChangeKeepsTheSelectionOnItsText(t *testing.T) {
	for _, c := range []struct {
		name                  string
		before, after         string
		anchor, caret         int // in before
		wantAnchor, wantCaret int // in after
	}{
		{"line appended after a selection", "one\ntwo\n", "one\ntwo\nthree\n", 0, 3, 0, 3},
		{"line inserted before a selection", "one\ntwo\n", "zero\none\ntwo\n", 4, 7, 9, 12},
		{"backwards selection moved", "one\ntwo\n", "zero\none\ntwo\n", 7, 4, 12, 9},
		{"caret at the end follows a log", "one\n", "one\ntwo\n", 4, 4, 8, 8},
		{"caret before inserted text stays", "ab", "aXb", 1, 1, 1, 1},
		{"caret inside replaced text goes after it", "a1234b", "aXYb", 3, 3, 3, 3},
		{"multibyte prefix", "中文a", "中文ba", 2, 3, 2, 4},
		{"text removed before the caret", "drop this keep", "keep", 12, 14, 2, 4},
	} {
		field := bound(c.before, c.anchor, c.caret)
		field.follow(c.after)
		if got := field.Text(); got != c.after {
			t.Fatalf("%s: text %q, want %q", c.name, got, c.after)
		}
		if anchor, caret := selectionOf(field); anchor != c.wantAnchor || caret != c.wantCaret {
			t.Errorf("%s: selection %d..%d, want %d..%d", c.name, anchor, caret, c.wantAnchor, c.wantCaret)
		}
	}
}

// A field that only takes some characters is set whole, as SetText sets it.
func TestAProgramChangeOfARestrictedFieldSetsItWhole(t *testing.T) {
	field := bound("123", 0, 0)
	field.ensure().Restrict(func(r rune) bool { return r >= '0' && r <= '9' }, 0)
	field.follow("12345")
	if got := field.Text(); got != "12345" {
		t.Fatalf("text %q", got)
	}
	if _, caret := selectionOf(field); caret != 5 {
		t.Fatalf("caret %d, want it at the end as SetText puts it", caret)
	}
}

// BenchmarkFieldFollowAppendedLine is a log of 20000 lines bound to a TextArea growing by
// one line: what the page does to the field for it.
func BenchmarkFieldFollowAppendedLine(b *testing.B) {
	var text strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&text, "line %d the quick brown fox jumps over the lazy dog 中文混排\n", i)
	}
	base := text.String()
	texts := make([]string, 64)
	for i := range texts {
		texts[i] = base + strings.Repeat(fmt.Sprintf("appended %d\n", i), i%2+1)
	}
	var field Field
	field.SetText(base)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		field.follow(texts[i%len(texts)])
	}
}

// BenchmarkFieldSetTextAppendedLine is BenchmarkFieldFollowAppendedLine with the text set
// whole, as a bound string was set before.
func BenchmarkFieldSetTextAppendedLine(b *testing.B) {
	var text strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&text, "line %d the quick brown fox jumps over the lazy dog 中文混排\n", i)
	}
	base := text.String()
	texts := make([]string, 64)
	for i := range texts {
		texts[i] = base + strings.Repeat(fmt.Sprintf("appended %d\n", i), i%2+1)
	}
	var field Field
	field.SetText(base)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		field.SetText(texts[i%len(texts)])
	}
}
