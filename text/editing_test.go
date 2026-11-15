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
	if got := LineRange("first\nsecond line\nthird", 9); got != (Range{Start: 6, End: 18}) {
		t.Fatalf("line = %+v", got)
	}
	if got := LineRange("first\nsecond line\nthird", 18); got != (Range{Start: 18, End: 23}) {
		t.Fatalf("last line = %+v", got)
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

func TestDeleteToLineBoundaryStopsAtNewlinesAndUndoesAsOne(t *testing.T) {
	editor := NewEditor("first\nsecond line\nthird")
	editor.MoveTo(12, false) // "second| line"
	editor.DeleteToLineBoundary(false)
	if editor.String() != "first\n line\nthird" || editor.Caret() != 6 {
		t.Fatalf("to line start: %q caret %d", editor.String(), editor.Caret())
	}
	editor.DeleteToLineBoundary(true)
	if editor.String() != "first\n\nthird" || editor.Caret() != 6 {
		t.Fatalf("to line end: %q caret %d", editor.String(), editor.Caret())
	}
	// On the boundary it joins the neighbouring line instead of doing nothing.
	editor.DeleteToLineBoundary(false)
	if editor.String() != "first\nthird" || editor.Caret() != 5 {
		t.Fatalf("at line start: %q caret %d", editor.String(), editor.Caret())
	}
	if !editor.Undo() || editor.String() != "first\n\nthird" || !editor.Undo() || editor.String() != "first\n line\nthird" {
		t.Fatalf("undo: %q", editor.String())
	}
	// A selection is what goes, not the line.
	editor.SetSelection(Range{Start: 0, End: 2})
	editor.DeleteToLineBoundary(true)
	if editor.String() != "rst\n line\nthird" {
		t.Fatalf("selection: %q", editor.String())
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
