package app

import (
	"strconv"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/internal/semantic"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestPointerFocusDoesNotFlashKeyboardRing(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "focus", Width: 240, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 90}).Default("Save")
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(2), ui.Rect{X: 110, Y: 10, Width: 90}).Secondary("Cancel")
	}
	painter := newTestPainter(t, &canvas, config)
	wantBounds := canvas.Rectangles()[0].Bounds
	wantCount := len(canvas.Rectangles())
	for _, kind := range []platform.PointerEventKind{platform.PointerDown, platform.PointerUp} {
		if _, err := painter.pointer(platform.PointerEvent{Kind: kind, X: 20, Y: 20}); err != nil {
			t.Fatal(err)
		}
		if len(canvas.Rectangles()) != wantCount || canvas.Rectangles()[0].Bounds != wantBounds || !canvas.FocusVisible().IsZero() || canvas.Focused() != ui.Item(1) {
			t.Fatal("mouse interaction changed geometry or painted a keyboard focus ring")
		}
		if focused, ok := painter.router.Focused(); !ok || focused != ui.Item(1) {
			t.Fatal("mouse focus must remain in the shared input router")
		}
	}
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if canvas.FocusVisible() != ui.Item(2) || canvas.Focused() != ui.Item(2) || len(canvas.Rectangles()) <= wantCount {
		t.Fatal("Tab should move to the next control and expose its keyboard focus ring")
	}
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 120, Y: 20}); err != nil {
		t.Fatal(err)
	}
	if !canvas.FocusVisible().IsZero() || canvas.Focused() != ui.Item(2) || len(canvas.Rectangles()) != wantCount {
		t.Fatal("switching back to mouse input should hide the keyboard focus ring")
	}
}

func TestFocusTrapEscapeDismissesAndRestoresFocus(t *testing.T) {
	var canvas ui.Canvas
	open := false
	var clicks []ui.ID
	config := Config{Title: "overlay", Width: 160, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{Width: 50, Height: 20}, "Open")
		if !open {
			return
		}
		canvas.PushFocusScope(ui.Item(99))
		canvas.HitWithLabel(ui.Item(2), ui.Rect{X: 60, Width: 50, Height: 20}, "Close")
		canvas.PopFocusScope()
		canvas.TrapFocus(ui.Item(99))
		canvas.SetEscapeTarget(ui.Item(2))
	}
	onClick(&config, func(id ui.ID) {
		clicks = append(clicks, id)
		if id == ui.Item(2) {
			open = false
		}
	})
	painter := newTestPainter(t, &canvas, config)
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(1) {
		t.Fatalf("outer focus = %d", painter.focused)
	}

	open = true
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if painter.router.ActiveScope() != ui.Item(99) || !painter.focused.IsZero() {
		t.Fatalf("opened scope = %d, focus = %d", painter.router.ActiveScope(), painter.focused)
	}
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(2) {
		t.Fatalf("overlay focus = %d", painter.focused)
	}
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyEscape}); err != nil {
		t.Fatal(err)
	}
	if open || len(clicks) != 1 || clicks[0] != ui.Item(2) {
		t.Fatalf("escape result open=%v clicks=%v", open, clicks)
	}
	if !painter.router.ActiveScope().IsZero() || painter.focused != ui.Item(1) {
		t.Fatalf("restored scope = %d, focus = %d", painter.router.ActiveScope(), painter.focused)
	}
}

func TestTypedHitPublishesSemanticRoleAndState(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "semantics", Width: 120, Height: 60}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(7), ui.Rect{Width: 100, Height: 24}, "Updates", ui.HitOptions{
			Role: ui.RoleCheckbox, Value: "enabled", Hint: "Receive updates",
			Checked: true, Selected: true, Expanded: true,
		})
	}
	painter := newTestPainter(t, &canvas, config)
	if len(painter.accessibility) != 1 {
		t.Fatalf("accessibility = %+v", painter.accessibility)
	}
	node := painter.accessibility[0]
	if node.Role != uint8(semantic.RoleCheckbox) || node.Value != "enabled" || node.Hint != "Receive updates" || !node.Checked || !node.Selected || !node.Expanded {
		t.Fatalf("typed accessibility = %+v", node)
	}
}

func TestFocusedTextHitPublishesNativeEditorAndDisablesOnBlur(t *testing.T) {
	var canvas ui.Canvas
	input := TextInput{
		Bounds:   ui.Rect{X: 12, Y: 10, Width: 76, Height: 32},
		FontSize: 13, Color: ui.Color{Alpha: 1},
		State: TextState{Value: "Viva", Selection: TextRange{Start: 3, End: 3}},
	}
	input.OnEvent = func(event TextEvent) (TextState, error) {
		if event.Kind != TextInsert || event.Text != " 原生" {
			t.Fatalf("text event = %+v", event)
		}
		return TextState{Value: "Viva 原生", Selection: TextRange{Start: 6, End: 6}}, nil
	}
	config := Config{Title: "text", Width: 180, Height: 70}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(7), ui.Rect{X: 10, Y: 10, Width: 100, Height: 32}, "Name", ui.HitOptions{Role: ui.RoleTextField})
		canvas.HitWithLabel(ui.Item(8), ui.Rect{X: 120, Y: 10, Width: 50, Height: 32}, "Save")
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if context.ID != ui.Item(7) || context.Bounds != (ui.Rect{X: 10, Y: 10, Width: 100, Height: 32}) {
			t.Fatalf("text context = %+v", context)
		}
		return &input
	}
	painter := newTestPainter(t, &canvas, config)
	frame, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 20, Y: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !frame.TextInput.Changed || frame.TextInput.Input == nil || frame.TextInput.Input.State.Value != "Viva" {
		t.Fatalf("text input update = %+v", frame.TextInput)
	}
	next, err := frame.TextInput.Input.OnEvent(platform.TextEvent{Kind: platform.TextInsert, Text: " 原生"})
	if err != nil || next.Value != "Viva 原生" || input.State.Value != "Viva 原生" {
		t.Fatalf("text result = %+v, input = %+v, err = %v", next, input.State, err)
	}
	frame, err = painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 130, Y: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !frame.TextInput.Changed || frame.TextInput.Input != nil {
		t.Fatalf("blur update = %+v", frame.TextInput)
	}
}

func TestTextInputRestoresStaticLabelInBlurFrame(t *testing.T) {
	tab := input.KeyTab
	for _, method := range []string{"background", "button", "tab"} {
		t.Run(method, func(t *testing.T) {
			var canvas ui.Canvas
			input := TextInput{
				Bounds: ui.Rect{X: 22, Y: 10, Width: 176, Height: 32}, FontSize: 13,
				State: TextState{Value: "before"},
			}
			input.OnEvent = func(event TextEvent) (TextState, error) {
				return TextState{Value: "edited 中😀", Selection: TextRange{Start: 9, End: 9}}, nil
			}
			config := Config{Title: "blur", Width: 300, Height: 100}
			config.Paint = func(canvas *ui.Canvas, size ui.Size) {
				components.PaintInput(canvas, ui.LightTheme(), ui.Rect{X: 10, Y: 10, Width: 200, Height: 32}, components.InputSnapshot{
					ID: ui.Item(7), Value: input.State.Value, PlatformEditing: canvas.Focused() == ui.Item(7),
				})
				canvas.HitWithLabel(ui.Item(8), ui.Rect{X: 220, Y: 10, Width: 60, Height: 32}, "Save")
			}
			config.TextInput = func(context TextInputContext) *TextInput { return &input }
			painter := newTestPainter(t, &canvas, config)
			for _, kind := range []platform.PointerEventKind{platform.PointerDown, platform.PointerUp} {
				if _, err := painter.pointer(platform.PointerEvent{Kind: kind, X: 30, Y: 20}); err != nil {
					t.Fatal(err)
				}
			}
			if len(canvas.Texts()) != 0 || !painter.textEnabled || !canvas.FocusVisible().IsZero() {
				t.Fatal("pointer focus did not hand text exclusively to the native editor")
			}
			if _, err := painter.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "edited 中😀"}); err != nil {
				t.Fatal(err)
			}
			var frame platform.Frame
			var err error
			switch method {
			case "background":
				frame, err = painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 290, Y: 80})
			case "button":
				frame, err = painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 230, Y: 20})
			case "tab":
				frame, err = painter.key(platform.KeyEvent{Key: tab})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !frame.TextInput.Changed || frame.TextInput.Input != nil || painter.textEnabled {
				t.Fatal("blur did not disable native text")
			}
			if len(frame.TextLabels) != 1 || frame.TextLabels[0].Value != input.State.Value {
				t.Fatalf("blur frame lost the edited label: %+v", frame.TextLabels)
			}
			idle, err := painter.repaint()
			if err != nil || !idle.Update.Empty() || idle.TextLabels != nil || idle.TextInput.Changed {
				t.Fatalf("unchanged blurred repaint did work: %+v, %v", idle, err)
			}
			frame, err = painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 30, Y: 20})
			if err != nil || frame.TextInput.Input == nil || frame.TextInput.Input.State.Value != "edited 中😀" || len(canvas.Texts()) != 0 {
				t.Fatalf("refocus lost edited state: %+v, %v", frame.TextInput, err)
			}
		})
	}
}

func TestPainterTextSetSelectionPreservesRuneIndicesAndSynchronizesOwner(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		selection TextRange
	}{
		{name: "range", selection: TextRange{Start: 2, End: 4}},
		{name: "caret", selection: TextRange{Start: 3, End: 3}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var canvas ui.Canvas
			input := TextInput{
				Bounds:   ui.Rect{X: 10, Y: 10, Width: 100, Height: 32},
				FontSize: 13, Color: ui.Color{Alpha: 1},
				State: TextState{Value: "A😀中文Z", Selection: TextRange{Start: 5, End: 5}},
			}
			wantEvent := TextEvent{Kind: TextSetSelection, Selection: testCase.selection}
			wantState := TextState{Value: input.State.Value, Selection: testCase.selection}
			calls := 0
			input.OnEvent = func(event TextEvent) (TextState, error) {
				calls++
				if event != wantEvent {
					t.Fatalf("owner event = %+v, want %+v", event, wantEvent)
				}
				return wantState, nil
			}
			config := Config{Title: "selection", Width: 180, Height: 70}
			config.Paint = func(canvas *ui.Canvas, size ui.Size) {
				canvas.HitWithOptions(ui.Item(7), input.Bounds, "Name", ui.HitOptions{Role: ui.RoleTextField})
			}
			config.TextInput = func(context TextInputContext) *TextInput {
				if context.ID != ui.Item(7) {
					t.Fatalf("text context = %+v", context)
				}
				return &input
			}
			painter := newTestPainter(t, &canvas, config)
			frame, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 20, Y: 20})
			if err != nil {
				t.Fatal(err)
			}
			if !frame.TextInput.Changed || frame.TextInput.Input == nil {
				t.Fatalf("text input update = %+v", frame.TextInput)
			}
			selection := text.Range{Start: testCase.selection.Start, End: testCase.selection.End}
			next, err := frame.TextInput.Input.OnEvent(platform.TextEvent{Kind: platform.TextSetSelection, Selection: selection})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || input.State != wantState {
				t.Fatalf("owner calls = %d, state = %+v, want %+v", calls, input.State, wantState)
			}
			wantNative := platform.TextState{Value: wantState.Value, Selection: selection}
			if next != wantNative || painter.nativeText.State != wantNative {
				t.Fatalf("native result = %+v, state = %+v, want %+v", next, painter.nativeText.State, wantNative)
			}
			frame, err = painter.repaint()
			if err != nil {
				t.Fatal(err)
			}
			if frame.TextInput.Changed {
				t.Fatalf("synchronized selection was republished: %+v", frame.TextInput)
			}
		})
	}
}

func BenchmarkPainterButtonPointer(b *testing.B) {
	var canvas ui.Canvas
	config := Config{Title: "pointer", Width: 240, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 90}).Default("Save")
	}
	painter, err := newPainter(&canvas, ptrTo(normalizeOrDie(b, config)))
	if err != nil {
		b.Fatal(err)
	}
	for _, kind := range []platform.PointerEventKind{platform.PointerDown, platform.PointerUp} {
		if _, err := painter.pointer(platform.PointerEvent{Kind: kind, X: 20, Y: 20}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 20, Y: 20}); err != nil {
			b.Fatal(err)
		}
		if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerUp, X: 20, Y: 20}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestStablePointerHandlerDoesNotAllocate(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "allocs", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Hit(ui.Item(7), ui.Rect{Width: 50, Height: 20})
	}
	config.OnPointer = func(ui.PointerEvent) ui.EventResult { return ui.EventResult{} }
	painter := newTestPainter(t, &canvas, config)
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 10, Y: 10}); err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(1000, func() {
		if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 11, Y: 11}); err != nil {
			panic(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("stable pointer handler allocated %.2f objects", allocations)
	}
}

func size(width, height float32) platform.Size {
	return platform.Size{Width: width, Height: height}
}

func newTestPainter(t testing.TB, canvas *ui.Canvas, config Config) *painter {
	t.Helper()
	normalized, err := config.normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainter(canvas, &normalized)
	if err != nil {
		t.Fatal(err)
	}
	return painter
}

func TestPainterInitialSyncPublishesCanvas(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Label("Viva", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})

	painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 100, Height: 100})
	if len(painter.renderer.Rectangles()) != 1 {
		t.Fatalf("renderer rectangles = %+v", painter.renderer.Rectangles())
	}
	if len(painter.labels) != 1 || painter.labels[0].Value != "Viva" {
		t.Fatalf("labels = %+v", painter.labels)
	}
}

func TestStaticCanvasRetainsContentWhileHandlingEvents(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{Width: 20, Height: 20}, ui.Color{Alpha: 1})
	canvas.HitWithLabel(ui.Item(7), ui.Rect{Width: 20, Height: 20}, "Static")
	called := false
	config := Config{Title: "static", Width: 100, Height: 100}
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		called = event.Target == ui.Item(7)
		return ui.EventResult{Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	frame, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 10, Y: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !called || len(canvas.Rectangles()) != 1 || len(canvas.Hits()) != 1 {
		t.Fatalf("called=%v rectangles=%d hits=%d", called, len(canvas.Rectangles()), len(canvas.Hits()))
	}
	if !frame.Update.Empty() {
		t.Fatalf("static repaint produced renderer work: %+v", frame.Update)
	}
}

func TestRepaintWithoutChangePerformsNoRendererWork(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Label("Viva", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	var calls int
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		calls++
		canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
		canvas.Label("Viva", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	}
	painter := newTestPainter(t, &canvas, config)

	frame, err := painter.resize(size(320, 200))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("paint calls = %d, want 2 (initial repaint + resize)", calls)
	}
	if !frame.Update.Empty() {
		t.Fatalf("unchanged repaint produced renderer update %+v", frame.Update)
	}
	if frame.TextLabels != nil {
		t.Fatalf("unchanged repaint rebuilt %d labels", len(frame.TextLabels))
	}
}

// Scrolling can move a clip while a label stays put (the page grows or the
// window resizes), so a changed clip alone must reach the platform.
func TestRepaintWithOnlyTheClipChangedRepublishesTheLabel(t *testing.T) {
	var canvas ui.Canvas
	clip := ui.Rect{X: 0, Y: 0, Width: 100, Height: 10}
	config := Config{Title: "clip", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Clip(clip)
		canvas.Label("cut", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
		canvas.Unclip()
	}
	painter := newTestPainter(t, &canvas, config)
	if frame, err := painter.repaint(); err != nil || frame.TextLabels != nil {
		t.Fatalf("unchanged repaint = %+v, %v; want no labels", frame.TextLabels, err)
	}

	clip.Height = 8
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 1 || !frame.TextLabels[0].Clipped || frame.TextLabels[0].Clip.Height != 8 {
		t.Fatalf("labels after the clip moved = %+v", frame.TextLabels)
	}
}

func TestRepaintLayerOnlyChangePublishesCompositingUpdate(t *testing.T) {
	var canvas ui.Canvas
	layer := uint16(0)
	config := Config{Title: "layers", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.PushLayer(layer)
		canvas.Fill(ui.Rect{Width: 20, Height: 20}, ui.Color{Alpha: 1})
		canvas.Label("layered", ui.Rect{Width: 50, Height: 12}, ui.TextStyle{Size: 12})
		canvas.PopLayer()
	}
	painter := newTestPainter(t, &canvas, config)

	layer = 5
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Update.Rebuild || frame.Update.Count != 1 || frame.Rectangles[0].Layer != 5 {
		t.Fatalf("rectangle layer update = %+v, rectangles = %+v", frame.Update, frame.Rectangles)
	}
	if len(frame.TextLabels) != 1 || frame.TextLabels[0].Layer != 5 {
		t.Fatalf("text layer update = %+v", frame.TextLabels)
	}
}

// The render store orders instances by scene insertion, so rectangles that
// first appear after a triangle exists sit behind it in the store. Layers are
// a property of the primitive, not of its index: an overlay opened later must
// keep its layer, and the triangle must not inherit it.
func TestRepaintLayersFollowPrimitivesWhenRectanglesGrowAfterTriangles(t *testing.T) {
	var canvas ui.Canvas
	overlay := false
	white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	base := ui.Rect{Width: 100, Height: 100}
	panel := ui.Rect{X: 30, Y: 10, Width: 60, Height: 40}
	config := Config{Title: "layers", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(base, white)
		canvas.DrawPath(
			ui.NewPath(10, 10, ui.MoveTo(0, 0), ui.LineTo(10, 0), ui.LineTo(5, 10), ui.ClosePath()),
			ui.Rect{X: 5, Y: 60, Width: 20, Height: 20}, ui.PathStyle{Fill: white},
		)
		canvas.Label("base", ui.Rect{X: 5, Y: 90, Width: 40, Height: 10}, ui.TextStyle{Size: 10})
		if overlay {
			canvas.PushLayer(600)
			canvas.Fill(panel, white)
			canvas.Fill(ui.Rect{X: 35, Y: 15, Width: 10, Height: 10}, white)
			canvas.Label("top", ui.Rect{X: 35, Y: 30, Width: 40, Height: 10}, ui.TextStyle{Size: 10})
			canvas.PopLayer()
		}
	}
	painter := newTestPainter(t, &canvas, config)

	overlay = true
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	var triangles, base0, overlays int
	for _, rectangle := range frame.Rectangles {
		switch {
		case rectangle.Kind == scene.KindTriangle:
			triangles++
			if rectangle.Layer != 0 {
				t.Fatalf("triangle inherited layer %d from an overlay rectangle", rectangle.Layer)
			}
		case rectangle.Bounds.Width == base.Width:
			base0++
			if rectangle.Layer != 0 {
				t.Fatalf("base rectangle layer = %d, want 0", rectangle.Layer)
			}
		default:
			overlays++
			if rectangle.Layer != 600 {
				t.Fatalf("overlay rectangle %+v layer = %d, want 600", rectangle.Bounds, rectangle.Layer)
			}
		}
	}
	if triangles == 0 || base0 != 1 || overlays != 2 {
		t.Fatalf("unexpected primitive mix: triangles=%d base=%d overlays=%d", triangles, base0, overlays)
	}
	if len(frame.TextLabels) != 2 || frame.TextLabels[0].Layer != 0 || frame.TextLabels[1].Layer != 600 {
		t.Fatalf("text layers = %+v", frame.TextLabels)
	}

	// Closing and reopening exercises both removal and re-insertion.
	for _, open := range []bool{false, true, false} {
		overlay = open
		frame, err = painter.repaint()
		if err != nil {
			t.Fatal(err)
		}
		for _, rectangle := range frame.Rectangles {
			want := uint16(0)
			if rectangle.Kind != scene.KindTriangle && rectangle.Bounds.Width != base.Width {
				want = 600
			}
			if rectangle.Layer != want {
				t.Fatalf("overlay=%v: instance %+v layer = %d, want %d", open, rectangle.Bounds, rectangle.Layer, want)
			}
		}
	}
}

func TestRepaintWithMovedRectangleUpdatesOneInstance(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Fill(ui.Rect{X: 20, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{X: 0, Y: 0, Width: size.Width / 4, Height: 10}, ui.Color{Alpha: 1})
		canvas.Fill(ui.Rect{X: 20, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	}
	painter := newTestPainter(t, &canvas, config)

	frame, err := painter.resize(size(320, 200))
	if err != nil {
		t.Fatal(err)
	}
	if frame.Update.Rebuild || frame.Update.First != 0 || frame.Update.Count != 1 {
		t.Fatalf("localized update = %+v, want one instance at 0", frame.Update)
	}
	if frame.TextLabels != nil {
		t.Fatal("rectangle-only repaint rebuilt labels")
	}
}

func TestRepaintMovesLabelWithoutChangingText(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "position", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Label("same", ui.Rect{X: size.Width / 2, Y: 5, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	}
	painter := newTestPainter(t, &canvas, config)
	frame, err := painter.resize(size(200, 100))
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels == nil || frame.TextLabels[0].X != 100 {
		t.Fatalf("moved label was not published: %+v", frame.TextLabels)
	}
	unchanged, err := painter.resize(size(200, 100))
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.TextLabels != nil {
		t.Fatal("unchanged label produced an update")
	}
}

func TestRepaintWithTextChangeRebuildsLabels(t *testing.T) {
	var canvas ui.Canvas
	canvas.Label("100", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	counter := 100
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		counter++
		canvas.Label(strconv.Itoa(counter), ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	}
	painter := newTestPainter(t, &canvas, config)

	frame, err := painter.resize(size(320, 200))
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels == nil || len(frame.TextLabels) != 1 || frame.TextLabels[0].Value != "102" {
		t.Fatalf("labels after repaint = %+v", frame.TextLabels)
	}
}

func TestRepaintShrinkingListRemovesSceneNodes(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Fill(ui.Rect{X: 20, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Fill(ui.Rect{X: 40, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	}
	painter := newTestPainter(t, &canvas, config)

	frame, err := painter.resize(size(320, 200))
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Update.Empty() {
		t.Fatalf("resize after initial shrink produced work: %+v", frame.Update)
	}
	if len(painter.renderer.Rectangles()) != 1 {
		t.Fatalf("renderer kept %d rectangles", len(painter.renderer.Rectangles()))
	}
}

func TestPointerHoverEntersAndExitsOnce(t *testing.T) {
	var canvas ui.Canvas
	canvas.Hit(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20})
	var calls int
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		calls++
		canvas.Hit(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20})
	}
	painter := newTestPainter(t, &canvas, config)

	enter, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 10, Y: 10})
	if err != nil {
		t.Fatal(err)
	}
	if painter.hovered != ui.Item(7) || !enter.Update.Empty() {
		t.Fatalf("hover enter state = %d, update = %+v (paint draws nothing, renderer must stay idle)", painter.hovered, enter.Update)
	}
	if calls != 2 {
		t.Fatalf("paint calls after enter = %d, want 2 (initial + hover)", calls)
	}

	idle, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 12, Y: 12})
	if err != nil {
		t.Fatal(err)
	}
	if !idle.Update.Empty() {
		t.Fatal("moving inside the same region repainted")
	}
	if calls != 2 {
		t.Fatalf("paint calls after idle move = %d, want 2", calls)
	}

	exit, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerExit})
	if err != nil {
		t.Fatal(err)
	}
	if !painter.hovered.IsZero() || !exit.Update.Empty() {
		t.Fatalf("hover exit state = %d, update = %+v", painter.hovered, exit.Update)
	}
	if calls != 3 {
		t.Fatalf("paint calls = %d, want 3", calls)
	}
}

func TestPointerClickActivatesAndPressOnlyCountsInsideRegion(t *testing.T) {
	var canvas ui.Canvas
	canvas.Hit(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20})
	var clicks []ui.ID
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Hit(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20})
	}
	onClick(&config, func(id ui.ID) { clicks = append(clicks, id) })
	painter := newTestPainter(t, &canvas, config)

	// Press inside, release inside: one click.
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 10, Y: 10}); err != nil {
		t.Fatal(err)
	}
	if painter.pressed != ui.Item(7) {
		t.Fatalf("pressed = %d", painter.pressed)
	}
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerUp, X: 10, Y: 10}); err != nil {
		t.Fatal(err)
	}
	if len(clicks) != 1 || clicks[0] != ui.Item(7) {
		t.Fatalf("clicks = %v", clicks)
	}

	// Press inside, drag out, release outside: no click.
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 10, Y: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 80, Y: 80}); err != nil {
		t.Fatal(err)
	}
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerUp, X: 80, Y: 80}); err != nil {
		t.Fatal(err)
	}
	if len(clicks) != 1 {
		t.Fatalf("drag-out release produced click %v", clicks)
	}
	if !painter.pressed.IsZero() {
		t.Fatalf("pressed after release = %d", painter.pressed)
	}
}

func TestPointerHandlerCapturesTargetAndSuppressesClick(t *testing.T) {
	var canvas ui.Canvas
	var events [3]ui.PointerEvent
	count := 0
	clicks := 0
	config := Config{Title: "pointer events", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(7), ui.Rect{Width: 40, Height: 20}, "Open")
	}
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		events[count] = event
		count++
		return ui.EventResult{Handled: true}
	}
	onClick(&config, func(ui.ID) { clicks++ })
	painter := newTestPainter(t, &canvas, config)

	sequence := []platform.PointerEvent{
		{Kind: platform.PointerDown, Button: ui.PointerButtonSecondary, Modifiers: input.ModifierShift, X: 10, Y: 10},
		{Kind: platform.PointerMove, X: 80, Y: 80},
		{Kind: platform.PointerUp, Button: ui.PointerButtonSecondary, X: 80, Y: 80},
	}
	for _, event := range sequence {
		if _, err := painter.pointer(event); err != nil {
			t.Fatal(err)
		}
	}
	if count != len(sequence) || clicks != 0 {
		t.Fatalf("events=%d clicks=%d", count, clicks)
	}
	if events[0].Target != ui.Item(7) || events[0].Captured || events[0].Button != ui.PointerButtonSecondary || events[0].Modifiers != ui.ModifierShift || events[0].Viewport != (ui.Size{Width: 100, Height: 100}) {
		t.Fatalf("down event = %+v", events[0])
	}
	for _, event := range events[1:] {
		if event.Target != ui.Item(7) || !event.Captured {
			t.Fatalf("captured event = %+v", event)
		}
	}
}

func TestSecondaryPointerDoesNotClick(t *testing.T) {
	var canvas ui.Canvas
	clicks := 0
	config := Config{Title: "secondary", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(7), ui.Rect{Width: 40, Height: 20}, "Menu")
	}
	onClick(&config, func(ui.ID) { clicks++ })
	painter := newTestPainter(t, &canvas, config)
	for _, kind := range []platform.PointerEventKind{platform.PointerDown, platform.PointerUp} {
		if _, err := painter.pointer(platform.PointerEvent{Kind: kind, Button: ui.PointerButtonSecondary, X: 10, Y: 10}); err != nil {
			t.Fatal(err)
		}
	}
	if clicks != 0 {
		t.Fatalf("secondary pointer produced %d clicks", clicks)
	}
}

func TestEventResultsControlRepaintFocusKeyboardAndScroll(t *testing.T) {
	var canvas ui.Canvas
	paints := 0
	pointerRepaint := false
	keyHandled := true
	scrollHandled := true
	var scroll ui.ScrollEvent
	config := Config{Title: "results", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		paints++
		canvas.HitWithLabel(ui.Item(7), ui.Rect{Width: 40, Height: 20}, "First")
		canvas.HitWithLabel(ui.Item(8), ui.Rect{Y: 30, Width: 40, Height: 20}, "Second")
	}
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		return ui.EventResult{Focus: ui.Item(8), FocusVisible: true, Repaint: pointerRepaint}
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		return ui.EventResult{Handled: keyHandled}
	}
	config.OnScroll = func(event ui.ScrollEvent) ui.EventResult {
		scroll = event
		return ui.EventResult{Handled: scrollHandled}
	}
	painter := newTestPainter(t, &canvas, config)

	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 10, Y: 10}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(8) || !painter.keyboardFocus {
		t.Fatalf("focus=%d visible=%v", painter.focused, painter.keyboardFocus)
	}
	before := paints
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 11, Y: 11}); err != nil {
		t.Fatal(err)
	}
	if paints != before {
		t.Fatalf("unchanged pointer repainted: %d -> %d", before, paints)
	}
	pointerRepaint = true
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 12, Y: 12}); err != nil {
		t.Fatal(err)
	}
	if paints != before+1 {
		t.Fatalf("explicit repaint calls=%d want=%d", paints, before+1)
	}

	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(8) {
		t.Fatalf("handled Tab moved focus to %d", painter.focused)
	}
	keyHandled = false
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(7) {
		t.Fatalf("default Tab focus=%d", painter.focused)
	}

	if _, err := painter.scroll(platform.ScrollEvent{
		Modifiers: input.ModifierControl, X: 10, Y: 35,
		DeltaX: 5, DeltaY: 9, Precise: true,
	}); err != nil {
		t.Fatal(err)
	}
	if scroll != (ui.ScrollEvent{
		Modifiers: ui.ModifierControl, X: 10, Y: 35,
		DeltaX: 5, DeltaY: 9, Viewport: ui.Size{Width: 100, Height: 100}, Target: ui.Item(8), Precise: true,
	}) || painter.scrollX != 0 || painter.scrollY != 0 {
		t.Fatalf("handled scroll=%+v offsets=(%v,%v)", scroll, painter.scrollX, painter.scrollY)
	}
	scrollHandled = false
	if _, err := painter.scroll(platform.ScrollEvent{DeltaX: 5, DeltaY: 9}); err != nil {
		t.Fatal(err)
	}
	if painter.scrollX != 5 || painter.scrollY != 9 {
		t.Fatalf("default offsets=(%v,%v)", painter.scrollX, painter.scrollY)
	}
}

func TestEventResultFocusCanTargetAHitRevealedByTheEvent(t *testing.T) {
	var canvas ui.Canvas
	fieldVisible := false
	editor := text.NewEditor("")
	config := Config{Title: "revealed focus", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 200, Height: 100}, "Terminal", ui.HitOptions{Role: ui.RoleTerminal, Autofocus: true})
		if fieldVisible {
			canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 20, Y: 20, Width: 160, Height: 30}, "Find", ui.HitOptions{Role: ui.RoleTextField})
		}
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		fieldVisible = true
		return ui.EventResult{Focus: ui.Item(2), FocusVisible: true, Handled: true}
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if context.ID == ui.Item(2) {
			return &TextInput{Editor: editor}
		}
		return nil
	}
	painter := newTestPainter(t, &canvas, config)
	if painter.focused != ui.Item(1) {
		t.Fatalf("initial focus = %d", painter.focused)
	}
	frame, err := painter.key(platform.KeyEvent{Key: input.KeyUnknown})
	if err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(2) || !painter.keyboardFocus || canvas.Focused() != ui.Item(2) || canvas.FocusVisible() != ui.Item(2) {
		t.Fatalf("focused=%d keyboard=%t canvas=%d/%d", painter.focused, painter.keyboardFocus, canvas.Focused(), canvas.FocusVisible())
	}
	if !frame.TextInput.Changed || frame.TextInput.Input == nil || painter.activeText == nil || painter.activeText.Editor != editor {
		t.Fatalf("text input = %+v active=%+v", frame.TextInput, painter.activeText)
	}
}

func BenchmarkPainterRevealAndFocus(b *testing.B) {
	var canvas ui.Canvas
	fieldVisible := false
	config := Config{Title: "revealed focus", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		id, role := uint32(1), ui.RoleTerminal
		if fieldVisible {
			id, role = 2, ui.RoleTextField
		}
		canvas.HitWithOptions(ui.Item(int(id)), ui.Rect{Width: 200, Height: 100}, "Control", ui.HitOptions{Role: role})
	}
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		fieldVisible = !fieldVisible
		focus := uint32(1)
		if fieldVisible {
			focus = 2
		}
		return ui.EventResult{Focus: ui.Item(int(focus)), FocusVisible: true, Handled: true}
	}
	painter := newTestPainter(b, &canvas, config)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := painter.key(platform.KeyEvent{Key: input.KeyUnknown}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestKeyTabTraversesFocusAndEnterActivates(t *testing.T) {
	var canvas ui.Canvas
	canvas.HitWithLabel(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20}, "Save")
	canvas.HitWithLabel(ui.Item(8), ui.Rect{X: 0, Y: 40, Width: 50, Height: 20}, "Cancel")
	var clicks []ui.ID
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20}, "Save")
		canvas.HitWithLabel(ui.Item(8), ui.Rect{X: 0, Y: 40, Width: 50, Height: 20}, "Cancel")
	}
	onClick(&config, func(id ui.ID) { clicks = append(clicks, id) })
	painter := newTestPainter(t, &canvas, config)

	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(7) {
		t.Fatalf("focused after first tab = %d", painter.focused)
	}
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(8) {
		t.Fatalf("focused after second tab = %d", painter.focused)
	}
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyEnter}); err != nil {
		t.Fatal(err)
	}
	if len(clicks) != 1 || clicks[0] != ui.Item(8) {
		t.Fatalf("enter activation clicks = %v", clicks)
	}

	// Space must activate the still-focused target without moving focus.
	if _, err := painter.key(platform.KeyEvent{Key: input.KeySpace}); err != nil {
		t.Fatal(err)
	}
	if len(clicks) != 2 || clicks[1] != ui.Item(8) {
		t.Fatalf("space activation clicks = %v", clicks)
	}
}

func TestAccessibilityNodesFollowHitsAndFocus(t *testing.T) {
	var canvas ui.Canvas
	canvas.HitWithLabel(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20}, "Save")
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20}, "Save")
	}
	painter := newTestPainter(t, &canvas, config)
	if len(painter.accessibility) != 1 || painter.accessibility[0].Label != "Save" {
		t.Fatalf("initial accessibility = %+v", painter.accessibility)
	}

	frame, err := painter.key(platform.KeyEvent{Key: input.KeyTab})
	if err != nil {
		t.Fatal(err)
	}
	if frame.Accessibility == nil {
		t.Fatal("focus change did not publish accessibility updates")
	}
	if len(frame.Accessibility) != 1 || frame.Accessibility[0].Label != "Save" {
		t.Fatalf("updated accessibility = %+v", frame.Accessibility)
	}

	// An unrelated key with no focus or activation change publishes nothing.
	idle, err := painter.key(platform.KeyEvent{Key: input.KeyEscape})
	if err != nil {
		t.Fatal(err)
	}
	if idle.Accessibility != nil || !idle.Update.Empty() {
		t.Fatalf("idle key produced work: %+v", idle)
	}
}

func TestScrollClampsAtZeroAndRepaints(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	var paints int
	config := Config{Title: "test", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		paints++
		canvas.Fill(ui.Rect{X: 0, Y: 10 - canvas.ScrollY(), Width: 10, Height: 10}, ui.Color{Alpha: 1})
	}
	painter := newTestPainter(t, &canvas, config)

	up, err := painter.scroll(platform.ScrollEvent{DeltaY: -20})
	if err != nil {
		t.Fatal(err)
	}
	if !up.Update.Empty() || painter.scrollY != 0 {
		t.Fatalf("scroll above the top: scrollY=%f update=%+v", painter.scrollY, up.Update)
	}
	if paints != 1 {
		t.Fatalf("clamped scroll repainted %d times, want only the initial repaint", paints)
	}

	down, err := painter.scroll(platform.ScrollEvent{DeltaY: 24})
	if err != nil {
		t.Fatal(err)
	}
	if down.Update.Empty() {
		t.Fatal("productive scroll produced no renderer work")
	}
	if painter.scrollY != 24 {
		t.Fatalf("scrollY = %f, want 24", painter.scrollY)
	}
	if paints != 2 {
		t.Fatalf("productive scroll repainted %d times, want initial + 1", paints)
	}
	if len(painter.clipped) != 1 || painter.clipped[0].Bounds.Y != -14 {
		t.Fatalf("scrolled rectangle = %+v", painter.clipped)
	}
}

func TestClipKeepsCutRectanglesWholeAndDropsHiddenShapesAndText(t *testing.T) {
	var canvas ui.Canvas
	clip := ui.Rect{X: 0, Y: 0, Width: 50, Height: 100}
	canvas.Clip(clip)
	canvas.Fill(ui.Rect{X: 40, Y: 0, Width: 40, Height: 10}, ui.Color{Alpha: 1})
	canvas.Fill(ui.Rect{X: 60, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Fill(ui.Rect{X: 10, Y: 20, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Label("visible", ui.Rect{X: 4, Y: 20, Width: 40, Height: 10}, ui.TextStyle{Size: 10})
	canvas.Label("hidden", ui.Rect{X: 70, Y: 20, Width: 40, Height: 10}, ui.TextStyle{Size: 10})
	canvas.Unclip()
	canvas.Fill(ui.Rect{X: 80, Y: 40, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 100, Height: 100})

	instances := painter.renderer.Rectangles()
	if len(instances) != 3 {
		t.Fatalf("renderer holds %d rectangles, want the cut one, the inside one and the unclipped one: %+v", len(instances), instances)
	}
	if cut := instances[0]; cut.Bounds != (scene.Rect{X: 40, Y: 0, Width: 40, Height: 10}) || !cut.Clipped || cut.Clip != scene.Rect(clip) {
		t.Errorf("cut rectangle = %+v, want its full bounds and the clip %+v", cut, clip)
	}
	for _, index := range []int{1, 2} {
		if instances[index].Clipped || instances[index].Clip != (scene.Rect{}) {
			t.Errorf("rectangle %d carries a clip it does not need: %+v", index, instances[index])
		}
	}
	if len(painter.clippedTexts) != 1 || painter.clippedTexts[0].Value != "visible" {
		t.Fatalf("clipped texts = %+v", painter.clippedTexts)
	}
}

// A label the clip cuts partway reaches the platform with that clip, so the
// renderer can scissor it; one wholly inside carries none, and one wholly
// outside is not sent at all.
func TestPartlyClippedLabelsCarryTheirClipToThePlatform(t *testing.T) {
	var canvas ui.Canvas
	clip := ui.Rect{X: 0, Y: 20, Width: 100, Height: 60}
	canvas.Clip(clip)
	canvas.Label("above", ui.Rect{X: 4, Y: 10, Width: 40, Height: 20}, ui.TextStyle{Size: 10})
	canvas.Label("inside", ui.Rect{X: 4, Y: 30, Width: 40, Height: 20}, ui.TextStyle{Size: 10})
	canvas.Label("below", ui.Rect{X: 4, Y: 70, Width: 40, Height: 20}, ui.TextStyle{Size: 10})
	canvas.Label("gone", ui.Rect{X: 4, Y: 90, Width: 40, Height: 20}, ui.TextStyle{Size: 10})
	canvas.Unclip()
	canvas.Label("free", ui.Rect{X: 4, Y: 0, Width: 40, Height: 10}, ui.TextStyle{Size: 10})
	painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 100, Height: 120})

	got := map[string]platform.TextLabel{}
	for _, label := range painter.labels {
		got[label.Value] = label
	}
	if _, sent := got["gone"]; sent || len(got) != 4 {
		t.Fatalf("labels sent = %v, want all but the hidden one", got)
	}
	for _, name := range []string{"above", "below"} {
		if label := got[name]; !label.Clipped || label.Clip != clip {
			t.Errorf("%s: clipped=%v clip=%+v, want the canvas clip %+v", name, label.Clipped, label.Clip, clip)
		}
	}
	for _, name := range []string{"inside", "free"} {
		if label := got[name]; label.Clipped || label.Clip != (ui.Rect{}) {
			t.Errorf("%s carries a clip it does not need: %+v", name, label)
		}
	}
}

// A path stroke a clip cuts is not cut by the application: its triangles reach
// the renderer whole, with the canvas's own edge masks and the clip; strokes
// wholly inside carry no clip and strokes wholly outside are not sent.
func TestClippedStrokeReachesTheRendererWholeWithItsClip(t *testing.T) {
	var canvas ui.Canvas
	clip := ui.Rect{Width: 10, Height: 10}
	style := ui.PathStyle{Stroke: ui.Color{Alpha: 1}, StrokeWidth: 2, Cap: ui.CapButt}
	canvas.Clip(clip)
	canvas.DrawPath(ui.NewPath(100, 10, ui.MoveTo(-20, 5), ui.LineTo(80, 5)), ui.Rect{X: -20, Width: 100, Height: 10}, style)
	canvas.DrawPath(ui.NewPath(10, 10, ui.MoveTo(2, 5), ui.LineTo(8, 5)), ui.Rect{Width: 10, Height: 10}, style)
	canvas.DrawPath(ui.NewPath(10, 10, ui.MoveTo(0, 5), ui.LineTo(10, 5)), ui.Rect{X: 30, Y: 30, Width: 10, Height: 10}, style)
	canvas.Unclip()
	painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 100, Height: 100})

	painted := canvas.Triangles()
	instances := painter.renderer.Rectangles()
	if len(painted) != 6 || len(instances) != 4 {
		t.Fatalf("canvas painted %d triangles and the renderer holds %d, want 6 and 4", len(painted), len(instances))
	}
	for index, instance := range instances {
		cut := index < 2
		if instance.Clipped != cut || cut && instance.Clip != scene.Rect(clip) || !cut && instance.Clip != (scene.Rect{}) {
			t.Errorf("instance %d: clipped=%v clip=%+v, want clipped=%v", index, instance.Clipped, instance.Clip, cut)
		}
		if instance.EdgeMask != painted[index].EdgeMask || instance.Points[0] != scene.Point(painted[index].Points[0]) {
			t.Errorf("instance %d was altered: %+v, canvas painted %+v", index, instance, painted[index])
		}
	}
}

func TestVectorFringeKeepsItsClip(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2, 3} {
		for _, x := range []float32{-0.25, 0.25} {
			var canvas ui.Canvas
			canvas.SetPixelScale(scale)
			clip := ui.Rect{Width: 10, Height: 10}
			canvas.Clip(clip)
			// The first edge is just outside or just inside the clip. In both
			// cases its AA fringe intersects the clip and must keep the scissor.
			canvas.DrawPath(ui.NewPath(10, 10, ui.MoveTo(x/scale, 2), ui.LineTo(8, 2), ui.LineTo(x/scale, 8), ui.ClosePath()), clip, ui.PathStyle{Fill: ui.Color{Alpha: 1}})
			canvas.Unclip()
			painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 10, Height: 10})
			instances := painter.renderer.Rectangles()
			if len(instances) == 0 {
				t.Fatal("visible fringe was culled")
			}
			for _, instance := range instances {
				if !instance.Clipped || instance.Clip != scene.Rect(clip) {
					t.Fatalf("scale %g, offset %g: fringe lost its clip: %+v", scale, x, instance)
				}
			}
		}
	}
}

func BenchmarkPainterVirtualizedScroll(b *testing.B) {
	var canvas ui.Canvas
	config := Config{Title: "bench", Width: 640, Height: 560}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		viewport := ui.Rect{X: 40, Y: 40, Width: 500, Height: 440}
		canvas.Clip(viewport)
		first := int(canvas.ScrollY() / 36)
		for index := first; index < first+14 && index < 1000; index++ {
			y := viewport.Y + float32(index)*36 - canvas.ScrollY()
			canvas.Fill(ui.Rect{X: viewport.X, Y: y, Width: viewport.Width, Height: 36}, ui.Color{Alpha: 1})
			canvas.Label("Row", ui.Rect{X: viewport.X + 8, Y: y + 8, Width: 100, Height: 20}, ui.TextStyle{Size: 13})
		}
		canvas.Unclip()
	}
	painter, err := newPainter(&canvas, ptrTo(normalizeOrDie(b, config)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := painter.scroll(platform.ScrollEvent{DeltaY: 7, Precise: true}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPainterHandledScroll(b *testing.B) {
	var canvas ui.Canvas
	config := Config{Title: "bench", Width: 640, Height: 560}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.Hit(ui.Item(1), ui.Rect{Width: 640, Height: 560})
	}
	config.OnScroll = func(ui.ScrollEvent) ui.EventResult {
		return ui.EventResult{Handled: true}
	}
	painter, err := newPainter(&canvas, ptrTo(normalizeOrDie(b, config)))
	if err != nil {
		b.Fatal(err)
	}
	event := platform.ScrollEvent{X: 320, Y: 280, DeltaY: 7, Precise: true}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := painter.scroll(event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPainterSteadyStateRepaint(b *testing.B) {
	var canvas ui.Canvas
	canvas.Hit(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20})
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	canvas.Label("Viva", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	config := Config{Title: "bench", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Hit(ui.Item(7), ui.Rect{X: 0, Y: 0, Width: 50, Height: 20})
		canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 10, Height: 10}, ui.Color{Alpha: 1})
		canvas.Label("Viva", ui.Rect{X: 2, Y: 2, Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	}
	painter, err := newPainter(&canvas, ptrTo(normalizeOrDie(b, config)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := painter.resize(size(320, 200)); err != nil {
			b.Fatal(err)
		}
	}
}

func normalizeOrDie(tb testing.TB, config Config) Config {
	normalized, err := config.normalize()
	if err != nil {
		tb.Fatal(err)
	}
	return normalized
}

func ptrTo(config Config) *Config {
	return &config
}

func TestModalOpenMovesFocusInsideWithinTheSameFrameAndCloseRestoresIt(t *testing.T) {
	const (
		opener  = 1
		dismiss = 2
		closeID = 3
		field   = 4
	)
	var canvas ui.Canvas
	open := false
	paints := 0
	var focusSeenByPaint ui.ID
	config := Config{Title: "modal", Width: 240, Height: 160}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		paints++
		canvas.HitWithLabel(ui.Item(int(opener)), ui.Rect{Width: 60, Height: 24}, "Open")
		if open {
			dialog := components.BeginDialog(canvas, ui.DarkTheme(), ui.Rect{Width: size.Width, Height: size.Height}, components.DialogOptions{
				Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(int(dismiss)), CloseID: ui.Item(int(closeID)), InitialFocusID: ui.Item(int(field)),
				ContentSize: ui.Size{Width: 200, Height: 140},
			})
			layout := dialog.Begin("Title", "Description")
			canvas.HitWithOptions(ui.Item(int(field)), ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: 24}, "Field", ui.HitOptions{Role: ui.RoleTextField})
			dialog.End()
		}
		focusSeenByPaint = canvas.Focused()
	}
	onClick(&config, func(id ui.ID) {
		switch id {
		case ui.Item(opener):
			open = true
		case ui.Item(dismiss), ui.Item(closeID):
			open = false
		}
	})
	config.TextInput = func(context TextInputContext) *TextInput {
		if context.ID != ui.Item(int(field)) {
			return nil
		}
		return &TextInput{
			Bounds: context.Bounds, FontSize: 13,
			OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
		}
	}
	painter := newTestPainter(t, &canvas, config)

	var frame platform.Frame
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		var err error
		if frame, err = painter.pointer(platform.PointerEvent{Kind: kind, X: 10, Y: 10}); err != nil {
			t.Fatal(err)
		}
	}
	if painter.focused != ui.Item(int(field)) || canvas.Focused() != ui.Item(int(field)) || focusSeenByPaint != ui.Item(int(field)) {
		t.Fatalf("after open: painter=%d canvas=%d paint saw=%d, want the dialog field %d",
			painter.focused, canvas.Focused(), focusSeenByPaint, field)
	}
	if !frame.TextInput.Changed || frame.TextInput.Input == nil {
		t.Fatalf("native text input was not enabled on the opening frame: %+v", frame.TextInput)
	}

	// Steady state: the scope is already open, so repainting must neither
	// pull focus back nor paint twice.
	before := paints
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if paints != before+1 || painter.focused != ui.Item(int(field)) {
		t.Fatalf("steady repaint ran %d paints with focus %d", paints-before, painter.focused)
	}

	if frame, err := painter.key(platform.KeyEvent{Key: input.KeyEscape}); err != nil {
		t.Fatal(err)
	} else if !frame.TextInput.Changed || frame.TextInput.Input != nil {
		t.Fatalf("native text input stayed enabled after the dialog closed: %+v", frame.TextInput)
	}
	if open || painter.focused != ui.Item(int(opener)) || canvas.Focused() != ui.Item(int(opener)) || focusSeenByPaint != ui.Item(int(opener)) {
		t.Fatalf("after close: open=%v painter=%d canvas=%d paint saw=%d, want the opener %d",
			open, painter.focused, canvas.Focused(), focusSeenByPaint, opener)
	}
}

func BenchmarkPainterModalToggle(b *testing.B) {
	var canvas ui.Canvas
	open := false
	config := Config{Title: "modal", Width: 240, Height: 160}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{Width: 60, Height: 24}, "Open")
		if open {
			dialog := components.BeginDialog(canvas, ui.DarkTheme(), ui.Rect{Width: size.Width, Height: size.Height}, components.DialogOptions{
				Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(2), CloseID: ui.Item(3), InitialFocusID: ui.Item(4),
				ContentSize: ui.Size{Width: 200, Height: 140},
			})
			layout := dialog.Begin("Title", "Description")
			canvas.HitWithOptions(ui.Item(4), ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: 24}, "Field", ui.HitOptions{Role: ui.RoleTextField})
			dialog.End()
		}
	}
	painter, err := newPainter(&canvas, ptrTo(normalizeOrDie(b, config)))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		open = !open
		if _, err := painter.repaint(); err != nil {
			b.Fatal(err)
		}
	}
}

// A custom titlebar's drag strip must yield to a control inside it, or the
// control can never be clicked; away from controls it keeps dragging.
func TestPressAtReportsControlsMoveRegionsAndTheClientArea(t *testing.T) {
	var canvas ui.Canvas
	modal := false
	config := Config{Title: "bar", Width: 400, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.MoveRegion(ui.Rect{Width: size.Width, Height: 32})
		canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 300, Y: 8, Width: 40, Height: 16}, "Language")
		if modal {
			canvas.PushFocusScope(ui.Item(9))
			canvas.HitWithLabel(ui.Item(2), ui.Rect{X: 100, Y: 60, Width: 100, Height: 60}, "OK")
			canvas.PopFocusScope()
			canvas.TrapFocus(ui.Item(9))
		}
	}
	painter := newTestPainter(t, &canvas, config)

	if painter.pressAt(310, 16) != platform.PressControl {
		t.Fatal("a control in the titlebar strip must keep receiving clicks")
	}
	if painter.pressAt(150, 16) != platform.PressMove {
		t.Fatal("a move region must move the window")
	}
	if painter.pressAt(150, 100) != platform.PressClient {
		t.Fatal("a press outside every move region must stay the application's")
	}
	modal = true
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if painter.pressAt(310, 16) == platform.PressControl {
		t.Fatal("a control under a modal is not interactive, so the strip stops yielding to it")
	}
}

// The user-visible bug: pressing the empty part of a dialog closed it. The
// panel is an opaque block, so only a press on the backdrop dismisses.
func TestPressingAModalPanelDoesNotDismissItButPressingTheBackdropDoes(t *testing.T) {
	const opener, dismiss, button = 1, 2, 3
	var canvas ui.Canvas
	open := true
	var clicked []ui.ID
	config := Config{Title: "modal", Width: 300, Height: 200}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(int(opener)), ui.Rect{Width: 40, Height: 20}, "Open")
		if !open {
			return
		}
		dialog := components.BeginDialog(canvas, ui.DarkTheme(), ui.Rect{Width: size.Width, Height: size.Height}, components.DialogOptions{
			Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(int(dismiss)), CloseID: ui.Item(99), ContentSize: ui.Size{Width: 200, Height: 140},
		})
		layout := dialog.Begin("Title", "Description")
		canvas.HitWithLabel(ui.Item(int(button)), ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: 40, Height: 20}, "OK")
		dialog.End()
	}
	onClick(&config, func(id ui.ID) {
		clicked = append(clicked, id)
		if id == ui.Item(int(dismiss)) {
			open = false
		}
	})
	painter := newTestPainter(t, &canvas, config)
	press := func(x, y float32) {
		for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
			if _, err := painter.pointer(platform.PointerEvent{Kind: kind, X: x, Y: y}); err != nil {
				t.Fatal(err)
			}
		}
	}

	panelPadding := ui.Rect{X: 150, Y: 100} // inside the centered 200x140 panel, away from its controls
	press(panelPadding.X, panelPadding.Y+30)
	if len(clicked) != 0 || !open {
		t.Fatalf("pressing the panel's empty space clicked %v and open=%v", clicked, open)
	}
	// The block is an occluder, not a target, and the modal hides the page and
	// its own backdrop from assistive technology.
	if len(painter.accessibility) != 2 || painter.accessibility[0].Label != "Close dialog" || painter.accessibility[1].Label != "OK" {
		t.Fatalf("accessibility = %+v, want only the dialog's controls", painter.accessibility)
	}
	press(5, 150) // the dimmed backdrop outside the panel
	if len(clicked) != 1 || clicked[0] != ui.Item(int(dismiss)) || open {
		t.Fatalf("pressing the backdrop clicked %v and open=%v, want the dismiss action", clicked, open)
	}
}

// The user-visible bug: with a menu or dialog open, a click in a custom
// titlebar's strip did not dismiss it, because the platform took the press for a
// window drag. The strip now hands the press to the backdrop, and a drag cancels
// it with a PointerExit instead of dismissing.
func TestAClickOnABackdropInTheTitlebarStripDismissesButADragDoesNot(t *testing.T) {
	const dismiss = 2
	var canvas ui.Canvas
	open := true
	config := Config{Title: "bar", Width: 400, Height: 300}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.MoveRegion(ui.Rect{Width: size.Width, Height: 32})
		if !open {
			return
		}
		dialog := components.BeginDialog(canvas, ui.DarkTheme(), ui.Rect{Width: size.Width, Height: size.Height}, components.DialogOptions{
			Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(dismiss), CloseID: ui.Item(99), ContentSize: ui.Size{Width: 200, Height: 140},
		})
		dialog.Paint("Title", "Description")
	}
	onClick(&config, func(id ui.ID) {
		if id == ui.Item(dismiss) {
			open = false
		}
	})
	painter := newTestPainter(t, &canvas, config)
	send := func(kind platform.PointerEventKind) {
		t.Helper()
		if _, err := painter.pointer(platform.PointerEvent{Kind: kind, X: 20, Y: 16}); err != nil {
			t.Fatal(err)
		}
	}

	if painter.pressAt(20, 16) != platform.PressMoveOnDrag {
		t.Fatal("a move region under the backdrop must hand the press to the application and move the window on a drag")
	}
	send(platform.PointerDown)
	send(platform.PointerExit) // the platform started a window move
	send(platform.PointerUp)
	if !open {
		t.Fatal("a press that became a window move dismissed the dialog")
	}
	send(platform.PointerMove)
	send(platform.PointerDown)
	send(platform.PointerUp)
	if open {
		t.Fatal("a click on the backdrop in the titlebar strip did not dismiss the dialog")
	}
}

// targets returns the interaction targets among the canvas hits.
func targets(canvas *ui.Canvas) []ui.Hit {
	var result []ui.Hit
	for _, hit := range canvas.Hits() {
		if !hit.Block {
			result = append(result, hit)
		}
	}
	return result
}

// A theme switch repaints identical geometry in new colors. Every retained
// instance, icon strokes included, must come out in its new color and the
// published update must cover each one; otherwise part of the window keeps
// the old theme until something else forces a rebuild.
func TestRepaintRecoloringEveryPrimitiveRefreshesEveryInstance(t *testing.T) {
	icon := ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound,
		Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 4), ui.LineTo(20, 20), ui.LineTo(4, 20)}}
	background := ui.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}
	tint := ui.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
	var canvas ui.Canvas
	config := Config{Title: "recolor", Width: 200, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: 200, Height: 120}, background)
		canvas.DrawIcon(icon, ui.Rect{X: 10, Y: 10, Width: 16, Height: 16}, tint)
		canvas.Rounded(ui.Rect{X: 40, Y: 10, Width: 30, Height: 16}, 4, background)
		canvas.DrawIcon(icon, ui.Rect{X: 80, Y: 10, Width: 16, Height: 16}, tint)
		canvas.Label("text", ui.Rect{X: 10, Y: 40, Width: 60, Height: 16}, ui.TextStyle{Size: 12, Color: tint})
	}
	painter := newTestPainter(t, &canvas, config)
	if len(canvas.Triangles()) == 0 || len(canvas.Rectangles()) < 2 {
		t.Fatalf("the scene has %d triangles and %d rectangles", len(canvas.Triangles()), len(canvas.Rectangles()))
	}

	background = ui.Color{Red: 0.97, Green: 0.97, Blue: 0.98, Alpha: 1}
	tint = ui.Color{Red: 0.1, Green: 0.1, Blue: 0.15, Alpha: 1}
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("update=%+v instances=%d", frame.Update, len(frame.Rectangles))
	if frame.Update.Empty() {
		t.Fatal("recoloring published no renderer work")
	}
	covered := func(index int) bool {
		return frame.Update.Rebuild || (index >= frame.Update.First && index < frame.Update.First+frame.Update.Count)
	}
	wantBackground, wantTint := scene.Color{Red: 0.97, Green: 0.97, Blue: 0.98, Alpha: 1}, scene.Color{Red: 0.1, Green: 0.1, Blue: 0.15, Alpha: 1}
	for index, instance := range frame.Rectangles {
		want := wantBackground
		if instance.Kind == scene.KindTriangle {
			want = wantTint
		}
		if instance.Color != want {
			t.Errorf("instance %d (kind %d) is %+v, want %+v", index, instance.Kind, instance.Color, want)
		}
		if !covered(index) {
			t.Errorf("instance %d changed color but is outside the update %+v", index, frame.Update)
		}
	}
	if len(frame.TextLabels) != 1 || frame.TextLabels[0].Color.Red > 0.2 {
		t.Fatalf("the label was not recolored: %+v", frame.TextLabels)
	}
}

func TestRepaintRecoloringWhileTheTriangleCountChangesRefreshesEveryInstance(t *testing.T) {
	short := ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound,
		Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 4), ui.LineTo(20, 20)}}
	long := ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound,
		Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 4), ui.LineTo(20, 20), ui.LineTo(4, 20), ui.LineTo(4, 4), ui.LineTo(12, 12)}}
	tint := ui.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
	second := short
	var canvas ui.Canvas
	config := Config{Title: "recolor", Width: 200, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: 200, Height: 120}, tint)
		canvas.DrawIcon(short, ui.Rect{X: 10, Y: 10, Width: 16, Height: 16}, tint)
		canvas.DrawIcon(second, ui.Rect{X: 80, Y: 10, Width: 16, Height: 16}, tint)
		canvas.DrawIcon(short, ui.Rect{X: 120, Y: 10, Width: 16, Height: 16}, tint)
	}
	painter := newTestPainter(t, &canvas, config)
	before := len(canvas.Triangles())

	second = long
	tint = ui.Color{Red: 0.1, Green: 0.1, Blue: 0.15, Alpha: 1}
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if len(canvas.Triangles()) == before {
		t.Fatal("the triangle count did not change; the test is not exercising what it claims")
	}
	want := scene.Color{Red: 0.1, Green: 0.1, Blue: 0.15, Alpha: 1}
	for index, instance := range frame.Rectangles {
		if instance.Color != want {
			t.Fatalf("instance %d (kind %d) is %+v, want %+v; update=%+v", index, instance.Kind, instance.Color, want, frame.Update)
		}
	}
}

// Rectangles that first appear after triangles exist sit behind them in the
// scene's child order. When the triangle list later shrinks while those
// rectangles are recolored, the recolored rectangle is queued before the
// deletes that change its previous sibling. The renderer used to reject that
// batch, and because the scene had already taken the new values the skipped
// nodes kept their old colors forever: a theme switch left most icons in the
// previous theme.
func TestRepaintRecoloringRectanglesBehindShrinkingTrianglesRefreshesEverything(t *testing.T) {
	short := ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound,
		Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 4), ui.LineTo(20, 20)}}
	long := ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound,
		Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 4), ui.LineTo(20, 20), ui.LineTo(4, 20), ui.LineTo(4, 4), ui.LineTo(12, 12)}}
	icon, extra := long, 0
	tint := ui.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
	var canvas ui.Canvas
	config := Config{Title: "recolor", Width: 200, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: 200, Height: 120}, tint)
		canvas.DrawIcon(icon, ui.Rect{X: 10, Y: 10, Width: 16, Height: 16}, tint)
		for index := 0; index < extra; index++ {
			canvas.Fill(ui.Rect{X: float32(40 + index*10), Y: 40, Width: 8, Height: 8}, tint)
		}
	}
	painter := newTestPainter(t, &canvas, config)

	extra = 3 // new rectangle nodes, appended behind the triangles
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	trianglesBefore := len(canvas.Triangles())

	icon = short
	tint = ui.Color{Red: 0.1, Green: 0.1, Blue: 0.15, Alpha: 1}
	frame, err := painter.repaint()
	if err != nil {
		t.Fatalf("repaint: %v", err)
	}
	if len(canvas.Triangles()) >= trianglesBefore {
		t.Fatal("the triangle list did not shrink; the test is not exercising what it claims")
	}
	if painter.resyncs != 0 {
		t.Fatalf("the renderer rejected the batch and had to be rebuilt %d times", painter.resyncs)
	}
	want := scene.Color{Red: 0.1, Green: 0.1, Blue: 0.15, Alpha: 1}
	if len(frame.Rectangles) != len(canvas.Rectangles())+len(canvas.Triangles()) {
		t.Fatalf("the renderer holds %d instances, the canvas painted %d", len(frame.Rectangles), len(canvas.Rectangles())+len(canvas.Triangles()))
	}
	for index, instance := range frame.Rectangles {
		if instance.Color != want {
			t.Fatalf("instance %d (kind %d) kept an old color: %+v", index, instance.Kind, instance.Color)
		}
	}
}

// If the renderer ever disagrees with the scene anyway, the painter rebuilds
// both from the canvas instead of dropping the batch and carrying on with
// state that will never converge.
func TestRepaintRebuildsRenderStateThatDisagreesWithTheScene(t *testing.T) {
	tint := ui.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
	var canvas ui.Canvas
	config := Config{Title: "resync", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: 100, Height: 100}, tint)
		canvas.Rounded(ui.Rect{X: 10, Y: 10, Width: 30, Height: 30}, 4, tint)
	}
	painter := newTestPainter(t, &canvas, config)

	painter.renderer = render.Store{} // forget every node the scene believes was published
	tint = ui.Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}
	frame, err := painter.repaint()
	if err != nil {
		t.Fatalf("repaint: %v", err)
	}
	if painter.resyncs != 1 {
		t.Fatalf("resyncs = %d, want exactly one rebuild", painter.resyncs)
	}
	if !frame.Update.Rebuild || len(frame.Rectangles) != 2 {
		t.Fatalf("update=%+v instances=%d, want a full rebuild of both rectangles", frame.Update, len(frame.Rectangles))
	}
	for _, instance := range frame.Rectangles {
		if instance.Color != (scene.Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}) {
			t.Fatalf("instance color = %+v", instance.Color)
		}
	}

	// And it keeps working incrementally afterwards.
	tint = ui.Color{Red: 0.6, Green: 0.4, Blue: 0.2, Alpha: 1}
	frame, err = painter.repaint()
	if err != nil || frame.Update.Empty() || frame.Rectangles[0].Color.Red != 0.6 {
		t.Fatalf("incremental repaint after recovery: update=%+v err=%v", frame.Update, err)
	}
}

// A text input that sets nothing but its editor draws where the field painted
// its own text, at the size and color it used. Focusing the field must not make
// the text jump, which is what happened when applications guessed these.
func TestBareTextInputTakesTheGeometryTheComponentDeclared(t *testing.T) {
	theme := ui.DarkTheme()
	theme.Metrics = theme.Metrics.WithDensity(ui.DensityCompact)
	editor := text.NewEditor("hello")
	override := false
	var canvas ui.Canvas
	config := Config{Title: "geometry", Width: 300, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.PaintInput(canvas, theme, ui.Rect{X: 20, Y: 20, Width: 200, Height: 30}, components.InputSnapshot{ID: ui.Item(5), Label: "Name", Value: "hello"})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if override {
			return &TextInput{Editor: editor, Bounds: ui.Rect{X: 1, Y: 2, Width: 3, Height: 4}, FontSize: 9, Color: ui.Color{Red: 1, Alpha: 1}}
		}
		return &TextInput{Editor: editor}
	}
	painter := newTestPainter(t, &canvas, config)
	staticText := canvas.Texts()[0]

	frame, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 40, Y: 30})
	if err != nil {
		t.Fatal(err)
	}
	native := frame.TextInput.Input
	if native == nil {
		t.Fatal("focusing the field published no native editor")
	}
	if native.X != staticText.Bounds.X || native.Y != staticText.Bounds.Y ||
		native.Width != staticText.Bounds.Width || native.Height != staticText.Bounds.Height {
		t.Fatalf("native editor at %v,%v %vx%v; the static text sits at %+v", native.X, native.Y, native.Width, native.Height, staticText.Bounds)
	}
	if native.FontSize != staticText.Style.Size || native.FontSize != theme.Metrics.FontSizeM {
		t.Fatalf("native font size %v, static text %v, theme %v", native.FontSize, staticText.Style.Size, theme.Metrics.FontSizeM)
	}
	wantColor := platform.Color{Red: float64(staticText.Style.Color.Red), Green: float64(staticText.Style.Color.Green), Blue: float64(staticText.Style.Color.Blue), Alpha: float64(staticText.Style.Color.Alpha)}
	if native.Color != wantColor {
		t.Fatalf("native color %+v, static text %+v", native.Color, wantColor)
	}

	// Anything the application sets itself is respected.
	override = true
	var overridden ui.Canvas
	other := newTestPainter(t, &overridden, config)
	frame, err = other.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 40, Y: 30})
	if err != nil {
		t.Fatal(err)
	}
	native = frame.TextInput.Input
	if native == nil || native.X != 1 || native.Y != 2 || native.Width != 3 || native.Height != 4 || native.FontSize != 9 || native.Color.Red != 1 {
		t.Fatalf("explicit values were replaced: %+v", native)
	}
}

func TestPainterPassesTheSelfPaintedTextOverlayPolicyToThePlatform(t *testing.T) {
	theme := ui.DarkTheme()
	editor := text.NewEditor("value")
	var canvas ui.Canvas
	var providerContext TextInputContext
	config := Config{Title: "overlay policy", Width: 260, Height: 90}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 12, Y: 12, Width: 220, Height: 50}, "Source", ui.HitOptions{
			Role: ui.RoleTextField, Multiline: true, TextColor: ui.Color{}, HideNativeOverlays: true,
			TextBounds: ui.Rect{X: 16, Y: 16, Width: 210, Height: 42}, TextSize: theme.Metrics.FontSizeM,
		})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		providerContext = context
		return &TextInput{Editor: editor}
	}
	painter := newTestPainter(t, &canvas, config)
	frame, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 20, Y: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !providerContext.HideNativeOverlays {
		t.Fatal("the focused text context lost the overlay policy")
	}
	if frame.TextInput.Input == nil || !frame.TextInput.Input.HideNativeOverlays {
		t.Fatalf("native text input = %+v, want hidden platform caret and selection", frame.TextInput.Input)
	}
}

// A double click is a property of the press sequence, not something each
// application recomputes: the count the platform reports reaches OnPointer on
// the press and on its release, and move events carry none.
func TestPointerEventsCarryTheClickCountToTheApplication(t *testing.T) {
	var canvas ui.Canvas
	var seen []ui.PointerEvent
	config := Config{Title: "clicks", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 80, Height: 30}, "Row")
	}
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		seen = append(seen, event)
		return ui.EventResult{}
	}
	painter := newTestPainter(t, &canvas, config)
	for _, event := range []platform.PointerEvent{
		{Kind: platform.PointerMove, X: 20, Y: 20},
		{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20, ClickCount: 1},
		{Kind: platform.PointerUp, Button: ui.PointerButtonPrimary, X: 20, Y: 20, ClickCount: 1},
		{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20, ClickCount: 2},
		{Kind: platform.PointerUp, Button: ui.PointerButtonPrimary, X: 20, Y: 20, ClickCount: 2},
	} {
		if _, err := painter.pointer(event); err != nil {
			t.Fatal(err)
		}
	}
	var got []int
	for _, event := range seen {
		got = append(got, event.ClickCount)
	}
	want := []int{0, 1, 1, 2, 2}
	if len(got) != len(want) {
		t.Fatalf("click counts = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("click counts = %v, want %v", got, want)
		}
	}
	if seen[3].Target != ui.Item(1) || seen[3].Kind != ui.PointerDown {
		t.Fatalf("the double click's press = %+v", seen[3])
	}
}

func TestGridLinesPublishOnlyWhenTheirRunsChange(t *testing.T) {
	var canvas ui.Canvas
	color := ui.Color{Red: 1, Alpha: 1}
	runs := []ui.TextRun{{Bytes: 2, Cells: 2, Color: color}}
	style := ui.TextStyle{Size: 12}
	config := Config{Title: "grid", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		// The application edits one buffer in place between paints, as a
		// terminal view does.
		canvas.LabelRuns("ab", ui.Rect{X: 2, Y: 2, Width: 14, Height: 12}, style, 7, runs)
	}
	painter := newTestPainter(t, &canvas, config)

	frame, err := painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels != nil {
		t.Fatalf("an unchanged grid line was published again: %+v", frame.TextLabels)
	}

	runs[0].Color = ui.Color{Green: 1, Alpha: 1} // same text, new color
	frame, err = painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 1 || !frame.TextLabels[0].Mono || frame.TextLabels[0].CellWidth != 7 ||
		len(frame.TextLabels[0].Runs) != 1 || frame.TextLabels[0].Runs[0].Color.Green != 1 {
		t.Fatalf("a recolored run was not published: %+v", frame.TextLabels)
	}

	style.MonoFace = ui.MonoClassic
	frame, err = painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 1 || frame.TextLabels[0].MonoFace != ui.MonoClassic {
		t.Fatalf("a changed mono face was not published: %+v", frame.TextLabels)
	}

	frame, err = painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels != nil {
		t.Fatal("repainting the recolored line published it again")
	}

	runs = append(runs[:0], ui.TextRun{Bytes: 1, Cells: 1}, ui.TextRun{Column: 1, Bytes: 1, Cells: 1})
	frame, err = painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 1 || len(frame.TextLabels[0].Runs) != 2 {
		t.Fatalf("splitting a run was not published: %+v", frame.TextLabels)
	}
}

func TestOnResizeRunsBeforeTheRepaintOnlyWhenTheSizeChanges(t *testing.T) {
	var canvas ui.Canvas
	var resizes []ui.Size
	columns := 0
	config := Config{Title: "resize", Width: 100, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		// Paint only reads what OnResize derived.
		canvas.Label(strconv.Itoa(columns), ui.Rect{Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	}
	config.OnResize = func(size ui.Size) {
		resizes = append(resizes, size)
		columns = int(size.Width) / 10
	}
	painter := newTestPainter(t, &canvas, config)
	if len(resizes) != 0 {
		t.Fatalf("the initial size announced a resize: %v", resizes)
	}
	frame, err := painter.resize(size(200, 80))
	if err != nil {
		t.Fatal(err)
	}
	if len(resizes) != 1 || resizes[0] != (ui.Size{Width: 200, Height: 80}) {
		t.Fatalf("resizes = %v", resizes)
	}
	if len(frame.TextLabels) != 1 || frame.TextLabels[0].Value != "20" {
		t.Fatalf("the repaint did not see what OnResize derived: %+v", frame.TextLabels)
	}
	if _, err := painter.resize(size(200, 80)); err != nil {
		t.Fatal(err)
	}
	if len(resizes) != 1 {
		t.Fatalf("an unchanged size announced a resize: %v", resizes)
	}
}

func TestAutofocusTakesTheKeyboardWhenAHitAppearsAndNotWhileItStays(t *testing.T) {
	var canvas ui.Canvas
	page := 1
	config := Config{Title: "autofocus", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 80, Height: 20}, "Always there", ui.HitOptions{})
		switch page {
		case 1:
			canvas.HitWithOptions(ui.Item(10), ui.Rect{Y: 30, Width: 150, Height: 50}, "First page", ui.HitOptions{Role: ui.RoleTerminal, Autofocus: true})
		case 2:
			canvas.HitWithOptions(ui.Item(11), ui.Rect{Y: 30, Width: 150, Height: 50}, "Second page", ui.HitOptions{Role: ui.RoleTerminal, Autofocus: true})
		case 3:
			canvas.HitWithOptions(ui.Item(12), ui.Rect{Y: 30, Width: 150, Height: 50}, "Disabled", ui.HitOptions{Autofocus: true, Disabled: true})
		}
	}
	painter := newTestPainter(t, &canvas, config)
	if painter.focused != ui.Item(10) {
		t.Fatalf("the first paint left focus on %d, want the autofocus hit 10", painter.focused)
	}

	// The user moves on; repainting the same page must not pull focus back.
	if _, err := painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, X: 10, Y: 10}); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(1) {
		t.Fatalf("clicking the always-there hit left focus on %d", painter.focused)
	}
	if _, err := painter.resize(size(210, 100)); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(1) {
		t.Fatalf("a repaint of the same page moved focus to %d", painter.focused)
	}

	// A new page appears: its autofocus hit takes the keyboard.
	page = 2
	if _, err := painter.resize(size(220, 100)); err != nil {
		t.Fatal(err)
	}
	if painter.focused != ui.Item(11) {
		t.Fatalf("the new page's hit did not take focus: %d", painter.focused)
	}

	// An autofocus hit that cannot be focused leaves focus alone.
	page = 3
	if _, err := painter.resize(size(230, 100)); err != nil {
		t.Fatal(err)
	}
	if painter.focused == ui.Item(12) {
		t.Fatal("a disabled hit took focus")
	}
}

func TestAnAutofocusedFieldInTheFirstPaintIsOfferedToThePlatformAtStartup(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "initial", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: size.Width, Height: size.Height}, "Terminal", ui.HitOptions{Role: ui.RoleTerminal, Autofocus: true})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		if !context.Terminal {
			return nil
		}
		return &TextInput{Bounds: ui.Rect{X: 4, Y: 4, Width: 8, Height: 16}, FontSize: 13, OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil }}
	}
	painter := newTestPainter(t, &canvas, config)
	initial := painter.initialTextInput()
	if initial == nil || !initial.Terminal || initial.Width != 8 || initial.OnEvent == nil {
		t.Fatalf("the window would open without its terminal's text input: %+v", initial)
	}
	// Later paints do not announce it again; the platform already has it.
	if frame, err := painter.resize(size(210, 100)); err != nil || frame.TextInput.Changed {
		t.Fatalf("repaint announced the editor again: %+v, %v", frame.TextInput, err)
	}

	// A page with nothing focused offers nothing.
	var plain ui.Canvas
	painter = newTestPainter(t, &plain, Config{Title: "plain", Width: 100, Height: 100, Paint: func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 50, Height: 20}, "Button", ui.HitOptions{})
	}, TextInput: config.TextInput})
	if painter.initialTextInput() != nil {
		t.Fatal("a window with no focused editor offered one")
	}
}

func TestTerminalAltPolicyIsPublishedAndUpdated(t *testing.T) {
	var canvas ui.Canvas
	altSendsEscape := false
	monoFace := ui.MonoSystem
	config := Config{Title: "terminal-alt", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: size.Width, Height: size.Height}, "Terminal", ui.HitOptions{Role: ui.RoleTerminal, Autofocus: true})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		return &TextInput{
			Bounds: ui.Rect{X: 4, Y: 4, Width: 8, Height: 16}, FontSize: 13,
			AltSendsEscape: altSendsEscape, MonoFace: monoFace,
			OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
		}
	}
	painter := newTestPainter(t, &canvas, config)
	initial := painter.initialTextInput()
	if initial == nil || initial.AltSendsEscape {
		t.Fatalf("initial terminal input = %+v", initial)
	}

	altSendsEscape = true
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if !frame.TextInput.Changed || frame.TextInput.Input == nil || !frame.TextInput.Input.AltSendsEscape {
		t.Fatalf("Alt policy update = %+v", frame.TextInput)
	}
	monoFace = ui.MonoClassic
	frame, err = painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if !frame.TextInput.Changed || frame.TextInput.Input == nil || frame.TextInput.Input.MonoFace != ui.MonoClassic {
		t.Fatalf("mono face update = %+v", frame.TextInput)
	}
	if frame, err = painter.repaint(); err != nil || frame.TextInput.Changed {
		t.Fatalf("unchanged Alt policy republished = %+v, %v", frame.TextInput, err)
	}
}

// A dialog's dismiss area covers the whole window, titlebar strip included. It
// is not a control, so the strip keeps dragging the window while a dialog is
// open; the dialog's own buttons are controls.
func TestAnOpenDialogLeavesTheTitlebarStripDraggableAndDismissible(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "bar", Width: 400, Height: 300}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.MoveRegion(ui.Rect{Width: size.Width, Height: 32})
		dialog := components.BeginDialog(canvas, ui.DarkTheme(), ui.Rect{Width: size.Width, Height: size.Height}, components.DialogOptions{
			Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(2), CloseID: ui.Item(3), ContentSize: ui.Size{Width: 200, Height: 140},
		})
		dialog.Paint("Title", "Description")
	}
	painter := newTestPainter(t, &canvas, config)
	if painter.pressAt(20, 16) != platform.PressMoveOnDrag {
		t.Fatal("a move region under a dialog's backdrop must hand the press to the backdrop and still move the window on a drag")
	}
	close := painter.canvas.Hits()
	for _, hit := range close {
		if hit.ID == ui.Item(3) && painter.pressAt(hit.Bounds.X+2, hit.Bounds.Y+2) == platform.PressControl {
			return
		}
	}
	t.Fatal("the dialog's close button is not a control")
}

// A shadow reaches the renderer as one instance that keeps the casting rectangle
// and its placement, in the place it was painted among the rectangles; it is
// drawn when only its blur reaches into the visible area; and a repaint that
// changes nothing costs the platform nothing.
func TestShadowsReachTheRendererAndRepaintWithoutChangePerformsNoWork(t *testing.T) {
	paint := func(canvas *ui.Canvas, _ ui.Size) {
		canvas.Fill(ui.Rect{Width: 100, Height: 100}, ui.Color{Alpha: 1})
		canvas.Clip(ui.Rect{X: 0, Y: 0, Width: 100, Height: 100})
		canvas.Elevate(ui.Rect{X: 30, Y: 30, Width: 40, Height: 20}, 6, ui.ShadowLG)
		canvas.Rounded(ui.Rect{X: 30, Y: 30, Width: 40, Height: 20}, 6, ui.Color{Alpha: 1})
		// Its body is below the clip, but a blur of 15 reaches 12 pixels into it.
		canvas.Shadow(ui.Rect{X: 30, Y: 110, Width: 40, Height: 20}, 6, ui.ShadowStyle{Blur: 15}, ui.Color{Alpha: 0.1})
		// Its reach ends before the clip begins.
		canvas.Shadow(ui.Rect{X: 30, Y: 130, Width: 40, Height: 20}, 6, ui.ShadowStyle{Blur: 15}, ui.Color{Alpha: 0.1})
		canvas.Unclip()
	}
	var canvas ui.Canvas
	paint(&canvas, ui.Size{})
	painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 100, Height: 100, Paint: paint})

	instances := painter.renderer.Rectangles()
	if len(instances) != 5 {
		t.Fatalf("renderer holds %d instances, want the page, the two layers of the large shadow, the surface and the shadow that reaches in: %+v", len(instances), instances)
	}
	kinds := []scene.Kind{scene.KindRectangle, scene.KindShadow, scene.KindShadow, scene.KindRectangle, scene.KindShadow}
	for index, want := range kinds {
		if index >= len(instances) || instances[index].Kind != want {
			t.Fatalf("instance %d is not kind %d: %+v", index, want, instances)
		}
	}
	for _, index := range []int{1, 2} {
		shadow := instances[index]
		if shadow.Bounds != (scene.Rect{X: 30, Y: 30, Width: 40, Height: 20}) || shadow.Radius != 6 || shadow.Shadow.Blur == 0 {
			t.Errorf("layer %d = %+v, want a shadow cast by the surface", index, shadow)
		}
	}
	if reaching := instances[4]; !reaching.Clipped {
		t.Errorf("shadow reaching into the clip = %+v, want it cut by the clip", reaching)
	}

	frame, err := painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Update.Empty() {
		t.Fatalf("unchanged repaint produced renderer update %+v", frame.Update)
	}
}

// A shape that becomes a shadow, and back, keeps its place in paint order.
func TestAShapeBecomingAShadowKeepsItsPlace(t *testing.T) {
	soft := false
	paint := func(canvas *ui.Canvas, _ ui.Size) {
		canvas.Fill(ui.Rect{Width: 100, Height: 100}, ui.Color{Alpha: 1})
		bounds := ui.Rect{X: 20, Y: 20, Width: 40, Height: 40}
		if soft {
			canvas.Shadow(bounds, 4, ui.ShadowStyle{OffsetY: 2, Blur: 6}, ui.Color{Alpha: 0.2})
		} else {
			canvas.Rounded(bounds, 4, ui.Color{Alpha: 0.2})
		}
		canvas.Fill(ui.Rect{X: 30, Y: 30, Width: 10, Height: 10}, ui.Color{Alpha: 1})
	}
	var canvas ui.Canvas
	paint(&canvas, ui.Size{})
	painter := newTestPainter(t, &canvas, Config{Title: "test", Width: 100, Height: 100, Paint: paint})
	for _, want := range []scene.Kind{scene.KindShadow, scene.KindRectangle} {
		soft = !soft
		frame, err := painter.resize(size(100, 100))
		if err != nil {
			t.Fatal(err)
		}
		instances := painter.renderer.Rectangles()
		if len(instances) != 3 || instances[1].Kind != want {
			t.Fatalf("instances = %+v, want the middle one to be kind %d", instances, want)
		}
		if frame.Update.Rebuild || frame.Update.Count != 1 || frame.Update.First != 1 {
			t.Fatalf("update = %+v, want only the middle instance", frame.Update)
		}
	}
}

// A modal leaves only its own controls and labels in the accessibility tree;
// the covered page and the dismiss backdrop are not offered to a screen reader.
func TestModalHidesThePageAndBackdropFromAccessibility(t *testing.T) {
	config := Config{Title: "modal a11y", Width: 400, Height: 300}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 300, Y: 200, Width: 60, Height: 30}, "Page")
		dialog := components.BeginDialog(canvas, ui.LightTheme(), ui.Rect{Width: 400, Height: 300}, components.DialogOptions{
			Layer: 5, Scope: ui.Item(20), DismissID: ui.Item(21), CloseID: ui.Item(22), ContentSize: ui.Size{Width: 240, Height: 160},
		})
		layout := dialog.Begin("Title", "Description")
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(23), ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: 80, Height: 30}).Default("Save")
		dialog.End()
	}
	var canvas ui.Canvas
	painter := newTestPainter(t, &canvas, config)
	labels := map[string]bool{}
	for _, node := range painter.accessibility {
		labels[node.Label] = true
	}
	if !labels["Save"] {
		t.Fatalf("the dialog control is missing: %+v", painter.accessibility)
	}
	if labels["Page"] || labels["Dismiss dialog"] {
		t.Fatalf("the page or backdrop stayed accessible: %+v", painter.accessibility)
	}
}

// Escape during an input-method composition cancels the composition; it does not
// dismiss the overlay behind the field.
func TestEscapeDuringCompositionDoesNotDismissTheOverlay(t *testing.T) {
	var canvas ui.Canvas
	open := true
	clicks := 0
	config := Config{Title: "ime", Width: 200, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		if !open {
			return
		}
		canvas.HitWithLabel(ui.Item(2), ui.Rect{X: 10, Y: 10, Width: 80, Height: 24}, "Close")
		canvas.SetEscapeTarget(ui.Item(2))
	}
	onClick(&config, func(id ui.ID) {
		if id == ui.Item(2) {
			clicks++
			open = false
		}
	})
	painter := newTestPainter(t, &canvas, config)
	painter.nativeText.State.Composing = true
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyEscape}); err != nil {
		t.Fatal(err)
	}
	if clicks != 0 || !open {
		t.Fatalf("Escape while composing dismissed: clicks=%d open=%v", clicks, open)
	}
	painter.nativeText.State.Composing = false
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyEscape}); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 || open {
		t.Fatalf("Escape after composition: clicks=%d open=%v", clicks, open)
	}
}

// A non-modal popover takes its named initial focus once; Tab can leave it
// without the still-open popover pulling focus back on the next paint.
func TestPopoverInitialFocusIsHonoredOnce(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "popover focus", Width: 400, Height: 300}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(1), ui.Rect{X: 250, Y: 200, Width: 80, Height: 30}).Default("Page")
		popover := components.BeginPopover(canvas, ui.LightTheme(), ui.Rect{Width: 400, Height: 300}, ui.Rect{X: 20, Y: 20, Width: 80, Height: 30}, components.PopoverOptions{
			Layer: 5, Scope: ui.Item(9), DismissID: ui.Item(90), InitialFocusID: ui.Item(5), ContentSize: ui.Size{Width: 180, Height: 120},
		})
		layout := popover.Begin("Filters", "Narrow the results.")
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(5), ui.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: 70, Height: 28}).Outline("Apply")
		popover.End()
	}
	painter := newTestPainter(t, &canvas, config)
	if painter.focused != ui.Item(5) {
		t.Fatalf("initial focus = %v, want the popover control", painter.focused)
	}
	if _, err := painter.key(platform.KeyEvent{Key: input.KeyTab}); err != nil {
		t.Fatal(err)
	}
	if painter.focused == ui.Item(5) {
		t.Fatal("Tab could not leave the open popover")
	}
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if painter.focused == ui.Item(5) {
		t.Fatal("the focus request pulled focus back after Tab left")
	}
}

// Adding one shape at the end of a page (a tooltip, an overlay, a new row) must
// upload the new instance only, not rebuild the whole scene.
func TestPainterAppendsOneShapeWithoutRebuild(t *testing.T) {
	var canvas ui.Canvas
	count := 1
	painter := newTestPainter(t, &canvas, Config{Title: "append", Width: 200, Height: 200,
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			for index := 0; index < count; index++ {
				canvas.Fill(ui.Rect{X: float32(index * 10), Width: 8, Height: 8}, ui.Color{Alpha: 1})
			}
		}})
	count = 2
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Update.Rebuild || frame.Update != (render.Update{First: 1, Count: 1}) {
		t.Fatalf("appending one shape = %+v, want a one-instance suffix", frame.Update)
	}
}

func TestPainterDropsLabelsWithoutAreaOrFontSize(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Title: "invalid labels", Width: 120, Height: 60}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Label("no area", ui.Rect{Width: 0, Height: 12}, ui.TextStyle{Size: 12})
		canvas.Label("subpixel width", ui.Rect{Width: 0.3, Height: 12}, ui.TextStyle{Size: 12})
		canvas.Label("subpixel height", ui.Rect{Width: 40, Height: 0.3}, ui.TextStyle{Size: 12})
		canvas.Label("no font", ui.Rect{Width: 40, Height: 12}, ui.TextStyle{})
		canvas.Label("visible", ui.Rect{Width: 40, Height: 12}, ui.TextStyle{Size: 12})
	}
	painter := newTestPainter(t, &canvas, config)
	if len(painter.labels) != 1 || painter.labels[0].Value != "visible" {
		t.Fatalf("labels = %+v, want only the visible one", painter.labels)
	}
	if _, err := painter.repaint(); err != nil {
		t.Fatalf("repaint failed: %v", err)
	}
}

func TestPainterDropsSubpixelAndClippedImages(t *testing.T) {
	var canvas ui.Canvas
	pixels := make([]byte, 4*4*4)
	bitmap := ui.Bitmap{Pix: pixels, Width: 4, Height: 4, Stride: 16, ID: 1}

	config := Config{Title: "invalid images", Width: 120, Height: 60}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		// Zero area image
		canvas.DrawBitmap(bitmap, ui.Rect{Width: 0, Height: 20})
		// Subpixel width directly
		canvas.DrawBitmap(bitmap, ui.Rect{Width: 0.3, Height: 20})
		// Subpixel height directly
		canvas.DrawBitmap(bitmap, ui.Rect{Width: 20, Height: 0.4})
		// Large image clipped to subpixel width (< 0.5) by scissor
		canvas.Clip(ui.Rect{X: 0, Y: 0, Width: 0.3, Height: 20})
		canvas.DrawBitmap(bitmap, ui.Rect{Width: 20, Height: 20})
		canvas.Unclip()
		// Visible image
		canvas.DrawBitmap(bitmap, ui.Rect{X: 10, Y: 10, Width: 20, Height: 20})
	}
	painter := newTestPainter(t, &canvas, config)
	if len(painter.labels) != 1 {
		t.Fatalf("labels count = %d, want 1; labels = %+v", len(painter.labels), painter.labels)
	}
	if painter.labels[0].Width <= 0 || painter.labels[0].Height <= 0 {
		t.Fatalf("visible image label has non-positive dimensions: %+v", painter.labels[0])
	}
	if _, err := painter.repaint(); err != nil {
		t.Fatalf("repaint failed: %v", err)
	}
}

func TestPainterDisablesTextInputOnSubpixelBounds(t *testing.T) {
	var canvas ui.Canvas
	editor := text.NewEditor("Hello")
	fieldWidth := float32(100)

	config := Config{
		Title:  "subpixel text input",
		Width:  200,
		Height: 100,
		Paint: func(c *ui.Canvas, _ ui.Size) {
			c.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: fieldWidth, Height: 24}, "field", ui.HitOptions{Role: ui.RoleTextField})
		},
		TextInput: func(got TextInputContext) *TextInput {
			return &TextInput{
				Editor:   editor,
				Bounds:   ui.Rect{X: 10, Y: 10, Width: fieldWidth, Height: 24},
				FontSize: 13,
			}
		},
	}
	painter := newTestPainter(t, &canvas, config)

	// Focus the text field
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		frame, err := painter.pointer(platform.PointerEvent{Kind: kind, X: 15, Y: 15})
		if err != nil {
			t.Fatal(err)
		}
		if frame.TextInput.Changed && frame.TextInput.Input != nil {
			// focused and enabled
		}
	}
	if !painter.textEnabled || painter.activeText == nil {
		t.Fatal("expected text input to be enabled when bounds are normal")
	}

	// Shrink the text field bounds to subpixel (< 0.5) to simulate extreme resize compression
	fieldWidth = 0.3
	frame, err := painter.repaint()
	if err != nil {
		t.Fatalf("repaint failed when field is subpixel: %v", err)
	}
	if painter.textEnabled {
		t.Fatal("expected text input to be disabled when bounds are subpixel")
	}
	if !frame.TextInput.Changed || frame.TextInput.Input != nil {
		t.Fatalf("expected TextInput update to report disabled (Changed=true, Input=nil), got %+v", frame.TextInput)
	}

	// Expand the text field bounds back to normal
	fieldWidth = 100
	frame, err = painter.repaint()
	if err != nil {
		t.Fatalf("repaint failed after restoring field bounds: %v", err)
	}
	if !painter.textEnabled {
		t.Fatal("expected text input to be re-enabled after bounds restored")
	}
	if !frame.TextInput.Changed || frame.TextInput.Input == nil || frame.TextInput.Input.Width <= 0 {
		t.Fatalf("expected TextInput update to report re-enabled, got %+v", frame.TextInput)
	}
}

// A grid line's Block Elements reach the platform as rectangles, not as text a font would
// draw with side bearings and a filtered edge.
func TestGridLinesDrawBlockElementsAsRectanglesNotText(t *testing.T) {
	var canvas ui.Canvas
	color := ui.Color{Green: 1, Alpha: 1}
	runs := []ui.TextRun{
		{Bytes: 1, Cells: 1, Color: color},
		{Column: 1, Bytes: 3, Cells: 1, Color: color},
		{Column: 2, Bytes: 3, Cells: 1, Color: color},
		{Column: 3, Bytes: 1, Cells: 1, Color: color},
	}
	config := Config{Title: "blocks", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.LabelRuns("a██b", ui.Rect{X: 2, Y: 2, Width: 28, Height: 12}, ui.TextStyle{Size: 12}, 7, runs)
	}
	painter := newTestPainter(t, &canvas, config)

	for index := range runs {
		runs[index].Color.Red = 0.5 // a repaint with a change, which the platform is given
	}
	frame, err := painter.resize(size(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 1 || frame.TextLabels[0].Value != "a  b" {
		t.Fatalf("the platform was given %+v, want the blocks as blank cells", frame.TextLabels)
	}
	label := frame.TextLabels[0]
	if len(label.Runs) != 4 || label.Runs[1].Bytes != 1 || label.Runs[2].Column != 2 {
		t.Fatalf("runs after replacing the blocks = %+v", label.Runs)
	}
	var blocks int
	for _, rectangle := range frame.Rectangles {
		if rectangle.Color.Green == 1 {
			blocks++
			if rectangle.Bounds.X != 9 || rectangle.Bounds.Width != 14 || rectangle.Bounds.Y != 2 || rectangle.Bounds.Height != 12 {
				t.Fatalf("the two blocks are %+v, want one rectangle over cells 1 and 2", rectangle.Bounds)
			}
		}
	}
	if blocks != 1 {
		t.Fatalf("%d block rectangles in %+v", blocks, frame.Rectangles)
	}
	if got := canvas.Texts()[0].Value; got != "a██b" {
		t.Fatalf("the canvas's own text was rewritten: %q", got)
	}
}
