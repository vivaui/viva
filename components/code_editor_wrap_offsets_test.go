package components

import (
	"reflect"
	"strings"
	"testing"
)

func TestCodeEditorSoftWrapOffsetsUsePaintedWidthWithoutChangingLiveLayout(t *testing.T) {
	fixture := newCodeFixture("alpha beta gamma\nnext")
	fixture.bounds.Width = fixture.widthFor(8)
	fixture.snapshot.Wrap = false
	fixture.paint()
	beforeRows, beforeScroll, beforeCaret := len(fixture.editor.rows), fixture.editor.scrollX, fixture.editor.caretRect
	got := fixture.editor.SoftWrapOffsets(nil)
	if !reflect.DeepEqual(got, []int{6, 11}) {
		t.Fatalf("soft wrap offsets = %v", got)
	}
	if len(fixture.editor.rows) != beforeRows || fixture.editor.wrap || fixture.editor.scrollX != beforeScroll || fixture.editor.caretRect != beforeCaret {
		t.Fatal("soft-wrap query changed the painted editor")
	}
	fixture.bounds.Width = fixture.widthFor(25)
	fixture.paint()
	if widened := fixture.editor.SoftWrapOffsets(nil); len(widened) != 0 {
		t.Fatalf("resized editor kept stale wrap offsets %v", widened)
	}
}

func TestCodeEditorSoftWrapOffsetsMatchRenderedUnicodeTabsAndReuseBuffer(t *testing.T) {
	fixture := newCodeFixture("a\t漢字 alpha beta\nmore words")
	fixture.bounds.Width = fixture.widthFor(6)
	fixture.snapshot.TabWidth = 4
	fixture.paint()
	buffer := make([]int, 0, 12)
	got := fixture.editor.SoftWrapOffsets(buffer)
	fixture.snapshot.Wrap = true
	fixture.paint()
	var rendered []int
	for index := 1; index < len(fixture.editor.rows); index++ {
		if fixture.editor.rows[index].line == fixture.editor.rows[index-1].line {
			rendered = append(rendered, int(fixture.editor.rows[index].start))
		}
	}
	if len(got) == 0 || !reflect.DeepEqual(got, rendered) || cap(got) < cap(buffer) {
		t.Fatalf("queried breaks %v differ from rendered breaks %v", got, rendered)
	}
	if again := fixture.editor.SoftWrapOffsets(got[:0]); !reflect.DeepEqual(again, rendered) {
		t.Fatalf("already-wrapped offsets = %v, want %v", again, rendered)
	}
}

func TestCodeEditorSoftWrapOffsetsRequirePaintedLayout(t *testing.T) {
	var editor CodeEditor
	if got := editor.SoftWrapOffsets(nil); len(got) != 0 {
		t.Fatalf("unpainted editor returned breaks %v", got)
	}
}

func TestCodeEditorSoftWrapOffsetsReuseOneWrappedLayout(t *testing.T) {
	fixture := newCodeFixture("alpha beta gamma delta epsilon zeta")
	fixture.bounds.Width = fixture.widthFor(8)
	fixture.snapshot.Wrap = false
	fixture.paint()
	dst := make([]int, 0, 16)
	want := fixture.editor.SoftWrapOffsets(dst)
	if len(want) == 0 {
		t.Fatal("fixture did not soft-wrap")
	}
	// A caller that polls the offsets must not lay the text out again per call.
	if allocations := testing.AllocsPerRun(50, func() { _ = fixture.editor.SoftWrapOffsets(dst) }); allocations != 0 {
		t.Fatalf("repeated soft-wrap query allocated %g times", allocations)
	}
	if got := fixture.editor.SoftWrapOffsets(dst); !reflect.DeepEqual(got, want) {
		t.Fatalf("cached breaks %v, want %v", got, want)
	}
}

func TestCodeEditorSoftWrapOffsetsSkipFoldedLines(t *testing.T) {
	value := "header alpha beta gamma delta\nhidden one two three four\nlast"
	fixture := newCodeFixture(value)
	fixture.bounds.Width = fixture.widthFor(10)
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 1}}
	fixture.paint()
	got := fixture.editor.SoftWrapOffsets(nil)
	if len(got) == 0 {
		t.Fatal("the header line should still report its own soft wraps")
	}
	hiddenStart := strings.Index(value, "hidden")
	hiddenEnd := hiddenStart + len("hidden one two three four")
	for _, offset := range got {
		if offset >= hiddenStart && offset <= hiddenEnd {
			t.Fatalf("folded editor reported a break at %d, inside a hidden line", offset)
		}
	}
}
