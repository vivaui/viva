//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type rowPlacement struct{}

func (rowPlacement) OffsetAt(point ui.Point) (int, bool) {
	if point.X < 10 {
		return 1, true
	}
	return 4, true
}

func (rowPlacement) CaretBounds() (ui.Rect, bool) { return ui.Rect{}, false }

func TestPlacedEditorPointerSelectsCanvasOffsets(t *testing.T) {
	var canvas ui.Canvas
	editor := text.NewEditor("a中b")
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 100, Height: 40}, "source", ui.HitOptions{
				Role: ui.RoleTextField, Value: editor.String(), Multiline: true,
				HideNativeOverlays: true, Placement: rowPlacement{},
			})
		},
		TextInput: EditorInputs(func(id ui.ID) (*text.Editor, bool) { return editor, id == ui.Item(1) }),
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 40
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 5, Y: 10})
	if editor.Selection() != (text.Range{Start: 1, End: 1}) || !painter.textInput.Placed || !painter.textInput.HideNativeOverlays {
		t.Fatalf("click must place the canvas caret, got selection %+v, input %+v", editor.Selection(), painter.textInput)
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, X: 30, Y: 10})
	if editor.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("drag must extend by rune, got %+v", editor.Selection())
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 30, Y: 10})
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
	if !placedRowKey(TextInput{Placed: true}, input.KeyLeft, ui.ModifierMeta) || placedRowKey(TextInput{Placed: true}, input.KeyLeft, 0) {
		t.Fatal("only whole-line left navigation belongs to the placed editor")
	}
}

func TestPointerFocusMatchesNativeWithoutRing(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Paint: func(canvas *ui.Canvas, size ui.Size) {
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 90}).Default("Save")
		components.BeginButton(canvas, ui.LightTheme(), ui.Item(2), ui.Rect{X: 110, Y: 10, Width: 90}).Secondary("Cancel")
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 240, 100
	painter.repaint()
	wantCount := len(canvas.Rectangles())
	wantBounds := canvas.Rectangles()[0].Bounds
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	if len(canvas.Rectangles()) != wantCount || canvas.Rectangles()[0].Bounds != wantBounds || !canvas.FocusVisible().IsZero() {
		t.Fatal("mouse press must not change button geometry or flash a focus ring")
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	if painter.focus.Focused() != ui.Item(1) || canvas.Focused() != ui.Item(1) || !canvas.FocusVisible().IsZero() {
		t.Fatal("mouse focus should persist without a keyboard ring")
	}
	painter.keyDown(ui.KeyEvent{Key: ui.KeyTab})
	if painter.focus.Focused() != ui.Item(2) || canvas.FocusVisible() != ui.Item(2) || len(canvas.Rectangles()) <= wantCount {
		t.Fatal("Tab should advance from the clicked control and show its ring")
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 120, Y: 20})
	if !canvas.FocusVisible().IsZero() || len(canvas.Rectangles()) != wantCount {
		t.Fatal("pointer input should hide the keyboard ring")
	}
}

func TestPointerEventsCaptureTargetAndHandledSuppressesClick(t *testing.T) {
	var canvas ui.Canvas
	var events [3]ui.PointerEvent
	eventCount := 0
	clicks := 0
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(7), ui.Rect{Width: 100, Height: 100})
		},
		OnPointer: func(event ui.PointerEvent) ui.EventResult {
			events[eventCount] = event
			eventCount++
			return ui.EventResult{Handled: event.Kind == ui.PointerDown}
		},
	}
	onClick(&config, func(ui.ID) { clicks++ })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	if !painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Modifiers: ui.ModifierShift, X: 10, Y: 10}) {
		t.Fatal("pointer down should change pressed and focus state")
	}
	if !painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, Modifiers: ui.ModifierControl, X: 150, Y: 10}) {
		t.Fatal("moving outside should change hover state")
	}
	if !painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10}) {
		t.Fatal("pointer up should clear pressed state")
	}

	if eventCount != len(events) {
		t.Fatalf("event count = %d, want %d", eventCount, len(events))
	}
	if events[0].Target != ui.Item(7) || events[0].Captured || events[0].Modifiers != ui.ModifierShift || events[0].Viewport != (ui.Size{Width: 200, Height: 100}) {
		t.Fatalf("down event = %+v", events[0])
	}
	if events[1].Target != ui.Item(7) || !events[1].Captured || events[1].Modifiers != ui.ModifierControl {
		t.Fatalf("captured move event = %+v", events[1])
	}
	if events[2].Target != ui.Item(7) || !events[2].Captured {
		t.Fatalf("captured up event = %+v", events[2])
	}
	if clicks != 0 {
		t.Fatalf("handled press produced %d clicks", clicks)
	}
}

func TestPointerMoveWithoutStateChangeDoesNotRepaint(t *testing.T) {
	var canvas ui.Canvas
	paintCount := 0
	eventCount := 0
	response := ui.EventResult{}
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			paintCount++
			canvas.Hit(ui.Item(1), ui.Rect{Width: 100, Height: 100})
		},
		OnPointer: func(ui.PointerEvent) ui.EventResult {
			eventCount++
			return response
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 100
	painter.repaint()
	if !painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, X: 10, Y: 10}) {
		t.Fatal("first move should update hover")
	}
	before := paintCount
	if painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, X: 20, Y: 20}) {
		t.Fatal("move within the same target should not request a frame")
	}
	if paintCount != before || eventCount != 2 {
		t.Fatalf("paint count = %d, event count = %d; want %d and 2", paintCount, eventCount, before)
	}
	response.Repaint = true
	if !painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, X: 30, Y: 30}) {
		t.Fatal("explicit Repaint should request a frame")
	}
	if paintCount != before+1 || eventCount != 3 {
		t.Fatalf("explicit repaint counts = %d/%d, want %d/3", paintCount, eventCount, before+1)
	}
}

func TestClickRemainsPrimaryOnly(t *testing.T) {
	var canvas ui.Canvas
	var clicked ui.ID
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(5), ui.Rect{Width: 100, Height: 100})
		},
	}
	onClick(&config, func(id ui.ID) { clicked = id })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	if clicked != ui.Item(5) {
		t.Fatalf("primary click = %d, want 5", clicked)
	}
	clicked = ui.ID{}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, X: 10, Y: 10})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonSecondary, X: 10, Y: 10})
	if !clicked.IsZero() {
		t.Fatalf("secondary button produced a click %d", clicked)
	}
}

func TestKeyEventResultSuppressesDefaultsAndControlsFocus(t *testing.T) {
	var canvas ui.Canvas
	var received ui.KeyEvent
	response := ui.EventResult{Handled: true}
	clicks := 0
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(1), ui.Rect{Width: 50, Height: 50})
			canvas.Hit(ui.Item(2), ui.Rect{X: 60, Width: 50, Height: 50})
			canvas.SetEscapeTarget(ui.Item(9))
		},
		OnKey: func(event ui.KeyEvent) ui.EventResult {
			received = event
			return response
		},
	}
	onClick(&config, func(ui.ID) { clicks++ })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 120, 50
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	clicks = 0

	if painter.keyDown(ui.KeyEvent{Key: ui.KeyEscape, Modifiers: ui.ModifierAlt | ui.ModifierMeta, Repeat: true}) {
		t.Fatal("handled Escape without state changes should not request a frame")
	}
	if received.Target != ui.Item(1) || received.Modifiers != ui.ModifierAlt|ui.ModifierMeta || !received.Repeat {
		t.Fatalf("received key event = %+v", received)
	}
	if clicks != 0 {
		t.Fatalf("handled Escape produced %d clicks", clicks)
	}

	response = ui.EventResult{Handled: true, Focus: ui.Item(2), FocusVisible: true}
	if !painter.keyDown(ui.KeyEvent{Key: ui.KeyEnter}) {
		t.Fatal("focus result should request a frame")
	}
	if painter.focus.Focused() != ui.Item(2) || !painter.focus.KeyboardFocus() || canvas.Focused() != ui.Item(2) {
		t.Fatalf("focus state = %d/%v canvas=%d", painter.focus.Focused(), painter.focus.KeyboardFocus(), canvas.Focused())
	}
	if clicks != 0 {
		t.Fatalf("handled Enter produced %d clicks", clicks)
	}

	response = ui.EventResult{Handled: true, Blur: true}
	if !painter.keyDown(ui.KeyEvent{Key: ui.KeyLeft}) {
		t.Fatal("blur result should request a frame")
	}
	if !painter.focus.Focused().IsZero() || painter.focus.KeyboardFocus() || !canvas.Focused().IsZero() {
		t.Fatalf("blur left focus state = %d/%v canvas=%d", painter.focus.Focused(), painter.focus.KeyboardFocus(), canvas.Focused())
	}
}

func TestScrollEventSupportsAxesTargetAndHandledResult(t *testing.T) {
	var canvas ui.Canvas
	var received ui.ScrollEvent
	response := ui.EventResult{Handled: true}
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(3), ui.Rect{Width: 100, Height: 100})
		},
		OnScroll: func(event ui.ScrollEvent) ui.EventResult {
			received = event
			return response
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 100
	painter.repaint()
	if handled, changed := painter.wheel(ui.ScrollEvent{Modifiers: ui.ModifierShift, X: 10, Y: 10, DeltaX: 4, DeltaY: 7, Precise: true}); !handled || changed {
		t.Fatalf("handled scroll = handled %v, changed %v; want true, false", handled, changed)
	}
	if received != (ui.ScrollEvent{
		Modifiers: ui.ModifierShift, X: 10, Y: 10, DeltaX: 4, DeltaY: 7,
		Viewport: ui.Size{Width: 100, Height: 100}, Target: ui.Item(3), Precise: true,
	}) {
		t.Fatalf("received scroll = %+v", received)
	}
	if painter.scrollX != 0 || painter.scrollY != 0 {
		t.Fatalf("handled scroll changed offset to %v,%v", painter.scrollX, painter.scrollY)
	}

	response = ui.EventResult{}
	if handled, changed := painter.wheel(ui.ScrollEvent{DeltaX: 4, DeltaY: 7}); handled || !changed {
		t.Fatalf("default scroll = handled %v, changed %v; want false, true", handled, changed)
	}
	if canvas.ScrollX() != 4 || canvas.ScrollY() != 7 {
		t.Fatalf("canvas scroll = %v,%v, want 4,7", canvas.ScrollX(), canvas.ScrollY())
	}
	if handled, changed := painter.wheel(ui.ScrollEvent{}); handled || changed {
		t.Fatalf("zero scroll = handled %v, changed %v; want false, false", handled, changed)
	}

	config.ForwardWheel = func(float32, float32) {}
	response = ui.EventResult{}
	beforeX, beforeY := painter.scrollX, painter.scrollY
	if handled, changed := painter.wheel(ui.ScrollEvent{DeltaY: 9}); handled || changed {
		t.Fatalf("forwarded scroll = handled %v, changed %v; want false, false", handled, changed)
	}
	if painter.scrollX != beforeX || painter.scrollY != beforeY {
		t.Fatalf("forwarded scroll moved the canvas offset to %v,%v", painter.scrollX, painter.scrollY)
	}
}

func TestWebInputMappings(t *testing.T) {
	if webPointerButton(0) != ui.PointerButtonPrimary || webPointerButton(1) != ui.PointerButtonMiddle ||
		webPointerButton(2) != ui.PointerButtonSecondary || webPointerButton(3) != ui.PointerButtonBack ||
		webPointerButton(4) != ui.PointerButtonForward || webPointerButton(5) != ui.PointerButtonNone {
		t.Fatal("unexpected pointer button mapping")
	}
	modifiers := webModifiers(true, true, true, true)
	wantModifiers := ui.ModifierShift | ui.ModifierControl | ui.ModifierAlt | ui.ModifierMeta
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
	rectangles := []ui.Rectangle{
		{Layer: 7},
		{Layer: 3},
		{Layer: 7},
	}
	triangles := []ui.Triangle{
		{Layer: 9},
		{Layer: 3},
	}
	texts := []ui.Text{
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
	var canvas ui.Canvas
	readOnly := false
	providerCalls := 0
	var context TextInputContext
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(7), ui.Rect{X: 10, Y: 20, Width: 180, Height: 64}, "Message", ui.HitOptions{
				Role: ui.RoleTextField, Multiline: true, ReadOnly: readOnly,
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
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 30})
	id, input := painter.activeTextInput()
	if id != ui.Item(7) || input == nil || !input.Multiline || providerCalls != 1 {
		t.Fatalf("active editor = id %d input %+v calls=%d", id, input, providerCalls)
	}
	if context.ID != ui.Item(7) || context.Bounds != (ui.Rect{X: 10, Y: 20, Width: 180, Height: 64}) || !context.Multiline {
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
	var canvas ui.Canvas
	open := false
	var focusSeenByPaint ui.ID
	config := Config{
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			canvas.HitWithLabel(ui.Item(int(opener)), ui.Rect{Width: 60, Height: 24}, "Open")
			if open {
				canvas.PushFocusScope(ui.Item(9))
				canvas.HitWithOptions(ui.Item(int(field)), ui.Rect{X: 80, Width: 80, Height: 24}, "Field", ui.HitOptions{Role: ui.RoleTextField})
				canvas.PopFocusScope()
				canvas.TrapFocus(ui.Item(9))
				canvas.SetInitialFocus(ui.Item(int(field)))
				canvas.SetEscapeTarget(ui.Item(int(dismiss)))
			}
			focusSeenByPaint = canvas.Focused()
		},
	}
	onClick(&config, func(id ui.ID) { open = id == ui.Item(int(opener)) })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	if !open || painter.focus.Focused() != ui.Item(int(field)) || canvas.Focused() != ui.Item(int(field)) || focusSeenByPaint != ui.Item(int(field)) {
		t.Fatalf("after open: open=%v painter=%d canvas=%d paint saw=%d, want the dialog field %d",
			open, painter.focus.Focused(), canvas.Focused(), focusSeenByPaint, field)
	}

	// The open scope is announced again on every paint and must not steal
	// focus back from a user who has since moved it.
	painter.repaint()
	if painter.focus.Focused() != ui.Item(int(field)) {
		t.Fatalf("steady repaint moved focus to %d", painter.focus.Focused())
	}

	open = false
	painter.repaint()
	if painter.focus.Focused() != ui.Item(int(opener)) || canvas.Focused() != ui.Item(int(opener)) || focusSeenByPaint != ui.Item(int(opener)) {
		t.Fatalf("after close: painter=%d canvas=%d paint saw=%d, want the opener %d",
			painter.focus.Focused(), canvas.Focused(), focusSeenByPaint, opener)
	}
}

func TestPainterFillsUnsetTextInputGeometryFromTheDeclaredHit(t *testing.T) {
	var canvas ui.Canvas
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(7), ui.Rect{X: 10, Y: 20, Width: 180, Height: 32}, "Name", ui.HitOptions{
				Role: ui.RoleTextField, TextBounds: ui.Rect{X: 22, Y: 20, Width: 156, Height: 32},
				TextSize: 12, TextColor: ui.Color{Red: 0.5, Green: 0.25, Blue: 0.125, Alpha: 1},
			})
		},
		TextInput: func(TextInputContext) *TextInput {
			return &TextInput{OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil }}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 240, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 30, Y: 30})
	_, input := painter.activeTextInput()
	if input == nil {
		t.Fatal("no active editor")
	}
	if input.X != 22 || input.Y != 20 || input.Width != 156 || input.Height != 32 || input.FontSize != 12 ||
		input.Color.Red != 0.5 || input.Color.Green != 0.25 || input.Color.Blue != 0.125 || input.Color.Alpha != 1 {
		t.Fatalf("browser input = %+v", input)
	}
}

// A control that reads its click while it paints (ui.Canvas.Clicked) receives it
// in the event pass the painter runs, as it does natively.
func TestClickIsReadWhereTheControlIsPainted(t *testing.T) {
	var canvas ui.Canvas
	save := ui.Item(7)
	saved := 0
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(save, ui.Rect{Width: 100, Height: 100})
			if canvas.Clicked(save) {
				saved++
			}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()

	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
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
	var canvas ui.Canvas
	save := ui.Item(7)
	runs, shown := 0, 0
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			shown = runs
			canvas.Hit(save, ui.Rect{Width: 100, Height: 100})
			if _, ok := canvas.ClickTarget(); ok {
				canvas.AfterPaint(func() { runs++ })
			}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	if runs != 1 || shown != 1 {
		t.Fatalf("runs=%d, repaint saw %d, want 1 and 1", runs, shown)
	}
}

// onClick makes config's paint report every click the way an application does:
// by asking Clicked about the controls it declared, in the event pass. Call it
// after config.Paint is set.
func TestPointerEventPassDeliversPressesReleasesAndDrags(t *testing.T) {
	var canvas ui.Canvas
	var downs, doubles, drags int
	paint := func(canvas *ui.Canvas, _ ui.Size) {
		bounds := ui.Rect{X: 10, Y: 10, Width: 200, Height: 100}
		canvas.HitWithLabel(ui.Item(1), bounds, "Area")
		if event, ok := canvas.PointerOn(ui.Item(1)); ok {
			switch {
			case event.Kind == ui.PointerDown && event.Button == ui.PointerButtonSecondary:
				downs++
				canvas.Consume()
			case event.Kind == ui.PointerDown && event.ClickCount == 2:
				doubles++
			case event.Kind == ui.PointerMove && event.Captured:
				drags++
			}
		}
	}
	config := Config{Paint: paint}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 400, 200
	painter.repaint()

	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, X: 60, Y: 60})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonSecondary, X: 60, Y: 60})
	if downs != 1 {
		t.Fatalf("right press reached the control %d times, want 1", downs)
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, ClickCount: 2, X: 60, Y: 60})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, ClickCount: 2, X: 60, Y: 60})
	if doubles != 1 {
		t.Fatalf("double click reached the control %d times, want 1", doubles)
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 60, Y: 60})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, X: 90, Y: 70})
	if drags != 1 {
		t.Fatalf("drag reached the control %d times, want 1", drags)
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 90, Y: 70})
}

func TestABackdropThatPassesAPrimaryPressReachesTheControlBeneath(t *testing.T) {
	var canvas ui.Canvas
	open := true
	var clicks []ui.ID
	var downTargets []ui.ID
	paint := func(canvas *ui.Canvas, _ ui.Size) {
		canvas.Hit(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 100, Height: 40})
		if open {
			canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 0, Y: 0, Width: 200, Height: 100}, "Dismiss",
				ui.HitOptions{Backdrop: true, PassPrimary: true})
		}
	}
	config := Config{
		Paint: paint,
		OnPointer: func(event ui.PointerEvent) ui.EventResult {
			if event.Kind == ui.PointerDown {
				downTargets = append(downTargets, event.Target)
			}
			return ui.EventResult{}
		},
	}
	onClick(&config, func(id ui.ID) {
		clicks = append(clicks, id)
		if id == ui.Item(2) {
			open = false
		}
	})
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 60, Y: 20})
	if len(downTargets) != 2 || downTargets[0] != ui.Item(2) || downTargets[1] != ui.Item(1) {
		t.Fatalf("press targets = %v, want the backdrop then the control beneath", downTargets)
	}
	if len(clicks) != 1 || clicks[0] != ui.Item(2) {
		t.Fatalf("clicks = %v, want the dismissible backdrop", clicks)
	}
	if open {
		t.Fatal("the backdrop did not close")
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 60, Y: 20})
	if painter.focus.Focused() != ui.Item(1) {
		t.Fatalf("focus after the passed press = %v, want the control beneath", painter.focus.Focused())
	}
}

func TestPendingFocusIsVisibleToThePaintThatRevealsIt(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Paint: func(canvas *ui.Canvas, _ ui.Size) {
		// A virtualized page only lays the target out once the paint knows it
		// should have focus.
		if canvas.Focused() == ui.Item(7) {
			canvas.HitWithOptions(ui.Item(7), ui.Rect{X: 10, Y: 10, Width: 100, Height: 40}, "Target", ui.HitOptions{Role: ui.RoleButton})
		}
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.applyEventResult(ui.EventResult{Focus: ui.Item(7), FocusVisible: true})
	if painter.focus.PendingFocus() != ui.Item(7) {
		t.Fatalf("pending focus = %v", painter.focus.PendingFocus())
	}
	painter.repaint()
	if painter.focus.Focused() != ui.Item(7) || !painter.focus.KeyboardFocus() || canvas.Focused() != ui.Item(7) {
		t.Fatalf("resolved focus = %v/%v canvas=%v", painter.focus.Focused(), painter.focus.KeyboardFocus(), canvas.Focused())
	}
}

func TestAPressKeepsThePointerUntilItComesUpOutside(t *testing.T) {
	var canvas ui.Canvas
	var kinds []ui.PointerEventKind
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 100, Height: 100})
		},
		OnPointer: func(event ui.PointerEvent) ui.EventResult {
			kinds = append(kinds, event.Kind)
			return ui.EventResult{}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerExit, X: 20, Y: 20})
	if painter.pointer.Pressed() != ui.Item(1) {
		t.Fatalf("exit ended the press: %v", painter.pointer.Pressed())
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 300, Y: 20})
	if painter.pointer.Pressed() != (ui.ID{}) || !painter.pointer.Hovered().IsZero() {
		t.Fatalf("release outside left press=%v hover=%v", painter.pointer.Pressed(), painter.pointer.Hovered())
	}
	if len(kinds) == 0 || kinds[len(kinds)-1] != ui.PointerExit {
		t.Fatalf("event kinds = %v, want a final exit after the release", kinds)
	}
}

func TestACapturedDragPastThePageEdgeEndsHoverOnRelease(t *testing.T) {
	var canvas ui.Canvas
	var moveTarget ui.ID
	var moveCaptured bool
	var sawExit bool
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 100, Height: 100})
		},
		OnPointer: func(event ui.PointerEvent) ui.EventResult {
			switch event.Kind {
			case ui.PointerMove:
				moveTarget, moveCaptured = event.Target, event.Captured
			case ui.PointerExit:
				sawExit = true
			}
			return ui.EventResult{}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerMove, X: 300, Y: 20})
	if moveTarget != ui.Item(1) || !moveCaptured {
		t.Fatalf("drag past the edge delivered %v captured=%v", moveTarget, moveCaptured)
	}
	if !painter.pointerOutside {
		t.Fatal("a captured move past the edge did not mark the pointer outside")
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 300, Y: 20})
	if !sawExit || !painter.pointer.Hovered().IsZero() || !painter.pointer.Pressed().IsZero() {
		t.Fatalf("release past the edge: exit=%v hover=%v press=%v", sawExit, painter.pointer.Hovered(), painter.pointer.Pressed())
	}
}

func TestPointerCancelEndsThePressWithoutARelease(t *testing.T) {
	var canvas ui.Canvas
	clicks := 0
	var sawExit bool
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 100, Height: 100})
		},
		OnPointer: func(event ui.PointerEvent) ui.EventResult {
			if event.Kind == ui.PointerExit {
				sawExit = true
			}
			return ui.EventResult{}
		},
	}
	onClick(&config, func(ui.ID) { clicks++ })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 100
	painter.repaint()
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	painter.pointerCancel(ui.PointerEvent{Kind: ui.PointerExit, X: 20, Y: 20})
	if !painter.pointer.Pressed().IsZero() || !painter.pointer.Hovered().IsZero() {
		t.Fatalf("cancel left press=%v hover=%v", painter.pointer.Pressed(), painter.pointer.Hovered())
	}
	if !sawExit {
		t.Fatal("cancel did not deliver the exit")
	}
	painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	if clicks != 0 {
		t.Fatalf("a release after cancel produced %d clicks", clicks)
	}
}

func onClick(config *Config, sink func(id ui.ID)) {
	paint := config.Paint
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		paint(canvas, size)
		for _, hit := range canvas.Hits() {
			if canvas.Clicked(hit.ID) {
				sink(hit.ID)
			}
		}
	}
}

func TestAModifierPressedAndReleasedAloneIsATap(t *testing.T) {
	var taps []ui.ModifierTapEvent
	config := Config{Paint: func(*ui.Canvas, ui.Size) {}, OnModifierTap: func(event ui.ModifierTapEvent) ui.EventResult {
		taps = append(taps, event)
		return ui.EventResult{}
	}}
	var canvas ui.Canvas
	painter := newPainter(&canvas, &config)

	painter.modifierDown(ui.KeyShiftLeft, webKeyCode("ShiftLeft"), ui.ModifierShift, false)
	painter.modifierUp(webKeyCode("ShiftLeft"))
	if len(taps) != 1 || taps[0].Key != ui.KeyShiftLeft {
		t.Fatalf("taps = %+v", taps)
	}

	// Anything between the press and the release ends the tap.
	for name, interrupt := range map[string]func(){
		"a key":    func() { painter.keyDown(ui.KeyEvent{Key: ui.KeyTab}) },
		"a press":  func() { painter.pointerEvent(ui.PointerEvent{Kind: ui.PointerDown}) },
		"a scroll": func() { painter.wheel(ui.ScrollEvent{DeltaY: 1}) },
		"a chord":  func() { painter.shortcut(ui.KeyEvent{Text: "k"}) },
		"a second modifier": func() {
			painter.modifierDown(ui.KeyControlLeft, webKeyCode("ControlLeft"), ui.ModifierShift|ui.ModifierControl, false)
		},
	} {
		taps = taps[:0]
		painter.modifierDown(ui.KeyShiftLeft, webKeyCode("ShiftLeft"), ui.ModifierShift, false)
		interrupt()
		painter.modifierUp(webKeyCode("ShiftLeft"))
		if len(taps) != 0 {
			t.Errorf("%s: still a tap: %+v", name, taps)
		}
	}
}

func TestPainterUsesTheCallersStaticCanvasWithoutPaint(t *testing.T) {
	var canvas ui.Canvas
	canvas.Fill(ui.Rect{Width: 40, Height: 20}, ui.Color{Alpha: 1})
	painter := newPainter(&canvas, &Config{})
	painter.width, painter.height = 40, 20
	painter.repaint()
	if len(painter.rectangles) != 1 {
		t.Fatalf("static canvas rectangles = %d, want the caller's one", len(painter.rectangles))
	}
}

func TestPainterOrdersRegionsByLayerAndSkipsContainers(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Paint: func(canvas *ui.Canvas, _ ui.Size) {
		canvas.PushLayer(1)
		canvas.Hit(ui.Item(1), ui.Rect{Width: 100, Height: 40})
		canvas.PopLayer()
		canvas.Hit(ui.Item(2), ui.Rect{Width: 100, Height: 40})
		canvas.Container(ui.Rect{Width: 100, Height: 40}, ui.RoleGroup, "group")
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 100, 40
	painter.repaint()
	if len(painter.focus.Router().Regions()) != 2 || painter.focus.Router().Regions()[0].Target != ui.Item(2) || painter.focus.Router().Regions()[1].Target != ui.Item(1) {
		t.Fatalf("regions = %+v, want item 2 then the higher-layer item 1", painter.focus.Router().Regions())
	}
	if target, ok := painter.focus.Router().HitTest(10, 10); !ok || target != ui.Item(1) {
		t.Fatalf("hit test = %v, %v, want the front-most higher-layer item", target, ok)
	}
}

func TestNewAutofocusTakesFocusOnce(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Paint: func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 100, Height: 40}, "first", ui.HitOptions{Autofocus: true})
		canvas.Hit(ui.Item(2), ui.Rect{X: 100, Width: 100, Height: 40})
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()
	if painter.focus.Focused() != ui.Item(1) {
		t.Fatalf("autofocus chose %v, want item 1", painter.focus.Focused())
	}
	painter.focus.Router().Focus(input.Target(ui.Item(2)))
	painter.repaint()
	if painter.focus.Focused() != ui.Item(2) {
		t.Fatalf("an old autofocus hit stole focus back: %v", painter.focus.Focused())
	}
}

func TestFocusRequestMovesFocusOnce(t *testing.T) {
	var canvas ui.Canvas
	config := Config{Paint: func(canvas *ui.Canvas, _ ui.Size) {
		canvas.Hit(ui.Item(1), ui.Rect{Width: 100, Height: 40})
		canvas.Hit(ui.Item(2), ui.Rect{X: 100, Width: 100, Height: 40})
		canvas.RequestFocus(ui.Item(1))
	}}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()
	if painter.focus.Focused() != ui.Item(1) {
		t.Fatalf("focus request chose %v, want item 1", painter.focus.Focused())
	}
	painter.focus.Router().Focus(input.Target(ui.Item(2)))
	painter.repaint()
	if painter.focus.Focused() != ui.Item(2) {
		t.Fatalf("an unchanged focus request reclaimed focus: %v", painter.focus.Focused())
	}
}

func TestTerminalKeyReleasesPairPressesAndReleases(t *testing.T) {
	var canvas ui.Canvas
	var seen []ui.KeyEvent
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 200, Height: 80}, "terminal", ui.HitOptions{
				Role: ui.RoleTerminal,
			})
		},
		TextInput: func(TextInputContext) *platform.TextInput {
			return &platform.TextInput{
				KeyReleases: true,
				OnEvent:     func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
			}
		},
		OnKey: func(event ui.KeyEvent) ui.EventResult {
			seen = append(seen, event)
			return ui.EventResult{}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 80
	painter.repaint()
	painter.focus.Router().Focus(input.Target(ui.Item(1)))
	painter.repaint()
	code := webKeyCode("KeyA")
	painter.keyDownCode(ui.KeyEvent{Key: ui.KeyUnknown, Text: "a", Typed: "A"}, code)
	painter.keyUp(ui.KeyEvent{}, code)
	if len(seen) != 2 || seen[0].Release || seen[0].Text != "a" || seen[0].Typed != "A" || !seen[1].Release || seen[1].Text != "a" {
		t.Fatalf("press/release = %+v, want a matching pair", seen)
	}
	code = webKeyCode("KeyB")
	painter.keyDownCode(ui.KeyEvent{Key: ui.KeyUnknown, Text: "b"}, code)
	painter.focusLost()
	if len(seen) != 4 || !seen[3].Release || seen[3].Text != "b" {
		t.Fatalf("after focus loss = %+v, want a release for b", seen)
	}
	if painter.keys.Len() != 0 {
		t.Fatal("focus loss left a key down")
	}
}

func TestAccessibilityPressRunsThePointerClickPass(t *testing.T) {
	var canvas ui.Canvas
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(1), ui.Rect{Width: 100, Height: 40})
			canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 100, Width: 100, Height: 40}, "off", ui.HitOptions{Disabled: true})
		},
	}
	var clicks []ui.ID
	onClick(&config, func(id ui.ID) { clicks = append(clicks, id) })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()

	if !painter.accessibilityPress(ui.Item(1)) {
		t.Fatal("an accessible button press was rejected")
	}
	if len(clicks) != 1 || clicks[0] != ui.Item(1) {
		t.Fatalf("clicks = %v, want one on item 1", clicks)
	}
	if painter.accessibilityPress(ui.Item(2)) || len(clicks) != 1 {
		t.Fatalf("a disabled control was pressed: %v", clicks)
	}
	if painter.accessibilityPress(ui.Item(9)) || len(clicks) != 1 {
		t.Fatalf("a control that is not painted was pressed: %v", clicks)
	}
}

func TestAccessibilityFocusGivesTheKeyboardButNotAnExtraClick(t *testing.T) {
	var canvas ui.Canvas
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.Hit(ui.Item(1), ui.Rect{Width: 100, Height: 40})
			canvas.HitWithOptions(ui.Item(2), ui.Rect{X: 100, Width: 100, Height: 40}, "off", ui.HitOptions{Disabled: true})
		},
	}
	var clicks []ui.ID
	onClick(&config, func(id ui.ID) { clicks = append(clicks, id) })
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()

	if !painter.accessibilityFocus(ui.Item(1)) {
		t.Fatal("an accessible focus request was rejected")
	}
	if painter.focus.Focused() != ui.Item(1) || !painter.focus.KeyboardFocus() {
		t.Fatalf("focus = %v, visible = %v, want item 1 with a ring", painter.focus.Focused(), painter.focus.KeyboardFocus())
	}
	if len(clicks) != 0 {
		t.Fatalf("moving focus clicked a control: %v", clicks)
	}
	if painter.accessibilityFocus(ui.Item(2)) {
		t.Fatal("a disabled control took keyboard focus")
	}
}

func TestAccessibilitySetNumberPressesTheSlidersOwnKeys(t *testing.T) {
	var canvas ui.Canvas
	var keys []ui.Key
	config := Config{
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{Width: 200, Height: 40}, "Volume", ui.HitOptions{
				Role: ui.RoleSlider, Value: "5",
				Range: ui.ValueRange{Now: 5, Min: 0, Max: 10, Step: 1, Set: true},
			})
		},
		OnKey: func(event ui.KeyEvent) ui.EventResult {
			keys = append(keys, event.Key)
			return ui.EventResult{Handled: true}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 200, 40
	painter.repaint()

	if !painter.accessibilitySetNumber(ui.Item(1), 8) || painter.focus.Focused() != ui.Item(1) {
		t.Fatalf("set number = false or focus %v, want item 1", painter.focus.Focused())
	}
	if len(keys) != 3 || keys[0] != ui.KeyUp || keys[1] != ui.KeyUp || keys[2] != ui.KeyUp {
		t.Fatalf("keys = %v, want three steps up", keys)
	}
	keys = keys[:0]
	if !painter.accessibilitySetNumber(ui.Item(1), 2) || len(keys) != 3 || keys[0] != ui.KeyDown {
		t.Fatalf("keys = %v, want three steps down", keys)
	}
	keys = keys[:0]
	if !painter.accessibilitySetNumber(ui.Item(1), 0) || len(keys) != 1 || keys[0] != ui.KeyHome {
		t.Fatalf("keys = %v, want Home", keys)
	}
	keys = keys[:0]
	if !painter.accessibilitySetNumber(ui.Item(1), 10) || len(keys) != 1 || keys[0] != ui.KeyEnd {
		t.Fatalf("keys = %v, want End", keys)
	}
}
