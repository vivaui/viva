package keymap

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// The shortcut path runs on every key press that carries a modifier, so none of
// it may allocate.

func BenchmarkFromEvent(b *testing.B) {
	event := ui.KeyEvent{Modifiers: Primary | ui.ModifierShift, Text: "P"}
	b.ReportAllocs()
	for b.Loop() {
		if _, ok := FromEvent(event); !ok {
			b.Fatal("not a chord")
		}
	}
}

func BenchmarkLookup(b *testing.B) {
	table := NewTable[action](32)
	for index := 1; index < 32; index++ {
		table.Set(action(index), Chord{Modifiers: Primary, Char: byte('!' + index)})
	}
	chord := Chord{Modifiers: Primary, Char: byte('!' + 31)}
	b.ReportAllocs()
	for b.Loop() {
		if table.Lookup(chord) == 0 {
			b.Fatal("not found")
		}
	}
}

func BenchmarkKeycaps(b *testing.B) {
	chord := Chord{Modifiers: Primary | ui.ModifierShift | ui.ModifierAlt, Char: 'p'}
	var dst [6]string
	b.ReportAllocs()
	for b.Loop() {
		if len(chord.Keycaps(dst[:0])) != 4 {
			b.Fatal("wrong keycaps")
		}
	}
}

func TestShortcutPathDoesNotAllocate(t *testing.T) {
	table := newTestTable()
	event := ui.KeyEvent{Modifiers: Primary, Text: "w"}
	var dst [6]string
	allocs := testing.AllocsPerRun(100, func() {
		chord, _ := FromEvent(event)
		table.Lookup(chord)
		chord.Keycaps(dst[:0])
	})
	if allocs != 0 {
		t.Fatalf("the shortcut path allocates %v times", allocs)
	}
}
