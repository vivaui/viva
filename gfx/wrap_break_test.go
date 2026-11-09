package gfx

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// clusterWrapper measures every character (grapheme cluster) as `size` wide, so a
// width is a count of characters even when one is several runes.
func clusterWrapper() *Canvas {
	canvas := &Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		count := 0
		for index := 0; index < len(value); index = nextCluster(value, index) {
			count++
		}
		return float32(count) * size
	})
	return canvas
}

func firstRune(line string) rune { r, _ := utf8.DecodeRuneInString(line); return r }
func lastRune(line string) rune  { r, _ := utf8.DecodeLastRuneInString(line); return r }

func TestWrapLinesKeepsClosingPunctuationAndSmallKanaOffTheStartOfAJapaneseOrChineseLine(t *testing.T) {
	canvas := clusterWrapper()
	// Below the length of a run that may not be broken (ちょっと, 。ぁぃ, に）。) a line
	// has to start with something that should not, so the narrowest width is that run.
	for _, test := range []struct {
		text      string
		narrowest float32
	}{
		{"今天的天气非常好，我们一起去公园散步。然后回家吃饭，睡觉！好不好？", 2},
		{"コーヒーを飲みました。ちょっとだけ、ゆっくりと。ぁぃ", 4},
		{"彼は「はい」と言った（本当に）。【注意】《书名》“引用”", 5},
	} {
		text := test.text
		for width := test.narrowest; width <= 9; width++ {
			lines := canvas.WrapLines(nil, text, 1, 0, width, 50)
			if strings.Join(lines, "") != text {
				t.Fatalf("%q at %v lost characters: %q", text, width, lines)
			}
			for index, line := range lines {
				if utf8.RuneCountInString(line) > int(width) {
					t.Errorf("%q at %v: line %q is too wide", text, width, line)
				}
				if index > 0 && (closesLine(firstRune(line)) || nonStarter(firstRune(line))) {
					t.Errorf("%q at %v: line %q starts with %q, which may not start a line", text, width, line, firstRune(line))
				}
				if index < len(lines)-1 && opensLine(lastRune(line)) {
					t.Errorf("%q at %v: line %q ends with an opening bracket", text, width, line)
				}
			}
		}
	}
}

func TestWrapLinesBreaksALongPathAfterItsSeparators(t *testing.T) {
	canvas := clusterWrapper()
	path := "/usr/local/share/applications/firefox.desktop"
	lines := canvas.WrapLines(nil, path, 1, 0, 14, 10)
	if strings.Join(lines, "") != path {
		t.Fatalf("lost characters: %q", lines)
	}
	for index, line := range lines[:len(lines)-1] {
		if last := lastRune(line); last != '/' && last != '.' {
			t.Errorf("line %d %q was cut inside a name although a separator fit", index, line)
		}
	}
	// UAX #14 (LB13) does not break before a slash, even after a space: the path starts on
	// the line of the word before it.
	lines = canvas.WrapLines(nil, "see "+path, 1, 0, 14, 10)
	if lines[0] != "see /usr/" {
		t.Errorf("the path wrapped as %q", lines)
	}
	url := "https://example.com/docs/guide?page=2&sort=name"
	lines = canvas.WrapLines(nil, url, 1, 0, 16, 10)
	// A slash allows a break after it before a letter (LB31), so the scheme goes first and
	// the host after it.
	if strings.Join(lines, "") != url || lines[0] != "https://" || lines[1] != "example.com/" {
		t.Errorf("url wrapped as %q", lines)
	}
	// Versions and addresses are not cut at their dots when a better place exists.
	for _, line := range canvas.WrapLines(nil, "10.255.255.1/24", 1, 0, 9, 5) {
		if line == "." {
			t.Errorf("a lone dot got a line")
		}
	}
	// With no separator at all, the word is cut wherever it no longer fits.
	if got := canvas.WrapLines(nil, "0123456789abcdef", 1, 0, 6, 5); strings.Join(got, "|") != "012345|6789ab|cdef" {
		t.Errorf("hash wrapped as %q", got)
	}
	// A leading or doubled separator does not leave a line of its own.
	if got := canvas.WrapLines(nil, "--verbose-output-file", 1, 0, 9, 5); strings.Join(got, "|") != "--verbose|-output-|file" {
		t.Errorf("flag wrapped as %q", got)
	}
}

func TestWrapLinesNeverSplitsACharacter(t *testing.T) {
	canvas := clusterWrapper()
	for _, text := range []string{
		"éééééé", // e + combining acute
		"👨‍👩‍👧‍👦👨‍👩‍👧‍👦👨‍👩‍👧‍👦", // family: ZWJ sequences
		"🇯🇵🇺🇸🇫🇷🇩🇪🇨🇳",            // flags: pairs of regional indicators
		"❤️❤️❤️👍🏽👍🏽👍🏽",          // variation selector, skin tone
		"1⃣2⃣3⃣", // keycaps
		"กิขิคิ", // Thai with vowel marks
		"漢゙字゙",   // CJK with a combining voiced mark
		"abc 👨‍👩‍👧 déf 🇯🇵 ok", // mixed with spaces
	} {
		for width := float32(1); width <= 4; width++ {
			lines := canvas.WrapLines(nil, text, 1, 0, width, 100)
			if strings.Join(lines, "") != strings.ReplaceAll(text, " ", "") && strings.Join(lines, " ") != text {
				// Spaces at breaks are dropped: compare without them.
				if strings.ReplaceAll(strings.Join(lines, ""), " ", "") != strings.ReplaceAll(text, " ", "") {
					t.Fatalf("%q at %v lost characters: %q", text, width, lines)
				}
			}
			for _, line := range lines {
				if r := firstRune(line); unicode.Is(unicode.M, r) || r == 0x200D || r >= 0x1F3FB && r <= 0x1F3FF || r == 0xFE0F {
					t.Errorf("%q at %v: line %q starts with %U, which belongs to the character before", text, width, line, r)
				}
				if lastRune(line) == 0x200D {
					t.Errorf("%q at %v: line %q ends on a joiner", text, width, line)
				}
				if got := int(canvas.TextWidthWeight(line, 1, 0)); got > int(width) && got > 1 {
					t.Errorf("%q at %v: line %q is %d characters wide", text, width, line, got)
				}
			}
		}
	}
	// Two flags: one per line at width 1, both at width 2.
	if got := canvas.WrapLines(nil, "🇯🇵🇺🇸", 1, 0, 1, 5); len(got) != 2 || got[0] != "🇯🇵" {
		t.Errorf("flags at width 1: %q", got)
	}
	if got := canvas.WrapLines(nil, "🇯🇵🇺🇸", 1, 0, 2, 5); len(got) != 1 {
		t.Errorf("flags at width 2: %q", got)
	}
}

func TestEllipsisNeverLeavesHalfACharacter(t *testing.T) {
	canvas := clusterWrapper()
	for _, text := range []string{"a👨‍👩‍👧‍👦b👨‍👩‍👧‍👦c", "ééééé", "🇯🇵🇺🇸🇫🇷🇩🇪🇨🇳"} {
		for width := float32(2); width <= 4; width++ {
			lines := canvas.WrapLines(nil, text, 1, 0, width, 1)
			if len(lines) != 1 || !strings.HasSuffix(lines[0], "…") {
				t.Fatalf("%q at %v: %q", text, width, lines)
			}
			body := strings.TrimSuffix(lines[0], "…")
			if !strings.HasPrefix(text, body) {
				t.Errorf("%q at %v: %q is not a prefix of the text", text, width, body)
			}
			if body != "" && !(len(body) == len(text) || clusterBoundary(text, len(body))) {
				t.Errorf("%q at %v: %q ends inside a character", text, width, body)
			}
		}
	}
}

func TestWrapLinesFitsExactlyAtTheBoundary(t *testing.T) {
	canvas := wrapper()
	for _, test := range []struct {
		text  string
		width float32
		want  string
	}{
		{"abcde fghij", 5, "abcde|fghij"},
		{"abcde fghij", 11, "abcde fghij"},
		{"abcde fghij", 10.99, "abcde|fghij"},
		{"一二三四五六", 3, "一二三|四五六"},
		{"一二三四五六", 6, "一二三四五六"},
		{"一二三四，五", 4, "一二三四，|五"}, // the comma hangs at the end rather than start the next line
		{"一二三四五，六", 4, "一二三四|五，六"},
	} {
		got := strings.Join(canvas.WrapLines(nil, test.text, 1, 0, test.width, 5), "|")
		// The comma case: a comma may not start a line, so the char before it comes along.
		if got != test.want && test.text != "一二三四，五" {
			t.Errorf("%q in %v: %q, want %q", test.text, test.width, got, test.want)
		}
	}
	// A comma that would be the fifth character in four cells moves the fourth with it.
	if got := strings.Join(canvas.WrapLines(nil, "一二三四，五", 1, 0, 4, 5), "|"); got != "一二三|四，五" {
		t.Errorf("closing punctuation: %q", got)
	}
}

func TestWrappingStaysFreeOfAllocationsWithTheEllipsisCached(t *testing.T) {
	canvas := clusterWrapper()
	scratch := make([]string, 0, 8)
	var cache EllipsisCache
	text := "一段很长的中文，still some english and /a/very/long/path/to/somewhere/else.txt 。然后还有更多的字"
	run := func() { scratch = canvas.WrapLinesCached(scratch[:0], text, 1, 0, 14, 3, &cache) }
	run()
	if !strings.HasSuffix(scratch[len(scratch)-1], "…") {
		t.Fatalf("expected an ellipsized end: %q", scratch)
	}
	if allocations := testing.AllocsPerRun(100, run); allocations != 0 {
		t.Fatalf("wrapping with a warm ellipsis cache allocated %v times", allocations)
	}
}

func BenchmarkWrapLinesCJKAndPath(b *testing.B) {
	canvas := clusterWrapper()
	scratch := make([]string, 0, 16)
	var cache EllipsisCache
	text := strings.Repeat("连接服务器失败，请检查网络。/var/log/system/very-long-name.log ", 6)
	b.ReportAllocs()
	for range b.N {
		scratch = canvas.WrapLinesCached(scratch[:0], text, 1, 0, 30, 8, &cache)
	}
}

// opensLine reports whether r is an opening bracket or quote, which may not end a line.
func opensLine(r rune) bool {
	switch r {
	case '（', '「', '『', '《', '【', '〈', '〔', '〖', '〘', '〚', '［', '｛', '“', '‘', '〝', '(', '[', '{', '«', '‹':
		return true
	}
	return false
}

// WrapRows is the higher-level entry point applications use; it must share the
// wrap cost of WrapLines rather than add its own.
func BenchmarkWrapRowsCJKAndPath(b *testing.B) {
	canvas := clusterWrapper()
	rows := make([]TextRow, 0, 64)
	text := strings.Repeat("连接服务器失败，请检查网络。/var/log/system/very-long-name.log ", 6)
	b.ReportAllocs()
	for range b.N {
		rows = canvas.WrapRows(rows[:0], text, 1, 0, 30)
	}
	_ = rows
}
