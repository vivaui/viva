//go:build darwin || windows || linux

package app

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// wrappingNotes is a Textarea the way an application wires it: it holds the
// text in a text.Editor and a TextareaLayout, and the framework does the rest.
type wrappingNotes struct {
	editor *text.Editor
	layout components.TextareaLayout
	theme  gfx.Theme
	bounds gfx.Rect
}

func newWrappingNotes(value string) *wrappingNotes {
	return &wrappingNotes{editor: text.NewEditor(value), theme: gfx.LightTheme(), bounds: gfx.Rect{X: 10, Y: 10, Width: 200, Height: 120}}
}

func (notes *wrappingNotes) config() Config {
	config := Config{Title: "notes", Width: 300, Height: 200}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		value := notes.editor.String()
		selection := notes.editor.Selection()
		start, end := byteOffset(value, selection.Start), byteOffset(value, selection.End)
		caret := byteOffset(value, notes.editor.Caret())
		components.PaintTextarea(canvas, notes.theme, notes.bounds, components.TextareaSnapshot{
			ID: gfx.Item(5), Label: "Notes", Value: value, HideEditorText: true, ShowCaret: selection.Start == selection.End,
			Caret: caret, SelectionStart: start, SelectionEnd: end, Layout: &notes.layout,
		})
	}
	config.TextInput = func(TextInputContext) *TextInput { return &TextInput{Editor: notes.editor} }
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
		if event.Target != gfx.Item(5) || movement == components.TextareaMoveNone {
			return gfx.EventResult{}
		}
		value := notes.editor.String()
		to := notes.layout.Move(value, byteOffset(value, notes.editor.Caret()), movement)
		notes.editor.MoveTo(len([]rune(value[:to])), extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return config
}

func byteOffset(value string, runes int) int {
	for offset := range value {
		if runes == 0 {
			return offset
		}
		runes--
	}
	return len(value)
}

const notesText = "中文 the quick brown fox jumps over the lazy dog and keeps running through the forest until night"

// A click lands on the visual row under it, whatever the text before it is made
// of, and a drag selects across rows: the platform editor knows one line, so
// this is the runtime's job.
func TestDriverPlacesTheCaretByVisualRowAndSelectsAcrossRows(t *testing.T) {
	notes := newWrappingNotes(notesText)
	driver, err := NewDriver(notes.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	fontSize := gfx.DefaultMetrics().FontSizeM
	left, rowHeight := notes.bounds.X+11, fontSize+7
	top := notes.bounds.Y + 8

	// The first click focuses and places the caret in the same press.
	if err := driver.Click(left+1, top+2*rowHeight+10); err != nil {
		t.Fatal(err)
	}
	rows := (&gfx.Canvas{}).WrapRows(nil, notesText, fontSize, gfx.WeightRegular, notes.bounds.Width-22)
	if len(rows) < 3 {
		t.Fatalf("the text did not wrap: %+v", rows)
	}
	wantRune := len([]rune(notesText[:rows[2].Start]))
	if got := notes.editor.Caret(); got != wantRune {
		t.Fatalf("a click on the third row put the caret at rune %d, want %d (the row starts at byte %d)", got, wantRune, rows[2].Start)
	}

	// A drag from the second row to the fourth selects everything between.
	if err := driver.Press(left+1, top+rowHeight+10, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(left+1, top+3*rowHeight+10); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(left+1, top+3*rowHeight+10, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	selection := notes.editor.Selection()
	if selection.Start != len([]rune(notesText[:rows[1].Start])) || selection.End != len([]rune(notesText[:rows[3].Start])) {
		t.Fatalf("the drag selected runes %d..%d, want the starts of rows 1 and 3 (%d..%d); rows %+v", selection.Start, selection.End, len([]rune(notesText[:rows[1].Start])), len([]rune(notesText[:rows[3].Start])), rows)
	}
}

func TestDriverArrowKeysMoveByVisualRowAndKeepTheColumn(t *testing.T) {
	notes := newWrappingNotes(notesText)
	driver, err := NewDriver(notes.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	left, top := notes.bounds.X+11, notes.bounds.Y+8
	if err := driver.Click(left+1, top+10); err != nil { // the start of the first row
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	value := notes.editor.String()
	rows := (&gfx.Canvas{}).WrapRows(nil, value, gfx.DefaultMetrics().FontSizeM, gfx.WeightRegular, notes.bounds.Width-22)
	endOfFirst := len([]rune(strings.TrimRight(value[rows[0].Start:rows[0].End], " ")))
	if got := notes.editor.Caret(); got != endOfFirst {
		t.Fatalf("End went to rune %d, want the end of the first row at %d", got, endOfFirst)
	}
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	caret := byteOffset(value, notes.editor.Caret())
	if caret < rows[1].Start || caret > rows[1].End {
		t.Fatalf("Down left the caret at byte %d, outside the second row %+v", caret, rows[1])
	}
	if err := driver.Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	if got := notes.editor.Caret(); got != endOfFirst {
		t.Fatalf("Up came back to rune %d, want %d", got, endOfFirst)
	}
	// Shift extends from where the caret was.
	if err := driver.Key(gfx.KeyDown, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if selection := notes.editor.Selection(); selection.Start != endOfFirst || selection.End <= endOfFirst {
		t.Fatalf("Shift+Down selected %+v", selection)
	}
	if err := driver.Key(gfx.KeyHome, 0); err != nil {
		t.Fatal(err)
	}
	if got := notes.editor.Caret(); got != len([]rune(value[:rows[1].Start])) {
		t.Fatalf("Home on the second row went to rune %d", got)
	}
}

// Option and Command with an arrow, Backspace or Option with Delete are text-system commands
// on macOS even in a field that lays out its own rows: only Command with Left and
// Right, which end at the visual row, reach the application.
func TestDriverOptionAndCommandKeysEditByWordAndLine(t *testing.T) {
	notes := newWrappingNotes("alpha beta/gamma")
	driver, err := NewDriver(notes.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(notes.bounds.X+11, notes.bounds.Y+8+10); err != nil {
		t.Fatal(err)
	}
	notes.editor.MoveTo(16, false) // the end of "alpha beta/gamma"
	press := func(key gfx.Key, modifiers gfx.Modifiers) {
		t.Helper()
		if err := driver.Key(key, modifiers); err != nil {
			t.Fatal(err)
		}
	}
	press(gfx.KeyLeft, gfx.ModifierAlt)
	if got := notes.editor.Caret(); got != 11 {
		t.Fatalf("Option+Left went to %d, want the start of gamma at 11", got)
	}
	press(gfx.KeyRight, gfx.ModifierAlt|gfx.ModifierShift)
	if got := notes.editor.Selection(); got != (text.Range{Start: 11, End: 16}) {
		t.Fatalf("Option+Shift+Right selected %+v, want 11:16", got)
	}
	notes.editor.MoveTo(16, false)
	press(gfx.KeyBackspace, gfx.ModifierAlt)
	if got := notes.editor.String(); got != "alpha beta/" {
		t.Fatalf("Option+Backspace left %q", got)
	}
	notes.editor.Insert("\nsecond line")
	notes.editor.MoveTo(11, false)
	press(gfx.KeyBackspace, gfx.ModifierMeta)
	if got := notes.editor.String(); got != "\nsecond line" {
		t.Fatalf("Command+Backspace left %q, want the first line removed", got)
	}
}
