package app

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestPainterTentativeHandoffRetainsInstalledOwner(t *testing.T) {
	for _, policy := range []string{"document", "terminal", "secure"} {
		for _, mode := range []string{"editor replacement", "cached provider", "cached external switch", "provider disabled", "target replacement", "subpixel disabled", "provider removed", "same owner", "geometry only"} {
			t.Run(policy+"/"+mode, func(t *testing.T) {
				first, second := text.NewEditor("abc"), text.NewEditor("other")
				if !first.EnableTentativeComposition() || !second.EnableTentativeComposition() {
					t.Fatal("capability refused")
				}
				first.Insert("!")
				first.SetSelection(text.Range{Start: 1, End: 2})
				second.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "incoming"})
				incoming := second.String()
				current := first
				var extraWidth float32
				var canvas gfx.Canvas
				cached := &TextInput{Bounds: gfx.Rect{Width: 120, Height: 24}, FontSize: 13, Editor: first}
				role := gfx.RoleTextField
				if policy == "terminal" {
					role = gfx.RoleTerminal
				}
				config := Config{Title: "composition handoff", Width: 200, Height: 80}
				config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
					canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 24}, "First", gfx.HitOptions{Role: role, Autofocus: true, Secure: policy == "secure"})
					canvas.HitWithOptions(gfx.Item(12), gfx.Rect{Y: 30, Width: 120, Height: 24}, "Second", gfx.HitOptions{Role: role})
				}
				config.TextInput = func(context TextInputContext) *TextInput {
					if mode == "cached external switch" {
						return cached
					}
					if current == nil {
						return nil
					}
					bounds := context.Bounds
					bounds.Width += extraWidth
					if mode == "cached provider" {
						*cached = TextInput{Bounds: bounds, FontSize: 13, Editor: current}
						return cached
					}
					return &TextInput{Bounds: bounds, FontSize: 13, Editor: current}
				}
				owner := newTestPainter(t, &canvas, config)
				owner.syncTextInput()
				if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetMarked, Text: "ni", CompositionPhase: text.CompositionUpdate}); err != nil {
					t.Fatal(err)
				}
				if first.String() != "anic!" || !first.TentativeCompositionActive() {
					t.Fatal("preedit did not start")
				}
				switch mode {
				case "editor replacement", "cached provider":
					current = second
				case "cached external switch":
					current, cached.Editor = second, second
				case "provider disabled":
					current = nil
				case "target replacement":
					owner.focus.Apply(gfx.EventResult{Focus: gfx.Item(12)})
				case "subpixel disabled":
					extraWidth = -119.7
				case "provider removed":
					owner.textInput = nil
				case "geometry only":
					extraWidth = 10
				}
				update := owner.syncTextInput()
				if mode == "same owner" || mode == "geometry only" {
					if !first.TentativeCompositionActive() || first.String() != "anic!" {
						t.Fatal("unchanged owner lost its live composition")
					}
					return
				}
				want := "anic!"
				if policy == "terminal" {
					want = "ac!"
				}
				if first.CompositionActive() || first.TentativeCompositionActive() || first.String() != want || first.CommittedString() != want {
					t.Fatalf("outgoing owner not settled: text=%q committed=%q active=%t, want %q", first.String(), first.CommittedString(), first.CompositionActive(), want)
				}
				if mode == "provider disabled" || mode == "provider removed" || mode == "subpixel disabled" {
					if !update.Changed || update.Input != nil || owner.textEnabled || owner.activeText != nil {
						t.Fatal("disabled input did not release the native owner")
					}
				}
				if mode == "target replacement" && owner.nativeText.State.Composing {
					t.Fatal("same editor's new target published a stale composing snapshot")
				}
				if second.String() != incoming || !second.TentativeCompositionActive() {
					t.Fatal("handoff settled the incoming editor")
				}
				if policy == "secure" {
					if first.CanUndo() || first.CanRedo() {
						t.Fatal("secure handoff retained committed history")
					}
					return
				}
				undo, redo := []string{"abc!", "abc"}, []string{"abc!", want}
				if policy == "document" {
					undo, redo = []string{"ac!", "abc!", "abc"}, []string{"abc!", "ac!", want}
				}
				for _, value := range undo {
					if !first.Undo() || first.String() != value {
						t.Fatalf("handoff Undo = %q, want %q", first.String(), value)
					}
				}
				for _, value := range redo {
					if !first.Redo() || first.String() != value {
						t.Fatalf("handoff Redo = %q, want %q", first.String(), value)
					}
				}
			})
		}
	}
}

// A cached provider can hand back a different Editor than the one the
// platform was told about, before the next sync. A native text event arrives
// on the window thread in that window: it belongs to the installed owner, not
// to whatever the last published TextInput now names.
func TestPainterAppliesTextEventsToTheInstalledOwner(t *testing.T) {
	installed := text.NewEditor("abc")
	other := text.NewEditor("other")
	cached := &TextInput{Editor: installed, Bounds: gfx.Rect{Width: 120, Height: 24}, FontSize: 13}
	var canvas gfx.Canvas
	config := Config{Title: "installed owner", Width: 200, Height: 80}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 24}, "First", gfx.HitOptions{Role: gfx.RoleTextField, Autofocus: true})
	}
	config.TextInput = func(TextInputContext) *TextInput { return cached }
	owner := newTestPainter(t, &canvas, config)
	owner.syncTextInput()
	if owner.activeEditor != installed || owner.activeText == nil || owner.activeText.Editor != installed {
		t.Fatal("the painter did not install the cached editor")
	}
	// The provider swaps its cached Editor between paints; the platform still
	// talks to the installed one.
	cached.Editor = other
	owner.activeText.Editor = other
	if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := installed.String(); got != "abcx" {
		t.Fatalf("installed owner = %q, want the event applied to it", got)
	}
	if got := other.String(); got != "other" {
		t.Fatalf("the swapped-in editor = %q, want it untouched", got)
	}
}

func TestPainterCachedCustomHandoffPreservesProviderOwnership(t *testing.T) {
	first, second := text.NewEditor("first"), text.NewEditor("second")
	for _, editor := range []*text.Editor{first, second} {
		editor.EnableTentativeComposition()
		editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "ni"})
	}
	cached := &TextInput{State: TextStateOf(first), OnEvent: func(event TextEvent) (TextState, error) {
		t.Fatal("custom owner received an unsolicited handoff event")
		return TextState{}, nil
	}}
	var canvas gfx.Canvas
	config := Config{Title: "custom cached handoff", Width: 200, Height: 80}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 24}, "First", gfx.HitOptions{Role: gfx.RoleTextField, Autofocus: true})
		canvas.HitWithOptions(gfx.Item(12), gfx.Rect{Y: 30, Width: 120, Height: 24}, "Second", gfx.HitOptions{Role: gfx.RoleTextField})
	}
	config.TextInput = func(TextInputContext) *TextInput { return cached }
	owner := newTestPainter(t, &canvas, config)
	owner.syncTextInput()
	cached.Editor = second
	owner.focus.Apply(gfx.EventResult{Focus: gfx.Item(12)})
	owner.syncTextInput()
	if !first.TentativeCompositionActive() || !second.TentativeCompositionActive() {
		t.Fatal("cached custom handoff settled a provider-owned or incoming transaction")
	}
}

func BenchmarkPainterTextInputSync(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			editor := text.NewEditor("steady input")
			if enabled {
				editor.EnableTentativeComposition()
			}
			cached := &TextInput{Editor: editor, Bounds: gfx.Rect{Width: 120, Height: 24}, FontSize: 13}
			var canvas gfx.Canvas
			canvas.HitWithOptions(gfx.Item(11), cached.Bounds, "Field", gfx.HitOptions{Role: gfx.RoleTextField, Autofocus: true})
			owner := &painter{canvas: &canvas, textInput: func(TextInputContext) *TextInput { return cached }}
			owner.focus.Sync(&canvas)
			owner.syncTextInput()
			if !owner.textEnabled {
				b.Fatal("input not enabled")
			}
			b.ReportAllocs()
			for b.Loop() {
				owner.syncTextInput()
			}
			b.ReportMetric(float64(unsafe.Sizeof(painter{})), "painter-B")
		})
	}
}
