package text

import "testing"

func TestWordEditingPreservesGraphemesAndUndo(t *testing.T) {
	for _, value := range []string{"alpha beta", "主机 地址", "e\u0301 👩‍💻"} {
		editor := NewEditor(value)
		before := editor.Caret()
		editor.MoveWord(false, true)
		if editor.Caret() >= before || editor.Selection().End != before {
			t.Fatalf("selection: %+v", editor.Selection())
		}
		editor.MoveTo(before, false)
		editor.DeleteWord(false)
		if editor.String() == value {
			t.Fatal("word was not deleted")
		}
		if !editor.Undo() || editor.String() != value || editor.Caret() != before || editor.Selection().Start != before {
			t.Fatal("undo lost the original text or caret")
		}
		if !editor.Redo() || editor.String() == value {
			t.Fatal("redo did not delete the word")
		}
	}
}

func TestDeleteLineBoundaryPreservesTheCaretAndUndo(t *testing.T) {
	editor := NewEditor("first\nsecond")
	editor.SetSelection(Range{Start: 9, End: 9})
	if !editor.DeleteLineBoundary(false) {
		t.Fatal("delete to line start did not change the editor")
	}
	if editor.String() != "first\nond" || editor.Caret() != 6 {
		t.Fatalf("line start delete = %q at %d", editor.String(), editor.Caret())
	}
	if !editor.Undo() || editor.String() != "first\nsecond" || editor.Caret() != 9 {
		t.Fatalf("undo did not restore line start delete: %q at %d", editor.String(), editor.Caret())
	}

	editor.SetSelection(Range{Start: 2, End: 2})
	if !editor.DeleteLineBoundary(true) || editor.String() != "fi\nsecond" || editor.Caret() != 2 {
		t.Fatalf("line end delete = %q at %d", editor.String(), editor.Caret())
	}
}

func TestDeleteLineBoundarySelectionStartsASeparateUndoUnit(t *testing.T) {
	editor := NewEditor("abcdef")
	editor.SetSelection(Range{Start: 2, End: 4})
	if !editor.DeleteLineBoundary(false) || editor.String() != "abef" {
		t.Fatalf("selection delete = %q", editor.String())
	}
	if !editor.DeleteBackward() || editor.String() != "aef" {
		t.Fatalf("follow-up delete = %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "abef" {
		t.Fatalf("undo merged the follow-up delete: %q", editor.String())
	}
	if !editor.Undo() || editor.String() != "abcdef" {
		t.Fatalf("undo did not restore the line command: %q", editor.String())
	}
}

func TestAddressWordBoundariesAndLineNavigation(t *testing.T) {
	editor := NewEditor("host.example.com")
	for _, want := range []int{13, 12, 5, 4, 0} {
		editor.MoveWord(false, false)
		if editor.Caret() != want {
			t.Fatalf("caret=%d want=%d", editor.Caret(), want)
		}
	}
	if got := WordRange("e\u0301 👩‍💻 host", 4); got != (Range{Start: 3, End: 6}) {
		t.Fatalf("emoji word = %+v", got)
	}
	editor = NewEditor("first\nsecond")
	editor.MoveLineBoundary(false, true)
	if editor.Selection() != (Range{Start: 6, End: 12}) || editor.Caret() != 6 {
		t.Fatalf("Home selection=%+v", editor.Selection())
	}
	editor.MoveLineBoundary(true, false)
	if editor.Caret() != 12 {
		t.Fatal("End did not move to line end")
	}
	if got := SingleLine("\r\n中文\t主机\r\n地址\u2028\x00"); got != "中文 主机 地址 " {
		t.Fatalf("single line = %q", got)
	}
}

// Typing into a single-line field folds every insert, so a clean rune must not
// allocate; only text that actually has something to fold may.
func TestSingleLineTypingDoesNotAllocate(t *testing.T) {
	if allocs := testing.AllocsPerRun(100, func() { _ = SingleLine("h") }); allocs != 0 {
		t.Fatalf("a clean insert allocates %v times", allocs)
	}
	if got := SingleLine("主机"); got != "主机" {
		t.Fatalf("clean text changed: %q", got)
	}
}

func BenchmarkSingleLineTyping(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = SingleLine("h")
	}
}
