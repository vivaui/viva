//go:build darwin || windows || linux

package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// terminalProbe records what a focused terminal receives.
type terminalProbe struct {
	keys    []gfx.KeyEvent
	text    []TextEvent
	preedit TextState
	altMeta bool
	redraw  bool // ask for a frame with every committed text event
}

func (probe *terminalProbe) config() Config {
	config := Config{Title: "terminal", Width: 400, Height: 200}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: size.Width, Height: size.Height}, "Terminal", gfx.HitOptions{Role: gfx.RoleTerminal})
	}
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		probe.keys = append(probe.keys, event)
		return gfx.EventResult{Handled: true}
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if !context.Terminal {
			return nil
		}
		return &TextInput{
			Bounds: gfx.Rect{X: 8, Y: 16, Width: 8, Height: 16}, FontSize: 13,
			State: probe.preedit, AltSendsEscape: probe.altMeta,
			OnEvent: func(event TextEvent) (TextState, error) {
				probe.text = append(probe.text, event)
				if event.Kind == TextSetMarked {
					probe.preedit = TextState{Value: event.Text, Composition: TextRange{End: len([]rune(event.Text))}, Composing: true}
				} else {
					probe.preedit = TextState{}
				}
				result := probe.preedit
				result.Redraw = probe.redraw
				return result, nil
			},
		}
	}
	return config
}

func TestDriverHonorsATerminalsAltSendsEscapePolicy(t *testing.T) {
	probe := &terminalProbe{}
	driver, err := NewDriver(probe.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}

	if err := driver.Chord(gfx.ModifierAlt, "x"); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 0 || len(probe.text) != 1 || probe.text[0].Text != "x" {
		t.Fatalf("default Option route: keys=%+v text=%+v", probe.keys, probe.text)
	}

	probe.altMeta = true
	if err := driver.redraw(); err != nil {
		t.Fatal(err)
	}
	if err := driver.Chord(gfx.ModifierAlt, "X"); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 1 || probe.keys[0] != (gfx.KeyEvent{Modifiers: gfx.ModifierAlt, Target: gfx.Item(1), Text: "x"}) {
		t.Fatalf("Meta Option route: keys=%+v", probe.keys)
	}

	if err := driver.textEvent(platformSetMarked("ni")); err != nil {
		t.Fatal(err)
	}
	if err := driver.Chord(gfx.ModifierAlt, "x"); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != 1 || len(probe.text) != 3 || probe.text[2].Text != "x" {
		t.Fatalf("composing Option route: keys=%+v text=%+v", probe.keys, probe.text)
	}
}

func TestDriverDeliversATerminalsKeysRawAndItsTextThroughTheTextSystem(t *testing.T) {
	probe := &terminalProbe{}
	driver, err := NewDriver(probe.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Chord(gfx.ModifierControl, "c"); !errors.Is(err, ErrNoTerminal) {
		t.Fatalf("a chord with nothing focused: %v", err)
	}
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	if !driver.terminal() {
		t.Fatal("clicking the terminal did not give it the keyboard")
	}
	if input := driver.painter.nativeText; !input.Terminal || input.X != 8 || input.Height != 16 {
		t.Fatalf("native editor = %+v, want a terminal surface at the cursor cell", input)
	}

	// Named keys are raw, with the modifiers they came with and no text.
	for _, key := range []gfx.Key{gfx.KeyUp, gfx.KeyTab, gfx.KeyEnter, gfx.KeyEscape, gfx.KeyBackspace, gfx.KeyPageDown, gfx.KeyF5} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Key(gfx.KeyLeft, gfx.ModifierAlt|gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	// Control chords carry their character; Key is unknown.
	if err := driver.Chord(gfx.ModifierControl, "C"); err != nil {
		t.Fatal(err)
	}
	want := []gfx.KeyEvent{
		{Key: gfx.KeyUp, Target: gfx.Item(1)}, {Key: gfx.KeyTab, Target: gfx.Item(1)}, {Key: gfx.KeyEnter, Target: gfx.Item(1)},
		{Key: gfx.KeyEscape, Target: gfx.Item(1)}, {Key: gfx.KeyBackspace, Target: gfx.Item(1)}, {Key: gfx.KeyPageDown, Target: gfx.Item(1)}, {Key: gfx.KeyF5, Target: gfx.Item(1)},
		{Key: gfx.KeyLeft, Modifiers: gfx.ModifierAlt | gfx.ModifierShift, Target: gfx.Item(1)},
		{Modifiers: gfx.ModifierControl, Target: gfx.Item(1), Text: "c"},
	}
	if !reflect.DeepEqual(probe.keys, want) {
		t.Fatalf("raw keys =\n%+v\nwant\n%+v", probe.keys, want)
	}
	if len(probe.text) != 0 {
		t.Fatalf("raw keys leaked into the text system: %+v", probe.text)
	}

	// Text, including a plain space and CJK, takes the text path.
	if err := driver.Type("你好"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeySpace, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Chord(gfx.ModifierShift, "A"); err != nil {
		t.Fatal(err)
	}
	if len(probe.keys) != len(want) {
		t.Fatalf("text arrived as raw keys: %+v", probe.keys[len(want):])
	}
	got := []string{}
	for _, event := range probe.text {
		if event.Kind != TextInsert {
			t.Fatalf("event = %+v", event)
		}
		got = append(got, event.Text)
	}
	if !reflect.DeepEqual(got, []string{"你好", " ", "A"}) {
		t.Fatalf("inserted text = %q", got)
	}

	// Command+V is a paste, delivered untouched. Other Command chords remain
	// raw so the terminal application can implement tab actions and selection.
	driver.SetClipboard("echo hi\nls\n")
	if handled, err := driver.Shortcut('v'); err != nil || !handled {
		t.Fatalf("paste = %v, %v", handled, err)
	}
	if last := probe.text[len(probe.text)-1]; last.Kind != TextPaste || last.Text != "echo hi\nls\n" {
		t.Fatalf("paste event = %+v", last)
	}
	for _, character := range []rune{'a', 'c', 'x'} {
		if handled, err := driver.Shortcut(character); err != nil || !handled {
			t.Fatalf("Command+%c = %v, %v", character, handled, err)
		}
	}
	for index, character := range []string{"a", "c", "x"} {
		event := probe.keys[len(probe.keys)-3+index]
		if event.Modifiers != gfx.ModifierMeta || event.Text != character || event.Target != gfx.Item(1) {
			t.Errorf("Command+%s event = %+v", character, event)
		}
	}
}

func TestDriverKeepsApplicationClipboardRequestsPrivate(t *testing.T) {
	probe := &terminalProbe{}
	config := probe.config()
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		return gfx.EventResult{Handled: true, ClipboardText: "selected terminal text"}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.Shortcut('c'); err != nil || !handled {
		t.Fatalf("Command+C = %v, %v", handled, err)
	}
	if got := driver.Clipboard(); got != "selected terminal text" {
		t.Fatalf("private clipboard = %q", got)
	}
}

func BenchmarkDriverApplicationClipboardResult(b *testing.B) {
	probe := &terminalProbe{}
	config := probe.config()
	config.OnKey = func(gfx.KeyEvent) gfx.EventResult {
		return gfx.EventResult{Handled: true, ClipboardText: "selected terminal text"}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if handled, err := driver.Shortcut('c'); err != nil || !handled {
			b.Fatalf("Command+C = %v, %v", handled, err)
		}
	}
}

func TestDriverShowsATerminalsPreeditAtTheCursorWithoutAFocusFight(t *testing.T) {
	probe := &terminalProbe{}
	driver, err := NewDriver(probe.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	if err := driver.textEvent(platformSetMarked("ni")); err != nil {
		t.Fatal(err)
	}
	editor := driver.Snapshot().Editor
	if editor == nil || editor.Value != "ni" || !editor.Composing || editor.Bounds.X != 8 {
		t.Fatalf("preedit overlay = %+v", editor)
	}
	// Committing clears the preedit and the committed text reaches the program.
	if err := driver.Type("你"); err != nil {
		t.Fatal(err)
	}
	if editor := driver.Snapshot().Editor; editor == nil || editor.Value != "" || editor.Composing {
		t.Fatalf("overlay after commit = %+v", editor)
	}
	if last := probe.text[len(probe.text)-1]; last.Kind != TextInsert || last.Text != "你" {
		t.Fatalf("committed text = %+v", last)
	}
}

func TestDriverDoesNotRepaintATerminalForCommittedTextWithoutPreedit(t *testing.T) {
	probe := &terminalProbe{}
	config := probe.config()
	paints := 0
	paint := config.Paint
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		paints++
		paint(canvas, size)
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	before := paints
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if paints != before {
		t.Fatalf("plain terminal text repainted %d times", paints-before)
	}
	if len(probe.text) != 1 || probe.text[0].Text != "x" {
		t.Fatalf("terminal events = %+v", probe.text)
	}
	if err := driver.textEvent(platformSetMarked("ni")); err != nil {
		t.Fatal(err)
	}
	if paints != before+1 {
		t.Fatalf("starting preedit painted %d times, want 1", paints-before)
	}
}

func platformSetMarked(value string) platform.TextEvent {
	return platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: text.Range{Start: len([]rune(value)), End: len([]rune(value))}}
}

// A terminal that draws typed text ahead of the server's echo changes the
// screen on a committed-text event without changing its editor: it asks for the
// frame, and the request does not stick to the event after it.
func TestDriverRepaintsATerminalThatAsksForTheFrame(t *testing.T) {
	probe := &terminalProbe{redraw: true}
	config := probe.config()
	paints := 0
	paint := config.Paint
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		paints++
		paint(canvas, size)
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	before := paints
	if err := driver.Type("x"); err != nil {
		t.Fatal(err)
	}
	if paints != before+1 {
		t.Fatalf("a requested redraw painted %d times, want 1", paints-before)
	}
	probe.redraw = false
	before = paints
	if err := driver.Type("y"); err != nil {
		t.Fatal(err)
	}
	if paints != before {
		t.Fatalf("the next plain text repainted %d times; the request should not stick", paints-before)
	}
}
