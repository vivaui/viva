//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestEditingPriorityBelongsToTheBackend(t *testing.T) {
	editor := text.NewEditor("value")
	calls := 0
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", gfx.HitOptions{Role: gfx.RoleTextField})
		},
		TextInput:  func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnShortcut: func(gfx.KeyEvent) gfx.EventResult { calls++; return gfx.EventResult{Handled: true} },
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.ShortcutChord(gfx.ModifierControl, 'a'); !handled || err != nil {
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
	var shortcuts []gfx.KeyEvent
	config := Config{
		Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", gfx.HitOptions{Role: gfx.RoleTextField})
		},
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			shortcuts = append(shortcuts, event)
			return gfx.EventResult{Handled: event.Text == "k", Repaint: event.Text == "k"}
		},
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Chord(gfx.ModifierControl, "K"); err != nil {
		t.Fatal(err)
	}
	if len(shortcuts) != 1 || shortcuts[0].Text != "k" || shortcuts[0].Modifiers != gfx.ModifierControl {
		t.Fatalf("shortcuts = %+v", shortcuts)
	}

	if handled, err := driver.Shortcut('c'); err != nil || handled {
		t.Fatalf("unfocused native editing shortcut = %t, %v", handled, err)
	}
	if len(shortcuts) != 2 || shortcuts[1].Text != "c" || shortcuts[1].Modifiers != gfx.ModifierMeta {
		t.Fatalf("fallback shortcuts = %+v", shortcuts)
	}
}

func TestPrintableShortcutsLeaveUnhandledTextAndCompositionNative(t *testing.T) {
	editor := text.NewEditor("")
	calls := 0
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", gfx.HitOptions{Role: gfx.RoleTextField})
		},
		TextInput: func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			calls++
			return gfx.EventResult{Handled: event.Text == "x"}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	for _, modifiers := range []gfx.Modifiers{0, gfx.ModifierShift, gfx.ModifierAlt} {
		if err := driver.Chord(modifiers, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 || editor.String() != "" {
		t.Fatalf("bound keys: calls=%d editor=%q", calls, editor.String())
	}
	if err := driver.Chord(0, "a"); err != ErrNoTerminal {
		t.Fatalf("unhandled key did not return to text handling: %v", err)
	}
	if err := driver.Type("a"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Compose("拼音"); err != nil {
		t.Fatal(err)
	}
	before := calls
	if err := driver.Chord(0, "x"); err != ErrNoTerminal || calls != before {
		t.Fatalf("composition reached shortcut handler: calls=%d err=%v", calls, err)
	}
	if err := driver.Type("中文"); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "a中文" {
		t.Fatalf("native committed text = %q", editor.String())
	}
}

func TestDriverShortcutChordPreservesEveryModifier(t *testing.T) {
	want := gfx.ModifierMeta | gfx.ModifierAlt | gfx.ModifierShift
	var got gfx.KeyEvent
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(*gfx.Canvas, gfx.Size) {},
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			got = event
			return gfx.EventResult{Handled: true}
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
	var got []gfx.KeyEvent
	config := Config{
		Width: 320, Height: 180,
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			got = append(got, event)
			return gfx.EventResult{Handled: event.Text == "w"}
		},
	}
	normalized, err := config.normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainter(&gfx.Canvas{}, &normalized)
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
	if len(got) != 2 || got[0].Text != "w" || got[0].Modifiers != gfx.ModifierControl {
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
		Paint: func(*gfx.Canvas, gfx.Size) {},
		OnShortcut: func(gfx.KeyEvent) gfx.EventResult {
			return gfx.EventResult{Handled: true}
		},
	}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()

	b.ReportAllocs()
	for b.Loop() {
		handled, err := driver.ShortcutChord(gfx.ModifierMeta|gfx.ModifierAlt, 'x')
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
	var offered []gfx.KeyEvent
	send := true
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 200, Height: 90}, "Message", gfx.HitOptions{Role: gfx.RoleTextField, Multiline: true})
		},
		TextInput: func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			offered = append(offered, event)
			return gfx.EventResult{Handled: send}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, gfx.ModifierControl); err != nil {
		t.Fatal(err)
	}
	if len(offered) != 1 || offered[0].Key != gfx.KeyEnter || offered[0].Modifiers != gfx.ModifierControl || editor.String() != "hi" {
		t.Fatalf("offered %+v, text %q; the application should have taken Control+Enter", offered, editor.String())
	}
	send = false
	if err := driver.Key(gfx.KeyEnter, gfx.ModifierControl); err != nil {
		t.Fatal(err)
	}
	if len(offered) != 2 || editor.String() != "hi\n" {
		t.Fatalf("offered %d times, text %q; a chord the application leaves belongs to the field", len(offered), editor.String())
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if len(offered) != 2 {
		t.Fatal("a plain Enter was offered as a shortcut")
	}
}
