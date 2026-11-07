//go:build js && wasm

package web

import (
	"math"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/interaction"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// painter coordinates the immediate canvas vocabulary with the shared input
// router, mirroring the native app painter without any platform coupling.
type painter struct {
	config        *Config
	canvas        *ui.Canvas
	focus         interaction.Focus
	pointer       interaction.Pointer
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
	placing       bool
	placeAnchor   int
	// pointerOutside says the pointer left the page while a button was down.
	// The press keeps the pointer until the button comes up.
	pointerOutside bool
	scrollX        float32
	scrollY        float32
	dirty          bool
	// keys follows keys a terminal asked to see as presses and releases. The
	// zero value tracks nothing, so a page without such a terminal pays nothing.
	keys        interaction.Keys
	cursor      ui.PointerCursor
	cursorImage *ui.CursorImage
	// again asks the runtime to render one more frame after this one: a key
	// release delivered while the focused terminal stopped asking for releases
	// may have changed what the application paints.
	again bool
	// tap recognizes a modifier pressed and released on its own
	// (Config.OnModifierTap).
	tap input.ModifierTap
}

func newPainter(canvas *ui.Canvas, config *Config) *painter {
	clear := ui.Color{Red: 0.067, Green: 0.067, Blue: 0.086, Alpha: 1}
	if config.ClearColor != (ui.Color{}) {
		clear = config.ClearColor
	}
	return &painter{
		config: config,
		canvas: canvas,
		clear:  clear,
	}
}

// resize reflows at a new viewport size.
func (p *painter) resize(width, height int) {
	p.width = float32(width)
	p.height = float32(height)
	p.repaint()
}

// keyWanted reports whether the focused terminal asked for the whole keyboard
// as presses and releases (TextInput.KeyReleases).
func (p *painter) keyWanted() bool {
	return !p.textInputID.IsZero() && p.config.OnKey != nil &&
		p.textInput.Terminal && p.textInput.KeyReleases
}

// keyDown applies one normalized keyboard event and reports whether rendering
// state changed. Application handling runs before built-in traversal,
// activation, and Escape dismissal.
func (p *painter) keyDown(event ui.KeyEvent) bool {
	p.tap.Cancel()
	return p.keyDownCode(event, 0)
}

// keyDownCode is keyDown for a platform event that names its physical key. The
// code lets a terminal that asked for releases pair the press with its release
// even when the modifiers changed in between.
func (p *painter) keyDownCode(event ui.KeyEvent, code uint32) bool {
	beforeFocus, beforeKeyboard := p.focus.Focused(), p.focus.KeyboardFocus()
	event.Target = p.focus.Focused()
	repeat := event.Repeat
	if p.config.OnKey != nil && p.keyWanted() && code != 0 {
		again, _ := p.keys.Press(input.HeldKey{
			Code: code, Key: input.Key(event.Key), Text: event.Text, Target: p.focus.Focused(),
		})
		repeat = repeat || again
	}
	event.Repeat = repeat
	response := ui.EventResult{}
	if p.config.OnKey != nil {
		response = p.config.OnKey(event)
	}
	if event.Key.IsModifier() {
		focusChanged := p.applyEventResult(response)
		if !focusChanged && !response.Repaint {
			return false
		}
		p.repaint()
		return true
	}
	continued := false
	if response.ContinueTraversal {
		// The application closed an overlay in this handler. Show that frame
		// first, then let the same key traverse the page now underneath.
		p.applyEventResult(response)
		p.repaint()
		response = ui.EventResult{}
		continued = true
	}
	if !response.Handled && event.Key == ui.KeyEscape && !p.focus.EscapeTarget().IsZero() && p.canClick() &&
		!p.textInput.State.Composing {
		if p.deliverClick(p.focus.EscapeTarget()) {
			p.repaint()
			return true
		}
	}
	result := input.Result{Target: input.Target(p.focus.Focused())}
	if !response.Handled {
		result = p.focus.Router().KeyDown(input.KeyEvent{Key: event.Key, Modifiers: event.Modifiers, Repeat: event.Repeat})
	}
	activated := false
	if !response.Handled && result.Activated && p.deliverClick(result.Target) {
		activated = true
	}
	p.focus.Refresh()
	if event.Key == ui.KeyTab {
		p.focus.SetKeyboardFocus(!p.focus.Focused().IsZero())
	}
	focusChanged := p.applyEventResult(response)
	if p.focus.Focused() == beforeFocus && p.focus.KeyboardFocus() == beforeKeyboard && !activated && !focusChanged && !response.Repaint && !continued {
		return false
	}
	p.repaint()
	return true
}

// keyUp applies one key going up. Only a key the application saw go down is
// delivered, so an orphan release, such as the end of a key an input method
// consumed, does nothing.
func (p *painter) keyUp(event ui.KeyEvent, code uint32) bool {
	beforePending := p.focus.PendingFocus()
	response := ui.EventResult{}
	delivered := false
	if p.keyWanted() {
		if held, ok := p.keys.Release(code); ok && p.config.OnKey != nil {
			response = p.config.OnKey(ui.KeyEvent{
				Key: held.Key, Modifiers: event.Modifiers, Target: held.Target, Release: true, Text: held.Text,
			})
			delivered = true
		}
	}
	focusChanged := p.applyEventResult(response)
	changed := delivered || focusChanged || response.Repaint || p.focus.PendingFocus() != beforePending
	if changed {
		p.repaint()
	}
	return changed
}

// releaseKeys releases every key still down, last pressed first, the way the
// keyboard would have if it had stayed with them.
func (p *painter) releaseKeys() bool {
	if p.keys.Len() == 0 {
		return false
	}
	var storage [input.MaxHeldKeys]input.HeldKey
	held := p.keys.Take(storage[:0])
	repaint := false
	for index := len(held) - 1; index >= 0 && p.config.OnKey != nil; index-- {
		response := p.config.OnKey(ui.KeyEvent{Key: held[index].Key, Target: held[index].Target, Release: true, Text: held[index].Text})
		repaint = p.applyEventResult(response) || response.Repaint || repaint
	}
	return repaint
}

// focusLost releases the keys that were down when the page stops receiving the
// keyboard: their releases will go to whatever has it now.
func (p *painter) focusLost() bool {
	p.tap.Cancel()
	if !p.releaseKeys() {
		return false
	}
	p.repaint()
	return true
}

// noteKeyboardTarget records, after a paint, whether the held keys still have a
// terminal that wants them; if not the next repaint ends their presses.
func (p *painter) noteKeyboardTarget(target ui.ID, wanted bool) {
	p.keys.NoteTarget(target, wanted)
}

// takeAgain reports and clears a request for one more frame.
func (p *painter) takeAgain() bool {
	again := p.again
	p.again = false
	return again
}

// modifierDown notes a modifier key going down. Any other key ends the
// candidate: keyDown cancels it before this runs for a modifier.
func (p *painter) modifierDown(key input.Key, code uint32, modifiers ui.Modifiers, repeat bool) {
	if p.config.OnModifierTap != nil {
		p.tap.Press(key, code, modifiers, repeat)
	}
}

// modifierUp reports the tap of a modifier that went up while it was still a
// candidate, and whether rendering state changed.
func (p *painter) modifierUp(code uint32) bool {
	if p.config.OnModifierTap == nil {
		return false
	}
	key, ok := p.tap.Release(code)
	if !ok {
		return false
	}
	beforeFocus := p.focus.Focused()
	response := p.config.OnModifierTap(ui.ModifierTapEvent{Key: key, Target: p.focus.Focused()})
	focusChanged := p.applyEventResult(response)
	if focusChanged || response.Repaint || p.focus.Focused() != beforeFocus {
		p.repaint()
		return true
	}
	return false
}

func (p *painter) shortcut(event ui.KeyEvent) (handled, changed bool) {
	p.tap.Cancel()
	if p.config.OnShortcut == nil {
		return false, false
	}
	event.Target = p.focus.Focused()
	response := p.config.OnShortcut(event)
	focusChanged := p.applyEventResult(response)
	if focusChanged || response.Repaint {
		p.repaint()
		changed = true
	}
	return response.Handled, changed
}

// fileDrop applies one step of a drag of files over the page. The point is
// resolved to the hit under it, as a pointer's is, and the answer is whether
// that hit takes files.
func (p *painter) fileDrop(phase ui.FileDropPhase, x, y float32, files []platform.FileRef) (handled, changed bool) {
	if p.config.OnFileDrop == nil {
		return false, false
	}
	target := ui.ID{}
	if phase != ui.FileDropLeave {
		if hit, ok := p.focus.Router().HitTest(x, y); ok {
			target = hit
		}
	}
	event := ui.FileDropEvent{Phase: phase, X: x, Y: y, Target: target}
	if phase == ui.FileDropDrop {
		event.Files = make([]ui.FileDropFile, len(files))
		for index, file := range files {
			event.Files[index] = droppedFile{FileRef: file}
		}
	}
	response := p.config.OnFileDrop(event)
	focusChanged := p.applyEventResult(response)
	if response.Repaint || focusChanged {
		p.repaint()
		changed = true
	}
	return response.Handled, changed
}

// accessibleHit finds the control a request names. Labels share their
// control's ID and are not controls, and a region declared by Container has no
// target.
func (p *painter) accessibleHit(id ui.ID) (ui.Hit, bool) {
	return interaction.Control(p.canvas.Hits(), id)
}

// accessibilityFocus gives a control the keyboard, as a Tab onto it would.
func (p *painter) accessibilityFocus(id ui.ID) bool {
	if p.focus.Focused() == id && p.focus.KeyboardFocus() {
		// A text field reports the DOM focus we just gave it. Answering that
		// with another focus pass would re-enter the mirror for no change.
		return true
	}
	hit, ok := p.accessibleHit(id)
	if !ok || !hit.Focusable || hit.Disabled && !hit.FocusableDisabled {
		return false
	}
	p.applyEventResult(ui.EventResult{Focus: id, FocusVisible: true})
	p.repaint()
	return true
}

// accessibilitySetNumber brings a slider to the value a range control reports.
// It presses the same keys a person would, so the application's slider logic
// stays the only source of truth; the DOM range never writes state directly.
func (p *painter) accessibilitySetNumber(id ui.ID, number float64) bool {
	hit, ok := p.accessibleHit(id)
	if !ok || hit.Disabled || hit.Role != ui.RoleSlider || !hit.Focusable ||
		!hit.Range.Set || math.IsNaN(number) || math.IsInf(number, 0) {
		return false
	}
	if p.focus.Focused() != id {
		p.applyEventResult(ui.EventResult{Focus: id, FocusVisible: true})
	}
	number = min(max(number, hit.Range.Min), hit.Range.Max)
	key, steps := interaction.SliderSteps(hit.Range, number)
	if key == ui.KeyUnknown {
		return false
	}
	if steps == 0 {
		p.repaint()
		return true
	}
	for range steps {
		p.keyDown(ui.KeyEvent{Key: key})
	}
	return true
}

// accessibilityPress carries out the default action a screen reader asks for,
// through the same focus and click paths a pointer uses.
func (p *painter) accessibilityPress(id ui.ID) bool {
	hit, ok := p.accessibleHit(id)
	if !ok || hit.Disabled {
		return false
	}
	if hit.Focusable {
		p.applyEventResult(ui.EventResult{Focus: id, FocusVisible: true})
	}
	if hit.Role == ui.RoleTextField || hit.Role == ui.RoleTerminal {
		// Pressing a field is putting the caret in it.
		p.repaint()
		return true
	}
	if !p.canClick() {
		return false
	}
	p.deliverClick(id)
	p.repaint()
	return true
}

// droppedFile gives the shared drop event the browser's file without exposing
// the platform interface to ui.
type droppedFile struct{ platform.FileRef }

// wheel applies one normalized scroll gesture. It reports whether the
// application used the gesture, and whether rendering state changed. Handled
// events leave the default canvas offset untouched; an unused gesture can be
// forwarded to an embedding page.
func (p *painter) wheel(event ui.ScrollEvent) (handled, changed bool) {
	p.tap.Cancel()
	if event.DeltaX == 0 && event.DeltaY == 0 {
		return false, false
	}
	event.Target = ui.ID{}
	if target, ok := p.focus.Router().HitTest(event.X, event.Y); ok {
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
	return p.focus.Apply(result)
}

// paint reruns the application's paint callback on a cleared canvas with the
// current interaction state.
func (p *painter) paint() {
	if p.config.Paint == nil {
		// RunCanvas accepts a canvas painted once by the caller; there is no
		// callback to run, so the primitives already on it stay as they are.
		return
	}
	p.canvas.Reset()
	focused, visible := p.focus.PaintTarget()
	p.canvas.SetInteraction(p.pointer.Hovered(), p.pointer.Pressed(), focused, visible)
	p.canvas.SetScroll(p.scrollX, p.scrollY)
	p.canvas.BeginFrame()
	p.config.Paint(p.canvas, ui.Size{Width: p.width, Height: p.height})
	p.canvas.EndFrame()
}

// canClick reports whether a click on a control has anywhere to go: whether the
// last paint asked ui.Canvas.Clicked about anything.
func (p *painter) canClick() bool {
	return p.config.Paint != nil && p.canvas.ListensForClicks()
}

// deliverClick carries out a completed click on id: the paint runs once more as
// the event pass of this click, the one pass in which Clicked(id) is true and the
// application may change state, exactly as the native painter does. That pass's
// drawing is discarded: the caller repaints next. It reports whether a paint
// was in a position to consume the click at all.
func (p *painter) deliverClick(id ui.ID) bool {
	if !p.canClick() {
		return false
	}
	p.canvas.BeginEventPass(id)
	p.paint()
	p.canvas.EndEventPass()
	p.canvas.RunAfterPaint()
	return true
}

// deliverPointer runs the paint once more as the event pass of a pointer event
// when the last paint asked ui.Canvas.PointerOn or PointerIn, and reports
// whether a control consumed the event. Only presses, releases and the moves of
// a held press are offered: hovering is read from Hovered, not delivered.
func (p *painter) deliverPointer(event ui.PointerEvent) bool {
	if p.config.Paint == nil || !p.canvas.ListensForPointer() {
		return false
	}
	p.canvas.BeginPointerPass(event)
	p.paint()
	return p.canvas.EndPointerPass()
}

// repaint reruns the paint callback with the current interaction state. A
// modal that opens or closes, or an event that reveals its focus target, moves
// focus while the router syncs, so the frame is painted a second time to show
// the new focus rather than lag behind it.
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

		before := p.focus.Focused()
		p.focus.Sync(p.canvas)
		if p.focus.Focused() == before || pass > 0 {
			break
		}
	}
	p.accessibility = describeAccessibility(p.accessibility[:0], p.canvas.Hits(), p.focus.Router().ActiveScope())
	p.syncTextInput()
	p.syncCursor()
	p.dirty = true
	if p.keys.TakeDue() && p.releaseKeys() {
		p.again = true
	}
}

func (p *painter) syncTextInput() {
	p.textInputID = ui.ID{}
	if p.config.TextInput == nil {
		p.noteKeyboardTarget(ui.ID{}, false)
		return
	}
	field, found := interaction.TextInputHit(p.canvas.Hits(), p.focus.Focused())
	if !found {
		p.noteKeyboardTarget(ui.ID{}, false)
		return
	}
	context := TextInputContext{
		ID: field.ID, Bounds: field.Bounds, Layer: field.Layer, Multiline: field.Multiline,
		ReadOnly: field.ReadOnly, Secure: field.Secure, NavigationKeys: field.NavigationKeys,
		Terminal: field.Role == ui.RoleTerminal, TypeAhead: field.TypeAhead && field.Role != ui.RoleTerminal,
		TextBounds: field.TextBounds, FontSize: field.TextSize, FontWeight: field.TextWeight, TextColor: field.TextColor,
		HideNativeOverlays: field.HideNativeOverlays,
	}
	if field.ReadOnly {
		p.noteKeyboardTarget(field.ID, false)
		return
	}
	input := p.config.TextInput(context)
	if input == nil || input.OnEvent == nil {
		p.noteKeyboardTarget(field.ID, false)
		return
	}
	p.textInputID = field.ID
	p.textInput = *input
	p.textInput.Multiline = field.Multiline
	p.textInput.Placed = field.Placement != nil
	p.textInput.Terminal = context.Terminal || context.TypeAhead
	p.textInput.Secure = field.Secure
	p.textInput.NavigationKeys = input.NavigationKeys || context.NavigationKeys
	p.textInput.HideNativeOverlays = input.HideNativeOverlays || field.HideNativeOverlays
	if p.textInput.Width == 0 && p.textInput.Height == 0 {
		bounds := field.TextBounds
		if bounds == (ui.Rect{}) {
			bounds = field.Bounds
		}
		p.textInput.X, p.textInput.Y = bounds.X, bounds.Y
		p.textInput.Width, p.textInput.Height = bounds.Width, bounds.Height
	}
	if p.textInput.FontSize == 0 {
		p.textInput.FontSize = field.TextSize
	}
	if p.textInput.Weight == 0 {
		p.textInput.Weight = field.TextWeight
	}
	if p.textInput.Color == (platform.Color{}) {
		tint := field.TextColor
		p.textInput.Color = platform.Color{
			Red: float64(tint.Red), Green: float64(tint.Green), Blue: float64(tint.Blue), Alpha: float64(tint.Alpha),
		}
	}
	p.noteKeyboardTarget(field.ID, p.textInput.Terminal && p.textInput.KeyReleases)
}

// syncCursor gives the page the pointer the hit under it declared. A press
// keeps its target's pointer until the button comes up, as on a desktop.
func (p *painter) syncCursor() {
	next, image := interaction.CursorForTarget(p.canvas.Hits(), p.pointer.Hovered(), p.pointer.PressTarget())
	if next == p.cursor && image == p.cursorImage {
		return
	}
	if next == ui.PointerCursorImage {
		if image == nil {
			next = ui.PointerCursorDefault
		} else if err := SetCursorImage(image); err != nil {
			// A picture the browser refuses leaves the ordinary arrow, and is
			// not asked for again until it changes.
			next, image = ui.PointerCursorDefault, nil
			_ = SetCursorShape(uint8(next))
		}
	} else {
		_ = SetCursorShape(uint8(next))
	}
	p.cursor, p.cursorImage = next, image
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
// state changed. A nonzero press target captures move and up delivery until the
// press ends, while hover continues to track the actual hit target.
func (p *painter) pointerEvent(event ui.PointerEvent) bool {
	if event.Kind == ui.PointerDown {
		p.tap.Cancel()
	}
	event.Viewport = ui.Size{Width: p.width, Height: p.height}
	beforeHovered, beforePressed := p.pointer.Hovered(), p.pointer.Pressed()
	beforeFocus, beforeKeyboard := p.focus.Focused(), p.focus.KeyboardFocus()
	inside := event.X >= 0 && event.Y >= 0 && event.X < p.width && event.Y < p.height
	if event.Kind == ui.PointerMove && p.pointerOutside && inside {
		p.pointerOutside = false
	}
	if !inside && event.Kind != ui.PointerExit && !p.pointer.PressTarget().IsZero() {
		// Pointer capture keeps delivering moves past the page edge, but the
		// browser need not send pointerleave while capture is active.
		p.pointerOutside = true
	}
	if event.Kind == ui.PointerExit && !p.pointer.PressTarget().IsZero() {
		// The button is still down, so the press keeps the pointer until it
		// comes up; only hover is gone. The release is delivered outside.
		p.pointerOutside = true
		p.pointer.SetHovered(ui.ID{})
		if beforeHovered.IsZero() {
			return false
		}
		p.repaint()
		return true
	}
	passes := event.Kind == ui.PointerDown &&
		(event.Button == ui.PointerButtonSecondary && p.focus.Router().PassesSecondary(event.X, event.Y) ||
			event.Button == ui.PointerButtonPrimary && p.focus.Router().PassesPrimary(event.X, event.Y))
	result, focusChanged, placed := p.pointerStep(event)
	if passes {
		if !result.Handled && p.deliverClick(p.pointer.Hovered()) {
			result.Handled = true
		}
		p.paint()
		p.focus.Sync(p.canvas)
		if !p.focus.Router().PassesSecondary(event.X, event.Y) && !p.focus.Router().PassesPrimary(event.X, event.Y) {
			var againFocus, againPlaced bool
			result, againFocus, againPlaced = p.pointerStep(event)
			focusChanged, placed = focusChanged || againFocus, placed || againPlaced
		}
		result.Repaint = true
	}
	if p.pointer.Hovered() == beforeHovered && p.pointer.Pressed() == beforePressed &&
		p.focus.Focused() == beforeFocus && p.focus.KeyboardFocus() == beforeKeyboard &&
		!focusChanged && !result.Repaint && !placed {
		return false
	}
	p.repaint()
	return true
}

// pointerCancel ends a press the browser took for itself. Unlike a release, no
// release will follow, so the press ends here and the application is told the
// pointer is gone.
func (p *painter) pointerCancel(event ui.PointerEvent) bool {
	p.pointerOutside = false
	p.pointer.Up()
	event.Kind = ui.PointerExit
	return p.pointerEvent(event)
}

// pointerStep is one pass of a pointer event through hit testing, the press
// state, the application and the controls' event pass.
func (p *painter) pointerStep(event ui.PointerEvent) (result ui.EventResult, focusChanged, placed bool) {
	p.pointer.SetHovered(ui.ID{})
	if event.Kind != ui.PointerExit {
		if target, ok := p.focus.Router().HitTest(event.X, event.Y); ok {
			p.pointer.SetHovered(target)
		}
	}
	if event.Kind == ui.PointerDown {
		p.pointer.Down(event.Button)
		p.focus.PointerDown(event.X, event.Y, p.pointer.Hovered())
	}
	event.Target, event.Captured = p.pointer.Delivery(event.Kind)
	if p.config.OnPointer != nil {
		result = p.config.OnPointer(event)
	}
	if !result.Handled && pointerIsDelivered(event.Kind, event.Target, event.Captured) {
		if p.deliverPointer(event) {
			result.Handled, result.Repaint = true, true
		}
	}
	p.pointer.NoteHandled(event.Kind, result.Handled)
	focusChanged = p.applyEventResult(result)
	placed = p.placeCaret(event)
	switch event.Kind {
	case ui.PointerUp:
		if id, ok := p.pointer.Click(); ok && !result.Handled {
			if p.deliverClick(id) {
				result.Repaint = true
			}
		}
		p.pointer.Up()
		if p.pointerOutside {
			// The button came up beyond the page, so the press is over and the
			// pointer is gone: tell the application now that it may clear hover
			// and hover-owned state.
			p.pointerOutside = false
			if p.config.OnPointer != nil {
				gone := p.config.OnPointer(ui.PointerEvent{Kind: ui.PointerExit, Viewport: event.Viewport})
				result.Repaint = result.Repaint || gone.Repaint
			}
		}
	case ui.PointerExit:
		p.pointer.Exit()
	}
	return result, focusChanged, placed
}

// pointerIsDelivered says which pointer events the event pass offers to
// controls: presses and releases aimed at a control, and the moves of a press
// that is held. A move over a control is hover, which a control reads from
// Canvas.Hovered.
func pointerIsDelivered(kind ui.PointerEventKind, target ui.ID, captured bool) bool {
	switch kind {
	case ui.PointerDown, ui.PointerUp:
		return !target.IsZero()
	case ui.PointerMove:
		return captured
	}
	return false
}

func (p *painter) placeCaret(event ui.PointerEvent) bool {
	switch event.Kind {
	case ui.PointerDown:
		p.placing = false
		if event.Button != ui.PointerButtonPrimary || p.pointer.Hovered() != p.focus.Focused() || p.focus.Focused().IsZero() {
			return false
		}
	case ui.PointerMove:
		if !p.placing {
			return false
		}
	case ui.PointerUp:
		wasPlacing := p.placing
		p.placing = false
		if !wasPlacing {
			return false
		}
	default:
		return false
	}
	for _, hit := range p.canvas.Hits() {
		if hit.ID != p.focus.Focused() || hit.Placement == nil || hit.ReadOnly || hit.Disabled {
			continue
		}
		if p.textInputID != p.focus.Focused() {
			p.syncTextInput()
		}
		if p.textInputID != p.focus.Focused() {
			return false
		}
		offset, ok := hit.Placement.OffsetAt(ui.Point{X: event.X, Y: event.Y})
		if !ok {
			return false
		}
		value := p.textInput.State.Value
		index := utf8.RuneCountInString(value[:max(0, min(offset, len(value)))])
		if event.Kind == ui.PointerDown {
			p.placing, p.placeAnchor = true, index
		}
		next, err := p.textInput.OnEvent(platform.TextEvent{
			Kind: platform.TextSetSelection, Selection: text.Range{Start: p.placeAnchor, End: index},
		})
		if err != nil {
			return false
		}
		changed := next != p.textInput.State
		p.textInput.State = next
		return changed
	}
	return false
}
