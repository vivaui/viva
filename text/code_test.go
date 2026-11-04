package text

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// fixture builds an editor from text marked with its selection: ‸ is a caret,
// and « » surround a selection that the caret ends at ».
func fixture(marked string) *Editor {
	var value []rune
	anchor, caret := -1, -1
	for _, r := range marked {
		switch r {
		case '‸':
			anchor, caret = len(value), len(value)
		case '«':
			anchor = len(value)
		case '»':
			caret = len(value)
		default:
			value = append(value, r)
		}
	}
	editor := NewEditor(string(value))
	if anchor >= 0 {
		editor.SetSelection(Range{Start: anchor, End: caret})
	}
	return editor
}

// render is fixture's inverse.
func render(editor *Editor) string {
	runes := []rune(editor.String())
	selection := editor.Selection()
	var out []rune
	for index := 0; index <= len(runes); index++ {
		if selection.Start == selection.End && index == selection.Start {
			out = append(out, '‸')
		} else if index == selection.Start {
			out = append(out, '«')
		} else if index == selection.End {
			out = append(out, '»')
		}
		if index < len(runes) {
			out = append(out, runes[index])
		}
	}
	return string(out)
}

var codeStyle = CodeOptions{TabWidth: 4, AutoIndent: true, AutoClose: true, IndentAfter: "{[(", LineComment: "//"}

// checkOperation applies operation to the fixture, expects want, and expects one
// Undo to bring back the starting text and selection.
func checkOperation(t *testing.T, name, marked, want string, operation func(*Editor)) {
	t.Helper()
	editor := fixture(marked)
	operation(editor)
	if got := render(editor); got != want {
		t.Errorf("%s: %q became %q, want %q", name, marked, got, want)
		return
	}
	if err := checkEditor(editor); err != nil {
		t.Errorf("%s: %v", name, err)
	}
	if editor.String() != fixture(marked).String() {
		if !editor.Undo() || render(editor) != marked {
			t.Errorf("%s: undo gave %q, want %q", name, render(editor), marked)
		}
	}
}

func TestInsertNewlineCodeIndents(t *testing.T) {
	cases := []struct {
		name, from, to string
		options        CodeOptions
	}{
		{"keeps the indent", "  foo‸", "  foo\n  ‸", codeStyle},
		{"indents after an opener", "func main() {‸", "func main() {\n    ‸", codeStyle},
		{"opens a block between a pair", "if x {‸}", "if x {\n    ‸\n}", codeStyle},
		{"block keeps the outer indent", "  {‸}", "  {\n      ‸\n  }", codeStyle},
		{"a blank indented line is emptied", "a\n    ‸", "a\n\n    ‸", codeStyle},
		{"tabs", "\tx {‸", "\tx {\n\t\t‸", CodeOptions{AutoIndent: true, HardTabs: true, IndentAfter: "{"}},
		{"selection is replaced", "  x«yz»w", "  x\n  ‸w", codeStyle},
		{"plain without auto indent", "  foo‸", "  foo\n‸", CodeOptions{}},
		{"mid-line split keeps indent", "  ab‸cd", "  ab\n  ‸cd", codeStyle},
		{"yaml colon", "key:‸", "key:\n  ‸", CodeOptions{AutoIndent: true, TabWidth: 2, IndentAfter: ":"}},
		{"trailing spaces before an opener", "x {  ‸", "x {  \n    ‸", codeStyle},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, func(e *Editor) { e.InsertNewlineCode(c.options) })
	}
}

func TestInsertTabCode(t *testing.T) {
	spaces4 := CodeOptions{TabWidth: 4}
	hard := CodeOptions{HardTabs: true}
	cases := []struct {
		name, from, to string
		options        CodeOptions
	}{
		{"to the next tab stop", "ab‸", "ab  ‸", spaces4},
		{"a full stop from the margin", "‸", "    ‸", spaces4},
		{"wide characters count as two columns", "中‸", "中  ‸", spaces4},
		{"hard tab", "ab‸", "ab\t‸", hard},
		{"partial selection is replaced", "a«b»c", "a   ‸c", spaces4},
		{"selected lines are indented", "«a\nb»", "«    a\n    b»", spaces4},
		{"the line a selection ends before is left alone", "«a\nb\n»c", "«    a\n    b\n»c", spaces4},
		{"empty lines stay empty", "«a\n\nb»", "«    a\n\n    b»", spaces4},
		{"hard tabs indent lines", "«a\nb»", "«\ta\n\tb»", hard},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, func(e *Editor) { e.InsertTabCode(c.options) })
	}
}

func TestOutdentCode(t *testing.T) {
	options := CodeOptions{TabWidth: 4}
	cases := []struct{ name, from, to string }{
		{"one indent of spaces", "    ab‸", "ab‸"},
		{"less than an indent", "  ab‸", "ab‸"},
		{"a tab", "\tab‸", "ab‸"},
		{"only one level", "        ab‸", "    ab‸"},
		{"caret inside the removed spaces", "  ‸  x", "‸x"},
		{"nothing to remove", "ab‸", "ab‸"},
		{"selected lines", "«    a\n  b\n\tc»", "«a\nb\nc»"},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, func(e *Editor) { e.OutdentCode(options) })
	}
}

func TestToggleLineComment(t *testing.T) {
	hash := CodeOptions{LineComment: "#"}
	cases := []struct {
		name, from, to string
		options        CodeOptions
	}{
		{"comments a line and keeps the caret on its text", "foo‸", "// foo‸", codeStyle},
		{"uncomments", "// foo‸", "foo‸", codeStyle},
		{"uncomments without the space", "//foo‸", "foo‸", codeStyle},
		{"a hash prefix", "a‸", "# a‸", hash},
		{"the prefix goes at the shallowest indent", "«a\n  b»", "«// a\n//   b»", codeStyle},
		{"indented block", "  «a\n    b»", "  «// a\n  //   b»", codeStyle},
		{"empty lines are skipped", "«a\n\nb»", "«// a\n\n// b»", codeStyle},
		{"a block with an uncommented line is commented", "«// a\nb»", "«// // a\n// b»", codeStyle},
		{"uncomments a block", "«// a\n// b»", "«a\nb»", codeStyle},
		{"keeps indentation when uncommenting", "  // a‸", "  a‸", codeStyle},
		{"nothing without a prefix", "a‸", "a‸", CodeOptions{}},
		{"only empty lines", "‸\n", "‸\n", codeStyle},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, func(e *Editor) { e.ToggleLineComment(c.options) })
	}
}

func TestLineOperations(t *testing.T) {
	cases := []struct {
		name, from, to string
		operation      func(*Editor)
	}{
		{"duplicate", "a‸\nb", "a\na‸\nb", func(e *Editor) { e.DuplicateLines(false) }},
		{"duplicate a selection of lines", "«a\nb»", "a\nb\n«a\nb»", func(e *Editor) { e.DuplicateLines(false) }},
		{"duplicate up leaves the caret on the new line", "a\nb‸", "a\nb‸\nb", func(e *Editor) { e.DuplicateLines(true) }},
		{"duplicate lines up", "x\n«a\nb»", "x\n«a\nb»\na\nb", func(e *Editor) { e.DuplicateLines(true) }},
		{"move up", "a\nb‸", "b‸\na", func(e *Editor) { e.MoveLines(true) }},
		{"move down", "a‸\nb", "b\na‸", func(e *Editor) { e.MoveLines(false) }},
		{"move a block down", "«a\nb»\nc", "c\n«a\nb»", func(e *Editor) { e.MoveLines(false) }},
		{"move up at the first line", "a‸\nb", "a‸\nb", func(e *Editor) { e.MoveLines(true) }},
		{"move down at the last line", "a\nb‸", "a\nb‸", func(e *Editor) { e.MoveLines(false) }},
		{"delete a middle line", "a\nb‸\nc", "a\n‸c", (*Editor).DeleteLines},
		{"delete the last line", "a\nb‸", "a‸", (*Editor).DeleteLines},
		{"delete the only line", "a‸", "‸", (*Editor).DeleteLines},
		{"delete lines", "a\n«b\nc»\nd", "a\n‸d", (*Editor).DeleteLines},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, c.operation)
	}
}

func TestTypeCodePairs(t *testing.T) {
	cases := []struct {
		name, from, typed, to string
		options               CodeOptions
	}{
		{"opener pairs", "‸", "(", "(‸)", codeStyle},
		{"opener pairs after text", "foo‸", "(", "foo(‸)", codeStyle},
		{"the closer is stepped over", "(‸)", ")", "()‸", codeStyle},
		{"no pair before a letter", "‸x", "(", "(‸x", codeStyle},
		{"pair before a closer", "(‸)", "[", "([‸])", codeStyle},
		{"quote pairs", "‸", "\"", "\"‸\"", codeStyle},
		{"apostrophe in a word stays alone", "don‸", "'", "don'‸", codeStyle},
		{"the closing quote is stepped over", "\"a‸\"", "\"", "\"a\"‸", codeStyle},
		{"selection is wrapped", "«ab»", "(", "(«ab»)", codeStyle},
		{"selection is wrapped in quotes", "x «ab»", "\"", "x \"«ab»\"", codeStyle},
		{"a closer replaces the selection", "«ab»", ")", ")‸", codeStyle},
		{"braces", "‸", "{", "{‸}", codeStyle},
		{"nothing without AutoClose", "‸", "(", "(‸", CodeOptions{}},
		{"several characters are plain text", "‸", "ab", "ab‸", codeStyle},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, func(e *Editor) { e.TypeCode(c.typed, c.options) })
	}
}

func TestDeleteBackwardCode(t *testing.T) {
	cases := []struct {
		name, from, to string
		options        CodeOptions
	}{
		{"removes an empty pair", "(‸)", "‸", codeStyle},
		{"removes empty quotes", "\"‸\"", "‸", codeStyle},
		{"a pair with text between is not removed", "(‸x)", "‸x)", codeStyle},
		{"back to the previous tab stop", "      ‸", "    ‸", codeStyle},
		{"a whole indent", "    ‸", "‸", codeStyle},
		{"with text after the caret", "    ‸x", "‸x", codeStyle},
		{"one space after text", "a  ‸", "a ‸", codeStyle},
		{"without AutoIndent a space is one rune", "    ‸", "   ‸", CodeOptions{TabWidth: 4}},
		{"a selection is deleted", "a«bc»d", "a‸d", codeStyle},
		{"at the start", "‸a", "‸a", codeStyle},
	}
	for _, c := range cases {
		checkOperation(t, c.name, c.from, c.to, func(e *Editor) { e.DeleteBackwardCode(c.options) })
	}
}

func TestMoveLineStartSmart(t *testing.T) {
	editor := fixture("a\n    text‸ here")
	editor.MoveLineStartSmart(false)
	if got := render(editor); got != "a\n    ‸text here" {
		t.Fatalf("first press = %q", got)
	}
	editor.MoveLineStartSmart(false)
	if got := render(editor); got != "a\n‸    text here" {
		t.Fatalf("second press = %q", got)
	}
	editor.MoveLineStartSmart(false)
	if got := render(editor); got != "a\n    ‸text here" {
		t.Fatalf("third press = %q", got)
	}
	editor.MoveLineStartSmart(true)
	if got := render(editor); got != "a\n«    »text here" && got != "a\n‸    text here" {
		t.Fatalf("extending = %q", got)
	}
}

func TestCodeOperationsLeaveAnActiveCompositionToTheInputMethod(t *testing.T) {
	editor := fixture("ab‸")
	editor.SetMarkedText("你好", Range{Start: 2, End: 2})
	editor.InsertNewlineCode(codeStyle)
	if _, composing := editor.Composition(); composing {
		t.Fatal("the line break did not end the composition")
	}
	// As for a commit from the input method, the line break replaces the marked text.
	if got := editor.String(); got != "ab\n" {
		t.Fatalf("line break during composition = %q", got)
	}
	editor = fixture("ab‸")
	editor.SetMarkedText("(", Range{Start: 1, End: 1})
	editor.TypeCode(")", codeStyle)
	if got := editor.String(); got != "ab)" {
		t.Fatalf("typing during composition = %q, want it passed through", got)
	}
}

func TestMatchBracket(t *testing.T) {
	cases := []struct {
		value        string
		offset       int
		open, closer int
		ok           bool
	}{
		{"(a)", 0, 0, 2, true},
		{"(a)", 1, 0, 2, true}, // the caret is just after the opener
		{"(a)", 3, 0, 2, true}, // just after the closer
		{"(a)", 2, 0, 2, true}, // just before the closer
		{"((a)b)", 1, 0, 5, true},
		{"((a)b)", 2, 1, 3, true},
		{"{[(x)]}", 3, 2, 4, true}, // before x: after the opener
		{"(]", 0, 0, 0, false},     // a different kind never pairs
		{"((", 0, 0, 0, false},
		{"a", 1, 0, 0, false},
		{"", 0, 0, 0, false},
		{"([)]", 0, 0, 2, true}, // brackets of other kinds are ignored while matching
		{"中(文)", 4, 3, 7, true},
	}
	for _, c := range cases {
		open, closer, ok := MatchBracket(c.value, c.offset)
		if ok != c.ok || ok && (open != c.open || closer != c.closer) {
			t.Errorf("MatchBracket(%q, %d) = %d, %d, %v; want %d, %d, %v", c.value, c.offset, open, closer, ok, c.open, c.closer, c.ok)
		}
	}
}

func TestMatchBracketIsBounded(t *testing.T) {
	value := "(" + strings.Repeat("x", bracketScanLimit+10) + ")"
	if _, _, ok := MatchBracket(value, 0); ok {
		t.Fatal("matched a partner beyond the scan limit")
	}
	value = "(" + strings.Repeat("x", bracketScanLimit/2) + ")"
	if _, _, ok := MatchBracket(value, 0); !ok {
		t.Fatal("missed a partner inside the scan limit")
	}
}

func TestPositionAndOffset(t *testing.T) {
	value := "ab\n中文x\n\nlast"
	cases := []struct {
		offset   int
		position Position
	}{
		{0, Position{0, 0}},
		{2, Position{0, 2}},
		{3, Position{1, 0}},
		{6, Position{1, 1}},
		{7, Position{1, 1}}, // inside the second byte of 中... resolves to the character start
		{9, Position{1, 2}},
		{10, Position{1, 3}},
		{12, Position{3, 0}},
		{len(value), Position{3, 4}},
	}
	for _, c := range cases {
		if got := PositionAt(value, c.offset); got != c.position {
			t.Errorf("PositionAt(%d) = %+v, want %+v", c.offset, got, c.position)
		}
	}
	for offset := 0; offset <= len(value); offset++ {
		if !utf8.RuneStart(byteAt(value, offset)) {
			continue
		}
		if back := OffsetAt(value, PositionAt(value, offset)); back != offset {
			t.Errorf("offset %d came back as %d", offset, back)
		}
	}
	if got := OffsetAt(value, Position{Line: 99, Column: 99}); got != len(value) {
		t.Errorf("a line past the end = %d", got)
	}
	if got := OffsetAt(value, Position{Line: 0, Column: 99}); got != 2 {
		t.Errorf("a column past the end of the line = %d", got)
	}
	if got := LineCount(value); got != 4 {
		t.Errorf("LineCount = %d", got)
	}
	if LineCount("") != 1 || LineCount("a\n") != 2 {
		t.Error("an empty line follows a final newline")
	}
}

func byteAt(value string, offset int) byte {
	if offset >= len(value) {
		return 0
	}
	return value[offset]
}

func TestDetectIndent(t *testing.T) {
	cases := []struct {
		name, value string
		hard        bool
		width       int
	}{
		{"two spaces", "a:\n  b:\n    c: 1\n  d: 2\n", false, 2},
		{"four spaces", "def f():\n    if x:\n        y\n    z\n", false, 4},
		{"tabs", "func f() {\n\tif x {\n\t\ty()\n\t}\n}\n", true, 4},
		{"nothing to go by", "a\nb\n", false, 4},
		{"empty", "", false, 4},
		{"blank lines are ignored", "a:\n\n  b: 1\n\n  c: 2\n", false, 2},
		{"mostly tabs", "\ta\n\tb\n  c\n", true, 4},
	}
	for _, c := range cases {
		if hard, width := DetectIndent(c.value); hard != c.hard || width != c.width {
			t.Errorf("%s: DetectIndent = %v, %d; want %v, %d", c.name, hard, width, c.hard, c.width)
		}
	}
}

func TestByteOffsetAndRuneIndexMatchTheText(t *testing.T) {
	editor := NewEditor("a中é👍🏽\nxyz")
	value := editor.String()
	for index := 0; index <= editor.RuneCount(); index++ {
		want := len(string([]rune(value)[:index]))
		if got := editor.ByteOffset(index); got != want {
			t.Fatalf("ByteOffset(%d) = %d, want %d", index, got, want)
		}
		if back := editor.RuneIndex(want); back != index {
			t.Fatalf("RuneIndex(%d) = %d, want %d", want, back, index)
		}
	}
	// An offset inside a character belongs to the character.
	if got := editor.RuneIndex(2); got != 1 {
		t.Fatalf("RuneIndex inside 中 = %d, want 1", got)
	}
	if editor.ByteOffset(-5) != 0 || editor.ByteOffset(1<<20) != len(value) || editor.RuneIndex(1<<20) != editor.RuneCount() {
		t.Fatal("out-of-range arguments are not clamped")
	}
}

func TestByteOffsetFollowsEditsAndHistory(t *testing.T) {
	editor := NewEditor(strings.Repeat("héllo 中文\n", 50))
	random := rand.New(rand.NewSource(7))
	check := func(step string) {
		t.Helper()
		value := editor.String()
		for probe := 0; probe < 8; probe++ {
			index := random.Intn(editor.RuneCount() + 1)
			if got, want := editor.ByteOffset(index), len(string([]rune(value)[:index])); got != want {
				t.Fatalf("%s: ByteOffset(%d) = %d, want %d", step, index, got, want)
			}
			offset := random.Intn(len(value) + 1)
			want := utf8.RuneCountInString(strings.ToValidUTF8(value[:offset], "")) // floor to a boundary
			for want > 0 && len(string([]rune(value)[:want])) > offset {
				want--
			}
			if got := editor.RuneIndex(offset); got != want {
				t.Fatalf("%s: RuneIndex(%d) = %d, want %d", step, offset, got, want)
			}
		}
	}
	check("initial")
	for step := 0; step < 200; step++ {
		switch random.Intn(5) {
		case 0:
			editor.SetSelection(Range{Start: random.Intn(editor.RuneCount() + 1), End: random.Intn(editor.RuneCount() + 1)})
			editor.Insert(fragments[random.Intn(len(fragments))])
		case 1:
			editor.DeleteBackward()
		case 2:
			editor.Undo()
		case 3:
			editor.Redo()
		case 4:
			editor.MoveTo(random.Intn(editor.RuneCount()+1), false)
			editor.InsertNewlineCode(codeStyle)
		}
		check("step")
	}
}

func applyCodeOperation(editor *Editor, code, argument byte) {
	fragment := fragments[int(argument)%len(fragments)]
	switch code % 16 {
	case 0:
		editor.InsertNewlineCode(codeStyle)
	case 1:
		editor.InsertTabCode(codeStyle)
	case 2:
		editor.OutdentCode(codeStyle)
	case 3:
		editor.ToggleLineComment(codeStyle)
	case 4:
		editor.DuplicateLines(argument&1 == 1)
	case 5:
		editor.MoveLines(argument&1 == 1)
	case 6:
		editor.DeleteLines()
	case 7:
		editor.TypeCode(string([]rune("([{\"')]}x ")[int(argument)%10]), codeStyle)
	case 8:
		editor.DeleteBackwardCode(codeStyle)
	case 9:
		editor.MoveLineStartSmart(argument&1 == 1)
	case 10:
		editor.Insert(fragment)
	case 11:
		editor.SetSelection(Range{Start: int(argument>>4) - 2, End: int(argument&0x0f) + 3})
	case 12:
		editor.InsertTabCode(CodeOptions{HardTabs: true})
	case 13:
		editor.Undo()
	case 14:
		editor.Redo()
	case 15:
		editor.SetMarkedText(fragment, Range{})
	}
}

func runCodeOperations(t testing.TB, operations []byte) {
	editor := NewEditor("a\n  b {\n}\n")
	for index := 0; index+1 < len(operations); index += 2 {
		applyCodeOperation(editor, operations[index], operations[index+1])
		if err := checkEditor(editor); err != nil {
			t.Fatalf("after operation %d (%d, %d): %v", index/2, operations[index], operations[index+1], err)
		}
		value := editor.String()
		for _, probe := range [...]int{0, editor.Caret(), editor.RuneCount()} {
			if got, want := editor.ByteOffset(probe), len(string([]rune(value)[:probe])); got != want {
				t.Fatalf("after operation %d: ByteOffset(%d) = %d, want %d", index/2, probe, got, want)
			}
		}
		MatchBracket(value, editor.ByteOffset(editor.Caret()))
	}
}

func FuzzCodeOperations(f *testing.F) {
	f.Add([]byte{0, 0, 1, 0, 3, 0, 4, 0, 5, 1, 6, 0, 7, 0, 8, 0})
	f.Add([]byte{11, 0x2f, 3, 0, 3, 0, 13, 0, 14, 0, 2, 0, 1, 0})
	f.Add([]byte{7, 0, 7, 1, 7, 2, 8, 0, 8, 0, 15, 3, 0, 0})
	f.Fuzz(func(t *testing.T, operations []byte) { runCodeOperations(t, operations) })
}

func TestCodeRandomOperationsKeepTheirInvariants(t *testing.T) {
	random := rand.New(rand.NewSource(20261105))
	for sequence := 0; sequence < 3000; sequence++ {
		operations := make([]byte, 2*(5+random.Intn(60)))
		random.Read(operations)
		runCodeOperations(t, operations)
	}
}

func TestCodeOperationsAreEachOneUndoUnit(t *testing.T) {
	random := rand.New(rand.NewSource(99))
	for sequence := 0; sequence < 500; sequence++ {
		editor := NewEditor("a\n  b {\n}\nlast line\n")
		editor.SetSelection(Range{Start: random.Intn(10), End: random.Intn(10)})
		before := render(editor)
		for _, operation := range []func(){
			func() { editor.InsertNewlineCode(codeStyle) },
			func() { editor.InsertTabCode(codeStyle) },
			func() { editor.OutdentCode(codeStyle) },
			func() { editor.ToggleLineComment(codeStyle) },
			func() { editor.DuplicateLines(random.Intn(2) == 0) },
			func() { editor.IndentLinesCode(codeStyle) },
			func() { editor.MoveLines(random.Intn(2) == 0) },
			func() { editor.DeleteLines() },
		} {
			editor = NewEditor("a\n  b {\n}\nlast line\n")
			editor.SetSelection(Range{Start: random.Intn(10), End: random.Intn(10)})
			before = render(editor)
			text := editor.String()
			operation()
			if editor.String() != text {
				after := render(editor)
				if !editor.Undo() || render(editor) != before {
					t.Fatalf("one Undo of %q gave %q, want %q", after, render(editor), before)
				}
			}
		}
	}
}

func TestReplaceRangeIsOneUndoUnit(t *testing.T) {
	editor := fixture("one two‸ three")
	editor.ReplaceRange(Range{Start: 4, End: 7}, "2")
	if got := render(editor); got != "one 2‸ three" {
		t.Fatalf("ReplaceRange = %q", got)
	}
	if !editor.Undo() || render(editor) != "one two‸ three" {
		t.Fatalf("undo gave %q", render(editor))
	}
	editor.ReplaceRange(Range{Start: 99, End: -4}, "!")
	if got := editor.String(); got != "!one two three" && got != "one two three!" && got != "!" {
		t.Fatalf("a range outside the text is clamped, got %q", got)
	}
}

func TestAReplaceAllBiggerThanTheHistoryCapIsStillUndoable(t *testing.T) {
	big := strings.Repeat("abc\n", 200_000) // 800,000 runes, past the 256 KiB cap
	editor := NewEditor(big)
	editor.ReplaceRange(Range{Start: 0, End: editor.RuneCount()}, strings.Repeat("xy\n", 200_000))
	if !editor.CanUndo() {
		t.Fatal("a large replace lost the history")
	}
	if !editor.Undo() || editor.String() != big {
		t.Fatal("undo did not restore the text")
	}
	if !editor.Redo() || editor.RuneCount() != 600_000 {
		t.Fatal("redo did not reapply the replacement")
	}
	// An older unit makes way for it instead of the other way round.
	small := NewEditor("start")
	small.Insert("x")
	small.BreakUndoGroup()
	small.ReplaceRange(Range{Start: 0, End: small.RuneCount()}, strings.Repeat("y", 300_000))
	if !small.CanUndo() || !small.Undo() || small.String() != "startx" {
		t.Fatalf("undo after a big replacement gave %d runes", small.RuneCount())
	}
	// Edits beyond any reasonable size drop the history rather than keep it.
	huge := NewEditor("")
	huge.ReplaceRange(Range{}, strings.Repeat("z", maxSingleEditRunes+1))
	if huge.CanUndo() {
		t.Fatal("a multi-megabyte edit stayed in the history")
	}
}

func TestCellWidth(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{{'a', 1}, {' ', 1}, {'~', 1}, {'é', 1}, {'中', 2}, {'ひ', 2}, {'한', 2}, {'Ａ', 2}, {'👍', 2}, {'́', 0}, {'‍', 1}, {'ا', 1}}
	for _, c := range cases {
		if got := CellWidth(c.r); got != c.want {
			t.Errorf("CellWidth(%q) = %d, want %d", c.r, got, c.want)
		}
	}
}
