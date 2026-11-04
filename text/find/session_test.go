package find

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
)

// sessionFixture is a session searching an editor whose text is value.
func sessionFixture(value, query string, options Options) (*Session, *text.Editor) {
	session := &Session{Options: options}
	session.Query.Insert(query)
	editor := text.NewEditor(value)
	editor.SetSelection(text.Range{})
	session.Update(editor.String())
	return session, editor
}

func selected(editor *text.Editor) string {
	selection := editor.Selection()
	return editor.String()[editor.ByteOffset(selection.Start):editor.ByteOffset(selection.End)]
}

func TestSessionUpdateSearchesOnlyWhenSomethingChanged(t *testing.T) {
	session, editor := sessionFixture("a b a", "a", Options{CaseSensitive: true})
	if got := len(session.Matches()); got != 2 {
		t.Fatalf("%d matches", got)
	}
	if session.Update(editor.String()) {
		t.Fatal("Update searched again for nothing")
	}
	session.Options.WholeWord = true
	if !session.Update(editor.String()) {
		t.Fatal("a changed option did not search again")
	}
	session.Query.Insert("x")
	if !session.Update(editor.String()) || len(session.Matches()) != 0 {
		t.Fatalf("a changed query kept %d matches", len(session.Matches()))
	}
	editor.Insert("ax ")
	session.Query.SetSelection(text.Range{Start: 0, End: 2})
	session.Query.Insert("ax")
	if !session.Update(editor.String()) || len(session.Matches()) != 1 {
		t.Fatalf("changed text gave %d matches", len(session.Matches()))
	}
	var allocated = testing.AllocsPerRun(100, func() { session.Update(editor.String()) })
	if allocated != 0 {
		t.Fatalf("an unchanged Update allocated %v times", allocated)
	}
}

func TestSessionStepWrapsAndCounts(t *testing.T) {
	session, editor := sessionFixture("one two one two one", "one", Options{CaseSensitive: true})
	if !session.Seek(editor) || editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("Seek selected %+v", editor.Selection())
	}
	if current, total := session.Position(editor); current != 1 || total != 3 {
		t.Fatalf("position %d of %d", current, total)
	}
	for _, want := range []int{8, 16, 0, 8} {
		session.Step(editor, true)
		if editor.Selection().Start != want {
			t.Fatalf("forward went to %d, want %d", editor.Selection().Start, want)
		}
	}
	for _, want := range []int{0, 16, 8} {
		session.Step(editor, false)
		if editor.Selection().Start != want {
			t.Fatalf("backward went to %d, want %d", editor.Selection().Start, want)
		}
	}
	editor.SetSelection(text.Range{Start: 5, End: 5})
	if current, _ := session.Position(editor); current != 0 {
		t.Fatalf("a caret between matches is on match %d", current)
	}
	session.Step(editor, true)
	if editor.Selection().Start != 8 {
		t.Fatalf("Step from between went to %d", editor.Selection().Start)
	}
	empty, editor2 := sessionFixture("abc", "zzz", Options{})
	if empty.Step(editor2, true) || empty.Seek(editor2) {
		t.Fatal("stepped to a match that is not there")
	}
	// Positions are runes in the editor, not bytes.
	wide, editor3 := sessionFixture("中文 文字 文", "文", Options{})
	wide.Step(editor3, true)
	if editor3.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("selection %+v is not rune indexed", editor3.Selection())
	}
}

func TestSessionReplaceCurrentGoesOnToTheNext(t *testing.T) {
	session, editor := sessionFixture("cat sat cat", "cat", Options{CaseSensitive: true})
	session.Replacement.Insert("dog")
	session.Seek(editor)
	if !session.ReplaceCurrent(editor) {
		t.Fatal("did not replace")
	}
	if got := editor.String(); got != "dog sat cat" || selected(editor) != "cat" || editor.Selection().Start != 8 {
		t.Fatalf("after one replacement: %q with %q selected at %d", got, selected(editor), editor.Selection().Start)
	}
	if !session.ReplaceCurrent(editor) || editor.String() != "dog sat dog" {
		t.Fatalf("second replacement gave %q", editor.String())
	}
	// With no match left, the button replaces nothing.
	if session.ReplaceCurrent(editor) || editor.String() != "dog sat dog" {
		t.Fatalf("replaced with no match: %q", editor.String())
	}
	for range 2 {
		if !editor.Undo() {
			t.Fatal("each replacement is an undo unit")
		}
	}
	if editor.String() != "cat sat cat" {
		t.Fatalf("two undos gave %q", editor.String())
	}
}

func TestSessionReplaceCurrentOutsideAMatchOnlyMoves(t *testing.T) {
	session, editor := sessionFixture("a b a", "a", Options{CaseSensitive: true})
	session.Replacement.Insert("x")
	editor.SetSelection(text.Range{Start: 2, End: 3})
	if session.ReplaceCurrent(editor) {
		t.Fatal("replaced text that is not a match")
	}
	if editor.String() != "a b a" || editor.Selection() != (text.Range{Start: 4, End: 5}) {
		t.Fatalf("%q with selection %+v", editor.String(), editor.Selection())
	}
}

func TestSessionReplaceWithGroupsAndOverlappingGrowth(t *testing.T) {
	session, editor := sessionFixture("k1=v1 k2=v2", `(\w+)=(\w+)`, Options{CaseSensitive: true, Regexp: true})
	session.Replacement.Insert("$2:$1")
	session.Seek(editor)
	session.ReplaceCurrent(editor)
	if editor.String() != "v1:k1 k2=v2" {
		t.Fatalf("groups gave %q", editor.String())
	}
	if count := session.ReplaceAll(editor); count != 1 || editor.String() != "v1:k1 v2:k2" {
		t.Fatalf("ReplaceAll gave %d, %q", count, editor.String())
	}
}

func TestSessionReplaceAllIsOneUndoUnit(t *testing.T) {
	original := "x one\nx two\nx three\nand x\n"
	session, editor := sessionFixture(original, "x", Options{CaseSensitive: true, WholeWord: true})
	session.Replacement.Insert("yy")
	editor.SetSelection(text.Range{Start: 3, End: 3})
	if count := session.ReplaceAll(editor); count != 4 {
		t.Fatalf("replaced %d", count)
	}
	if got := editor.String(); got != "yy one\nyy two\nyy three\nand yy\n" {
		t.Fatalf("ReplaceAll gave %q", got)
	}
	if len(session.Matches()) != 0 {
		t.Fatalf("%d matches remain after replacing them", len(session.Matches()))
	}
	if !editor.Undo() || editor.String() != original {
		t.Fatalf("one undo gave %q", editor.String())
	}
	if !editor.Redo() || !strings.HasPrefix(editor.String(), "yy one") {
		t.Fatal("redo did not reapply")
	}
	none, other := sessionFixture("abc", "q", Options{})
	if none.ReplaceAll(other) != 0 || other.CanUndo() {
		t.Fatal("a replace of nothing touched the text")
	}
}

func TestSessionInvalidExpressionReportsAndFindsNothing(t *testing.T) {
	session, editor := sessionFixture("a(b", "(", Options{Regexp: true})
	if session.Err() == nil || len(session.Matches()) != 0 {
		t.Fatalf("err %v, %d matches", session.Err(), len(session.Matches()))
	}
	if session.Step(editor, true) || session.ReplaceAll(editor) != 0 {
		t.Fatal("acted on a query that does not compile")
	}
	session.Options.Regexp = false
	session.Update(editor.String())
	if session.Err() != nil || len(session.Matches()) != 1 {
		t.Fatalf("the same text as plain text: err %v, %d matches", session.Err(), len(session.Matches()))
	}
}

func TestSessionReportsTruncation(t *testing.T) {
	session, _ := sessionFixture(strings.Repeat("a", DefaultLimit+5), "a", Options{CaseSensitive: true})
	if !session.Truncated() || len(session.Matches()) != DefaultLimit {
		t.Fatalf("truncated %v with %d matches", session.Truncated(), len(session.Matches()))
	}
}

func TestReplaceAllEditSpansFirstToLastMatch(t *testing.T) {
	query := mustCompile(t, "a", Options{CaseSensitive: true})
	edit, count := query.ReplaceAllEdit("..a..a..", "bb")
	if count != 2 || edit != (Edit{Start: 2, End: 6, Text: "bb..bb"}) {
		t.Fatalf("edit %+v, %d", edit, count)
	}
	if edit, count = query.ReplaceAllEdit("none", "x"); count != 0 || edit != (Edit{}) {
		t.Fatalf("no matches gave %+v, %d", edit, count)
	}
	// Applying the edit gives what ReplaceAll gives.
	value := "a-b-a-c-a"
	edit, _ = query.ReplaceAllEdit(value, "")
	all, _ := query.ReplaceAll(value, "")
	if applied := value[:edit.Start] + edit.Text + value[edit.End:]; applied != all {
		t.Fatalf("edit gives %q, ReplaceAll %q", applied, all)
	}
}
