//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestEditingPriorityBelongsToTheBackend(t *testing.T) {
	editor := text.NewEditor("value")
	calls := 0
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", ui.HitOptions{Role: ui.RoleTextField})
		},
		TextInput:  func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnShortcut: func(ui.KeyEvent) ui.EventResult { calls++; return ui.EventResult{Handled: true} },
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(ui.ModifierControl, 'a'); !handled || err != nil {
		t.Fatalf("select all: %v, %v", handled, err)
	}
	if calls != 0 || editor.Selection() != (text.Range{End: 5}) {
		t.Fatal("application stole the driver's editing shortcut")
	}
	// A native backend that offers a chord has made its own routing decision.
	// The shared painter must not discard it using another platform's key map.
	_, handled, err := driver.painter.shortcut(platform.KeyEvent{Modifiers: input.ModifierControl, Text: "a"})
	if !handled || err != nil || calls != 1 {
		t.Fatalf("backend callback suppressed: handled=%v calls=%d err=%v", handled, calls, err)
	}
}

func TestDriverOffersGlobalShortcutsBeforeTerminalAndTextPaths(t *testing.T) {
	var shortcuts []ui.KeyEvent
	config := Config{
		Width: 320, Height: 180,
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", ui.HitOptions{Role: ui.RoleTextField})
		},
		OnShortcut: func(event ui.KeyEvent) ui.EventResult {
			shortcuts = append(shortcuts, event)
			return ui.EventResult{Handled: event.Text == "k", Repaint: event.Text == "k"}
		},
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Chord(ui.ModifierControl, "K"); err != nil {
		t.Fatal(err)
	}
	if len(shortcuts) != 1 || shortcuts[0].Text != "k" || shortcuts[0].Modifiers != ui.ModifierControl {
		t.Fatalf("shortcuts = %+v", shortcuts)
	}

	if handled, err := driver.Shortcut('c'); err != nil || handled {
		t.Fatalf("unfocused native editing shortcut = %t, %v", handled, err)
	}
	if len(shortcuts) != 2 || shortcuts[1].Text != "c" || shortcuts[1].Modifiers != ui.ModifierMeta {
		t.Fatalf("fallback shortcuts = %+v", shortcuts)
	}
}

func TestDriverShortcutChordPreservesEveryModifier(t *testing.T) {
	want := ui.ModifierMeta | ui.ModifierAlt | ui.ModifierShift
	var got ui.KeyEvent
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(*ui.Canvas, ui.Size) {},
		OnShortcut: func(event ui.KeyEvent) ui.EventResult {
			got = event
			return ui.EventResult{Handled: true}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	handled, err := driver.ShortcutChord(want, 'X')
	if err != nil || !handled {
		t.Fatalf("shortcut handled=%t err=%v", handled, err)
	}
	if got.Modifiers != want || got.Text != "x" {
		t.Fatalf("shortcut = %+v, want modifiers=%d text=x", got, want)
	}
}

func TestShortcutTextNormalizationIsPlatformIndependent(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{{"P", "p"}, {"7", "7"}, {"é", "é"}} {
		if got := normalizeShortcutText(test.input); got != test.want {
			t.Fatalf("normalizeShortcutText(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

// RunApplication opens its windows through prepareNativeWindow, not through the
// single-window path, and once handed the platform no shortcut handler: every
// application shortcut was dead in a real window while the headless driver, which
// calls the handler directly, still passed. Both paths take the handler from the
// painter now, and this is what they take.
func TestWindowsDeliverShortcutsToTheApplication(t *testing.T) {
	var got []ui.KeyEvent
	config := Config{
		Width: 320, Height: 180,
		OnShortcut: func(event ui.KeyEvent) ui.EventResult {
			got = append(got, event)
			return ui.EventResult{Handled: event.Text == "w"}
		},
	}
	normalized, err := config.normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainter(&ui.Canvas{}, &normalized)
	if err != nil {
		t.Fatal(err)
	}
	handler := painter.shortcutHandler(&normalized)
	if handler == nil {
		t.Fatal("the window was given no shortcut handler")
	}
	_, handled, err := handler(platform.KeyEvent{Modifiers: input.ModifierControl, Text: "W"})
	if err != nil || !handled {
		t.Fatalf("handled = %t, err = %v", handled, err)
	}
	if _, handled, _ := handler(platform.KeyEvent{Modifiers: input.ModifierControl, Text: "x"}); handled {
		t.Fatal("a chord the application did not take was reported handled")
	}
	if len(got) != 2 || got[0].Text != "w" || got[0].Modifiers != ui.ModifierControl {
		t.Fatalf("the application saw %+v", got)
	}
	// A window with no shortcut handler of its own does not claim chords.
	bare := Config{Width: 320, Height: 180}
	bareNormalized, err := bare.normalize()
	if err != nil {
		t.Fatal(err)
	}
	if painter.shortcutHandler(&bareNormalized) != nil {
		t.Fatal("a window without OnShortcut was given a handler")
	}
}

func BenchmarkDriverShortcutChord(b *testing.B) {
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(*ui.Canvas, ui.Size) {},
		OnShortcut: func(ui.KeyEvent) ui.EventResult {
			return ui.EventResult{Handled: true}
		},
	}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()

	b.ReportAllocs()
	for b.Loop() {
		handled, err := driver.ShortcutChord(ui.ModifierMeta|ui.ModifierAlt, 'x')
		if err != nil || !handled {
			b.Fatalf("shortcut handled=%t err=%v", handled, err)
		}
	}
}

// A message box that sends on Control+Enter needs the chord before the multiline
// field turns it into a line break; one the application leaves still reaches the
// field, and a plain Enter is never offered.
func TestControlEnterIsOfferedToTheApplicationBeforeAMultilineField(t *testing.T) {
	editor := text.NewEditor("hi")
	var offered []ui.KeyEvent
	send := true
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 200, Height: 90}, "Message", ui.HitOptions{Role: ui.RoleTextField, Multiline: true})
		},
		TextInput: func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnShortcut: func(event ui.KeyEvent) ui.EventResult {
			offered = append(offered, event)
			return ui.EventResult{Handled: send}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(ui.KeyEnter, ui.ModifierControl); err != nil {
		t.Fatal(err)
	}
	if len(offered) != 1 || offered[0].Key != ui.KeyEnter || offered[0].Modifiers != ui.ModifierControl || editor.String() != "hi" {
		t.Fatalf("offered %+v, text %q; the application should have taken Control+Enter", offered, editor.String())
	}
	send = false
	if err := driver.Key(ui.KeyEnter, ui.ModifierControl); err != nil {
		t.Fatal(err)
	}
	if len(offered) != 2 || editor.String() != "hi\n" {
		t.Fatalf("offered %d times, text %q; a chord the application leaves belongs to the field", len(offered), editor.String())
	}
	if err := driver.Key(ui.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if len(offered) != 2 {
		t.Fatal("a plain Enter was offered as a shortcut")
	}
}
