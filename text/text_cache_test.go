package text

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
)

// Through any sequence of edits, the text String keeps between them is the text the
// runes spell: typing, pasting, deleting both ways and by words, replacing a
// selection, composing with an input method, undoing and redoing.
func TestStringStaysTheTextAcrossEdits(t *testing.T) {
	pieces := []string{"a", "é", "中文", "😀", "👩‍💻", "\n", " ", "x̣", "hello world"}
	random := rand.New(rand.NewPCG(9, 4))
	for round := range 300 {
		editor := NewEditor(strings.Repeat("seed text ", random.IntN(4)))
		_ = editor.String()
		for step := range 60 {
			piece := pieces[random.IntN(len(pieces))]
			length := editor.RuneCount()
			switch random.IntN(11) {
			case 0, 1:
				editor.Insert(piece)
			case 2:
				editor.Paste(piece + piece)
			case 3:
				editor.DeleteBackward()
			case 4:
				editor.DeleteForward()
			case 5:
				editor.DeleteWord(random.IntN(2) == 0)
			case 6:
				a, b := random.IntN(length+1), random.IntN(length+1)
				editor.SetSelection(Range{Start: min(a, b), End: max(a, b)})
				editor.Insert(piece)
			case 7:
				editor.MoveTo(random.IntN(length+1), false)
			case 8:
				editor.SetMarkedText(piece, Range{Start: 0, End: len([]rune(piece))})
				if random.IntN(2) == 0 {
					editor.Unmark()
				} else {
					editor.CancelComposition()
				}
			case 9:
				editor.Undo()
			case 10:
				editor.Redo()
			}
			if random.IntN(3) == 0 {
				continue // edits between two Strings still keep the cache right
			}
			if got, want := editor.String(), editor.runes.String(); got != want {
				t.Fatalf("round %d step %d: String() = %q, the runes spell %q", round, step, got, want)
			}
		}
	}
}

// Typing in a large document costs the edit and a copy of its bytes, not encoding
// every rune again.
func BenchmarkTypeInLargeDocument(b *testing.B) {
	for _, size := range []int{1 << 14, 1 << 20} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			editor := NewEditor(strings.Repeat("abcdefghij", size/10))
			editor.MoveTo(editor.RuneCount()/2, false)
			_ = editor.String()
			b.ReportAllocs()
			for b.Loop() {
				editor.Insert("x")
				_ = editor.String()
				editor.DeleteBackward()
				_ = editor.String()
			}
		})
	}
}
