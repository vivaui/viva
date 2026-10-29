//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

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
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	if len(canvas.Rectangles()) != wantCount || canvas.Rectangles()[0].Bounds != wantBounds || !canvas.FocusVisible().IsZero() {
		t.Fatal("mouse press must not change button geometry or flash a focus ring")
	}
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 20, Y: 20})
	if painter.focused != ui.Item(1) || canvas.Focused() != ui.Item(1) || !canvas.FocusVisible().IsZero() {
		t.Fatal("mouse focus should persist without a keyboard ring")
	}
	painter.keyDown(ui.KeyEvent{Key: ui.KeyTab})
	if painter.focused != ui.Item(2) || canvas.FocusVisible() != ui.Item(2) || len(canvas.Rectangles()) != wantCount+1 {
		t.Fatal("Tab should advance from the clicked control and show its ring")
	}
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 120, Y: 20})
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

	if !painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Modifiers: ui.ModifierShift, X: 10, Y: 10}) {
		t.Fatal("pointer down should change pressed and focus state")
	}
	if !painter.pointer(ui.PointerEvent{Kind: ui.PointerMove, Modifiers: ui.ModifierControl, X: 150, Y: 10}) {
		t.Fatal("moving outside should change hover state")
	}
	if !painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10}) {
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
	if !painter.pointer(ui.PointerEvent{Kind: ui.PointerMove, X: 10, Y: 10}) {
		t.Fatal("first move should update hover")
	}
	before := paintCount
	if painter.pointer(ui.PointerEvent{Kind: ui.PointerMove, X: 20, Y: 20}) {
		t.Fatal("move within the same target should not request a frame")
	}
	if paintCount != before || eventCount != 2 {
		t.Fatalf("paint count = %d, event count = %d; want %d and 2", paintCount, eventCount, before)
	}
	response.Repaint = true
	if !painter.pointer(ui.PointerEvent{Kind: ui.PointerMove, X: 30, Y: 30}) {
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
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	if clicked != ui.Item(5) {
		t.Fatalf("primary click = %d, want 5", clicked)
	}
	clicked = ui.ID{}
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, X: 10, Y: 10})
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonSecondary, X: 10, Y: 10})
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
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
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
	if painter.focused != ui.Item(2) || !painter.keyboardFocus || canvas.Focused() != ui.Item(2) {
		t.Fatalf("focus state = %d/%v canvas=%d", painter.focused, painter.keyboardFocus, canvas.Focused())
	}
	if clicks != 0 {
		t.Fatalf("handled Enter produced %d clicks", clicks)
	}

	response = ui.EventResult{Handled: true, Blur: true}
	if !painter.keyDown(ui.KeyEvent{Key: ui.KeyLeft}) {
		t.Fatal("blur result should request a frame")
	}
	if !painter.focused.IsZero() || painter.keyboardFocus || !canvas.Focused().IsZero() {
		t.Fatalf("blur left focus state = %d/%v canvas=%d", painter.focused, painter.keyboardFocus, canvas.Focused())
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
		webPointerButton(2) != ui.PointerButtonSecondary || webPointerButton(3) != ui.PointerButtonNone {
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
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 20, Y: 30})
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

	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	if !open || painter.focused != ui.Item(int(field)) || canvas.Focused() != ui.Item(int(field)) || focusSeenByPaint != ui.Item(int(field)) {
		t.Fatalf("after open: open=%v painter=%d canvas=%d paint saw=%d, want the dialog field %d",
			open, painter.focused, canvas.Focused(), focusSeenByPaint, field)
	}

	// The open scope is announced again on every paint and must not steal
	// focus back from a user who has since moved it.
	painter.repaint()
	if painter.focused != ui.Item(int(field)) {
		t.Fatalf("steady repaint moved focus to %d", painter.focused)
	}

	open = false
	painter.repaint()
	if painter.focused != ui.Item(int(opener)) || canvas.Focused() != ui.Item(int(opener)) || focusSeenByPaint != ui.Item(int(opener)) {
		t.Fatalf("after close: painter=%d canvas=%d paint saw=%d, want the opener %d",
			painter.focused, canvas.Focused(), focusSeenByPaint, opener)
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
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 30, Y: 30})
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

	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
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
	painter.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	painter.pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	if runs != 1 || shown != 1 {
		t.Fatalf("runs=%d, repaint saw %d, want 1 and 1", runs, shown)
	}
}

// onClick makes config's paint report every click the way an application does:
// by asking Clicked about the controls it declared, in the event pass. Call it
// after config.Paint is set.
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

func TestTextNavigationReportsHandlingAndRefreshesEditorState(t *testing.T) {
	var canvas ui.Canvas
	editor := text.NewEditor("first\nsecond")
	response := ui.EventResult{Handled: true}
	var received ui.KeyEvent
	layout := &components.TextareaLayout{}
	config := Config{
		Paint: func(c *ui.Canvas, _ ui.Size) {
			c.HitWithOptions(ui.Item(7), ui.Rect{Width: 180, Height: 64}, "Message", ui.HitOptions{Role: ui.RoleTextField, Multiline: true, NavigationKeys: true, Placement: layout})
		},
		TextInput: func(context TextInputContext) *TextInput {
			if !context.NavigationKeys || !context.Placed {
				t.Fatalf("missing navigation context: %+v", context)
			}
			return &TextInput{State: platform.EditorState(editor), OnEvent: func(e TextEvent) (TextState, error) { e.Apply(editor); return platform.EditorState(editor), nil }}
		},
		OnKey: func(e ui.KeyEvent) ui.EventResult { received = e; editor.MoveTo(6, false); return response },
	}
	p := newPainter(&canvas, &config)
	p.width, p.height = 200, 100
	p.repaint()
	p.pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	_, active := p.activeTextInput()
	if !active.NavigationKeys || !active.Placed {
		t.Fatalf("missing navigation flags: %+v", active)
	}
	if handled, changed := p.keyDownResult(ui.KeyEvent{Key: ui.KeyHome, Repeat: true}); !handled || changed {
		t.Fatalf("handled=%v changed=%v", handled, changed)
	}
	if received.Target != ui.Item(7) || !received.Repeat {
		t.Fatalf("key=%+v", received)
	}
	p.syncTextInput()
	_, active = p.activeTextInput()
	if active.State.Selection.Start != 6 {
		t.Fatalf("stale caret: %+v", active.State)
	}
	response = ui.EventResult{Repaint: true}
	if handled, changed := p.keyDownResult(ui.KeyEvent{Key: ui.KeyEnd}); handled || !changed {
		t.Fatalf("repaint incorrectly consumed default: %v,%v", handled, changed)
	}
}

func TestMultilineReturnUsesApplicationTextCommands(t *testing.T) {
	editor := text.NewEditor("question")
	submits := 0
	input := TextInput{Multiline: true, OnEvent: func(event TextEvent) (TextState, error) {
		if event.Kind == platform.TextCommandEvent && event.Command == platform.TextInsertNewline {
			submits++
		} else {
			event.Apply(editor)
		}
		return platform.EditorState(editor), nil
	}}
	for _, mods := range []ui.Modifiers{0, ui.ModifierShift} {
		command, application := webTextKey(input, ui.KeyEnter, mods, false)
		if application {
			t.Fatal("multiline return went to OnKey")
		}
		input.OnEvent(TextEvent{Kind: platform.TextCommandEvent, Command: command})
	}
	if submits != 1 || editor.String() != "question\n" {
		t.Fatalf("submits=%d text=%q", submits, editor.String())
	}
}
