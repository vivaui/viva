package platform

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextLine(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		index int
		line  string
		first int
	}{
		{"empty", "", 0, "", 0},
		{"one line", "hello", 3, "hello", 0},
		{"caret in the first line", "ab\ncd\nef", 1, "ab", 0},
		{"caret before a break", "ab\ncd\nef", 2, "ab", 0},
		{"caret after a break", "ab\ncd\nef", 3, "cd", 3},
		{"caret at the end of a middle line", "ab\ncd\nef", 5, "cd", 3},
		{"caret in the last line", "ab\ncd\nef", 7, "ef", 6},
		{"caret past the end", "ab\ncd\nef", 99, "ef", 6},
		{"caret below the start", "ab\ncd", -4, "ab", 0},
		{"trailing break", "ab\n", 3, "", 3},
		{"empty line", "ab\n\ncd", 3, "", 3},
		{"carriage return and line feed", "ab\r\ncd", 4, "cd", 4},
		{"every kind of break", "a\vb\fc\u0085d\u2028e\u2029f", 8, "e", 8},
		{"wide characters count as one rune", "中文\n日本語", 5, "日本語", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			start, end, first := TextLine(test.value, test.index)
			if got := test.value[start:end]; got != test.line || first != test.first {
				t.Fatalf("TextLine(%q, %d) = %q from rune %d, want %q from rune %d", test.value, test.index, got, first, test.line, test.first)
			}
		})
	}
}

// TestTextLineIsOneLineOfTheValue holds for any text and caret: the line has no
// break in it, sits on rune boundaries, starts at the rune it says it does, and
// is bounded by breaks (or the ends of the value) on both sides.
func TestTextLineIsOneLineOfTheValue(t *testing.T) {
	alphabet := []rune("ab中\n\r\v\u2028\u0085😀")
	random := rand.New(rand.NewSource(1))
	for round := 0; round < 2000; round++ {
		runes := make([]rune, random.Intn(40))
		for index := range runes {
			runes[index] = alphabet[random.Intn(len(alphabet))]
		}
		value := string(runes)
		for index := -1; index <= len(runes)+1; index++ {
			start, end, first := TextLine(value, index)
			line := value[start:end]
			if start > end || strings.IndexFunc(line, isLineBreak) >= 0 {
				t.Fatalf("TextLine(%q, %d) = [%d, %d): %q holds a break", value, index, start, end, line)
			}
			if want := utf8.RuneCountInString(value[:start]); first != want {
				t.Fatalf("TextLine(%q, %d): first = %d, want %d", value, index, first, want)
			}
			if start > 0 {
				if last, _ := utf8.DecodeLastRuneInString(value[:start]); !isLineBreak(last) {
					t.Fatalf("TextLine(%q, %d): the line does not follow a break", value, index)
				}
			}
			if end < len(value) {
				if next, _ := utf8.DecodeRuneInString(value[end:]); !isLineBreak(next) {
					t.Fatalf("TextLine(%q, %d): the line is not ended by a break", value, index)
				}
			}
			clamped := min(max(index, 0), len(runes))
			if clamped < first || clamped > first+utf8.RuneCountInString(line) {
				t.Fatalf("TextLine(%q, %d): the caret is not on the line %q from rune %d", value, index, line, first)
			}
		}
	}
}

// BenchmarkTextLine is the cost of finding the caret's line in a source file of
// 27,000 lines, the size of the file that stopped the Windows editor, with the
// caret at its end: the worst case, a scan of all of it.
func BenchmarkTextLine(b *testing.B) {
	value := strings.Repeat("    return handler(request, response) // 中文注释\n", 27000)
	caret := utf8.RuneCountInString(value)
	b.SetBytes(int64(len(value)))
	b.ReportAllocs()
	for b.Loop() {
		TextLine(value, caret)
	}
}
