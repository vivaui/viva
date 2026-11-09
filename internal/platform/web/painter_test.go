//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type rowPlacement struct{}

func (rowPlacement) OffsetAt(point gfx.Point) (int, bool) {
	if point.X < 10 {
		return 1, true
	}
	return 4, true
}

func (rowPlacement) CaretBounds() (gfx.Rect, bool) { return gfx.Rect{}, false }

func TestPlacedEditorPointerSelectsCanvasOffsets(t *testing.T) {
	var canvas gfx.Canvas
	editor := text.NewEditor("a中b")
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 100, Height: 40}, "source", gfx.HitOptions{
				Role: gfx.RoleTextField, Value: editor.String(), Multiline: true,
				HideNativeOverlays: true, Placement: rowPlacement{},
			})
		},
		TextInput: EditorInputs(func(id gfx.ID) (*text.Editor, bool) { return editor, id == gfx.Item(1) }),
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 40
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 5, Y: 10})
	if editor.Selection() != (text.Range{Start: 1, End: 1}) || !painter.textInput.Placed || !painter.textInput.HideNativeOverlays {
		t.Fatalf("click must place the canvas caret, got selection %+v, input %+v", editor.Selection(), painter.textInput)
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 30, Y: 10})
	if editor.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("drag must extend by rune, got %+v", editor.Selection())
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 30, Y: 10})
	if editor.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("release must retain selection, got %+v", editor.Selection())
	}
}

func TestPlacedRowKeysBelongToCanvas(t *testing.T) {
	for _, key := range []input.Key{input.KeyUp, input.KeyDown, input.KeyHome, input.KeyEnd, input.KeyPageUp, input.KeyPageDown} {
		if !placedRowKey(TextInput{Placed: true}, key, 0) || placedRowKey(TextInput{}, key, 0) {
			t.Fatalf("row key %v must be routed only for placed editors", key)
		}
	}
	if !placedRowKey(TextInput{Placed: true}, input.KeyLeft, gfx.ModifierMeta) || placedRowKey(TextInput{Placed: true}, input.KeyLeft, 0) {
		t.Fatal("only whole-line left navigation belongs to the placed editor")
	}
}

func TestPointerFocusMatchesNativeWithoutRing(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		components.BeginButton(canvas, gfx.LightTheme(), gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 90}).Default("Save")
		components.BeginButton(canvas, gfx.LightTheme(), gfx.Item(2), gfx.Rect{X: 110, Y: 10, Width: 90}).Secondary("Cancel")
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 240, 100
	painter.repaint()
	wantCount := len(canvas.Rectangles())
	wantBounds := canvas.Rectangles()[0].Bounds
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 20, Y: 20})
	if len(canvas.Rectangles()) != wantCount || canvas.Rectangles()[0].Bounds != wantBounds || !canvas.FocusVisible().IsZero() {
		t.Fatal("mouse press must not change button geometry or flash a focus ring")
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 20, Y: 20})
	if painter.focus.Focused() != gfx.Item(1) || canvas.Focused() != gfx.Item(1) || !canvas.FocusVisible().IsZero() {
		t.Fatal("mouse focus should persist without a keyboard ring")
	}
	painter.keyDown(gfx.KeyEvent{Key: gfx.KeyTab})
	if painter.focus.Focused() != gfx.Item(2) || canvas.FocusVisible() != gfx.Item(2) || len(canvas.Rectangles()) <= wantCount {
		t.Fatal("Tab should advance from the clicked control and show its ring")
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 120, Y: 20})
	if !canvas.FocusVisible().IsZero() || len(canvas.Rectangles()) != wantCount {
		t.Fatal("pointer input should hide the keyboard ring")
	}
}

func TestPointerEventsCaptureTargetAndHandledSuppressesClick(t *testing.T) {
	var canvas gfx.Canvas
	var events [3]gfx.PointerEvent
	eventCount := 0
	clicks := 0
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(7), gfx.Rect{Width: 100, Height: 100})
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			events[eventCount] = event
			eventCount++
			return gfx.EventResult{Handled: event.Kind == gfx.PointerDown}
		},
	}
	onClick(&config, func(gfx.ID) { clicks++ })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	if !painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Modifiers: gfx.ModifierShift, X: 10, Y: 10}) {
		t.Fatal("pointer down should change pressed and focus state")
	}
	if !painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, Modifiers: gfx.ModifierControl, X: 150, Y: 10}) {
		t.Fatal("moving outside should change hover state")
	}
	if !painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 10, Y: 10}) {
		t.Fatal("pointer up should clear pressed state")
	}

	if eventCount != len(events) {
		t.Fatalf("event count = %d, want %d", eventCount, len(events))
	}
	if events[0].Target != gfx.Item(7) || events[0].Captured || events[0].Modifiers != gfx.ModifierShift || events[0].Viewport != (gfx.Size{Width: 200, Height: 100}) {
		t.Fatalf("down event = %+v", events[0])
	}
	if events[1].Target != gfx.Item(7) || !events[1].Captured || events[1].Modifiers != gfx.ModifierControl {
		t.Fatalf("captured move event = %+v", events[1])
	}
	if events[2].Target != gfx.Item(7) || !events[2].Captured {
		t.Fatalf("captured up event = %+v", events[2])
	}
	if clicks != 0 {
		t.Fatalf("handled press produced %d clicks", clicks)
	}
}

func TestPointerMoveWithoutStateChangeDoesNotRepaint(t *testing.T) {
	var canvas gfx.Canvas
	paintCount := 0
	eventCount := 0
	response := gfx.EventResult{}
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			paintCount++
			canvas.Hit(gfx.Item(1), gfx.Rect{Width: 100, Height: 100})
		},
		OnPointer: func(gfx.PointerEvent) gfx.EventResult {
			eventCount++
			return response
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 100
	painter.repaint()
	if !painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 10, Y: 10}) {
		t.Fatal("first move should update hover")
	}
	before := paintCount
	if painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 20, Y: 20}) {
		t.Fatal("move within the same target should not request a frame")
	}
	if paintCount != before || eventCount != 2 {
		t.Fatalf("paint count = %d, event count = %d; want %d and 2", paintCount, eventCount, before)
	}
	response.Repaint = true
	if !painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 30, Y: 30}) {
		t.Fatal("explicit Repaint should request a frame")
	}
	if paintCount != before+1 || eventCount != 3 {
		t.Fatalf("explicit repaint counts = %d/%d, want %d/3", paintCount, eventCount, before+1)
	}
}

func TestClickRemainsPrimaryOnly(t *testing.T) {
	var canvas gfx.Canvas
	var clicked gfx.ID
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(5), gfx.Rect{Width: 100, Height: 100})
		},
	}
	onClick(&config, func(id gfx.ID) { clicked = id })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	if clicked != gfx.Item(5) {
		t.Fatalf("primary click = %d, want 5", clicked)
	}
	clicked = gfx.ID{}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonSecondary, X: 10, Y: 10})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonSecondary, X: 10, Y: 10})
	if !clicked.IsZero() {
		t.Fatalf("secondary button produced a click %d", clicked)
	}
}

func TestKeyEventResultSuppressesDefaultsAndControlsFocus(t *testing.T) {
	var canvas gfx.Canvas
	var received gfx.KeyEvent
	response := gfx.EventResult{Handled: true}
	clicks := 0
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{Width: 50, Height: 50})
			canvas.Hit(gfx.Item(2), gfx.Rect{X: 60, Width: 50, Height: 50})
			canvas.SetEscapeTarget(gfx.Item(9))
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			received = event
			return response
		},
	}
	onClick(&config, func(gfx.ID) { clicks++ })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 120, 50
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	clicks = 0

	if painter.keyDown(gfx.KeyEvent{Key: gfx.KeyEscape, Modifiers: gfx.ModifierAlt | gfx.ModifierMeta, Repeat: true}) {
		t.Fatal("handled Escape without state changes should not request a frame")
	}
	if received.Target != gfx.Item(1) || received.Modifiers != gfx.ModifierAlt|gfx.ModifierMeta || !received.Repeat {
		t.Fatalf("received key event = %+v", received)
	}
	if clicks != 0 {
		t.Fatalf("handled Escape produced %d clicks", clicks)
	}

	response = gfx.EventResult{Handled: true, Focus: gfx.Item(2), FocusVisible: true}
	if !painter.keyDown(gfx.KeyEvent{Key: gfx.KeyEnter}) {
		t.Fatal("focus result should request a frame")
	}
	if painter.focus.Focused() != gfx.Item(2) || !painter.focus.KeyboardFocus() || canvas.Focused() != gfx.Item(2) {
		t.Fatalf("focus state = %d/%v canvas=%d", painter.focus.Focused(), painter.focus.KeyboardFocus(), canvas.Focused())
	}
	if clicks != 0 {
		t.Fatalf("handled Enter produced %d clicks", clicks)
	}

	response = gfx.EventResult{Handled: true, Blur: true}
	if !painter.keyDown(gfx.KeyEvent{Key: gfx.KeyLeft}) {
		t.Fatal("blur result should request a frame")
	}
	if !painter.focus.Focused().IsZero() || painter.focus.KeyboardFocus() || !canvas.Focused().IsZero() {
		t.Fatalf("blur left focus state = %d/%v canvas=%d", painter.focus.Focused(), painter.focus.KeyboardFocus(), canvas.Focused())
	}
}

func TestScrollEventSupportsAxesTargetAndHandledResult(t *testing.T) {
	var canvas gfx.Canvas
	var received gfx.ScrollEvent
	response := gfx.EventResult{Handled: true}
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(3), gfx.Rect{Width: 100, Height: 100})
		},
		OnScroll: func(event gfx.ScrollEvent) gfx.EventResult {
			received = event
			return response
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 100
	painter.repaint()
	if handled, changed := painter.wheel(gfx.ScrollEvent{Modifiers: gfx.ModifierShift, X: 10, Y: 10, DeltaX: 4, DeltaY: 7, Precise: true}); !handled || changed {
		t.Fatalf("handled scroll = handled %v, changed %v; want true, false", handled, changed)
	}
	if received != (gfx.ScrollEvent{
		Modifiers: gfx.ModifierShift, X: 10, Y: 10, DeltaX: 4, DeltaY: 7,
		Viewport: gfx.Size{Width: 100, Height: 100}, Target: gfx.Item(3), Precise: true,
	}) {
		t.Fatalf("received scroll = %+v", received)
	}
	if painter.scrollX != 0 || painter.scrollY != 0 {
		t.Fatalf("handled scroll changed offset to %v,%v", painter.scrollX, painter.scrollY)
	}

	response = gfx.EventResult{}
	if handled, changed := painter.wheel(gfx.ScrollEvent{DeltaX: 4, DeltaY: 7}); handled || !changed {
		t.Fatalf("default scroll = handled %v, changed %v; want false, true", handled, changed)
	}
	if canvas.ScrollX() != 4 || canvas.ScrollY() != 7 {
		t.Fatalf("canvas scroll = %v,%v, want 4,7", canvas.ScrollX(), canvas.ScrollY())
	}
	if handled, changed := painter.wheel(gfx.ScrollEvent{}); handled || changed {
		t.Fatalf("zero scroll = handled %v, changed %v; want false, false", handled, changed)
	}

	config.ForwardWheel = func(float32, float32) {}
	response = gfx.EventResult{}
	beforeX, beforeY := painter.scrollX, painter.scrollY
	if handled, changed := painter.wheel(gfx.ScrollEvent{DeltaY: 9}); handled || changed {
		t.Fatalf("forwarded scroll = handled %v, changed %v; want false, false", handled, changed)
	}
	if painter.scrollX != beforeX || painter.scrollY != beforeY {
		t.Fatalf("forwarded scroll moved the canvas offset to %v,%v", painter.scrollX, painter.scrollY)
	}
}

func TestWebInputMappings(t *testing.T) {
	if webPointerButton(0) != gfx.PointerButtonPrimary || webPointerButton(1) != gfx.PointerButtonMiddle ||
		webPointerButton(2) != gfx.PointerButtonSecondary || webPointerButton(3) != gfx.PointerButtonBack ||
		webPointerButton(4) != gfx.PointerButtonForward || webPointerButton(5) != gfx.PointerButtonNone {
		t.Fatal("unexpected pointer button mapping")
	}
	modifiers := webModifiers(true, true, true, true)
	wantModifiers := gfx.ModifierShift | gfx.ModifierControl | gfx.ModifierAlt | gfx.ModifierMeta
	if modifiers != wantModifiers {
		t.Fatalf("modifiers = %d, want %d", modifiers, wantModifiers)
	}
	if x, y, precise := wheelDeltas(2, 3, 0, 100, 200); x != 2 || y != 3 || !precise {
		t.Fatalf("pixel wheel = %v,%v,%v", x, y, precise)
	}
	if x, y, precise := wheelDeltas(2, 3, 1, 100, 200); x != 32 || y != 48 || precise {
		t.Fatalf("line wheel = %v,%v,%v", x, y, precise)
	}
	if x, y, precise := wheelDeltas(2, 3, 2, 100, 200); x != 200 || y != 600 || precise {
		t.Fatalf("page wheel = %v,%v,%v", x, y, precise)
	}
}

func TestCollectLayersSortsBaseTextBeforeOverlayRectangle(t *testing.T) {
	rectangles := []gfx.Rectangle{
		{Layer: 7},
		{Layer: 3},
		{Layer: 7},
	}
	triangles := []gfx.Triangle{
		{Layer: 9},
		{Layer: 3},
	}
	texts := []gfx.Text{
		{Layer: 0},
		{Layer: 3},
	}
	layers := collectLayers(make([]uint16, 0, 4), rectangles, triangles, texts)
	want := []uint16{0, 3, 7, 9}
	if len(layers) != len(want) {
		t.Fatalf("layers = %v, want %v", layers, want)
	}
	for index := range want {
		if layers[index] != want[index] {
			t.Fatalf("layers = %v, want %v", layers, want)
		}
	}
}

func TestPainterResolvesOnlyFocusedEditableTextField(t *testing.T) {
	var canvas gfx.Canvas
	readOnly := false
	providerCalls := 0
	var context TextInputContext
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(7), gfx.Rect{X: 10, Y: 20, Width: 180, Height: 64}, "Message", gfx.HitOptions{
				Role: gfx.RoleTextField, Multiline: true, ReadOnly: readOnly,
			})
		},
		TextInput: func(current TextInputContext) *TextInput {
			providerCalls++
			context = current
			return &TextInput{OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil }}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 240, 100
	painter.repaint()
	if _, input := painter.activeTextInput(); input != nil || providerCalls != 0 {
		t.Fatalf("unfocused editor = %v calls=%d", input, providerCalls)
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 20, Y: 30})
	id, input := painter.activeTextInput()
	if id != gfx.Item(7) || input == nil || !input.Multiline || providerCalls != 1 {
		t.Fatalf("active editor = id %d input %+v calls=%d", id, input, providerCalls)
	}
	if context.ID != gfx.Item(7) || context.Bounds != (gfx.Rect{X: 10, Y: 20, Width: 180, Height: 64}) || !context.Multiline {
		t.Fatalf("provider context = %+v", context)
	}
	readOnly = true
	painter.repaint()
	if _, input = painter.activeTextInput(); input != nil || providerCalls != 1 {
		t.Fatalf("read-only editor = %v calls=%d", input, providerCalls)
	}
}

func TestModalOpenMovesFocusInsideWithinTheSameFrameAndCloseRestoresIt(t *testing.T) {
	const (
		opener  = 1
		dismiss = 2
		field   = 4
	)
	var canvas gfx.Canvas
	open := false
	var focusSeenByPaint gfx.ID
	config := Config{
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.HitWithLabel(gfx.Item(int(opener)), gfx.Rect{Width: 60, Height: 24}, "Open")
			if open {
				canvas.PushFocusScope(gfx.Item(9))
				canvas.HitWithOptions(gfx.Item(int(field)), gfx.Rect{X: 80, Width: 80, Height: 24}, "Field", gfx.HitOptions{Role: gfx.RoleTextField})
				canvas.PopFocusScope()
				canvas.TrapFocus(gfx.Item(9))
				canvas.SetInitialFocus(gfx.Item(int(field)))
				canvas.SetEscapeTarget(gfx.Item(int(dismiss)))
			}
			focusSeenByPaint = canvas.Focused()
		},
	}
	onClick(&config, func(id gfx.ID) { open = id == gfx.Item(int(opener)) })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	if !open || painter.focus.Focused() != gfx.Item(int(field)) || canvas.Focused() != gfx.Item(int(field)) || focusSeenByPaint != gfx.Item(int(field)) {
		t.Fatalf("after open: open=%v painter=%d canvas=%d paint saw=%d, want the dialog field %d",
			open, painter.focus.Focused(), canvas.Focused(), focusSeenByPaint, field)
	}

	// The open scope is announced again on every paint and must not steal
	// focus back from a user who has since moved it.
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(int(field)) {
		t.Fatalf("steady repaint moved focus to %d", painter.focus.Focused())
	}

	open = false
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(int(opener)) || canvas.Focused() != gfx.Item(int(opener)) || focusSeenByPaint != gfx.Item(int(opener)) {
		t.Fatalf("after close: painter=%d canvas=%d paint saw=%d, want the opener %d",
			painter.focus.Focused(), canvas.Focused(), focusSeenByPaint, opener)
	}
}

func TestPainterFillsUnsetTextInputGeometryFromTheDeclaredHit(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(7), gfx.Rect{X: 10, Y: 20, Width: 180, Height: 32}, "Name", gfx.HitOptions{
				Role: gfx.RoleTextField, TextBounds: gfx.Rect{X: 22, Y: 20, Width: 156, Height: 32},
				TextSize: 12, TextColor: gfx.Color{Red: 0.5, Green: 0.25, Blue: 0.125, Alpha: 1},
			})
		},
		TextInput: func(TextInputContext) *TextInput {
			return &TextInput{OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil }}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 240, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 30, Y: 30})
	_, input := painter.activeTextInput()
	if input == nil {
		t.Fatal("no active editor")
	}
	if input.X != 22 || input.Y != 20 || input.Width != 156 || input.Height != 32 || input.FontSize != 12 ||
		input.Color.Red != 0.5 || input.Color.Green != 0.25 || input.Color.Blue != 0.125 || input.Color.Alpha != 1 {
		t.Fatalf("browser input = %+v", input)
	}
}

// A control that reads its click while it paints (gfx.Canvas.Clicked) receives it
// in the event pass the painter runs, as it does natively.
func TestClickIsReadWhereTheControlIsPainted(t *testing.T) {
	var canvas gfx.Canvas
	save := gfx.Item(7)
	saved := 0
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(save, gfx.Rect{Width: 100, Height: 100})
			if canvas.Clicked(save) {
				saved++
			}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	if saved != 1 {
		t.Fatalf("the click was read %d times, want once", saved)
	}
	painter.repaint()
	if saved != 1 {
		t.Fatalf("an ordinary repaint read a click: %d", saved)
	}
}

// A function deferred with AfterPaint runs after the click paint, once, and the
// repaint that follows sees its effect.
func TestAfterPaintRunsAfterTheClickPaint(t *testing.T) {
	var canvas gfx.Canvas
	save := gfx.Item(7)
	runs, shown := 0, 0
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			shown = runs
			canvas.Hit(save, gfx.Rect{Width: 100, Height: 100})
			if _, ok := canvas.ClickTarget(); ok {
				canvas.AfterPaint(func() { runs++ })
			}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	if runs != 1 || shown != 1 {
		t.Fatalf("runs=%d, repaint saw %d, want 1 and 1", runs, shown)
	}
}

// onClick makes config's paint report every click the way an application does:
// by asking Clicked about the controls it declared, in the event pass. Call it
// after config.Paint is set.
func TestPointerEventPassDeliversPressesReleasesAndDrags(t *testing.T) {
	var canvas gfx.Canvas
	var downs, doubles, drags int
	paint := func(canvas *gfx.Canvas, _ gfx.Size) {
		bounds := gfx.Rect{X: 10, Y: 10, Width: 200, Height: 100}
		canvas.HitWithLabel(gfx.Item(1), bounds, "Area")
		if event, ok := canvas.PointerOn(gfx.Item(1)); ok {
			switch {
			case event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary:
				downs++
				canvas.Consume()
			case event.Kind == gfx.PointerDown && event.ClickCount == 2:
				doubles++
			case event.Kind == gfx.PointerMove && event.Captured:
				drags++
			}
		}
	}
	config := Config{Paint: paint}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 400, 200
	painter.repaint()

	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonSecondary, X: 60, Y: 60})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonSecondary, X: 60, Y: 60})
	if downs != 1 {
		t.Fatalf("right press reached the control %d times, want 1", downs)
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, ClickCount: 2, X: 60, Y: 60})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, ClickCount: 2, X: 60, Y: 60})
	if doubles != 1 {
		t.Fatalf("double click reached the control %d times, want 1", doubles)
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 60, Y: 60})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 90, Y: 70})
	if drags != 1 {
		t.Fatalf("drag reached the control %d times, want 1", drags)
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 90, Y: 70})
}

func TestABackdropThatPassesAPrimaryPressReachesTheControlBeneath(t *testing.T) {
	var canvas gfx.Canvas
	open := true
	var clicks []gfx.ID
	var downTargets []gfx.ID
	paint := func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.Hit(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 100, Height: 40})
		if open {
			canvas.HitWithOptions(gfx.Item(2), gfx.Rect{X: 0, Y: 0, Width: 200, Height: 100}, "Dismiss",
				gfx.HitOptions{Backdrop: true, PassPrimary: true})
		}
	}
	config := Config{
		Paint: paint,
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			if event.Kind == gfx.PointerDown {
				downTargets = append(downTargets, event.Target)
			}
			return gfx.EventResult{}
		},
	}
	onClick(&config, func(id gfx.ID) {
		clicks = append(clicks, id)
		if id == gfx.Item(2) {
			open = false
		}
	})
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 60, Y: 20})
	if len(downTargets) != 2 || downTargets[0] != gfx.Item(2) || downTargets[1] != gfx.Item(1) {
		t.Fatalf("press targets = %v, want the backdrop then the control beneath", downTargets)
	}
	if len(clicks) != 1 || clicks[0] != gfx.Item(2) {
		t.Fatalf("clicks = %v, want the dismissible backdrop", clicks)
	}
	if open {
		t.Fatal("the backdrop did not close")
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 60, Y: 20})
	if painter.focus.Focused() != gfx.Item(1) {
		t.Fatalf("focus after the passed press = %v, want the control beneath", painter.focus.Focused())
	}
}

func TestPendingFocusIsVisibleToThePaintThatRevealsIt(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		// A virtualized page only lays the target out once the paint knows it
		// should have focus.
		if canvas.Focused() == gfx.Item(7) {
			canvas.HitWithOptions(gfx.Item(7), gfx.Rect{X: 10, Y: 10, Width: 100, Height: 40}, "Target", gfx.HitOptions{Role: gfx.RoleButton})
		}
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.applyEventResult(gfx.EventResult{Focus: gfx.Item(7), FocusVisible: true})
	if painter.focus.PendingFocus() != gfx.Item(7) {
		t.Fatalf("pending focus = %v", painter.focus.PendingFocus())
	}
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(7) || !painter.focus.KeyboardFocus() || canvas.Focused() != gfx.Item(7) {
		t.Fatalf("resolved focus = %v/%v canvas=%v", painter.focus.Focused(), painter.focus.KeyboardFocus(), canvas.Focused())
	}
}

func TestAPressKeepsThePointerUntilItComesUpOutside(t *testing.T) {
	var canvas gfx.Canvas
	var kinds []gfx.PointerEventKind
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 100, Height: 100})
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			kinds = append(kinds, event.Kind)
			return gfx.EventResult{}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 20, Y: 20})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerExit, X: 20, Y: 20})
	if painter.pointer.Pressed() != gfx.Item(1) {
		t.Fatalf("exit ended the press: %v", painter.pointer.Pressed())
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 300, Y: 20})
	if painter.pointer.Pressed() != (gfx.ID{}) || !painter.pointer.Hovered().IsZero() {
		t.Fatalf("release outside left press=%v hover=%v", painter.pointer.Pressed(), painter.pointer.Hovered())
	}
	if len(kinds) == 0 || kinds[len(kinds)-1] != gfx.PointerExit {
		t.Fatalf("event kinds = %v, want a final exit after the release", kinds)
	}
}

func TestACapturedDragPastThePageEdgeEndsHoverOnRelease(t *testing.T) {
	var canvas gfx.Canvas
	var moveTarget gfx.ID
	var moveCaptured bool
	var sawExit bool
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 100, Height: 100})
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			switch event.Kind {
			case gfx.PointerMove:
				moveTarget, moveCaptured = event.Target, event.Captured
			case gfx.PointerExit:
				sawExit = true
			}
			return gfx.EventResult{}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 20, Y: 20})
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerMove, X: 300, Y: 20})
	if moveTarget != gfx.Item(1) || !moveCaptured {
		t.Fatalf("drag past the edge delivered %v captured=%v", moveTarget, moveCaptured)
	}
	if !painter.pointerOutside {
		t.Fatal("a captured move past the edge did not mark the pointer outside")
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 300, Y: 20})
	if !sawExit || !painter.pointer.Hovered().IsZero() || !painter.pointer.Pressed().IsZero() {
		t.Fatalf("release past the edge: exit=%v hover=%v press=%v", sawExit, painter.pointer.Hovered(), painter.pointer.Pressed())
	}
}

func TestPointerCancelEndsThePressWithoutARelease(t *testing.T) {
	var canvas gfx.Canvas
	clicks := 0
	var sawExit bool
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 100, Height: 100})
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			if event.Kind == gfx.PointerExit {
				sawExit = true
			}
			return gfx.EventResult{}
		},
	}
	onClick(&config, func(gfx.ID) { clicks++ })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: 20, Y: 20})
	painter.pointerCancel(gfx.PointerEvent{Kind: gfx.PointerExit, X: 20, Y: 20})
	if !painter.pointer.Pressed().IsZero() || !painter.pointer.Hovered().IsZero() {
		t.Fatalf("cancel left press=%v hover=%v", painter.pointer.Pressed(), painter.pointer.Hovered())
	}
	if !sawExit {
		t.Fatal("cancel did not deliver the exit")
	}
	painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, X: 20, Y: 20})
	if clicks != 0 {
		t.Fatalf("a release after cancel produced %d clicks", clicks)
	}
}

func onClick(config *Config, sink func(id gfx.ID)) {
	paint := config.Paint
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		paint(canvas, size)
		for _, hit := range canvas.Hits() {
			if canvas.Clicked(hit.ID) {
				sink(hit.ID)
			}
		}
	}
}

func TestAModifierPressedAndReleasedAloneIsATap(t *testing.T) {
	var taps []gfx.ModifierTapEvent
	config := Config{Paint: func(*gfx.Canvas, gfx.Size) {}, OnModifierTap: func(event gfx.ModifierTapEvent) gfx.EventResult {
		taps = append(taps, event)
		return gfx.EventResult{}
	}}
	var canvas gfx.Canvas
	painter := newPainter(&canvas, &config)

	painter.modifierDown(gfx.KeyShiftLeft, webKeyCode("ShiftLeft"), gfx.ModifierShift, false)
	painter.modifierUp(webKeyCode("ShiftLeft"))
	if len(taps) != 1 || taps[0].Key != gfx.KeyShiftLeft {
		t.Fatalf("taps = %+v", taps)
	}

	// Anything between the press and the release ends the tap.
	for name, interrupt := range map[string]func(){
		"a key":    func() { painter.keyDown(gfx.KeyEvent{Key: gfx.KeyTab}) },
		"a press":  func() { painter.pointerEvent(gfx.PointerEvent{Kind: gfx.PointerDown}) },
		"a scroll": func() { painter.wheel(gfx.ScrollEvent{DeltaY: 1}) },
		"a chord":  func() { painter.shortcut(gfx.KeyEvent{Text: "k"}) },
		"a second modifier": func() {
			painter.modifierDown(gfx.KeyControlLeft, webKeyCode("ControlLeft"), gfx.ModifierShift|gfx.ModifierControl, false)
		},
	} {
		taps = taps[:0]
		painter.modifierDown(gfx.KeyShiftLeft, webKeyCode("ShiftLeft"), gfx.ModifierShift, false)
		interrupt()
		painter.modifierUp(webKeyCode("ShiftLeft"))
		if len(taps) != 0 {
			t.Errorf("%s: still a tap: %+v", name, taps)
		}
	}
}

func TestPainterUsesTheCallersStaticCanvasWithoutPaint(t *testing.T) {
	var canvas gfx.Canvas
	canvas.Fill(gfx.Rect{Width: 40, Height: 20}, gfx.Color{Alpha: 1})
	painter := newPainter(&canvas, &Config{})
	painter.width, painter.height = 40, 20
	painter.repaint()
	if len(painter.rectangles) != 1 {
		t.Fatalf("static canvas rectangles = %d, want the caller's one", len(painter.rectangles))
	}
}

func TestPainterOrdersRegionsByLayerAndSkipsContainers(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.PushLayer(1)
		canvas.Hit(gfx.Item(1), gfx.Rect{Width: 100, Height: 40})
		canvas.PopLayer()
		canvas.Hit(gfx.Item(2), gfx.Rect{Width: 100, Height: 40})
		canvas.Container(gfx.Rect{Width: 100, Height: 40}, gfx.RoleGroup, "group")
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 40
	painter.repaint()
	if len(painter.focus.Router().Regions()) != 2 || painter.focus.Router().Regions()[0].Target != gfx.Item(2) || painter.focus.Router().Regions()[1].Target != gfx.Item(1) {
		t.Fatalf("regions = %+v, want item 2 then the higher-layer item 1", painter.focus.Router().Regions())
	}
	if target, ok := painter.focus.Router().HitTest(10, 10); !ok || target != gfx.Item(1) {
		t.Fatalf("hit test = %v, %v, want the front-most higher-layer item", target, ok)
	}
}

func TestNewAutofocusTakesFocusOnce(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 100, Height: 40}, "first", gfx.HitOptions{Autofocus: true})
		canvas.Hit(gfx.Item(2), gfx.Rect{X: 100, Width: 100, Height: 40})
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(1) {
		t.Fatalf("autofocus chose %v, want item 1", painter.focus.Focused())
	}
	painter.focus.Router().Focus(input.Target(gfx.Item(2)))
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(2) {
		t.Fatalf("an old autofocus hit stole focus back: %v", painter.focus.Focused())
	}
}

func TestFocusRequestMovesFocusOnce(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.Hit(gfx.Item(1), gfx.Rect{Width: 100, Height: 40})
		canvas.Hit(gfx.Item(2), gfx.Rect{X: 100, Width: 100, Height: 40})
		canvas.RequestFocus(gfx.Item(1))
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(1) {
		t.Fatalf("focus request chose %v, want item 1", painter.focus.Focused())
	}
	painter.focus.Router().Focus(input.Target(gfx.Item(2)))
	painter.repaint()
	if painter.focus.Focused() != gfx.Item(2) {
		t.Fatalf("an unchanged focus request reclaimed focus: %v", painter.focus.Focused())
	}
}

func TestTerminalKeyReleasesPairPressesAndReleases(t *testing.T) {
	var canvas gfx.Canvas
	var seen []gfx.KeyEvent
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 200, Height: 80}, "terminal", gfx.HitOptions{
				Role: gfx.RoleTerminal,
			})
		},
		TextInput: func(TextInputContext) *platform.TextInput {
			return &platform.TextInput{
				KeyReleases: true,
				OnEvent:     func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
			}
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			seen = append(seen, event)
			return gfx.EventResult{}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 80
	painter.repaint()
	painter.focus.Router().Focus(input.Target(gfx.Item(1)))
	painter.repaint()
	code := webKeyCode("KeyA")
	painter.keyDownCode(gfx.KeyEvent{Key: gfx.KeyUnknown, Text: "a", Typed: "A"}, code)
	painter.keyUp(gfx.KeyEvent{}, code)
	if len(seen) != 2 || seen[0].Release || seen[0].Text != "a" || seen[0].Typed != "A" || !seen[1].Release || seen[1].Text != "a" {
		t.Fatalf("press/release = %+v, want a matching pair", seen)
	}
	code = webKeyCode("KeyB")
	painter.keyDownCode(gfx.KeyEvent{Key: gfx.KeyUnknown, Text: "b"}, code)
	painter.focusLost()
	if len(seen) != 4 || !seen[3].Release || seen[3].Text != "b" {
		t.Fatalf("after focus loss = %+v, want a release for b", seen)
	}
	if painter.keys.Len() != 0 {
		t.Fatal("focus loss left a key down")
	}
}

func TestAccessibilityPressRunsThePointerClickPass(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{Width: 100, Height: 40})
			canvas.HitWithOptions(gfx.Item(2), gfx.Rect{X: 100, Width: 100, Height: 40}, "off", gfx.HitOptions{Disabled: true})
		},
	}
	var clicks []gfx.ID
	onClick(&config, func(id gfx.ID) { clicks = append(clicks, id) })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()

	if !painter.accessibilityPress(gfx.Item(1)) {
		t.Fatal("an accessible button press was rejected")
	}
	if len(clicks) != 1 || clicks[0] != gfx.Item(1) {
		t.Fatalf("clicks = %v, want one on item 1", clicks)
	}
	if painter.accessibilityPress(gfx.Item(2)) || len(clicks) != 1 {
		t.Fatalf("a disabled control was pressed: %v", clicks)
	}
	if painter.accessibilityPress(gfx.Item(9)) || len(clicks) != 1 {
		t.Fatalf("a control that is not painted was pressed: %v", clicks)
	}
}

func TestAccessibilityFocusGivesTheKeyboardButNotAnExtraClick(t *testing.T) {
	var canvas gfx.Canvas
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Hit(gfx.Item(1), gfx.Rect{Width: 100, Height: 40})
			canvas.HitWithOptions(gfx.Item(2), gfx.Rect{X: 100, Width: 100, Height: 40}, "off", gfx.HitOptions{Disabled: true})
		},
	}
	var clicks []gfx.ID
	onClick(&config, func(id gfx.ID) { clicks = append(clicks, id) })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()

	if !painter.accessibilityFocus(gfx.Item(1)) {
		t.Fatal("an accessible focus request was rejected")
	}
	if painter.focus.Focused() != gfx.Item(1) || !painter.focus.KeyboardFocus() {
		t.Fatalf("focus = %v, visible = %v, want item 1 with a ring", painter.focus.Focused(), painter.focus.KeyboardFocus())
	}
	if len(clicks) != 0 {
		t.Fatalf("moving focus clicked a control: %v", clicks)
	}
	if painter.accessibilityFocus(gfx.Item(2)) {
		t.Fatal("a disabled control took keyboard focus")
	}
}

func TestAccessibilitySetNumberPressesTheSlidersOwnKeys(t *testing.T) {
	var canvas gfx.Canvas
	var keys []gfx.Key
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 200, Height: 40}, "Volume", gfx.HitOptions{
				Role: gfx.RoleSlider, Value: "5",
				Range: gfx.ValueRange{Now: 5, Min: 0, Max: 10, Step: 1, Set: true},
			})
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			keys = append(keys, event.Key)
			return gfx.EventResult{Handled: true}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()

	if !painter.accessibilitySetNumber(gfx.Item(1), 8) || painter.focus.Focused() != gfx.Item(1) {
		t.Fatalf("set number = false or focus %v, want item 1", painter.focus.Focused())
	}
	if len(keys) != 3 || keys[0] != gfx.KeyUp || keys[1] != gfx.KeyUp || keys[2] != gfx.KeyUp {
		t.Fatalf("keys = %v, want three steps up", keys)
	}
	keys = keys[:0]
	if !painter.accessibilitySetNumber(gfx.Item(1), 2) || len(keys) != 3 || keys[0] != gfx.KeyDown {
		t.Fatalf("keys = %v, want three steps down", keys)
	}
	keys = keys[:0]
	if !painter.accessibilitySetNumber(gfx.Item(1), 0) || len(keys) != 1 || keys[0] != gfx.KeyHome {
		t.Fatalf("keys = %v, want Home", keys)
	}
	keys = keys[:0]
	if !painter.accessibilitySetNumber(gfx.Item(1), 10) || len(keys) != 1 || keys[0] != gfx.KeyEnd {
		t.Fatalf("keys = %v, want End", keys)
	}
}
