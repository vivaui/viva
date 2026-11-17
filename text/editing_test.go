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

func TestLineEndNavigationKeepsCRLFAtomic(t *testing.T) {
	for _, separator := range []string{"\n", "\r\n"} {
		t.Run(lineEndingName(separator), func(t *testing.T) {
			original := "ab" + separator + "cd"
			editor := NewEditor(original)
			editor.MoveTo(1, false)
			editor.MoveLineBoundary(true, false)
			if editor.Caret() != 2 {
				t.Fatalf("End caret=%d, want before %q", editor.Caret(), separator)
			}
			editor.Insert("X")
			committed := "abX" + separator + "cd"
			if editor.String() != committed || !editor.Undo() || editor.String() != original || !editor.Redo() || editor.String() != committed {
				t.Fatalf("End insertion history=%q, want %q", editor.String(), committed)
			}

			editor = NewEditor(original)
			editor.MoveTo(1, false)
			editor.MoveLineBoundary(true, true)
			if editor.Selection() != (Range{Start: 1, End: 2}) || editor.Caret() != 2 {
				t.Fatalf("Shift-End selection=%+v caret=%d", editor.Selection(), editor.Caret())
			}
		})
	}
}

func TestDeleteToLineEndKeepsCRLFAtomic(t *testing.T) {
	for _, separator := range []string{"\n", "\r\n"} {
		t.Run(lineEndingName(separator), func(t *testing.T) {
			original := "ab" + separator + "cd"
			editor := NewEditor(original)
			editor.MoveTo(1, false)
			editor.DeleteToLineBoundary(true)
			remaining := "a" + separator + "cd"
			if editor.String() != remaining || !editor.Undo() || editor.String() != original || !editor.Redo() || editor.String() != remaining {
				t.Fatalf("delete to End history=%q, want %q", editor.String(), remaining)
			}

			editor = NewEditor(original)
			editor.MoveTo(2, false)
			editor.DeleteToLineBoundary(true)
			if editor.String() != "abcd" || !editor.Undo() || editor.String() != original || !editor.Redo() || editor.String() != "abcd" {
				t.Fatalf("delete at End history=%q, want joined lines", editor.String())
			}
		})
	}
}

func TestLineNavigationFromInsideCRLFDoesNotSelectHalfTheEnding(t *testing.T) {
	const original = "ab\r\ncd"
	for _, scenario := range []struct {
		name      string
		end       bool
		extend    bool
		selection Range
		caret     int
	}{
		{"End", true, false, Range{Start: 2, End: 2}, 2},
		{"Shift-End", true, true, Range{Start: 2, End: 2}, 2},
		{"Home", false, false, Range{}, 0},
		{"Shift-Home", false, true, Range{Start: 0, End: 2}, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := NewEditor(original)
			editor.MoveTo(3, false)
			editor.MoveLineBoundary(scenario.end, scenario.extend)
			if editor.Selection() != scenario.selection || editor.Caret() != scenario.caret {
				t.Fatalf("selection=%+v caret=%d, want %+v caret=%d", editor.Selection(), editor.Caret(), scenario.selection, scenario.caret)
			}
		})
	}
	t.Run("reverse-anchor", func(t *testing.T) {
		editor := NewEditor(original)
		editor.SetSelection(Range{Start: 3, End: 1})
		editor.MoveLineBoundary(false, true)
		if editor.Selection() != (Range{Start: 0, End: 2}) || editor.Caret() != 0 {
			t.Fatalf("selection=%+v caret=%d", editor.Selection(), editor.Caret())
		}
	})
}

func TestDeleteToLineBoundaryFromInsideCRLFPreservesThePair(t *testing.T) {
	const original = "ab\r\ncd"
	for _, scenario := range []struct {
		name  string
		end   bool
		want  string
		caret int
	}{
		{"to-end", true, "abcd", 2},
		{"to-start", false, "\r\ncd", 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := NewEditor(original)
			editor.MoveTo(3, false)
			editor.DeleteToLineBoundary(scenario.end)
			if editor.String() != scenario.want || editor.Caret() != scenario.caret {
				t.Fatalf("deleted=%q caret=%d, want %q caret=%d", editor.String(), editor.Caret(), scenario.want, scenario.caret)
			}
			if !editor.Undo() || editor.String() != original || editor.Caret() != 3 {
				t.Fatalf("Undo restored %q caret=%d", editor.String(), editor.Caret())
			}
			if !editor.Redo() || editor.String() != scenario.want || editor.Caret() != scenario.caret {
				t.Fatalf("Redo restored %q caret=%d", editor.String(), editor.Caret())
			}
		})
	}
}

func TestDeleteToLineBoundaryJoinsLinesFromInsideEmptyCRLF(t *testing.T) {
	const original = "x\n\r\ncd" // the caret at 3 sits between CR and LF of the empty line
	for _, scenario := range []struct {
		name string
		end  bool
		want string
	}{
		{"to-start", false, "x\r\ncd"},
		{"to-end", true, "x\ncd"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := NewEditor(original)
			editor.MoveTo(3, false)
			editor.DeleteToLineBoundary(scenario.end)
			if editor.String() != scenario.want {
				t.Fatalf("deleted=%q, want %q", editor.String(), scenario.want)
			}
			if !editor.Undo() || editor.String() != original {
				t.Fatalf("Undo restored %q, want %q", editor.String(), original)
			}
		})
	}
}

func lineEndingName(separator string) string {
	if separator == "\r\n" {
		return "CRLF"
	}
	return "LF"
}

func BenchmarkLineEndBoundary(b *testing.B) {
	for _, separator := range []string{"\n", "\r\n"} {
		b.Run(lineEndingName(separator), func(b *testing.B) {
			editor := NewEditor("ab" + separator + "cd")
			editor.MoveTo(1, false)
			b.ReportAllocs()
			position := 0
			for b.Loop() {
				position = editor.lineBoundary(true)
			}
			if position < 0 {
				b.Fatal(position)
			}
		})
	}
}

func BenchmarkMoveLineBoundary(b *testing.B) {
	for _, separator := range []string{"\n", "\r\n"} {
		b.Run(lineEndingName(separator), func(b *testing.B) {
			editor := NewEditor("ab" + separator + "cd")
			b.ReportAllocs()
			for b.Loop() {
				editor.MoveTo(1, false)
				editor.MoveLineBoundary(true, false)
			}
			if editor.Caret() < 0 {
				b.Fatal(editor.Caret())
			}
		})
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
