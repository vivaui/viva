package text

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// The []rune forms WordRange and LineRange had: what the string forms must find.
func runeWordRange(value string, index int) Range {
	runes := []rune(value)
	if len(runes) == 0 {
		return Range{}
	}
	index = max(0, min(index, len(runes)-1))
	start := 0
	for next := nextBoundary(runes, start); next <= index; next = nextBoundary(runes, start) {
		start = next
	}
	kind := wordClass(runes[start])
	end := nextBoundary(runes, start)
	for start > 0 {
		previous := previousBoundary(runes, start)
		if wordClass(runes[previous]) != kind {
			break
		}
		start = previous
	}
	for end < len(runes) && wordClass(runes[end]) == kind {
		end = nextBoundary(runes, end)
	}
	return Range{Start: start, End: end}
}

func runeLineRange(value string, index int) Range {
	runes := []rune(value)
	if len(runes) == 0 {
		return Range{}
	}
	index = max(0, min(index, len(runes)-1))
	start := index
	for start > 0 && runes[start-1] != '\n' {
		start--
	}
	end := index
	for end < len(runes) && runes[end] != '\n' {
		end++
	}
	if end < len(runes) {
		end++
	}
	return Range{Start: start, End: end}
}

// Whatever the text and the index, reading the string in place finds the word and
// the line the rune form found: marks, emoji sequences, flags, CJK and newlines.
func TestWordAndLineRangeReadTheStringInPlace(t *testing.T) {
	pieces := []string{"a", "_", "7", " ", "\n", ".", "/", "é", "中文", "👩‍💻", "🇯🇵", "🇯", "x̣", "\t", "-"}
	random := rand.New(rand.NewPCG(3, 5))
	for round := range 3000 {
		var b strings.Builder
		for range random.IntN(14) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		value := b.String()
		for index := -1; index <= len([]rune(value))+1; index++ {
			if got, want := WordRange(value, index), runeWordRange(value, index); got != want {
				t.Fatalf("round %d: WordRange(%q, %d) = %v, want %v", round, value, index, got, want)
			}
			if got, want := LineRange(value, index), runeLineRange(value, index); got != want {
				t.Fatalf("round %d: LineRange(%q, %d) = %v, want %v", round, value, index, got, want)
			}
		}
	}
}

// A drag by words through a long text costs the word, not a copy of the text.
func BenchmarkWordRange(b *testing.B) {
	value := strings.Repeat("connect to host-01.example.com over ssh\n", 25000) // about 1 MB
	index := len([]rune(value)) / 2
	b.ReportAllocs()
	for b.Loop() {
		WordRange(value, index)
	}
}
