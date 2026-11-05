package components

import (
	"reflect"
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
