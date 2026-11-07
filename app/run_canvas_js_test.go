//go:build js && wasm

package app

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func captureWeb(t *testing.T) *web.Config {
	t.Helper()
	var got web.Config
	previous := runWeb
	runWeb = func(config web.Config) error { got = config; return nil }
	t.Cleanup(func() { runWeb = previous })
	return &got
}

func TestRunCanvasHandsTheApplicationToThePage(t *testing.T) {
	got := captureWeb(t)
	var tree ui.Tree
	var invalidator Invalidator
	canvas := &ui.Canvas{}
	woken := false
	err := RunCanvas(Config{
		Title: "Notes", Tree: &tree, Invalidator: &invalidator, OnWake: func() { woken = true },
		Paint:     func(*ui.Canvas, ui.Size) {},
		TextInput: func(TextInputContext) *TextInput { return nil },
	}, canvas)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Notes" || got.Width <= 0 || got.Height <= 0 {
		t.Errorf("page got title %q and size %dx%d, want the title and the default size", got.Title, got.Width, got.Height)
	}
	if got.Canvas != canvas {
		t.Error("the page must paint into the caller's canvas")
	}
	if got.Paint == nil || got.TextInput == nil {
		t.Error("Paint and TextInput must reach the page")
	}
	if got.Waker == nil {
		t.Fatal("an Invalidator must reach the page as its waker")
	}
	got.OnWake()
	if !woken {
		t.Error("OnWake must reach the page")
	}
	// What the page binds is what Invalidator.Invalidate calls.
	called := false
	got.Waker.Bind(func() { called = true })
	invalidator.Invalidate()
	if !called {
		t.Error("Invalidate must wake the page")
	}
}

func TestRunCanvasRejectsWhatNativeRejects(t *testing.T) {
	got := captureWeb(t)
	if err := RunCanvas(Config{Width: -1, Height: 10}, nil); err == nil {
		t.Error("a negative window size must be an error, as on every platform")
	}
	if got.Paint != nil {
		t.Error("the page must not start for an invalid configuration")
	}
}

func TestRunStartsThePageAsTheWindow(t *testing.T) {
	got := captureWeb(t)
	if err := Run(Config{Title: "Static"}); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Static" || got.Paint != nil {
		t.Fatalf("Run must mount a page with the config and no paint callback, got %q %v", got.Title, got.Paint != nil)
	}
}

func TestWebTextInputConnectsAnEditorToTheBrowser(t *testing.T) {
	editor := text.NewEditor("hello")
	provider := webTextInput(func(context TextInputContext) *TextInput {
		if context.ID != ui.Item(7) {
			return nil
		}
		return &TextInput{Editor: editor}
	})
	if provider(web.TextInputContext{ID: ui.Item(8)}) != nil {
		t.Fatal("a field the application does not edit must not become a browser editor")
	}
	input := provider(web.TextInputContext{ID: ui.Item(7), Bounds: ui.Rect{X: 5, Y: 6, Width: 100, Height: 20}, FontSize: 14})
	if input == nil || input.State.Value != "hello" || input.Target != ui.Item(7) {
		t.Fatalf("the browser must start from the editor's text, got %+v", input)
	}
	state, err := input.OnEvent(platform.TextEvent{Kind: platform.TextInsert, Text: " there"})
	if err != nil {
		t.Fatal(err)
	}
	if editor.String() != "hello there" || state.Value != "hello there" {
		t.Errorf("an edit from the browser must reach the editor: editor %q, state %q", editor.String(), state.Value)
	}
}

func TestWebTextInputPreservesAlignment(t *testing.T) {
	for _, testcase := range []struct {
		name      string
		value     string
		alignment ui.Alignment
		want      ui.Alignment
	}{
		{"right", "hello", ui.AlignRight, ui.AlignRight},
		{"start RTL", "שלום", ui.AlignStart, ui.AlignRight},
		{"end RTL", "שלום", ui.AlignEnd, ui.AlignLeft},
		{"end LTR", "hello", ui.AlignEnd, ui.AlignRight},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			provider := webTextInput(func(TextInputContext) *TextInput {
				return &TextInput{Editor: text.NewEditor(testcase.value), Alignment: testcase.alignment}
			})
			input := provider(web.TextInputContext{ID: ui.Item(7), Bounds: ui.Rect{Width: 200, Height: 30}})
			if input == nil || input.Alignment != testcase.want {
				t.Fatalf("browser alignment = %+v, want %v", input, testcase.want)
			}
		})
	}
}

func TestWebTextInputPreservesPaintedEditorOverlays(t *testing.T) {
	editor := text.NewEditor("source")
	provider := webTextInput(func(context TextInputContext) *TextInput {
		if !context.HideNativeOverlays {
			t.Fatal("the painted editor must declare that it owns the caret and selection")
		}
		return &TextInput{Editor: editor}
	})
	input := provider(web.TextInputContext{ID: ui.Item(1), HideNativeOverlays: true})
	if input == nil || !input.HideNativeOverlays {
		t.Fatal("browser text input must not draw a second caret or selection over the canvas")
	}
}

func TestWebTextInputPassesEventsToAHandlerThatOwnsThem(t *testing.T) {
	var seen TextEvent
	provider := webTextInput(func(TextInputContext) *TextInput {
		return &TextInput{
			State: TextState{Value: "abc"},
			OnEvent: func(event TextEvent) (TextState, error) {
				seen = event
				return TextState{Value: "abcX"}, nil
			},
		}
	})
	input := provider(web.TextInputContext{ID: ui.Item(1)})
	state, err := input.OnEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if seen.Kind != TextInsert || seen.Text != "X" || state.Value != "abcX" {
		t.Errorf("handler saw %+v and answered %q", seen, state.Value)
	}
}

func TestWebTextInputKeepsTerminalKeyboardSemantics(t *testing.T) {
	provider := webTextInput(func(TextInputContext) *TextInput {
		return &TextInput{KeyReleases: true, AltSendsEscape: true}
	})
	input := provider(web.TextInputContext{ID: ui.Item(1), Terminal: true})
	if input == nil || !input.Terminal || !input.KeyReleases || !input.AltSendsEscape {
		t.Fatalf("terminal input = %+v, want Terminal and KeyReleases and AltSendsEscape", input)
	}
	typeAhead := provider(web.TextInputContext{ID: ui.Item(2), TypeAhead: true})
	if typeAhead == nil || !typeAhead.Terminal || typeAhead.KeyReleases {
		t.Fatalf("type-ahead input = %+v, want Terminal without releases", typeAhead)
	}
}
