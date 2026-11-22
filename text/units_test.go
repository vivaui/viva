package text

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// The string conversions agree with the editor's own, rune for rune: characters
// outside the Basic Multilingual Plane take two UTF-16 units, a malformed byte one
// rune and one unit, and every index past either end is clamped.
func TestUnitConversionsAgreeWithTheEditor(t *testing.T) {
	pieces := []string{"a", "é", "中", "😀", "👩‍💻", "\xff", "\n", "𝄞"}
	random := rand.New(rand.NewPCG(1, 2))
	for range 1000 {
		var b strings.Builder
		for range random.IntN(12) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		value := b.String()
		editor := NewEditor(value)
		runes := []rune(value)
		for index := -1; index <= len(runes)+1; index++ {
			if got, want := UTF16Offset(value, index), editor.RuneIndexToUTF16(index); got != want {
				t.Fatalf("UTF16Offset(%q, %d) = %d, want %d", value, index, got, want)
			}
			want := len(string(runes[:max(0, min(index, len(runes)))]))
			if got := ByteOffsetOf(value, index); got != want && !strings.ContainsRune(value, 0xfffd) && !strings.Contains(value, "\xff") {
				t.Fatalf("ByteOffsetOf(%q, %d) = %d, want %d", value, index, got, want)
			}
		}
		for units := -1; units <= 2*len(runes)+1; units++ {
			if got, want := RuneIndexAtUTF16(value, units), editor.UTF16ToRuneIndex(units); got != want {
				t.Fatalf("RuneIndexAtUTF16(%q, %d) = %d, want %d", value, units, got, want)
			}
		}
	}
}
