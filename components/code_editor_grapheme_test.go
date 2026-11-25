package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestCodeEditorClickAndMoveKeepGraphemesWhole(t *testing.T) {
	const joined = "\U0001f469\u200d\U0001f4bb"
	const value = "abc\n" + joined + "x"
	f := newCodeFixture(value)
	f.paint()
	cell, _, line := f.cell()
	const start, end = len("abc\n"), len("abc\n" + joined)
	for _, check := range []struct {
		column float32
		offset int
	}{{1.75, start}, {3.25, end}} {
		point := gfx.Point{X: f.editor.content.X + check.column*cell, Y: f.editor.content.Y + 1.5*line}
		if got, ok := f.editor.OffsetAt(point); !ok || got != check.offset {
			t.Errorf("click at column %.2f returned %d, %t; want offset %d", check.column, got, ok, check.offset)
		}
	}
	if got := f.editor.Move(value, 2, TextareaMoveDown); got != start {
		t.Errorf("Down from column 2 landed at %d; want offset %d", got, start)
	}
	f.snapshot.Caret = start + len("\U0001f469")
	f.paint()
	if caret, ok := f.editor.CaretBounds(); !ok || !near(caret.X, f.editor.content.X) {
		t.Errorf("a caret inside the sequence was painted at %+v, %t; want the start of the cluster", caret, ok)
	}
}

func TestCodeEditorSoftWrapKeepsGraphemesWhole(t *testing.T) {
	const joined = "\U0001f469\u200d\U0001f4bb"
	t.Run("after a wrapped word", func(t *testing.T) {
		value := "x ab" + joined + "c"
		f := newCodeFixture(value)
		f.editor.fit(value, 4, true, 4)
		if got, want := f.rowTexts(), []string{"x ", "ab", joined, "c"}; !slices.Equal(got, want) {
			t.Errorf("rows %q; want %q", got, want)
		}
	})
	const value = "a" + joined + "b"
	f := newCodeFixture(value)
	f.snapshot.Wrap = true
	f.bounds.Width = f.widthFor(3)
	f.paint()
	if got, want := f.rowTexts(), []string{"a", joined, "b"}; !slices.Equal(got, want) {
		t.Errorf("painted rows %q; want %q", got, want)
	}
	want := []int{1, 1 + len(joined)}
	if got := f.editor.SoftWrapOffsets(nil); !slices.Equal(got, want) {
		t.Errorf("live wrap offsets %v; want %v", got, want)
	}
	f.snapshot.Wrap = false
	f.paint()
	if got := f.editor.SoftWrapOffsets(nil); !slices.Equal(got, want) {
		t.Errorf("preview wrap offsets %v; want %v", got, want)
	}
}
