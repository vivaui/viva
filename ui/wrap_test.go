package ui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// wrapper measures every character as `size` wide, so a width is a count of
// characters and the tests do not depend on a font.
func wrapper() *Canvas {
	canvas := &Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(utf8.RuneCountInString(value)) * size })
	return canvas
}

func TestGraphemeStartClampsAndPreservesWholeCharacters(t *testing.T) {
	value := "a👩‍💻中"
	for _, test := range []struct{ offset, want int }{
		{-3, 0}, {0, 0}, {1, 1}, {2, 1},
		{len("a👩") + 1, 1},
		{len("a👩‍💻"), len("a👩‍💻")},
		{len(value) + 3, len(value)},
	} {
		if got := GraphemeStart(value, test.offset); got != test.want {
			t.Errorf("GraphemeStart(%d) = %d, want %d", test.offset, got, test.want)
		}
	}
}

func TestWrapLinesBreaksAtSpaces(t *testing.T) {
	canvas := wrapper()
	for _, test := range []struct {
		value string
		width float32 // characters
		max   int
		want  []string
	}{
		{"short", 10, 3, []string{"short"}},
		{"connect to host now", 10, 5, []string{"connect to", "host now"}},
		{"connect to host now", 7, 5, []string{"connect", "to host", "now"}},
		{"one two three four", 8, 5, []string{"one two", "three", "four"}},
		{"  leading and   gaps  ", 11, 5, []string{"  leading", "and   gaps"}},
		{"trailing   ", 20, 5, []string{"trailing"}},
		{"", 10, 3, nil},
	} {
		got := canvas.WrapLines(nil, test.value, 1, 0, test.width, test.max)
		if strings.Join(got, "|") != strings.Join(test.want, "|") {
			t.Errorf("%q in %v: %q, want %q", test.value, test.width, got, test.want)
		}
	}
}

func TestWrapLinesCutsWordsWiderThanTheLine(t *testing.T) {
	canvas := wrapper()
	got := canvas.WrapLines(nil, "SHA256:abcdefghijklmnopqrstuvwxyz0123456789", 1, 0, 16, 5)
	if strings.Join(got, "") != "SHA256:abcdefghijklmnopqrstuvwxyz0123456789" {
		t.Fatalf("cutting a long word lost characters: %q", got)
	}
	for _, line := range got {
		if utf8.RuneCountInString(line) > 16 {
			t.Errorf("line %q is wider than the 16 characters it was given", line)
		}
	}
	if len(got) != 3 {
		t.Errorf("42 characters in 16 took %d lines: %q", len(got), got)
	}
	// A width narrower than one character still makes progress.
	if got := canvas.WrapLines(nil, "abc", 1, 0, 0.5, 5); len(got) != 3 {
		t.Errorf("a sliver of width gave %q", got)
	}
}

func TestWrapLinesBreaksBetweenCJKCharactersButNotBeforeClosingPunctuation(t *testing.T) {
	canvas := wrapper()
	got := canvas.WrapLines(nil, "主机密钥已变更,未发送任何数据。", 1, 0, 7, 5)
	if strings.Join(got, "") != "主机密钥已变更,未发送任何数据。" {
		t.Fatalf("lost characters: %q", got)
	}
	for _, line := range got {
		if utf8.RuneCountInString(line) > 7 {
			t.Errorf("line %q is too wide", line)
		}
		if r, _ := utf8.DecodeRuneInString(line); strings.ContainsRune(",。", r) {
			t.Errorf("line %q starts with closing punctuation", line)
		}
	}
	// Latin words inside Chinese text stay whole.
	got = canvas.WrapLines(nil, "连接 SSH 服务器失败", 1, 0, 8, 5)
	for _, line := range got {
		if strings.Contains(line, "SS") && !strings.Contains(line, "SSH") {
			t.Errorf("a word was split: %q", got)
		}
	}
}

func TestWrapLinesKeepsNewlinesAndEllipsizesTheLastLine(t *testing.T) {
	canvas := wrapper()
	if got := canvas.WrapLines(nil, "first\nsecond line here", 1, 0, 9, 5); strings.Join(got, "|") != "first|second|line here" {
		t.Errorf("newline handling: %q", got)
	}
	got := canvas.WrapLines(nil, "one two three four five six seven eight", 1, 0, 9, 2)
	if len(got) != 2 || !strings.HasSuffix(got[1], "…") || utf8.RuneCountInString(got[1]) > 9 {
		t.Fatalf("overflow: %q", got)
	}
	if got[0] != "one two" {
		t.Errorf("first line = %q", got[0])
	}
	// Text that only just fits is not ellipsized.
	if got := canvas.WrapLines(nil, "one two three", 1, 0, 9, 2); strings.Join(got, "|") != "one two|three" {
		t.Errorf("exact fit: %q", got)
	}
	// With room for one line only, that line is the ellipsized start.
	if got := canvas.WrapLines(nil, "a long single line", 1, 0, 8, 1); len(got) != 1 || !strings.HasSuffix(got[0], "…") || utf8.RuneCountInString(got[0]) > 8 {
		t.Errorf("one line: %q", got)
	}
	// A line that fits but has more text after a newline still says so.
	if got := canvas.WrapLines(nil, "fits\nand more", 1, 0, 12, 1); len(got) != 1 || got[0] != "fits…" {
		t.Errorf("a cut paragraph: %q", got)
	}
	// Blank lines between paragraphs are kept, a trailing newline is not.
	if got := canvas.WrapLines(nil, "a\n\nb\n", 1, 0, 5, 5); strings.Join(got, "|") != "a||b" {
		t.Errorf("blank lines: %q", got)
	}
}

func TestWrapLinesAppendsAfterWhatIsThereAndHandlesNothingToDo(t *testing.T) {
	canvas := wrapper()
	got := canvas.WrapLines([]string{"kept"}, "a b", 1, 0, 5, 2)
	if len(got) != 2 || got[0] != "kept" || got[1] != "a b" {
		t.Errorf("dst was not appended to: %q", got)
	}
	if got := canvas.WrapLines(nil, "text", 1, 0, 0, 3); got != nil {
		t.Errorf("zero width: %q", got)
	}
	if got := canvas.WrapLines(nil, "text", 1, 0, 10, 0); got != nil {
		t.Errorf("zero lines: %q", got)
	}
}

func TestWrapLinesBuildsNothingWhenTheTextFitsOrBreaksAtSpaces(t *testing.T) {
	canvas := wrapper()
	scratch := make([]string, 0, 4)
	text := "connect to host now\nand then"
	if allocations := testing.AllocsPerRun(100, func() { scratch = canvas.WrapLines(scratch[:0], text, 1, 0, 10, 4) }); allocations != 0 {
		t.Fatalf("wrapping allocated %v times", allocations)
	}
}

// The last line of a wrapped text ends in an ellipsis, and a control painted every
// frame wraps the same text again at the same width: the canvas remembers the cut, so
// only the first wrap builds the string. A text field whose placeholder is wider than
// the field is the case that first exposed this.
func TestWrapLinesWithAnEllipsisBuildsTheCutLineOnce(t *testing.T) {
	canvas := wrapper()
	scratch := make([]string, 0, 1)
	text := "Search hosts or ssh user@hostname"
	const width = 30
	lines := canvas.WrapLines(scratch[:0], text, 1, 0, width, 1)
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "…") {
		t.Fatalf("one cut line = %q", lines)
	}
	cut := lines[0]
	if allocations := testing.AllocsPerRun(100, func() { scratch = canvas.WrapLines(scratch[:0], text, 1, 0, width, 1) }); allocations != 0 {
		t.Fatalf("re-wrapping an ellipsized line allocated %v times", allocations)
	}
	if got := canvas.WrapLines(scratch[:0], text, 1, 0, width, 1)[0]; got != cut {
		t.Fatalf("remembered cut = %q, want %q", got, cut)
	}
	// A different measurement answers differently: the memo never serves a stale cut.
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size / 2
	})
	if got := canvas.WrapLines(scratch[:0], text, 1, 0, width, 1)[0]; got == cut {
		t.Fatalf("cut survived a measurer change: %q", got)
	}
}

func BenchmarkWrapLines(b *testing.B) {
	canvas := wrapper()
	scratch := make([]string, 0, 4)
	text := "Connection failed: connect to 10.255.255.1:22: dial tcp 10.255.255.1:22: i/o timeout"
	b.ReportAllocs()
	for range b.N {
		scratch = canvas.WrapLines(scratch[:0], text, 1, 0, 30, 3)
	}
}

// Whatever the text, width and line limit: no panic, no more lines than asked,
// every line within the width (a single character wider than the line aside),
// and with room enough, every non-blank character of the text in order.
func TestWrapLinesProperties(t *testing.T) {
	canvas := wrapper()
	alphabet := []rune("ab c  \n主机，。x-/:")
	random := newTestRandom(5)
	for round := 0; round < 3000; round++ {
		runes := make([]rune, random(40))
		for index := range runes {
			runes[index] = alphabet[random(len(alphabet))]
		}
		value := string(runes)
		width := float32(1 + random(12))
		limit := 1 + random(6)
		lines := canvas.WrapLines(nil, value, 1, 0, width, limit)
		if len(lines) > limit {
			t.Fatalf("%q at %v: %d lines for a limit of %d", value, width, len(lines), limit)
		}
		for _, line := range lines {
			if n := utf8.RuneCountInString(line); float32(n) > width && nextCluster(line, 0) < len(line) {
				t.Fatalf("%q at %v: line %q is %d wide", value, width, line, n)
			}
		}
		if roomy := canvas.WrapLines(nil, value, 1, 0, width, 100); true {
			squeeze := func(parts ...string) string {
				return strings.NewReplacer(" ", "", "\n", "").Replace(strings.Join(parts, ""))
			}
			if squeeze(roomy...) != squeeze(value) {
				t.Fatalf("%q at %v: wrapping changed the text: %q", value, width, roomy)
			}
		}
	}
}

// newTestRandom is a tiny deterministic generator: n in [0, bound).
func newTestRandom(seed uint32) func(bound int) int {
	state := seed
	return func(bound int) int {
		state = state*1664525 + 1013904223
		return int(state>>8) % max(bound, 1)
	}
}

// A heavier line is wider, so the same text breaks into more lines, and each
// line fits at the weight it is drawn with.
func TestWrapLinesMeasuresAtTheWeightTheLinesAreDrawnWith(t *testing.T) {
	var canvas Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * (1 + weight*2)
	})
	const text = "one two three four five six seven eight"
	regular := canvas.WrapLines(nil, text, 1, WeightRegular, 14, 10)
	heavy := canvas.WrapLines(nil, text, 1, WeightBold, 14, 10)
	if len(heavy) <= len(regular) {
		t.Fatalf("heavy text wrapped to %d lines, regular to %d: %q / %q", len(heavy), len(regular), heavy, regular)
	}
	for _, line := range heavy {
		if width := canvas.TextWidthWeight(line, 1, WeightBold); width > 14 {
			t.Errorf("heavy line %q is %.1f wide, over 14", line, width)
		}
	}
	if got := canvas.WrapLines(nil, "a long single line", 1, WeightBold, 8, 1); len(got) != 1 || canvas.TextWidthWeight(got[0], 1, WeightBold) > 8 {
		t.Errorf("ellipsized heavy line = %q", got)
	}
}

// Text from a terminal or the clipboard may hold bytes that are not UTF-8. They
// decode to U+FFFD, three bytes as a rune and one in the string, which once cut
// a slice past the end of the line.
func TestWrapLinesSurvivesInvalidUTF8(t *testing.T) {
	canvas := wrapper()
	for _, value := range []string{"  \xff\xfe  ", "\xff", "ab\xffcd\xfe\xfd", "\xe4\xbd"} {
		for _, width := range []float32{0.5, 1, 2, 5} {
			lines := canvas.WrapLines(nil, value, 1, 0, width, 6)
			if len(lines) > 6 {
				t.Errorf("%q in %v: %d lines", value, width, len(lines))
			}
		}
	}
}

// ellipsize is a bisection now; it must give what trying every shorter prefix in turn
// gives, for any width, and build one string, not one per attempt.
func TestEllipsizeAgreesWithTryingEveryPrefixAndAllocatesOnce(t *testing.T) {
	var canvas Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size / 2 })
	linear := func(line string, size, weight, width float32) string {
		for end := len(line); end > 0; {
			if candidate := trimSpaceRight(line[:end]) + "…"; canvas.TextWidthWeight(candidate, size, weight) <= width {
				return candidate
			}
			_, w := utf8.DecodeLastRuneInString(line[:end])
			end -= w
		}
		return "…"
	}
	for _, line := range []string{"a long file name with spaces.tar.gz", "日本語のとても長いファイル名.txt", "x", "trailing spaces      ", "mixed 日本語 and ascii text"} {
		for width := float32(0); width <= 200; width += 3.5 {
			if got, want := canvas.ellipsize(line, 10, 0, width), linear(line, 10, 0, width); got != want {
				t.Fatalf("ellipsize(%q, %v) = %q, trying every prefix gives %q", line, width, got, want)
			}
		}
	}
	if allocs := testing.AllocsPerRun(100, func() { _ = canvas.ellipsize("a long file name with spaces.tar.gz", 10, 0, 60) }); allocs > 1 {
		t.Errorf("ellipsize allocated %v times, want 1", allocs)
	}
}

func TestFitLineKeepsWhatFitsAndCutsTheRestWithAnEllipsis(t *testing.T) {
	canvas := wrapper()
	if got := canvas.FitLine("abcde", 10, 0, 50); got != "abcde" {
		t.Errorf("line that fits = %q", got)
	}
	if got := canvas.FitLine("abcdefgh", 10, 0, 50); got != "abcd…" {
		t.Errorf("cut line = %q, want abcd…", got)
	}
	if got := canvas.FitLine("abc", 10, 0, 0); got != "" {
		t.Errorf("line with no room = %q", got)
	}
}

func TestFitLineRemembersCutLinesAndNeverMixesThemUp(t *testing.T) {
	canvas := wrapper()
	lines := make([]string, 2000)
	for index := range lines {
		lines[index] = "line number " + strings.Repeat("x", index%7) + string(rune('a'+index%26)) + string(rune('A'+index/26%26)) + "end"
	}
	// More lines than the memo has slots: a slot is replaced, never answered for another line.
	for pass := 0; pass < 2; pass++ {
		for _, line := range lines {
			for _, width := range []float32{90, 130} {
				if got, want := canvas.FitLine(line, 10, 0, width), canvas.ellipsize(line, 10, 0, width); got != want {
					t.Fatalf("FitLine(%q, %v) = %q, want %q", line, width, got, want)
				}
			}
		}
	}
	line := "a long file name with spaces.tar.gz"
	canvas.FitLine(line, 10, 0, 100)
	if allocs := testing.AllocsPerRun(100, func() { _ = canvas.FitLine(line, 10, 0, 100) }); allocs != 0 {
		t.Errorf("a remembered line allocated %v times", allocs)
	}
	if got := canvas.FitLine(line, 10, 0, 100); got != "a long fi…" {
		t.Fatalf("cut = %q", got)
	}
	// A different measurement answers differently: nothing stale survives it.
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size / 2
	})
	if got := canvas.FitLine(line, 10, 0, 100); got != "a long file name wi…" {
		t.Fatalf("cut after the measurer changed = %q", got)
	}
}

func TestFitLineReusesOneLabelAtDifferentControlWidths(t *testing.T) {
	canvas := wrapper()
	label := strings.Repeat("a long group name ", 10)
	paint := func() {
		for _, width := range []float32{248, 282} {
			_ = canvas.FitLine(label, 14, WeightRegular, width)
		}
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("input and popup labels evict each other: %g allocations", allocations)
	}
	for _, width := range []float32{248, 282} {
		if got, want := canvas.FitLine(label, 14, WeightRegular, width), canvas.ellipsize(label, 14, WeightRegular, width); got != want {
			t.Fatalf("width %v: %q, want %q", width, got, want)
		}
	}
}
