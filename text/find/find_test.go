package find

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func m(start, end int) Match { return Match{Start: start, End: end} }

// ms is the matches of consecutive start, end pairs.
func ms(pairs ...int) []Match {
	var out []Match
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, m(pairs[i], pairs[i+1]))
	}
	return out
}

func mustCompile(t testing.TB, pattern string, options Options) *Query {
	t.Helper()
	query, err := Compile(pattern, options)
	if err != nil {
		t.Fatalf("Compile(%q, %+v): %v", pattern, options, err)
	}
	return query
}

func spans(t testing.TB, pattern string, options Options, value string) []Match {
	t.Helper()
	matches, _ := mustCompile(t, pattern, options).Matches(nil, value, 0)
	return matches
}

func TestMatchesPlainText(t *testing.T) {
	sensitive := Options{CaseSensitive: true}
	cases := []struct {
		name, pattern, value string
		options              Options
		want                 []Match
	}{
		{"case sensitive", "ab", "ab AB aB ab", sensitive, ms(0, 2, 9, 11)},
		{"ignoring case", "ab", "ab AB aB xb", Options{}, ms(0, 2, 3, 5, 6, 8)},
		{"multi-byte offsets are bytes", "文", "中文 文", sensitive, ms(3, 6, 7, 10)},
		{"ignoring case in other scripts", "ÄB", "xäb ÄB", Options{}, ms(1, 4, 5, 8)},
		{"matches do not overlap", "aa", "aaaaa", sensitive, ms(0, 2, 2, 4)},
		{"special characters are plain", "a.b*", "a.b* axb", Options{}, ms(0, 4)},
		{"empty pattern", "", "abc", sensitive, nil},
		{"no match", "z", "abc", sensitive, nil},
		{"across lines", "b\nc", "a b\nc", sensitive, ms(2, 5)},
	}
	for _, c := range cases {
		if got := spans(t, c.pattern, c.options, c.value); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestWholeWord(t *testing.T) {
	cases := []struct {
		name, pattern, value string
		options              Options
		want                 []Match
	}{
		{"skips inner matches", "cat", "cat concat cats cat_ cat.", Options{CaseSensitive: true, WholeWord: true}, ms(0, 3, 21, 24)},
		{"a longer match may start inside a rejected one", "aa", "aaa aa", Options{CaseSensitive: true, WholeWord: true}, ms(4, 6)},
		{"ignoring case", "Cat", "CAT cats", Options{WholeWord: true}, ms(0, 3)},
		{"words in other scripts", "文字", "文字 汉文字", Options{CaseSensitive: true, WholeWord: true}, ms(0, 6)},
		{"punctuation at the end needs no boundary", "foo(", "foo( xfoo( foo(x", Options{CaseSensitive: true, WholeWord: true}, ms(0, 4, 11, 15)},
		{"regular expression", `c.t`, "cat cut cats", Options{CaseSensitive: true, WholeWord: true, Regexp: true}, ms(0, 3, 4, 7)},
	}
	for _, c := range cases {
		if got := spans(t, c.pattern, c.options, c.value); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestMatchesRegularExpressions(t *testing.T) {
	multiline := Options{CaseSensitive: true, Regexp: true}
	cases := []struct {
		name, pattern, value string
		options              Options
		want                 []Match
	}{
		{"digits", `\d+`, "a1 b22 c333", multiline, ms(1, 2, 4, 6, 8, 11)},
		{"line anchors", `^b`, "a\nb\nbc", multiline, ms(2, 3, 4, 5)},
		{"end of line", `a$`, "ba\nab\na", multiline, ms(1, 2, 6, 7)},
		{"ignoring case", `a+`, "aA a", Options{Regexp: true}, ms(0, 2, 3, 4)},
		{"zero-width matches", `^`, "a\nb", multiline, ms(0, 0, 2, 2)},
		{"dot does not cross a line", `a.b`, "a\nb axb", multiline, ms(4, 7)},
	}
	for _, c := range cases {
		if got := spans(t, c.pattern, c.options, c.value); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	if _, err := Compile(`(`, Options{Regexp: true}); err == nil {
		t.Error("an unbalanced group compiled")
	}
	if _, err := Compile(`(`, Options{}); err != nil {
		t.Errorf("a plain parenthesis is not an expression: %v", err)
	}
}

func TestMatchesAreLimited(t *testing.T) {
	value := strings.Repeat("a ", 100)
	query := mustCompile(t, "a", Options{CaseSensitive: true})
	matches, truncated := query.Matches(nil, value, 10)
	if len(matches) != 10 || !truncated {
		t.Fatalf("limit 10 gave %d matches, truncated %v", len(matches), truncated)
	}
	if matches, truncated = query.Matches(nil, value, 100); len(matches) != 100 || truncated {
		t.Fatalf("limit 100 gave %d matches, truncated %v", len(matches), truncated)
	}
	if matches, _ = query.Matches(nil, strings.Repeat("a", DefaultLimit+50), 0); len(matches) != DefaultLimit {
		t.Fatalf("the default limit gave %d matches", len(matches))
	}
	regular := mustCompile(t, "a", Options{Regexp: true})
	if matches, truncated = regular.Matches(nil, value, 10); len(matches) != 10 || !truncated {
		t.Fatalf("regular expression limit gave %d matches, truncated %v", len(matches), truncated)
	}
}

func TestMatchesAppendToDst(t *testing.T) {
	query := mustCompile(t, "a", Options{CaseSensitive: true})
	dst := make([]Match, 1, 8)
	dst[0] = Match{Start: -1}
	got, _ := query.Matches(dst, "aa", 0)
	if len(got) != 3 || got[0].Start != -1 || got[1] != (m(0, 1)) {
		t.Fatalf("Matches did not append: %+v", got)
	}
}

func TestReplaceAll(t *testing.T) {
	cases := []struct {
		name, pattern, value, template string
		options                        Options
		want                           string
		count                          int
	}{
		{"plain", "a", "banana", "o", Options{CaseSensitive: true}, "bonono", 3},
		{"a dollar in plain text stays", "a", "ba", "$1", Options{CaseSensitive: true}, "b$1", 1},
		{"groups", `(\w+)@(\w+)`, "me@host you@there", "$2:$1", Options{CaseSensitive: true, Regexp: true}, "host:me there:you", 2},
		{"a replacement is not searched again", "a", "aa", "aa", Options{CaseSensitive: true}, "aaaa", 2},
		{"insert at line starts", `^`, "a\nb", "// ", Options{Regexp: true, CaseSensitive: true}, "// a\n// b", 2},
		{"deleting", "x", "axbxc", "", Options{CaseSensitive: true}, "abc", 2},
		{"whole words", "cat", "cat concat", "dog", Options{CaseSensitive: true, WholeWord: true}, "dog concat", 1},
		{"no match", "z", "abc", "y", Options{CaseSensitive: true}, "abc", 0},
		{"empty pattern", "", "abc", "y", Options{CaseSensitive: true}, "abc", 0},
		{"ignoring case", "ab", "AB ab", "x", Options{}, "x x", 2},
		{"multi-byte", "文", "中文x", "字", Options{CaseSensitive: true}, "中字x", 1},
	}
	for _, c := range cases {
		got, count := mustCompile(t, c.pattern, c.options).ReplaceAll(c.value, c.template)
		if got != c.want || count != c.count {
			t.Errorf("%s: %q, %d; want %q, %d", c.name, got, count, c.want, c.count)
		}
	}
}

func TestReplacementOfOneMatch(t *testing.T) {
	query := mustCompile(t, `(\d+)-(\d+)`, Options{CaseSensitive: true, Regexp: true})
	value := "1-2 and 30-40"
	matches, _ := query.Matches(nil, value, 0)
	if len(matches) != 2 {
		t.Fatal(matches)
	}
	if got := query.Replacement(value, matches[1], "$2/$1"); got != "40/30" {
		t.Fatalf("Replacement = %q", got)
	}
	plain := mustCompile(t, "a", Options{CaseSensitive: true})
	if got := plain.Replacement("a", m(0, 1), "$1"); got != "$1" {
		t.Fatalf("plain replacement = %q", got)
	}
}

func TestNextPreviousIndex(t *testing.T) {
	matches := ms(2, 3, 5, 6, 9, 10)
	nexts := []struct{ offset, want int }{{0, 0}, {2, 0}, {3, 1}, {5, 1}, {6, 2}, {10, 0}, {99, 0}}
	for _, c := range nexts {
		if got := Next(matches, c.offset); got != c.want {
			t.Errorf("Next(%d) = %d, want %d", c.offset, got, c.want)
		}
	}
	previous := []struct{ offset, want int }{{0, 2}, {2, 2}, {3, 0}, {5, 0}, {6, 1}, {10, 2}, {99, 2}}
	for _, c := range previous {
		if got := Previous(matches, c.offset); got != c.want {
			t.Errorf("Previous(%d) = %d, want %d", c.offset, got, c.want)
		}
	}
	if Next(nil, 0) != -1 || Previous(nil, 0) != -1 {
		t.Error("no matches should give -1")
	}
	if Index(matches, 5, 6) != 1 || Index(matches, 5, 7) != -1 || Index(matches, 4, 4) != -1 || Index(nil, 0, 0) != -1 {
		t.Error("Index finds only an exact match")
	}
}

// naive is a reference for plain case-sensitive search.
func naive(value, pattern string) []Match {
	var out []Match
	for from := 0; from < len(value); {
		index := strings.Index(value[from:], pattern)
		if index < 0 {
			break
		}
		out = append(out, Match{Start: from + index, End: from + index + len(pattern)})
		from += index + len(pattern)
	}
	return out
}

func TestPlainSearchAgreesWithStrings(t *testing.T) {
	random := rand.New(rand.NewSource(5))
	alphabet := []string{"a", "b", "A", "中", "é", " ", "\n"}
	build := func(n int) string {
		var out strings.Builder
		for i := 0; i < n; i++ {
			out.WriteString(alphabet[random.Intn(len(alphabet))])
		}
		return out.String()
	}
	for round := 0; round < 500; round++ {
		value, pattern := build(random.Intn(60)), build(1+random.Intn(3))
		if got, want := spans(t, pattern, Options{CaseSensitive: true}, value), naive(value, pattern); !reflect.DeepEqual(got, want) && (len(got) != 0 || len(want) != 0) {
			t.Fatalf("%q in %q: %+v, want %+v", pattern, value, got, want)
		}
		// Ignoring case, a match is a case-insensitive equal of the pattern.
		for _, match := range spans(t, pattern, Options{}, value) {
			if !strings.EqualFold(value[match.Start:match.End], pattern) {
				t.Fatalf("%q matched %q ignoring case", pattern, value[match.Start:match.End])
			}
		}
	}
}

func checkMatches(t testing.TB, value string, matches []Match) {
	t.Helper()
	valid := utf8.ValidString(value)
	previous := 0
	for index, match := range matches {
		if match.Start < previous || match.End < match.Start || match.End > len(value) {
			t.Fatalf("match %d %+v is out of order or range in %d bytes (previous end %d)", index, match, len(value), previous)
		}
		if valid && (!utf8.ValidString(value[match.Start:match.End]) || match.Start < len(value) && !utf8.RuneStart(value[match.Start])) {
			t.Fatalf("match %d %+v is not on character boundaries of %q", index, match, value)
		}
		previous = match.End
	}
}

func FuzzFind(f *testing.F) {
	f.Add("a", "banana", uint8(0))
	f.Add(`(\w+)`, "hello world", uint8(7))
	f.Add("^", "a\nb\n", uint8(5))
	f.Add("文", "中文 文字", uint8(2))
	f.Add("", "x", uint8(1))
	f.Fuzz(func(t *testing.T, pattern, value string, flags uint8) {
		options := Options{CaseSensitive: flags&1 != 0, WholeWord: flags&2 != 0, Regexp: flags&4 != 0}
		query, err := Compile(pattern, options)
		if err != nil {
			return
		}
		matches, _ := query.Matches(nil, value, 50)
		checkMatches(t, value, matches)
		replaced, count := query.ReplaceAll(value, "")
		if count == 0 && replaced != value {
			t.Fatalf("no replacements changed %q into %q", value, replaced)
		}
		if len(matches) > 0 {
			_ = query.Replacement(value, matches[0], "$1")
		}
		Next(matches, len(value)/2)
		Previous(matches, len(value)/2)
	})
}

func BenchmarkMatches(b *testing.B) {
	value := strings.Repeat("the quick brown fox jumps over the lazy dog\n", 24_000) // about 1 MiB
	for _, c := range []struct {
		name    string
		pattern string
		options Options
	}{
		{"plain", "lazy", Options{CaseSensitive: true}},
		{"ignoring case", "LAZY", Options{}},
		{"whole word", "the", Options{CaseSensitive: true, WholeWord: true}},
		{"regular expression", `qu\w+k`, Options{CaseSensitive: true, Regexp: true}},
	} {
		query := mustCompile(b, c.pattern, c.options)
		var dst []Match
		b.Run(c.name, func(b *testing.B) {
			b.SetBytes(int64(len(value)))
			b.ReportAllocs()
			for b.Loop() {
				dst, _ = query.Matches(dst[:0], value, 0)
			}
		})
	}
}
