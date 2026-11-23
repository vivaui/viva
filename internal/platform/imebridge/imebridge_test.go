package imebridge

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// newSession is an editor holding initial, with the caret at its end, behind a session.
func newSession(initial string) (*Session, *text.Editor) {
	editor := text.NewEditor(initial)
	editor.SetSelection(text.Range{Start: editor.RuneCount(), End: editor.RuneCount()})
	session := &Session{
		Handler: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(editor)
			return platform.EditorState(editor), nil
		},
		State: platform.EditorState(editor),
	}
	return session, editor
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommitTypesAtTheCaretAndLeavesItAfter(t *testing.T) {
	session, editor := newSession("ab")
	must(t, session.Commit("cd", 1))
	if editor.String() != "abcd" || editor.Caret() != 4 {
		t.Fatalf("text %q caret %d", editor.String(), editor.Caret())
	}
}

// A keyboard that commits with a cursor before the text, or past it, moves the caret
// as commitText says.
func TestCommitPlacesTheCaretByItsNewCursorPosition(t *testing.T) {
	for _, test := range []struct{ newCursor, caret int }{{1, 7}, {0, 4}, {-1, 3}, {2, 7}, {-9, 0}, {9, 7}} {
		session, editor := newSession("abcd")
		must(t, session.Commit("xyz", test.newCursor))
		if editor.String() != "abcdxyz" || editor.Caret() != test.caret {
			t.Errorf("newCursor %d: text %q caret %d, want caret %d", test.newCursor, editor.String(), editor.Caret(), test.caret)
		}
	}
}

// Composing, as a pinyin keyboard does: the letters are marked, replaced by the
// candidate, and committed over the composition, which ends it.
func TestAComposedWordIsMarkedReplacedAndCommitted(t *testing.T) {
	session, editor := newSession("")
	must(t, session.SetComposing("ni", 1))
	if composition, composing := editor.Composition(); !composing || editor.String() != "ni" || composition != (text.Range{Start: 0, End: 2}) {
		t.Fatalf("after the first letters: %q composing=%v %v", editor.String(), composing, composition)
	}
	must(t, session.SetComposing("你", 1))
	if composition, composing := editor.Composition(); !composing || editor.String() != "你" || composition != (text.Range{Start: 0, End: 1}) {
		t.Fatalf("after the candidate: %q composing=%v %v", editor.String(), composing, composition)
	}
	must(t, session.Commit("你好", 1))
	if _, composing := editor.Composition(); composing || editor.String() != "你好" || editor.Caret() != 2 {
		t.Fatalf("after the commit: %q composing=%v caret %d", editor.String(), composing, editor.Caret())
	}
}

func TestFinishKeepsWhatIsComposed(t *testing.T) {
	session, editor := newSession("a")
	must(t, session.SetComposing("bc", 1))
	must(t, session.Finish())
	if _, composing := editor.Composition(); composing || editor.String() != "abc" {
		t.Fatalf("text %q composing=%v", editor.String(), composing)
	}
	must(t, session.Finish()) // nothing composing: nothing happens
}

// Ranges count UTF-16 units, so an emoji is two, and a range may not end in the middle
// of a character.
func TestRangesCountUTF16UnitsOverSurrogatePairs(t *testing.T) {
	session, editor := newSession("a😀b")
	mirror := session.Mirror()
	if mirror.Text != "a😀b" || mirror.SelStart != 4 || mirror.SelEnd != 4 {
		t.Fatalf("mirror %+v", mirror)
	}
	must(t, session.Select(1, 3)) // the emoji
	if selection := editor.Selection(); selection != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("selection %v", selection)
	}
	must(t, session.Replace(1, 3, "x"))
	if editor.String() != "axb" {
		t.Fatalf("text %q", editor.String())
	}
}

func TestDeleteSurroundingCountsUnitsOrCodePoints(t *testing.T) {
	session, editor := newSession("a😀b")
	must(t, session.Select(3, 3))
	must(t, session.DeleteSurrounding(2, 0, false)) // the emoji is two units
	if editor.String() != "ab" {
		t.Fatalf("by units: %q", editor.String())
	}
	session, editor = newSession("a😀b")
	must(t, session.Select(3, 3))
	must(t, session.DeleteSurrounding(1, 1, true)) // one code point either side
	if editor.String() != "a" {
		t.Fatalf("by code points: %q", editor.String())
	}
	must(t, session.DeleteSurrounding(0, 0, false)) // nothing to delete
}

func TestMarkRangeReopensAWordForCorrection(t *testing.T) {
	session, editor := newSession("hello wrld")
	must(t, session.MarkRange(6, 10))
	composition, composing := editor.Composition()
	if !composing || composition != (text.Range{Start: 6, End: 10}) {
		t.Fatalf("composition %v %v", composition, composing)
	}
	must(t, session.SetComposing("world", 1))
	must(t, session.Commit("world", 1))
	if editor.String() != "hello world" {
		t.Fatalf("text %q", editor.String())
	}
}

func TestSetMarkedSelectsInsideTheMarkedText(t *testing.T) {
	session, editor := newSession("")
	must(t, session.SetMarked("abc", 1, 1))
	if editor.Caret() != 1 {
		t.Fatalf("caret %d, want it inside the marked text", editor.Caret())
	}
}

func TestMirrorOfAComposingEditorNamesItsComposition(t *testing.T) {
	session, _ := newSession("")
	must(t, session.SetComposing("😀a", 1))
	mirror := session.Mirror()
	if !mirror.Composing || mirror.CompStart != 0 || mirror.CompEnd != 3 {
		t.Fatalf("mirror %+v", mirror)
	}
	must(t, session.Finish())
	if mirror := session.Mirror(); mirror.Composing || mirror.CompStart != -1 || mirror.CompEnd != -1 {
		t.Fatalf("mirror %+v after finishing", mirror)
	}
}

func keyInto(t *testing.T, session *Session, key gfx.Key, modifiers input.Modifiers, character string, field *platform.TextInput) bool {
	t.Helper()
	action, ok := KeyEdit(key, modifiers, character, field)
	if !ok {
		return false
	}
	must(t, action.Apply(session))
	return true
}

// A hardware keyboard types into a field and edits it as a desktop's does: characters and
// the space are typed, Backspace deletes, the arrows move, and a word's worth with Control.
func TestKeysTypeAndEditAFocusedField(t *testing.T) {
	session, editor := newSession("")
	field := &platform.TextInput{}
	for _, step := range []struct {
		key       gfx.Key
		modifiers input.Modifiers
		character string
	}{
		{gfx.KeyUnknown, 0, "h"}, {gfx.KeyUnknown, 0, "i"}, {gfx.KeySpace, 0, " "}, {gfx.KeyUnknown, 0, "x"}, {gfx.KeyBackspace, 0, ""},
	} {
		if !keyInto(t, session, step.key, step.modifiers, step.character, field) {
			t.Fatalf("key %v was not taken", step.key)
		}
	}
	if editor.String() != "hi " {
		t.Fatalf("text %q", editor.String())
	}
	keyInto(t, session, gfx.KeyLeft, 0, "", field)
	if editor.Caret() != 2 {
		t.Fatalf("caret %d after Left", editor.Caret())
	}
	keyInto(t, session, gfx.KeyLeft, input.ModifierShift, "", field)
	if selection := editor.Selection(); selection.End-selection.Start != 1 {
		t.Fatalf("selection %v after Shift+Left", selection)
	}
	keyInto(t, session, gfx.KeyBackspace, input.ModifierControl, "", field)
	if editor.String() != "h " {
		t.Fatalf("text %q after deleting the selection", editor.String())
	}
}

// Enter is a newline in a field of several lines and the application's in one of a line;
// a terminal's keys are all the application's.
func TestEnterAndTerminalKeysBelongToTheApplicationWhereTheyShould(t *testing.T) {
	if _, ok := KeyEdit(gfx.KeyEnter, 0, "", &platform.TextInput{}); ok {
		t.Error("Enter in a one-line field was taken")
	}
	if action, ok := KeyEdit(gfx.KeyEnter, 0, "", &platform.TextInput{Multiline: true}); !ok || action.Command != platform.TextInsertNewline {
		t.Errorf("Enter in a multiline field: %+v %v", action, ok)
	}
	for _, key := range []gfx.Key{gfx.KeyBackspace, gfx.KeyLeft, gfx.KeySpace, gfx.KeyUnknown} {
		if _, ok := KeyEdit(key, 0, "x", &platform.TextInput{Terminal: true}); ok {
			t.Errorf("key %v of a terminal was taken", key)
		}
	}
	if _, ok := KeyEdit(gfx.KeyUp, 0, "", &platform.TextInput{}); ok {
		t.Error("Up was taken: moving between rows is the application's")
	}
}

func TestSelectRunesMovesOnlyTheEndAndTurnsRoundWhenCrossed(t *testing.T) {
	session, editor := newSession("hello world")
	must(t, session.SelectRunes(6, 11))
	if got := session.SelectedText(); got != "world" {
		t.Fatalf("selected %q, want world", got)
	}
	// The end handle dragged back past the start: the selection is the text between.
	must(t, session.SelectRunes(6, 2))
	if got := session.SelectedText(); got != "llo " {
		t.Fatalf("after crossing: %q, want %q", got, "llo ")
	}
	if start, end := session.SelectionBounds(); start != 2 || end != 6 {
		t.Fatalf("bounds %d, %d, want 2, 6", start, end)
	}
	must(t, session.SelectRunes(0, 99))
	if got := editor.String()[:0] + session.SelectedText(); got != "hello world" {
		t.Fatalf("a drag past the end selected %q, want all of it", got)
	}
}

func TestCutTakesTheSelectionAndPasteReplacesIt(t *testing.T) {
	session, editor := newSession("one two three")
	must(t, session.SelectRunes(4, 8))
	cut, err := session.Cut()
	must(t, err)
	if cut != "two " || editor.String() != "one three" {
		t.Fatalf("cut %q left %q", cut, editor.String())
	}
	must(t, session.SelectRunes(0, 3))
	must(t, session.Paste("1", false))
	if editor.String() != "1 three" {
		t.Fatalf("paste left %q", editor.String())
	}
}

func TestPasteOnOneLineJoinsTheLines(t *testing.T) {
	session, editor := newSession("")
	must(t, session.Paste("a\nb\r\nc", false))
	if editor.String() != "a b c" {
		t.Fatalf("one-line paste: %q", editor.String())
	}
	session, editor = newSession("")
	must(t, session.Paste("a\nb", true))
	if editor.String() != "a\nb" {
		t.Fatalf("multi-line paste: %q", editor.String())
	}
}

func TestACaretCutsNothing(t *testing.T) {
	session, editor := newSession("abc")
	cut, err := session.Cut()
	must(t, err)
	if cut != "" || editor.String() != "abc" {
		t.Fatalf("cut %q left %q from a caret", cut, editor.String())
	}
}

// A keyboard that marks what it has just typed as the word it may correct, and then sends
// the next key, must not lose the first letter: a key types, it does not commit over the
// composition. This is what the emulator's keyboard does for every character typed with adb.
func TestAKeyAfterAComposingRegionTypesAfterIt(t *testing.T) {
	session, editor := newSession("")
	for _, character := range []string{"h", "e", "l", "l", "o"} {
		must(t, KeyAction{Text: character}.Apply(session))
		must(t, session.MarkRange(0, len([]rune(editor.String())))) // the keyboard reopens the word
	}
	if editor.String() != "hello" {
		t.Fatalf("typed hello with the keyboard reopening the word after each letter: %q", editor.String())
	}
}

func TestAKeyAfterAnExplicitCompositionKeepsItAndTypesAfterIt(t *testing.T) {
	session, editor := newSession("")
	must(t, session.SetComposing("ni", 1))
	must(t, KeyAction{Text: "x"}.Apply(session))
	if editor.String() != "nix" || session.State.Composing {
		t.Fatalf("key after composing text: %q composing=%v, want nix and the composition ended", editor.String(), session.State.Composing)
	}
}

// setComposingRegion changes what is composing and nothing else: the caret stays where it
// was, so a region that ends before it is not marked. A keyboard typing fast reopens a word it has
// already moved past, and a caret dragged back to it would put the next letters inside.
func TestMarkingARegionLeavesTheCaretWhereItWas(t *testing.T) {
	session, editor := newSession("hel")
	must(t, session.MarkRange(0, 2))
	if got := session.State.Selection; got != (text.Range{Start: 3, End: 3}) {
		t.Fatalf("the caret moved to %v, want it at 3", got)
	}
	must(t, KeyAction{Text: "l"}.Apply(session))
	if editor.String() != "hell" {
		t.Fatalf("typed after marking an earlier region: %q, want hell", editor.String())
	}
}

func TestMarkingARegionKeepsASelectionInsideIt(t *testing.T) {
	session, _ := newSession("hello")
	must(t, session.SelectRunes(1, 3))
	must(t, session.MarkRange(0, 5))
	if got := session.State.Selection; got != (text.Range{Start: 1, End: 3}) {
		t.Fatalf("the selection became %v, want 1..3", got)
	}
}
