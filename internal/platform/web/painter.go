//go:build js && wasm

package web

import (
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// painter coordinates the immediate canvas vocabulary with the shared input
// router, mirroring the native app painter without any platform coupling.
type painter struct {
	config        *Config
	canvas        *ui.Canvas
	router        input.Router
	regions       []input.Region
	width         float32
	height        float32
	clear         ui.Color
	rectangles    []ui.Rectangle
	shadows       []ui.Shadow
	triangles     []ui.Triangle
	texts         []ui.Text
	layers        []uint16
	accessibility []accessibilityDescription
	textInputID   ui.ID
	textInput     TextInput
	hovered       ui.ID
	pressed       ui.ID
	pressTarget   ui.ID
	pressHandled  bool
	focused       ui.ID
	keyboardFocus bool
	escapeTarget  ui.ID
	scrollX       float32
	scrollY       float32
	dirty         bool
}

func newPainter(canvas *ui.Canvas, config *Config) *painter {
	return &painter{
		config: config,
		canvas: canvas,
		clear:  ui.Color{Red: 0.067, Green: 0.067, Blue: 0.086, Alpha: 1},
	}
}

// resize reflows at a new viewport size.
func (p *painter) resize(width, height int) {
	p.width = float32(width)
	p.height = float32(height)
	p.repaint()
}

// keyDown applies one normalized keyboard event and reports whether rendering
// state changed. Application handling runs before built-in traversal,
// activation, and Escape dismissal.
func (p *painter) keyDown(event ui.KeyEvent) bool {
	_, changed := p.keyDownResult(event)
	return changed
}

// keyDownResult distinguishes consuming a key from requesting a frame. A
// handled key can leave the canvas unchanged and must still stop DOM editing.
func (p *painter) keyDownResult(event ui.KeyEvent) (handled, changed bool) {
	beforeFocus, beforeKeyboard := p.focused, p.keyboardFocus
	event.Target = p.focused
	response := ui.EventResult{}
	if p.config.OnKey != nil {
		response = p.config.OnKey(event)
	}
	if !response.Handled && event.Key == ui.KeyEscape && !p.escapeTarget.IsZero() && p.canClick() {
		p.deliverClick(p.escapeTarget)
		p.repaint()
		return true, true
	}
	result := input.Result{Target: input.Target(p.focused)}
	if !response.Handled {
		result = p.router.KeyDown(input.KeyEvent{Key: event.Key, Modifiers: event.Modifiers, Repeat: event.Repeat})
	}
	activated := false
	if !response.Handled && result.Activated && p.canClick() {
		p.deliverClick(result.Target)
		activated = true
	}
	handled = response.Handled || result.FocusChanged || activated || event.Key == ui.KeyTab
	p.focused = result.Target
	if event.Key == ui.KeyTab {
		p.keyboardFocus = !p.focused.IsZero()
	}
	focusChanged := p.applyEventResult(response)
	if p.focused == beforeFocus && p.keyboardFocus == beforeKeyboard && !activated && !focusChanged && !response.Repaint {
		return handled, false
	}
	p.repaint()
	return handled, true
}

func (p *painter) shortcut(event ui.KeyEvent) (handled, changed bool) {
	if p.config.OnShortcut == nil {
		return false, false
	}
	event.Target = p.focused
	response := p.config.OnShortcut(event)
	focusChanged := p.applyEventResult(response)
	if focusChanged || response.Repaint {
		p.repaint()
		changed = true
	}
	return response.Handled, changed
}

// wheel applies one normalized scroll gesture. It reports whether the
// application used the gesture, and whether rendering state changed. Handled
// events leave the default canvas offset untouched; an unused gesture can be
// forwarded to an embedding page.
func (p *painter) wheel(event ui.ScrollEvent) (handled, changed bool) {
	if event.DeltaX == 0 && event.DeltaY == 0 {
		return false, false
	}
	event.Target = ui.ID{}
	if target, ok := p.router.HitTest(event.X, event.Y); ok {
		event.Target = target
	}
	event.Viewport = ui.Size{Width: p.width, Height: p.height}
	response := ui.EventResult{}
	if p.config.OnScroll != nil {
		response = p.config.OnScroll(event)
	}
	focusChanged := p.applyEventResult(response)
	moved := false
	// An embedding page that forwards unhandled wheel gestures owns the default
	// scroll: only an explicit application response uses the gesture.
	if !response.Handled && p.config.ForwardWheel == nil {
		beforeX, beforeY := p.scrollX, p.scrollY
		p.scrollX = max(0, p.scrollX+event.DeltaX)
		p.scrollY = max(0, p.scrollY+event.DeltaY)
		moved = p.scrollX != beforeX || p.scrollY != beforeY
	}
	changed = moved || focusChanged || response.Repaint
	if changed {
		p.repaint()
	}
	return response.Handled || response.Repaint || focusChanged, changed
}

func (p *painter) applyEventResult(result ui.EventResult) bool {
	if result.ClipboardText != "" {
		setClipboardTextFromGesture(result.ClipboardText)
	}
	before, beforeVisible := p.focused, p.keyboardFocus
	if result.Blur {
		p.router.Blur()
	} else if !result.Focus.IsZero() {
		p.router.Focus(input.Target(result.Focus))
	}
	if focused, ok := p.router.Focused(); ok {
		p.focused = focused
	} else {
		p.focused = ui.ID{}
	}
	if p.focused != before {
		p.keyboardFocus = result.FocusVisible && !p.focused.IsZero()
	} else if !result.Focus.IsZero() || result.Blur {
		p.keyboardFocus = result.FocusVisible && !p.focused.IsZero()
	}
	return p.focused != before || p.keyboardFocus != beforeVisible
}

// paint reruns the application's paint callback on a cleared canvas with the
// current interaction state.
func (p *painter) paint() {
	p.canvas.Reset()
	p.canvas.SetInteraction(p.hovered, p.pressed, p.focused, p.keyboardFocus)
	p.canvas.SetScroll(p.scrollX, p.scrollY)
	p.config.Paint(p.canvas, ui.Size{Width: p.width, Height: p.height})
}

// canClick reports whether a click on a control has anywhere to go: whether the
// last paint asked ui.Canvas.Clicked about anything.
func (p *painter) canClick() bool {
	return p.config.Paint != nil && p.canvas.ListensForClicks()
}

// deliverClick carries out a completed click on id: the paint runs once more as
// the event pass of this click, the one pass in which Clicked(id) is true and the
// application may change state, exactly as the native painter does. That pass's
// drawing is discarded: the caller repaints next. Callers check canClick first.
func (p *painter) deliverClick(id ui.ID) {
	p.canvas.BeginEventPass(id)
	p.paint()
	p.canvas.EndEventPass()
	p.canvas.RunAfterPaint()
}

// repaint reruns the paint callback with the current interaction state. A
// modal that opens or closes moves focus while the router syncs, so the frame
// is painted a second time to show the new focus rather than lag behind it.
func (p *painter) repaint() {
	for pass := 0; ; pass++ {
		p.paint()
		p.rectangles = p.canvas.Rectangles()
		p.shadows = p.canvas.Shadows()
		p.triangles = p.canvas.Triangles()
		p.texts = p.canvas.Texts()
		p.layers = collectLayers(p.layers[:0], p.rectangles, p.triangles, p.texts)
		for _, shadow := range p.shadows {
			p.layers = insertLayer(p.layers, shadow.Layer)
		}

		p.regions = p.regions[:0]
		for _, hit := range p.canvas.Hits() {
			p.regions = append(p.regions, input.Region{
				Target:      input.Target(hit.ID),
				Bounds:      input.Rect{X: hit.Bounds.X, Y: hit.Bounds.Y, Width: hit.Bounds.Width, Height: hit.Bounds.Height},
				Scope:       hit.Scope,
				Focusable:   hit.Focusable,
				Disabled:    hit.Disabled,
				Blocks:      hit.Block,
				MovesWindow: hit.MovesWindow,
			})
		}
		p.router.SetRegions(p.regions)
		before := p.focused
		p.router.SetActiveScope(p.canvas.TrappedFocusScope(), input.Target(p.canvas.InitialFocus()))
		if focused, ok := p.router.Focused(); ok {
			p.focused = focused
		} else {
			p.focused = ui.ID{}
		}
		if p.focused == before || pass > 0 {
			break
		}
	}
	p.escapeTarget = p.canvas.EscapeTarget()
	p.accessibility = describeAccessibility(p.accessibility[:0], p.canvas.Hits(), p.router.ActiveScope())
	p.syncTextInput()
	p.dirty = true
}

func (p *painter) syncTextInput() {
	p.textInputID = ui.ID{}
	if p.config.TextInput == nil {
		return
	}
	for _, hit := range p.canvas.Hits() {
		if hit.ID != p.focused || hit.Role != ui.RoleTextField || hit.Disabled || hit.ReadOnly {
			continue
		}
		input := p.config.TextInput(TextInputContext{
			ID: hit.ID, Bounds: hit.Bounds, Multiline: hit.Multiline,
			NavigationKeys: hit.NavigationKeys, Placed: hit.Placement != nil,
			TextBounds: hit.TextBounds, FontSize: hit.TextSize, TextColor: hit.TextColor,
		})
		if input == nil || input.OnEvent == nil {
			return
		}
		p.textInputID = hit.ID
		p.textInput = *input
		p.textInput.Multiline = hit.Multiline
		p.textInput.NavigationKeys = p.textInput.NavigationKeys || hit.NavigationKeys
		p.textInput.Placed = p.textInput.Placed || hit.Placement != nil
		if p.textInput.Width == 0 && p.textInput.Height == 0 {
			bounds := hit.TextBounds
			if bounds == (ui.Rect{}) {
				bounds = hit.Bounds
			}
			p.textInput.X, p.textInput.Y = bounds.X, bounds.Y
			p.textInput.Width, p.textInput.Height = bounds.Width, bounds.Height
		}
		if p.textInput.FontSize == 0 {
			p.textInput.FontSize = hit.TextSize
		}
		if p.textInput.Color == (platform.Color{}) {
			tint := hit.TextColor
			p.textInput.Color = platform.Color{
				Red: float64(tint.Red), Green: float64(tint.Green), Blue: float64(tint.Blue), Alpha: float64(tint.Alpha),
			}
		}
		return
	}
}

func (p *painter) activeTextInput() (ui.ID, *TextInput) {
	if p.textInputID.IsZero() {
		return ui.ID{}, nil
	}
	return p.textInputID, &p.textInput
}

func collectLayers(destination []uint16, rectangles []ui.Rectangle, triangles []ui.Triangle, texts []ui.Text) []uint16 {
	for _, rectangle := range rectangles {
		destination = insertLayer(destination, rectangle.Layer)
	}
	for _, triangle := range triangles {
		destination = insertLayer(destination, triangle.Layer)
	}
	for _, text := range texts {
		destination = insertLayer(destination, text.Layer)
	}
	return destination
}

func insertLayer(layers []uint16, layer uint16) []uint16 {
	index := 0
	for index < len(layers) && layers[index] < layer {
		index++
	}
	if index < len(layers) && layers[index] == layer {
		return layers
	}
	layers = append(layers, 0)
	copy(layers[index+1:], layers[index:])
	layers[index] = layer
	return layers
}

// pointer applies one normalized pointer event and reports whether rendering
// state changed. A nonzero press target captures move and up delivery until
// the press ends, while hover continues to track the actual hit target.
func (p *painter) pointer(event ui.PointerEvent) bool {
	event.Viewport = ui.Size{Width: p.width, Height: p.height}
	beforeHovered, beforePressed := p.hovered, p.pressed
	beforeFocus, beforeKeyboard := p.focused, p.keyboardFocus
	p.hovered = ui.ID{}
	if event.Kind != ui.PointerExit {
		if target, ok := p.router.HitTest(event.X, event.Y); ok {
			p.hovered = target
		}
	}

	if event.Kind == ui.PointerDown {
		p.pressed = p.hovered
		p.pressTarget = p.hovered
		p.router.PointerDown(event.X, event.Y)
		if p.hovered.IsZero() && p.router.ActiveScope().IsZero() {
			p.router.Blur()
		}
		if focused, ok := p.router.Focused(); ok {
			p.focused = focused
		} else {
			p.focused = ui.ID{}
		}
		p.keyboardFocus = false
	}

	event.Target = p.hovered
	event.Captured = false
	if event.Kind != ui.PointerDown && !p.pressTarget.IsZero() {
		event.Target = p.pressTarget
		event.Captured = true
	}
	result := ui.EventResult{}
	if p.config.OnPointer != nil {
		result = p.config.OnPointer(event)
	}
	if event.Kind == ui.PointerDown {
		p.pressHandled = result.Handled
	} else if !p.pressTarget.IsZero() && result.Handled {
		p.pressHandled = true
	}
	focusChanged := p.applyEventResult(result)

	switch event.Kind {
	case ui.PointerUp:
		if event.Button == ui.PointerButtonPrimary && !p.pressHandled && !result.Handled &&
			!p.pressed.IsZero() && p.pressed == p.hovered && p.pressed == p.pressTarget && p.canClick() {
			p.deliverClick(p.pressed)
			result.Repaint = true
		}
		p.pressed = ui.ID{}
		p.pressTarget = ui.ID{}
		p.pressHandled = false
	case ui.PointerExit:
		p.pressed = ui.ID{}
		p.pressTarget = ui.ID{}
		p.pressHandled = false
	}

	if p.hovered == beforeHovered && p.pressed == beforePressed && p.focused == beforeFocus && p.keyboardFocus == beforeKeyboard && !focusChanged && !result.Repaint {
		return false
	}
	p.repaint()
	return true
}
