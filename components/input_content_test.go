package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

// The content of a field is what PaintInput paints inside its border: the same
// texts, selection and caret, over whatever rectangle the caller chose. A field
// the application dresses itself must edit exactly as the shadcn one does.
func TestInputContentPaintsTheTextOfInputWithoutItsSurface(t *testing.T) {
	theme := ui.LightTheme()
	snapshot := InputSnapshot{
		ID: ui.Item(7), Label: "Email", Value: "user@example.com", Invalid: true, ShowCaret: true,
		Caret: 4, SelectionStart: 0, SelectionEnd: 4,
	}
	bounds := ui.Rect{X: 10, Y: 20, Width: 180, Height: 32}
	inset := theme.Metrics.TextInsetX()
	content := ui.Rect{X: bounds.X + inset, Y: bounds.Y, Width: bounds.Width - 2*inset, Height: bounds.Height}

	var full, bare ui.Canvas
	full.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(7), true)
	bare.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(7), true)
	PaintInput(&full, theme, bounds, snapshot)
	PaintInputContent(&bare, theme, bounds, content, snapshot)

	if !slices.Equal(full.Texts(), bare.Texts()) {
		t.Fatalf("texts differ:\nfull %+v\nbare %+v", full.Texts(), bare.Texts())
	}
	// The surface is the full field's first shapes; the content adds nothing else of its own.
	surface := len(full.Rectangles()) - len(bare.Rectangles())
	if surface <= 0 {
		t.Fatalf("the full field painted %d rectangles and the content %d: no surface told apart", len(full.Rectangles()), len(bare.Rectangles()))
	}
	if !slices.Equal(full.Rectangles()[surface:], bare.Rectangles()) {
		t.Fatalf("selection and caret differ:\nfull %+v\nbare %+v", full.Rectangles()[surface:], bare.Rectangles())
	}
}

// The pointer hits the whole control and the platform puts the caret and the input
// method in the content rectangle, so the value does not move when the field opens.
func TestInputContentHitFollowsTheCallersRectangles(t *testing.T) {
	theme := ui.LightTheme()
	bounds := ui.Rect{X: 2, Y: 1, Width: 100, Height: 24}
	content := ui.Rect{X: 5, Y: 3, Width: 90, Height: 20}
	var canvas ui.Canvas
	PaintInputContent(&canvas, theme, bounds, content, InputSnapshot{ID: ui.Item(2), Label: "Cell", Value: "x", Hint: "h", ReadOnly: true})
	if len(canvas.Hits()) != 1 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	hit := canvas.Hits()[0]
	if hit.Role != ui.RoleTextField || hit.Bounds != bounds || hit.TextBounds != content || hit.Cursor != ui.PointerCursorText || !hit.ReadOnly || hit.Hint != "h" {
		t.Fatalf("hit = %+v, want a text field over %+v with its text in %+v", hit, bounds, content)
	}
	if hit.TextSize != theme.Metrics.FontSizeM {
		t.Fatalf("text size = %v, want the theme's %v", hit.TextSize, theme.Metrics.FontSizeM)
	}
}

func TestInputContentWithoutIDDeclaresNoHit(t *testing.T) {
	var canvas ui.Canvas
	PaintInputContent(&canvas, ui.LightTheme(), ui.Rect{Width: 90, Height: 20}, ui.Rect{Width: 90, Height: 20}, InputSnapshot{Value: "preview"})
	if len(canvas.Hits()) != 0 || len(canvas.Texts()) != 1 {
		t.Fatalf("hits = %+v texts = %+v", canvas.Hits(), canvas.Texts())
	}
}

func TestInputContentPlatformEditingLeavesTheTextToThePlatform(t *testing.T) {
	var canvas ui.Canvas
	PaintInputContent(&canvas, ui.LightTheme(), ui.Rect{Width: 90, Height: 20}, ui.Rect{Width: 90, Height: 20}, InputSnapshot{ID: ui.Item(2), Value: "x", PlatformEditing: true})
	if len(canvas.Texts()) != 0 || len(canvas.Hits()) != 1 {
		t.Fatalf("texts = %+v hits = %+v", canvas.Texts(), canvas.Hits())
	}
}

func TestInputContentSecureNeverExposesTheValue(t *testing.T) {
	var canvas ui.Canvas
	PaintInputContent(&canvas, ui.LightTheme(), ui.Rect{Width: 90, Height: 20}, ui.Rect{Width: 90, Height: 20}, InputSnapshot{ID: ui.Item(2), Value: "hunter2", Secure: true})
	if canvas.Hits()[0].Value != "" {
		t.Fatalf("hit exposes the value: %+v", canvas.Hits()[0])
	}
	for _, text := range canvas.Texts() {
		if text.Value == "hunter2" {
			t.Fatalf("secure value painted: %+v", text)
		}
	}
}

func TestInputContentIgnoresAnEmptyControl(t *testing.T) {
	var canvas ui.Canvas
	PaintInputContent(&canvas, ui.LightTheme(), ui.Rect{}, ui.Rect{}, InputSnapshot{ID: ui.Item(2), Value: "x"})
	PaintInputContent(nil, ui.LightTheme(), ui.Rect{Width: 10, Height: 10}, ui.Rect{Width: 10, Height: 10}, InputSnapshot{})
	if len(canvas.Hits()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("painted into nothing: %+v %+v", canvas.Hits(), canvas.Texts())
	}
}

// Padding larger than the control leaves the text no room: nothing is drawn, but the
// control is still a field, so a click focuses it and the keyboard reaches it.
func TestInputContentWithNoRoomForTextStillHits(t *testing.T) {
	var canvas ui.Canvas
	PaintInputContent(&canvas, ui.LightTheme(), ui.Rect{Width: 20, Height: 20}, ui.Rect{}, InputSnapshot{ID: ui.Item(2), Value: "x"})
	if len(canvas.Texts()) != 0 || len(canvas.Hits()) != 1 || canvas.Hits()[0].Role != ui.RoleTextField {
		t.Fatalf("texts = %+v hits = %+v", canvas.Texts(), canvas.Hits())
	}
}

func TestInputContentPaintIsAllocationFreeAfterWarmup(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	content := ui.Rect{Width: 200, Height: 32}
	snapshot := InputSnapshot{ID: ui.Item(1), Value: "Input", Focused: true, ShowCaret: true, Caret: 2, SelectionStart: 1, SelectionEnd: 3}
	PaintInputContent(&canvas, theme, content, content, snapshot)
	allocs := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		PaintInputContent(&canvas, theme, content, content, snapshot)
	})
	if allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}

func BenchmarkInputPaintContent(b *testing.B) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	content := ui.Rect{Width: 200, Height: 32}
	snapshot := InputSnapshot{ID: ui.Item(1), Value: "Input", Focused: true}
	PaintInputContent(&canvas, theme, content, content, snapshot)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.Reset()
		PaintInputContent(&canvas, theme, content, content, snapshot)
	}
}
