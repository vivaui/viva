//go:build darwin || windows || linux

package headless

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestSessionDrivesATerminalWithRawKeysChordsTextAndPaste(t *testing.T) {
	var keys []ui.KeyEvent
	var typed []string
	config := app.Config{Title: "term", Width: 300, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: size.Width, Height: size.Height}, "Terminal", ui.HitOptions{Role: ui.RoleTerminal})
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		keys = append(keys, event)
		result := ui.EventResult{Handled: true}
		if event.Modifiers == ui.ModifierMeta && event.Text == "c" {
			result.ClipboardText = "selected"
		}
		return result
	}
	config.TextInput = func(app.TextInputContext) *app.TextInput {
		return &app.TextInput{
			Bounds: ui.Rect{X: 4, Y: 4, Width: 8, Height: 16}, FontSize: 13,
			AltSendsEscape: true,
			OnEvent: func(event app.TextEvent) (app.TextState, error) {
				typed = append(typed, event.Text)
				return app.TextState{}, nil
			},
		}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := session.Keys("ctrl+c"); err == nil {
		t.Fatal("a Control chord with no terminal focused was accepted")
	}
	if err := session.ClickLabel("Terminal"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("cmd+c"); err != nil {
		t.Fatal(err)
	}
	if got := session.Driver().Clipboard(); got != "selected" {
		t.Fatalf("private clipboard = %q", got)
	}
	for _, spec := range []string{"ctrl+c", "ctrl+D", "alt+x", "up", "pageup", "f5", "f12", "insert", "shift+tab", "alt+left", "enter", "esc"} {
		if err := session.Keys(spec); err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
	}
	if err := session.Type("ls -la"); err != nil {
		t.Fatal(err)
	}
	session.Driver().SetClipboard("a\nb")
	if err := session.Keys("cmd+v"); err != nil {
		t.Fatal(err)
	}
	want := []ui.KeyEvent{
		{Modifiers: ui.ModifierMeta, Text: "c"},
		{Modifiers: ui.ModifierControl, Text: "c"}, {Modifiers: ui.ModifierControl, Text: "d"},
		{Modifiers: ui.ModifierAlt, Text: "x"},
		{Key: ui.KeyUp}, {Key: ui.KeyPageUp}, {Key: ui.KeyF5}, {Key: ui.KeyF12}, {Key: ui.KeyInsert},
		{Key: ui.KeyTab, Modifiers: ui.ModifierShift}, {Key: ui.KeyLeft, Modifiers: ui.ModifierAlt},
		{Key: ui.KeyEnter}, {Key: ui.KeyEscape},
	}
	for index := range want {
		want[index].Target = ui.Item(1)
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("raw keys =\n%+v\nwant\n%+v", keys, want)
	}
	if !reflect.DeepEqual(typed, []string{"ls -la", "a\nb"}) {
		t.Fatalf("text = %q", typed)
	}
}

// compose is an input method's pinyin stage: the text is marked, not committed,
// and the editor overlay reports it as the composition until it is committed or
// ended.
func TestComposeMarksTextWithoutCommittingIt(t *testing.T) {
	var events []app.TextEvent
	var state app.TextState
	config := app.Config{Title: "compose", Width: 300, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: size.Width, Height: size.Height}, "Terminal", ui.HitOptions{Role: ui.RoleTerminal})
	}
	config.TextInput = func(app.TextInputContext) *app.TextInput {
		return &app.TextInput{
			Bounds: ui.Rect{X: 4, Y: 4, Width: 24, Height: 16}, FontSize: 13, State: state,
			OnEvent: func(event app.TextEvent) (app.TextState, error) {
				events = append(events, event)
				switch event.Kind {
				case app.TextSetMarked:
					state = app.TextState{Value: event.Text, Selection: event.MarkedSelection, Composition: app.TextRange{End: len([]rune(event.Text))}, Composing: true}
				default:
					state = app.TextState{}
				}
				return state, nil
			},
		}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := session.Compose("wo"); err == nil {
		t.Fatal("composing with no text field focused was accepted")
	}
	if err := session.ClickLabel("Terminal"); err != nil {
		t.Fatal(err)
	}
	if reply, _ := execute(session, "compose 你好 wo"); reply != "ok\n" {
		t.Fatalf("compose reply = %q", reply)
	}
	if len(events) != 1 || events[0].Kind != app.TextSetMarked || events[0].Text != "你好 wo" || events[0].MarkedSelection != (app.TextRange{Start: 5, End: 5}) {
		t.Fatalf("events = %+v: the text is marked with the caret at its end", events)
	}
	if editor := session.Snapshot().Editor; editor == nil || !editor.Composing || editor.Value != "你好 wo" || editor.Composition != [2]int{0, 5} {
		t.Fatalf("editor = %+v", editor)
	}
	if reply, _ := execute(session, "compose"); reply != "ok\n" {
		t.Fatalf("compose reply = %q", reply)
	}
	if last := events[len(events)-1]; last.Kind != app.TextUnmark {
		t.Fatalf("an empty compose sent %+v, want the composition ended", last)
	}
	if editor := session.Snapshot().Editor; editor == nil || editor.Composing {
		t.Fatalf("editor after ending = %+v", editor)
	}
}
