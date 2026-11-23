package text

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/grapheme"
)

// TestRuneStoreAgreesWithRunes edits a store and a []rune at random, across leaf
// boundaries and with text that is not UTF-8, and checks every way the editor
// reads the store against the runes.
func TestRuneStoreAgreesWithRunes(t *testing.T) {
	alphabet := []rune{'a', 'b', ' ', '\n', '\r', 'é', '中', '😀', 0x301, 0x200d, 0x1f3fd, 0xd800, -1, utf8.MaxRune + 1}
	random := rand.New(rand.NewSource(1))
	piece := func(limit int) []rune {
		runes := make([]rune, random.Intn(limit))
		for index := range runes {
			runes[index] = alphabet[random.Intn(len(alphabet))]
		}
		return runes
	}
	valid := func(runes []rune) []rune {
		out := make([]rune, len(runes))
		for index, r := range runes {
			if !utf8.ValidRune(r) {
				r = utf8.RuneError
			}
			out[index] = r
		}
		return out
	}
	var store runeStore
	var want []rune
	for step := 0; step < 1500; step++ {
		start := random.Intn(len(want) + 1)
		end := start + random.Intn(len(want)-start+1)
		if random.Intn(4) != 0 {
			end = min(start+random.Intn(8), len(want))
		}
		limit := 16
		if random.Intn(20) == 0 {
			limit = 6 * leafBytes
		}
		replacement := piece(limit)
		store.Replace(start, end, replacement)
		want = slices.Replace(want, start, end, valid(replacement)...)

		if store.Len() != len(want) || store.String() != string(want) || store.Bytes() != len(string(want)) {
			t.Fatalf("step %d: store holds %d runes %q, want %d", step, store.Len(), store.String(), len(want))
		}
		if store.Newlines() != countLabelLF(want) {
			t.Fatalf("step %d: %d newlines, want %d", step, store.Newlines(), countLabelLF(want))
		}
		for probe := 0; probe < 8; probe++ {
			index := random.Intn(len(want) + 1)
			bytes := len(string(want[:index]))
			units := len(utf16.Encode(want[:index]))
			if store.ByteOffset(index) != bytes || store.RuneIndex(bytes) != index || store.UTF16Offset(index) != units {
				t.Fatalf("step %d: rune %d at byte %d unit %d, want byte %d unit %d", step, index, store.ByteOffset(index), store.UTF16Offset(index), bytes, units)
			}
			if got, exact := store.RuneAtUTF16(units); got != index || !exact {
				t.Fatalf("step %d: unit %d is rune %d, want %d", step, units, got, index)
			}
			if index < len(want) {
				if store.At(index) != want[index] {
					t.Fatalf("step %d: rune %d is %q, want %q", step, index, store.At(index), want[index])
				}
				if utf8.RuneLen(want[index]) > 1 && store.RuneIndex(bytes+1) != index {
					t.Fatalf("step %d: an offset inside rune %d gives %d", step, index, store.RuneIndex(bytes+1))
				}
				if want[index] > 0xffff {
					if got, exact := store.RuneAtUTF16(units + 1); got != index || exact {
						t.Fatalf("step %d: inside a surrogate pair gives %d %v", step, got, exact)
					}
				}
			}
			if next := grapheme.NextRunes(want, index); store.Next(index) != next {
				t.Fatalf("step %d: cluster after %d ends at %d, want %d", step, index, store.Next(index), next)
			}
			if previous := grapheme.PrevRunes(want, index); store.Prev(index) != previous {
				t.Fatalf("step %d: cluster before %d starts at %d, want %d", step, index, store.Prev(index), previous)
			}
			lineStart, lineEnd := index, index
			for lineStart > 0 && want[lineStart-1] != '\n' {
				lineStart--
			}
			for lineEnd < len(want) && want[lineEnd] != '\n' {
				lineEnd++
			}
			if store.LineStart(index) != lineStart || store.LineEnd(index) != lineEnd || store.Line(index) != countLabelLF(want[:index]) {
				t.Fatalf("step %d: rune %d on line %d [%d, %d), want %d [%d, %d)", step, index, store.Line(index), store.LineStart(index), store.LineEnd(index), countLabelLF(want[:index]), lineStart, lineEnd)
			}
			to := index + random.Intn(len(want)-index+1)
			if !slices.Equal(store.Slice(index, to), want[index:to]) {
				t.Fatalf("step %d: runes [%d, %d) differ", step, index, to)
			}
			var builder strings.Builder
			if store.WriteBytes(&builder, bytes, len(string(want[:to]))); builder.String() != string(want[index:to]) {
				t.Fatalf("step %d: bytes of runes [%d, %d) differ", step, index, to)
			}
		}
		if len(want) > 8*leafBytes && random.Intn(3) == 0 {
			// Keep the text from growing without bound while it crosses leaves.
			store.Replace(0, len(want)/2, nil)
			want = want[len(want)/2:]
		}
	}
}

// TestRuneStoreRollbackAgreesWithRunes checks the rolled-back reading the input
// method planner uses against the runes it stands for.
func TestRuneStoreRollbackAgreesWithRunes(t *testing.T) {
	store, _ := newRuneStore("ab中😀cd\nef😀")
	removed := []rune("😀x")
	var text rolledBackText
	text.store = &store
	text.add(rolledBackPiece{start: 0, length: 3})
	text.add(rolledBackPiece{removed: removed, history: true, length: len(removed)})
	text.add(rolledBackPiece{start: 4, length: store.Len() - 4})
	want := append(append([]rune("ab中"), removed...), []rune("cd\nef😀")...)
	if text.Len() != len(want) || runeTextString(&text) != string(want) {
		t.Fatalf("rolled back to %q, want %q", runeTextString(&text), string(want))
	}
	for index := 0; index <= len(want); index++ {
		units := len(utf16.Encode(want[:index]))
		if index < len(want) && text.At(index) != want[index] {
			t.Fatalf("rune %d is %q, want %q", index, text.At(index), want[index])
		}
		if text.UTF16Offset(index) != units {
			t.Fatalf("rune %d at unit %d, want %d", index, text.UTF16Offset(index), units)
		}
		if got, exact := text.RuneAtUTF16(units); got != index || !exact {
			t.Fatalf("unit %d is rune %d, want %d", units, got, index)
		}
	}
}
