package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

func TestTextareaCapturesMultilineDisplayAndSemantics(t *testing.T) {
	var canvas ui.Canvas
	bounds := PaintTextarea(&canvas, ui.LightTheme(), ui.Rect{X: 8, Y: 9, Width: 220, Height: 20}, TextareaSnapshot{
		ID: ui.Item(3), Label: "Biography", Value: "Go\n桌面", Hint: "Short bio", ReadOnly: true,
		Focused: true, ShowCaret: true, Caret: 2, SelectionStart: 0, SelectionEnd: 2,
		CompositionStart: 3, CompositionEnd: len("Go\n桌面"),
	})
	if bounds.Height != 64 {
		t.Fatalf("height = %v", bounds.Height)
	}
	if len(canvas.Texts()) != 3 || canvas.Texts()[0].Value != "Go" || canvas.Texts()[2].Value != "桌面" {
		t.Fatalf("texts = %+v", canvas.Texts())
	}
	hit := canvas.Hits()[0]
	if hit.Role != ui.RoleTextField || !hit.Multiline || !hit.ReadOnly || hit.Value != "Go\n桌面" {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestTextareaDisabledPlaceholder(t *testing.T) {
	var canvas ui.Canvas
	PaintTextarea(&canvas, ui.DarkTheme(), ui.Rect{Width: 160}, TextareaSnapshot{
		ID: ui.Item(1), Label: "Notes", Placeholder: "Add notes", Disabled: true,
	})
	if text := canvas.Texts()[0]; text.Value != "Add notes" || text.Style.Color.Alpha >= ui.DarkTheme().MutedForeground.Alpha {
		t.Fatalf("placeholder = %+v", text)
	}
	if hit := canvas.Hits()[0]; !hit.Disabled || hit.Focusable || !hit.Multiline {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestTextareaTypedSemanticsAndCommandBoundary(t *testing.T) {
	snapshot := TextareaSnapshot{
		ID: ui.Item(7), Label: "Notes", Value: "A中B", DisplayValue: "A中B", Hint: "Details",
		Focused: true, Invalid: true, SelectionStart: 2, SelectionEnd: 4,
		CompositionStart: 5, CompositionEnd: 1,
	}
	want := TextareaSemantics{
		ID: ui.Item(7), Label: "Notes", Value: "A中B", Hint: "Details", Focused: true, Invalid: true,
		SelectionStart: 1, SelectionEnd: 4, CompositionStart: 1, CompositionEnd: 5,
	}
	if got := snapshot.Semantics(); got != want {
		t.Fatalf("semantics = %+v, want %+v", got, want)
	}
	if command, handled := TextareaCommandForKey(snapshot, input.KeyEnter); command != TextareaCommandInsertNewline || !handled {
		t.Fatalf("enter = (%v, %v)", command, handled)
	}
	for _, inert := range []TextareaSnapshot{{Disabled: true}, {ReadOnly: true}} {
		if command, handled := TextareaCommandForKey(inert, input.KeyEnter); command != TextareaCommandNone || handled {
			t.Fatalf("inert enter = (%v, %v)", command, handled)
		}
	}
	if command, handled := TextareaCommandForKey(snapshot, input.KeyTab); command != TextareaCommandNone || handled {
		t.Fatalf("tab = (%v, %v)", command, handled)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		_ = snapshot.Semantics()
		TextareaCommandForKey(snapshot, input.KeyEnter)
	}); allocations != 0 {
		t.Fatalf("textarea helpers allocations = %v", allocations)
	}
}

func TestTextareaNativeEditingLeavesGlyphsToPlatform(t *testing.T) {
	var canvas ui.Canvas
	PaintTextarea(&canvas, ui.LightTheme(), ui.Rect{Width: 180, Height: 80}, TextareaSnapshot{
		ID: ui.Item(9), Label: "Notes", Value: "native\ntext", NativeEditing: true, Focused: true,
	})
	if len(canvas.Texts()) != 0 {
		t.Fatalf("native editing painted duplicate text: %+v", canvas.Texts())
	}
	if hits := canvas.Hits(); len(hits) != 1 || hits[0].Value != "native\ntext" || !hits[0].Multiline {
		t.Fatalf("native editing semantics = %+v", hits)
	}
}

func TestTextareaPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		PaintTextarea(&canvas, ui.LightTheme(), ui.Rect{X: 1, Y: 2, Width: 180, Height: 80}, TextareaSnapshot{
			ID: ui.Item(4), Label: "Message", Value: "first\nsecond", Focused: true,
			ShowCaret: true, Caret: 8, SelectionStart: 1, SelectionEnd: 4,
			CompositionStart: 6, CompositionEnd: 12,
		})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical textarea snapshots produced different streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("textarea paint allocations = %v", allocations)
	}
}

func TestTextareaCanHideThePlatformEditorAndPaintItsOwnCaret(t *testing.T) {
	theme := ui.DarkTheme()
	paint := func(snapshot TextareaSnapshot) (*ui.Canvas, ui.Hit) {
		canvas := &ui.Canvas{}
		snapshot.ID = ui.Item(1)
		snapshot.Focused = true
		PaintTextarea(canvas, theme, ui.Rect{Width: 300, Height: 120}, snapshot)
		hits := canvas.Hits()
		if len(hits) != 1 {
			t.Fatalf("%d hits", len(hits))
		}
		return canvas, hits[0]
	}
	_, shown := paint(TextareaSnapshot{Value: "hi"})
	if shown.TextColor.Alpha == 0 {
		t.Fatal("the editor's text is invisible by default")
	}
	canvas, hidden := paint(TextareaSnapshot{Value: "hi", HideEditorText: true, ShowCaret: true, Caret: 2})
	if hidden.TextColor.Alpha != 0 {
		t.Errorf("HideEditorText left the editor's text visible: %+v", hidden.TextColor)
	}
	if !hidden.HideNativeOverlays {
		t.Error("HideEditorText did not hide the platform caret and selection")
	}
	if len(canvas.Texts()) == 0 {
		t.Error("the component did not paint the text it hid from the platform")
	}

	// An empty box shows its caret in front of the hint.
	empty, _ := paint(TextareaSnapshot{Placeholder: "Ask", ShowCaret: true})
	plain, _ := paint(TextareaSnapshot{Placeholder: "Ask"})
	if len(empty.Rectangles()) != len(plain.Rectangles())+1 {
		t.Errorf("caret on an empty box: %d rectangles against %d without it", len(empty.Rectangles()), len(plain.Rectangles()))
	}
}

func TestSelectedTextIsDrawnOnAnOpaqueSelection(t *testing.T) {
	// The selected text uses the semantic selection pair rather than the button
	// accent: a product can use a neutral action colour and still need a
	// readable editor selection.
	theme := ui.DarkTheme()
	for name, paint := range map[string]func(*ui.Canvas){
		"textarea": func(canvas *ui.Canvas) {
			PaintTextarea(canvas, theme, ui.Rect{Width: 200, Height: 80}, TextareaSnapshot{Value: "select me", SelectionStart: 0, SelectionEnd: 6})
		},
		"input": func(canvas *ui.Canvas) {
			PaintInput(canvas, theme, ui.Rect{Width: 200, Height: 32}, InputSnapshot{Value: "select me", SelectionStart: 0, SelectionEnd: 6})
		},
	} {
		var canvas ui.Canvas
		paint(&canvas)
		foundBackground, foundForeground := false, false
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Selection {
				foundBackground = true
			}
		}
		for _, text := range canvas.Texts() {
			if text.Style.Color == theme.SelectionForeground {
				foundForeground = true
			}
		}
		if !foundBackground {
			t.Errorf("%s: no selection filled with the semantic Selection color", name)
		}
		if !foundForeground {
			t.Errorf("%s: selected text did not use SelectionForeground", name)
		}
	}
}
