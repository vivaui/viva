package textmetrics

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/grapheme"
)

// bruteOffset is what OffsetAt meant before it bisected: every boundary measured,
// the first of the nearest kept.
func bruteOffset(value string, size, weight, x float32) int {
	best, bestDistance := 0, float32(-1)
	for offset := 0; ; {
		d := absf(Caret(value, size, weight, offset) - x)
		if bestDistance < 0 || d < bestDistance {
			best, bestDistance = offset, d
		}
		if offset >= len(value) {
			return best
		}
		offset = grapheme.Next(value, offset)
	}
}

// Whatever the line and wherever the click, the bisected offset is the one measuring
// every boundary finds: letters with marks, zero-width characters, wide ideographs,
// emoji sequences and right-to-left text included.
func TestOffsetAtFindsTheNearestBoundary(t *testing.T) {
	pieces := []string{"a", "W", " ", "é", "​", "中", "👩‍💻", "א", "ب", "ẍ̣", "1"}
	random := rand.New(rand.NewPCG(7, 11))
	for round := range 2000 {
		var b strings.Builder
		for range random.IntN(16) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		value := b.String()
		width := Width(value, 14, 0)
		for range 8 {
			x := random.Float32()*(width+20) - 10
			if got, want := OffsetAt(value, 14, 0, x), bruteOffset(value, 14, 0, x); got != want {
				t.Fatalf("round %d: OffsetAt(%q, x=%v) = %d, want %d", round, value, x, got, want)
			}
		}
	}
}

// A click on a long line measures a logarithmic number of its boundaries.
func BenchmarkOffsetAt(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		value := strings.Repeat("the quick brown fox ", n/20+1)[:n]
		x := Width(value, 14, 0) * 0.6
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				OffsetAt(value, 14, 0, x)
			}
		})
	}
}
