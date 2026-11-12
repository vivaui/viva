package text

import (
	"testing"

	"github.com/vivaui/viva/internal/textmetrics"
)

func TestChineseCompositionLifecycle(t *testing.T) {
	editor := NewEditor("搜索")
	editor.SetMarkedText("框", Range{Start: 1, End: 1})
	if got := editor.String(); got != "搜索框" {
		t.Fatalf("marked text = %q", got)
	}
	if composition, ok := editor.Composition(); !ok || composition != (Range{Start: 2, End: 3}) {
		t.Fatalf("composition = %+v, %v", composition, ok)
	}
	if selection := editor.Selection(); selection != (Range{Start: 3, End: 3}) {
		t.Fatalf("selection = %+v", selection)
	}

	editor.SetMarkedText("输入框", Range{Start: 3, End: 3})
	if got := editor.String(); got != "搜索输入框" {
		t.Fatalf("updated composition = %q", got)
	}
	editor.Unmark()
	if _, ok := editor.Composition(); ok {
		t.Fatal("composition remained active")
	}
}

func TestInsertReplacesSelectionAndComposition(t *testing.T) {
	editor := NewEditor("hello world")
	editor.SetSelection(Range{Start: 6, End: 11})
	editor.Insert("Viva")
	if got := editor.String(); got != "hello Viva" {
		t.Fatalf("selection replacement = %q", got)
	}
	editor.SetMarkedText("框架", Range{Start: 2, End: 2})
	editor.Insert("运行时")
	if got := editor.String(); got != "hello Viva运行时" {
		t.Fatalf("composition commit = %q", got)
	}
}

func TestCursorMovesAcrossGraphemeClusters(t *testing.T) {
	editor := NewEditor("Ae\u0301👩‍💻🇨🇳中")
	wantLeft := []int{8, 6, 3, 1, 0}
	for _, want := range wantLeft {
		if !editor.MoveLeft(false) {
			t.Fatalf("MoveLeft failed before %d", want)
		}
		if got := editor.Selection().Start; got != want {
			t.Fatalf("left caret = %d, want %d", got, want)
		}
	}
	wantRight := []int{1, 3, 6, 8, 9}
	for _, want := range wantRight {
		if !editor.MoveRight(false) {
			t.Fatalf("MoveRight failed before %d", want)
		}
		if got := editor.Selection().Start; got != want {
			t.Fatalf("right caret = %d, want %d", got, want)
		}
	}
}

func TestDeleteTreatsEmojiSequenceAsOneCluster(t *testing.T) {
	editor := NewEditor("A👩‍💻B")
	editor.MoveLeft(false)
	if !editor.DeleteBackward() {
		t.Fatal("DeleteBackward failed")
	}
	if got := editor.String(); got != "AB" {
		t.Fatalf("emoji deletion = %q", got)
	}
	editor.SetSelection(Range{Start: 0, End: 0})
	if !editor.DeleteForward() || editor.String() != "B" {
		t.Fatalf("forward deletion = %q", editor.String())
	}
}

func TestComplexScriptsMoveAndDeleteAsOneCluster(t *testing.T) {
	for _, text := range []string{
		"क्षि",               // Devanagari conjunct with a vowel sign
		"\u1112\u1161\u11ab", // decomposed Hangul jamo syllable
		"👨‍👩‍👧‍👦",            // family emoji
		"🇨🇳",                 // a flag
		"\r\n",
	} {
		editor := NewEditor("a" + text + "b")
		inner := len([]rune(text))
		editor.SetSelection(Range{Start: 1, End: 1})
		editor.MoveRight(false)
		if got := editor.Selection().Start; got != 1+inner {
			t.Fatalf("%q: MoveRight to %d, want %d", text, got, 1+inner)
		}
		if !editor.DeleteBackward() || editor.String() != "ab" {
			t.Fatalf("%q: DeleteBackward left %q", text, editor.String())
		}
	}
	editor := NewEditor("क्षिb")
	editor.SetSelection(Range{})
	if !editor.DeleteForward() || editor.String() != "b" {
		t.Fatalf("DeleteForward left %q", editor.String())
	}
}

func TestExtendedSelectionPreservesAnchorDirection(t *testing.T) {
	editor := NewEditor("abc")
	editor.MoveLeft(true)
	if selection := editor.Selection(); selection != (Range{Start: 2, End: 3}) {
		t.Fatalf("first extension = %+v", selection)
	}
	editor.MoveLeft(true)
	if selection := editor.Selection(); selection != (Range{Start: 1, End: 3}) {
		t.Fatalf("second extension = %+v", selection)
	}
	editor.MoveRight(true)
	if selection := editor.Selection(); selection != (Range{Start: 2, End: 3}) {
		t.Fatalf("first contraction = %+v", selection)
	}
	editor.MoveRight(true)
	if selection := editor.Selection(); selection != (Range{Start: 3, End: 3}) {
		t.Fatalf("second contraction = %+v", selection)
	}
}

func TestUTF16IndexConversion(t *testing.T) {
	editor := NewEditor("A中😀B")
	wantUnits := []int{0, 1, 2, 4, 5}
	for runeIndex, want := range wantUnits {
		if got := editor.RuneIndexToUTF16(runeIndex); got != want {
			t.Fatalf("rune %d -> UTF-16 %d, want %d", runeIndex, got, want)
		}
	}
	for runeIndex, units := range wantUnits {
		if got := editor.UTF16ToRuneIndex(units); got != runeIndex {
			t.Fatalf("UTF-16 %d -> rune %d, want %d", units, got, runeIndex)
		}
	}
	if got := editor.UTF16ToRuneIndex(3); got != 2 {
		t.Fatalf("surrogate interior -> rune %d, want 2", got)
	}
}

func BenchmarkCursorMoveMixedText(b *testing.B) {
	editor := NewEditor("Go中文👩‍💻🇨🇳e\u0301")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		editor.SetSelection(Range{Start: editor.RuneCount(), End: editor.RuneCount()})
		for editor.MoveLeft(false) {
		}
	}
}

// A line that needs reordering walks the bidi resolver on every arrow key;
// the mixed-text benchmark above never leaves left-to-right order. The line
// copy and the resolver's own scratch are what remain once the editor keeps
// the resolver between steps.
func BenchmarkCursorMoveBidiText(b *testing.B) {
	editor := NewEditor("שלום עולם hello world עברית")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		editor.SetSelection(Range{Start: editor.RuneCount(), End: editor.RuneCount()})
		for editor.MoveLeft(false) {
		}
	}
}

func BenchmarkReplaceComposition(b *testing.B) {
	editor := NewEditor("搜索")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		editor.SetSelection(Range{Start: 2, End: editor.RuneCount()})
		editor.SetMarkedText("输入框", Range{Start: 3, End: 3})
		editor.Unmark()
	}
}

func TestStringIsCachedAndFollowsEveryMutation(t *testing.T) {
	editor := NewEditor("héllo")
	if got := editor.String(); got != "héllo" {
		t.Fatalf("initial text = %q", got)
	}
	if allocations := testing.AllocsPerRun(100, func() { _ = editor.String() }); allocations != 0 {
		t.Fatalf("repeated String allocated %v times", allocations)
	}

	// Each way text can change must be seen by the next String call.
	steps := []struct {
		name string
		do   func()
		want string
	}{
		{"insert", func() { editor.Insert(" 世界") }, "héllo 世界"},
		{"delete backward", func() { editor.DeleteBackward() }, "héllo 世"},
		{"move and delete forward", func() { editor.SetSelection(Range{Start: 0, End: 0}); editor.DeleteForward() }, "éllo 世"},
		{"replace selection", func() { editor.SetSelection(Range{Start: 0, End: 1}); editor.Insert("E") }, "Ello 世"},
		{"marked text", func() {
			editor.SetSelection(Range{Start: 0, End: 0})
			editor.SetMarkedText("ni", Range{Start: 2, End: 2})
		}, "niEllo 世"},
		{"longer marked text", func() { editor.SetMarkedText("你好", Range{Start: 2, End: 2}) }, "你好Ello 世"},
		{"unmark keeps text", func() { editor.Unmark() }, "你好Ello 世"},
		{"delete selection", func() { editor.SetSelection(Range{Start: 0, End: 2}); editor.DeleteBackward() }, "Ello 世"},
	}
	for _, step := range steps {
		step.do()
		if got := editor.String(); got != step.want {
			t.Fatalf("after %s: %q, want %q", step.name, got, step.want)
		}
		if got := editor.String(); got != step.want {
			t.Fatalf("after %s the cached text changed: %q", step.name, got)
		}
	}

	var zero Editor
	if zero.String() != "" {
		t.Fatal("zero editor is not empty")
	}
	zero.Insert("x")
	if zero.String() != "x" {
		t.Fatalf("zero editor after insert = %q", zero.String())
	}
}

func digit(r rune) bool { return r >= '0' && r <= '9' }

func TestRestrictDropsUnacceptableRunesAndCapsTheLength(t *testing.T) {
	editor := NewEditor("")
	editor.Restrict(digit, 6)
	editor.Insert("12ab3")
	if got := editor.String(); got != "123" {
		t.Fatalf("typed 12ab3, got %q", got)
	}
	editor.Insert("45678")
	if got := editor.String(); got != "123456" {
		t.Fatalf("overflowing insert left %q, want the cap", got)
	}
	editor.Insert("9")
	if got := editor.String(); got != "123456" || editor.Selection() != (Range{Start: 6, End: 6}) {
		t.Fatalf("a full editor took another digit: %q, caret %+v", got, editor.Selection())
	}
}

func TestRestrictPasteIntoTheMiddleKeepsTheCaretHonest(t *testing.T) {
	editor := NewEditor("1234")
	editor.Restrict(digit, 6)
	editor.SetSelection(Range{Start: 2, End: 2})
	editor.Insert("abc9876")
	// Two digits fit; the caret follows what was actually inserted.
	if got := editor.String(); got != "129834" || editor.Selection() != (Range{Start: 4, End: 4}) {
		t.Fatalf("got %q with caret %+v, want 129834 with caret at 4", got, editor.Selection())
	}
}

func TestRestrictReplacingASelectionFreesItsRoom(t *testing.T) {
	editor := NewEditor("123456")
	editor.Restrict(digit, 6)
	editor.SetSelection(Range{Start: 1, End: 4})
	editor.Insert("789")
	if got := editor.String(); got != "178956" {
		t.Fatalf("replacing a selection in a full editor left %q", got)
	}
}

func TestRestrictNeverBlocksDeletingOrMoving(t *testing.T) {
	editor := NewEditor("12345678") // longer than the cap: existing text is not cut
	editor.Restrict(digit, 6)
	if editor.String() != "12345678" {
		t.Fatal("Restrict must not rewrite the text already there")
	}
	if !editor.DeleteBackward() || editor.String() != "1234567" {
		t.Fatalf("deleting was blocked: %q", editor.String())
	}
	editor.Insert("9")
	if editor.String() != "1234567" {
		t.Fatalf("an editor over its cap took a rune: %q", editor.String())
	}
	if !editor.MoveLeft(false) {
		t.Fatal("moving was blocked")
	}
}

func TestRestrictRejectedCompositionLeavesNothingMarked(t *testing.T) {
	editor := NewEditor("12")
	editor.Restrict(digit, 6)
	editor.SetMarkedText("nihao", Range{Start: 5, End: 5})
	if _, composing := editor.Composition(); composing || editor.String() != "12" {
		t.Fatalf("a pinyin composition in a digit field left %q composing=%v", editor.String(), composing)
	}
	editor.SetMarkedText("34", Range{Start: 2, End: 2})
	if composition, composing := editor.Composition(); !composing || composition != (Range{Start: 2, End: 4}) || editor.String() != "1234" {
		t.Fatalf("an acceptable composition was not marked: %q %+v %v", editor.String(), composition, composing)
	}
	editor.SetMarkedText("", Range{})
	if composition, composing := editor.Composition(); !composing || composition.Start != composition.End {
		t.Fatal("clearing marked text must still be a (empty) composition for the platform to unmark")
	}
}

func TestUndoAndRedoRestoreGroupedTypingAndSelection(t *testing.T) {
	editor := NewEditor("prefix ")
	editor.Insert("你")
	editor.Insert("好")
	if !editor.CanUndo() || editor.CanRedo() {
		t.Fatalf("history after typing: undo=%v redo=%v", editor.CanUndo(), editor.CanRedo())
	}
	if !editor.Undo() || editor.String() != "prefix " || editor.Selection() != (Range{Start: 7, End: 7}) {
		t.Fatalf("undo left %q at %+v", editor.String(), editor.Selection())
	}
	if editor.CanUndo() || !editor.CanRedo() {
		t.Fatalf("history after undo: undo=%v redo=%v", editor.CanUndo(), editor.CanRedo())
	}
	if !editor.Redo() || editor.String() != "prefix 你好" || editor.Selection() != (Range{Start: 9, End: 9}) {
		t.Fatalf("redo left %q at %+v", editor.String(), editor.Selection())
	}
}

func TestMovementPasteAndReplacementAreUndoBoundaries(t *testing.T) {
	editor := NewEditor("")
	editor.Insert("ab")
	editor.MoveLeft(false)
	editor.Insert("X")
	editor.Paste("YZ")
	editor.SetSelection(Range{Start: 0, End: 2})
	editor.Insert("Q")

	for _, want := range []string{"aXYZb", "aXb", "ab", ""} {
		if !editor.Undo() || editor.String() != want {
			t.Fatalf("undo left %q, want %q", editor.String(), want)
		}
	}
	if editor.Undo() {
		t.Fatal("an empty history still undid")
	}
}

func TestConsecutiveDeletesAreOneUndoUnit(t *testing.T) {
	backward := NewEditor("abcd")
	backward.DeleteBackward()
	backward.DeleteBackward()
	if !backward.Undo() || backward.String() != "abcd" || backward.Caret() != 4 {
		t.Fatalf("backward undo left %q at %d", backward.String(), backward.Caret())
	}

	forward := NewEditor("abcd")
	forward.SetSelection(Range{})
	forward.DeleteForward()
	forward.DeleteForward()
	if !forward.Undo() || forward.String() != "abcd" || forward.Caret() != 0 {
		t.Fatalf("forward undo left %q at %d", forward.String(), forward.Caret())
	}
}

func TestCompositionIsOneUndoUnit(t *testing.T) {
	editor := NewEditor("search: ")
	editor.SetMarkedText("ni", Range{Start: 2, End: 2})
	editor.SetMarkedText("你好", Range{Start: 2, End: 2})
	editor.Insert("你好")
	if _, composing := editor.Composition(); composing {
		t.Fatal("committed composition is still marked")
	}
	if !editor.Undo() || editor.String() != "search: " || editor.Caret() != 8 {
		t.Fatalf("composition undo left %q at %d", editor.String(), editor.Caret())
	}
	if !editor.Redo() || editor.String() != "search: 你好" || editor.Caret() != 10 {
		t.Fatalf("composition redo left %q at %d", editor.String(), editor.Caret())
	}
}

func TestNewEditAfterUndoDropsRedo(t *testing.T) {
	editor := NewEditor("")
	editor.Insert("first")
	editor.BreakUndoGroup()
	editor.Insert(" second")
	editor.Undo()
	editor.Insert(" replacement")
	if editor.CanRedo() {
		t.Fatal("a new edit kept the abandoned redo branch")
	}
	if !editor.Undo() || editor.String() != "first" {
		t.Fatalf("undo of replacement left %q", editor.String())
	}
}

func TestUndoBypassesCurrentRestrictionsToRestorePriorText(t *testing.T) {
	editor := NewEditor("12ab")
	editor.Restrict(func(r rune) bool { return r >= '0' && r <= '9' }, 4)
	editor.SetSelection(Range{Start: 2, End: 4})
	editor.Insert("34")
	if !editor.Undo() || editor.String() != "12ab" {
		t.Fatalf("undo through a digit restriction left %q", editor.String())
	}
}

func BenchmarkTypingWithUndoHistory(b *testing.B) {
	values := [...]string{"h", "e", "l", "l", "o", ",", " ", "世", "界"}
	b.ReportAllocs()
	for b.Loop() {
		editor := NewEditor("")
		for _, value := range values {
			editor.Insert(value)
		}
		editor.Undo()
		editor.Redo()
	}
}

func TestRestrictNilAndZeroMeanUnrestricted(t *testing.T) {
	editor := NewEditor("")
	editor.Restrict(digit, 3)
	editor.Restrict(nil, 0)
	editor.Insert("héllo wörld 123")
	if editor.String() != "héllo wörld 123" {
		t.Fatalf("lifting the restriction left %q", editor.String())
	}
}

func TestRestrictedInsertIsAllocationFreeWhenWarm(t *testing.T) {
	editor := NewEditor("")
	editor.Restrict(digit, 6)
	editor.Insert("123456")
	allocations := testing.AllocsPerRun(100, func() {
		editor.SetSelection(Range{Start: 0, End: 6})
		editor.DeleteBackward()
		editor.Insert("1")
	})
	// Insert converts its argument to runes once, and undo history keeps its
	// own copy of the runes a delete removed: both are the design's cost.
	if allocations > 2 {
		t.Fatalf("restricted editing allocated %v times per edit", allocations)
	}
}

func TestOverwriteReplacesTheRunesAfterTheCaret(t *testing.T) {
	editor := NewEditor("482901")
	editor.SetOverwrite(true)
	editor.SetSelection(Range{Start: 1, End: 1})
	editor.Insert("7")
	if got := editor.String(); got != "472901" || editor.Selection() != (Range{Start: 2, End: 2}) {
		t.Fatalf("typing at the caret gave %q with caret %+v, want 472901 with caret at 2", got, editor.Selection())
	}
	editor.Insert("56")
	if got := editor.String(); got != "475601" || editor.Selection() != (Range{Start: 4, End: 4}) {
		t.Fatalf("a paste of two overwrote %q with caret %+v, want 475601 with caret at 4", got, editor.Selection())
	}
}

func TestOverwriteAtTheEndAppends(t *testing.T) {
	editor := NewEditor("48")
	editor.SetOverwrite(true)
	editor.Insert("29")
	if got := editor.String(); got != "4829" || editor.Selection() != (Range{Start: 4, End: 4}) {
		t.Fatalf("typing past the end gave %q with caret %+v", got, editor.Selection())
	}
}

func TestOverwriteWithAnEmptyRestrictedInsertDeletesNothing(t *testing.T) {
	editor := NewEditor("4829")
	editor.Restrict(digit, 6)
	editor.SetOverwrite(true)
	editor.SetSelection(Range{Start: 1, End: 1})
	editor.Insert("x")
	if got := editor.String(); got != "4829" || editor.Selection() != (Range{Start: 1, End: 1}) {
		t.Fatalf("a rejected letter changed the code to %q or moved the caret to %+v", got, editor.Selection())
	}
}

func TestOverwriteInAFullFieldReplacesTheLastRune(t *testing.T) {
	editor := NewEditor("482901")
	editor.Restrict(digit, 6)
	editor.SetOverwrite(true)
	editor.Insert("7")
	if got := editor.String(); got != "482907" || editor.Selection() != (Range{Start: 6, End: 6}) {
		t.Fatalf("typing into a full code gave %q with caret %+v, want 482907 with the caret still at the end", got, editor.Selection())
	}
	editor.Insert("ab")
	if got := editor.String(); got != "482907" {
		t.Fatalf("rejected input changed a full code to %q", got)
	}
}

func TestOverwriteStillReplacesASelectionAndLeavesCompositionAlone(t *testing.T) {
	editor := NewEditor("1234")
	editor.SetOverwrite(true)
	editor.SetSelection(Range{Start: 1, End: 3})
	editor.Insert("9")
	if got := editor.String(); got != "194" {
		t.Fatalf("a selection was not simply replaced: %q", got)
	}
	editor = NewEditor("1234")
	editor.SetOverwrite(true)
	editor.SetSelection(Range{Start: 1, End: 1})
	editor.SetMarkedText("ab", Range{Start: 2, End: 2})
	if got := editor.String(); got != "1ab234" {
		t.Fatalf("a marked composition overwrote text: %q", got)
	}
	editor.Insert("cd") // committing the composition replaces the marked range only
	if got := editor.String(); got != "1cd234" {
		t.Fatalf("committing a composition gave %q", got)
	}
}

func TestInsertModeIsTheDefault(t *testing.T) {
	editor := NewEditor("1234")
	editor.SetSelection(Range{Start: 1, End: 1})
	editor.Insert("9")
	if got := editor.String(); got != "19234" {
		t.Fatalf("the default editor must insert: %q", got)
	}
}

// An input method that clears its marked text (the user backspaced the whole
// preedit) leaves an empty composition. The next Backspace deletes the character
// before the caret, which must end the composition instead of leaving a range
// past the end of the text.
func TestDeleteAfterEmptiedCompositionEndsTheComposition(t *testing.T) {
	for _, test := range []struct {
		name   string
		delete func(*Editor) bool
	}{
		{"backward", (*Editor).DeleteBackward},
		{"forward", (*Editor).DeleteForward},
	} {
		editor := NewEditor("你好")
		editor.SetSelection(Range{Start: 1, End: 1})
		editor.SetMarkedText("ni", Range{Start: 2, End: 2})
		editor.SetMarkedText("", Range{})
		if composition, composing := editor.Composition(); !composing || composition.Start != composition.End {
			t.Fatalf("%s: setup left composition %+v, composing=%v", test.name, composition, composing)
		}
		if !test.delete(editor) {
			t.Fatalf("%s: nothing was deleted", test.name)
		}
		if composition, composing := editor.Composition(); composing {
			t.Errorf("%s: composition %+v survived a delete in a text of %d runes", test.name, composition, editor.RuneCount())
		}
		if editor.RuneCount() != 1 {
			t.Errorf("%s: %q left, want one rune", test.name, editor.String())
		}
	}
}

// A left-to-right line must not convert itself to a string or allocate scratch
// just to discover that it needs no bidi walk: cursor movement is per-keystroke.
func TestArrowKeysOnALeftToRightLineDoNotAllocate(t *testing.T) {
	editor := NewEditor("Go中文👩‍💻🇨🇳e\u0301")
	editor.SetSelection(Range{Start: 0, End: 0})
	if allocs := testing.AllocsPerRun(50, func() {
		editor.MoveRight(false)
		editor.MoveLeft(false)
	}); allocs != 0 {
		t.Fatalf("a cursor move on a left-to-right line allocated %v times", allocs)
	}
}

// The Left key moves the caret to the left on the screen, also through right-to-left text.
func TestArrowKeysMoveVisuallyThroughRightToLeftText(t *testing.T) {
	editor := NewEditor("אבג")
	editor.SetSelection(Range{Start: 0, End: 0}) // the right end of the line
	editor.MoveLeft(false)
	if got := editor.Selection().End; got != 1 {
		t.Errorf("Left from the start of Hebrew went to %d, want 1 (further left on the screen)", got)
	}
	editor.MoveRight(false)
	editor.MoveRight(false) // off the right end: stays
	if got := editor.Selection().End; got != 0 {
		t.Errorf("Right went to %d, want the start of the text", got)
	}
	// Mixed: "ab אב cd": the screen shows ab, a space, then the Hebrew from the right.
	editor = NewEditor("ab אב cd")
	editor.SetSelection(Range{Start: 2, End: 2})
	editor.MoveRight(false) // over the space: before the first letter of the Hebrew
	if got := editor.Selection().End; got != 3 {
		t.Errorf("Right over the space went to %d, want 3", got)
	}
	editor.MoveRight(false) // over the letter at the left end of the Hebrew, which is the last one
	if got := editor.Selection().End; got != 4 {
		t.Errorf("Right moved to %d into the Hebrew, want 4", got)
	}
	editor.MoveLeft(false)
	if got := editor.Selection().End; got != 3 {
		t.Errorf("Left back over it went to %d, want 3", got)
	}
	// Left to right text is unchanged.
	editor = NewEditor("hello")
	editor.SetSelection(Range{Start: 2, End: 2})
	editor.MoveLeft(false)
	if got := editor.Selection().End; got != 1 {
		t.Errorf("latin Left went to %d", got)
	}
}

// Every key press moves the caret to a place further along the screen in the direction of the key, as
// the caret is drawn (textmetrics.Caret), and from one end of the line to the other the keys visit
// every place.
func TestArrowKeysAreMonotonicOnTheScreenInMixedText(t *testing.T) {
	for _, value := range []string{"hello", "אבג דהו", "ab אב cd", "abc אבג 123 דהו def", "مرحبا بالعالم hello", "(שלום) world!", "a\nאב\nc"} {
		x := func(editor *Editor) float32 {
			lineStart := editor.Selection().End
			runes := []rune(editor.String())
			for lineStart > 0 && runes[lineStart-1] != '\n' {
				lineStart--
			}
			lineEnd := editor.Selection().End
			for lineEnd < len(runes) && runes[lineEnd] != '\n' {
				lineEnd++
			}
			line := string(runes[lineStart:lineEnd])
			return textmetrics.Caret(line, 16, 0, len(string(runes[lineStart:editor.Selection().End])))
		}
		for _, direction := range []int{1, -1} {
			editor := NewEditor(value)
			editor.SetSelection(Range{})
			if direction < 0 {
				editor.SetSelection(Range{Start: editor.RuneCount(), End: editor.RuneCount()})
			}
			previous := x(editor)
			for steps := 0; steps < 100; steps++ {
				var moved bool
				if direction > 0 {
					moved = editor.MoveRight(false)
				} else {
					moved = editor.MoveLeft(false)
				}
				if !moved {
					break
				}
				here := x(editor)
				if float32(direction)*(here-previous) < -0.001 {
					t.Errorf("%q: key %d moved the caret from x=%v to x=%v", value, direction, previous, here)
				}
				previous = here
			}
		}
	}
}
