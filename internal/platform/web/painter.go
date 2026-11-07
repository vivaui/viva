//go:build js && wasm

package web

import (
	"cmp"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
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
	placing       bool
	placeAnchor   int
	hovered       ui.ID
	pressed       ui.ID
	pressTarget   ui.ID
	pressHandled  bool
	focused       ui.ID
	keyboardFocus bool
	escapeTarget  ui.ID
	// paintedIDs is the hit list of the previous paint, so a newly appearing
	// Autofocus hit takes the keyboard once (see applyAutofocus).
	paintedIDs     []ui.ID
	paintedScratch []ui.ID
	focusClaim     ui.ID
	pendingFocus   ui.ID
	pendingVisible bool
	scrollX        float32
	scrollY        float32
	dirty          bool
	// keys follows keys a terminal asked to see as presses and releases. The
	// zero value tracks nothing, so a page without such a terminal pays nothing.
	keys        input.KeyTracker
	keysTarget  ui.ID
	releaseDue  bool
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
	beforeFocus, beforeKeyboard := p.focused, p.keyboardFocus
	event.Target = p.focused
	repeat := event.Repeat
	if p.config.OnKey != nil && p.keyWanted() && code != 0 {
		again, tracked := p.keys.Press(input.HeldKey{
			Code: code, Key: input.Key(event.Key), Text: event.Text, Target: p.focused,
		})
		if tracked && !again && p.keys.Len() == 1 {
			p.keysTarget = p.focused
		}
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
	if !response.Handled && event.Key == ui.KeyEscape && !p.escapeTarget.IsZero() && p.canClick() {
		p.deliverClick(p.escapeTarget)
		p.repaint()
		return true
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
	p.focused = result.Target
	if event.Key == ui.KeyTab {
		p.keyboardFocus = !p.focused.IsZero()
	}
	focusChanged := p.applyEventResult(response)
	if p.focused == beforeFocus && p.keyboardFocus == beforeKeyboard && !activated && !focusChanged && !response.Repaint && !continued {
		return false
	}
	p.repaint()
	return true
}

// keyUp applies one key going up. Only a key the application saw go down is
// delivered, so an orphan release, such as the end of a key an input method
// consumed, does nothing.
func (p *painter) keyUp(event ui.KeyEvent, code uint32) bool {
	beforePending := p.pendingFocus
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
	changed := delivered || focusChanged || response.Repaint || p.pendingFocus != beforePending
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
	if p.keys.Len() > 0 && (!wanted || target != p.keysTarget) {
		p.releaseDue = true
	}
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
	beforeFocus := p.focused
	response := p.config.OnModifierTap(ui.ModifierTapEvent{Key: key, Target: p.focused})
	focusChanged := p.applyEventResult(response)
	if focusChanged || response.Repaint || p.focused != beforeFocus {
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
	event.Target = p.focused
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
		if hit, ok := p.router.HitTest(x, y); ok {
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
	if id.IsZero() {
		return ui.Hit{}, false
	}
	hits := p.canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.ID == id && !hit.IsLabel && !hit.Block && !hit.Backdrop && !hit.Container {
			return hit, true
		}
	}
	return ui.Hit{}, false
}

// accessibilityFocus gives a control the keyboard, as a Tab onto it would.
func (p *painter) accessibilityFocus(id ui.ID) bool {
	if p.focused == id && p.keyboardFocus {
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

const maxAccessibilitySliderKeys = 200

// accessibilitySetNumber brings a slider to the value a range control reports.
// It presses the same keys a person would, so the application's slider logic
// stays the only source of truth; the DOM range never writes state directly.
func (p *painter) accessibilitySetNumber(id ui.ID, number float64) bool {
	hit, ok := p.accessibleHit(id)
	if !ok || hit.Disabled || hit.Role != ui.RoleSlider || !hit.Focusable ||
		!hit.Range.Set || math.IsNaN(number) || math.IsInf(number, 0) {
		return false
	}
	if p.focused != id {
		p.applyEventResult(ui.EventResult{Focus: id, FocusVisible: true})
	}
	value := hit.Range
	number = min(max(number, value.Min), value.Max)
	key, steps := input.KeyUp, 0
	switch {
	case number <= value.Min:
		key, steps = input.KeyHome, 1
	case number >= value.Max:
		key, steps = input.KeyEnd, 1
	default:
		step := value.Step
		if step <= 0 {
			step = 1
		}
		steps = int(math.Round((number - value.Now) / step))
		if steps < 0 {
			key, steps = input.KeyDown, -steps
		}
		steps = min(steps, maxAccessibilitySliderKeys)
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
		p.pendingFocus = ui.ID{}
		p.pendingVisible = false
		p.router.Blur()
	} else if !result.Focus.IsZero() {
		if p.router.Focus(input.Target(result.Focus)) {
			p.pendingFocus = ui.ID{}
			p.pendingVisible = false
		} else if focused, ok := p.router.Focused(); !ok || focused != result.Focus {
			// The event may have revealed this hit. Resolve it against the hit
			// list produced by the repaint instead of making the application
			// stage focus through a second event.
			p.pendingFocus = result.Focus
			p.pendingVisible = result.FocusVisible
		}
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
	return p.focused != before || p.keyboardFocus != beforeVisible || !p.pendingFocus.IsZero()
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
	p.canvas.SetInteraction(p.hovered, p.pressed, p.focused, p.keyboardFocus)
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
// drawing is discarded: the caller repaints next. Callers check canClick first.
func (p *painter) deliverClick(id ui.ID) {
	p.canvas.BeginEventPass(id)
	p.paint()
	p.canvas.EndEventPass()
	p.canvas.RunAfterPaint()
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

		before := p.focused
		p.syncRegions()
		if p.focused == before || pass > 0 {
			break
		}
	}
	p.accessibility = describeAccessibility(p.accessibility[:0], p.canvas.Hits(), p.router.ActiveScope())
	p.syncTextInput()
	p.syncCursor()
	p.dirty = true
	if p.releaseDue {
		p.releaseDue = false
		if p.releaseKeys() {
			p.again = true
		}
	}
}

// syncRegions feeds the canvas hit list into the retained input router, moves
// focus into a modal or revealed target, and updates the cursor.
func (p *painter) syncRegions() {
	p.regions = p.regions[:0]
	hits := p.canvas.Hits()
	for index := range hits {
		hit := &hits[index]
		if hit.Container {
			continue // semantic regions are not interaction targets
		}
		p.regions = append(p.regions, input.Region{
			Target:            input.Target(hit.ID),
			Bounds:            input.Rect{X: hit.Bounds.X, Y: hit.Bounds.Y, Width: hit.Bounds.Width, Height: hit.Bounds.Height},
			Scope:             hit.Scope,
			Focusable:         hit.Focusable,
			Role:              hit.Role,
			SkipTab:           hit.SkipTab,
			Disabled:          hit.Disabled,
			FocusableDisabled: hit.FocusableDisabled,
			Blocks:            hit.Block,
			Label:             hit.IsLabel,
			Backdrop:          hit.Backdrop,
			MovesWindow:       hit.MovesWindow,
			PassSecondary:     hit.PassSecondary, PassPrimary: hit.PassPrimary,
			Layer: hit.Layer,
		})
	}
	// Hits are declared in paint order, which is not the order layers composite
	// in: a popup on a higher layer is often painted before the field it covers.
	// SetRegions expects the front-most region last.
	if !slices.IsSortedFunc(p.regions, func(left, right input.Region) int {
		return cmp.Compare(left.Layer, right.Layer)
	}) {
		slices.SortStableFunc(p.regions, func(left, right input.Region) int {
			return cmp.Compare(left.Layer, right.Layer)
		})
	}
	p.router.SetRegions(p.regions)
	p.router.SetActiveScope(p.canvas.TrappedFocusScope(), input.Target(p.canvas.InitialFocus()))
	p.applyAutofocus()
	if request := p.canvas.FocusRequest(); !request.IsZero() {
		if request != p.focusClaim {
			if focused, ok := p.router.Focused(); ok && focused == input.Target(request) {
				p.focusClaim = request
			} else if p.router.Focus(input.Target(request)) {
				p.focusClaim = request
			}
		}
	} else {
		p.focusClaim = ui.ID{}
	}
	if !p.pendingFocus.IsZero() {
		target := input.Target(p.pendingFocus)
		p.router.Focus(target)
		if focused, ok := p.router.Focused(); ok && focused == target {
			p.keyboardFocus = p.pendingVisible
		}
		p.pendingFocus = ui.ID{}
		p.pendingVisible = false
	}
	p.escapeTarget = p.canvas.EscapeTarget()
	if focused, ok := p.router.Focused(); ok {
		p.focused = focused
	} else {
		p.focused = ui.ID{}
	}
}

// applyAutofocus moves the keyboard to the first hit that asks for it and was
// not painted last time. A trapped focus scope keeps its own say.
func (p *painter) applyAutofocus() {
	hits := p.canvas.Hits()
	next := p.paintedScratch[:0]
	focused := false
	for index := range hits {
		hit := &hits[index]
		if !focused && hit.Autofocus && !hit.ID.IsZero() && !p.wasPainted(hit.ID) {
			if p.router.Focus(input.Target(hit.ID)) {
				focused = true
			}
		}
		if !hit.ID.IsZero() {
			next = append(next, hit.ID)
		}
	}
	p.paintedScratch = next
	p.paintedIDs, p.paintedScratch = next, p.paintedIDs[:0]
}

func (p *painter) wasPainted(id ui.ID) bool {
	for _, previous := range p.paintedIDs {
		if previous == id {
			return true
		}
	}
	return false
}

func (p *painter) syncTextInput() {
	p.textInputID = ui.ID{}
	if p.config.TextInput == nil {
		p.noteKeyboardTarget(ui.ID{}, false)
		return
	}
	var field ui.Hit
	found := false
	for _, hit := range p.canvas.Hits() {
		if hit.ID != p.focused || hit.Disabled {
			continue
		}
		if hit.Role == ui.RoleTextField || hit.Role == ui.RoleTerminal || hit.TypeAhead ||
			hit.Role == ui.RoleCombobox && hit.TextBounds != (ui.Rect{}) {
			field, found = hit, true
			break
		}
	}
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
	next, image := cursorForTarget(p.canvas.Hits(), p.hovered, p.pressTarget)
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

// cursorForTarget is the pointer policy shared by paint and tests: the press
// target wins over hover, and the front-most hit with that ID declares the
// pointer. A label is not a control; it keeps the one its control has.
func cursorForTarget(hits []ui.Hit, hovered, pressTarget ui.ID) (ui.PointerCursor, *ui.CursorImage) {
	target := hovered
	if !pressTarget.IsZero() {
		target = pressTarget
	}
	if target.IsZero() {
		return ui.PointerCursorDefault, nil
	}
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.ID != target || hit.Disabled || hit.IsLabel {
			continue
		}
		if hit.Cursor < ui.PointerCursorDefault || hit.Cursor > ui.PointerCursorImage {
			return ui.PointerCursorDefault, nil
		}
		if hit.Cursor == ui.PointerCursorImage && hit.CursorImage == nil {
			return ui.PointerCursorDefault, nil
		}
		return hit.Cursor, hit.CursorImage
	}
	return ui.PointerCursorDefault, nil
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
	if event.Kind == ui.PointerDown {
		p.tap.Cancel()
	}
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
	placed := p.placeCaret(event)

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

	if p.hovered == beforeHovered && p.pressed == beforePressed && p.focused == beforeFocus && p.keyboardFocus == beforeKeyboard && !focusChanged && !result.Repaint && !placed {
		return false
	}
	p.repaint()
	return true
}

func (p *painter) placeCaret(event ui.PointerEvent) bool {
	switch event.Kind {
	case ui.PointerDown:
		p.placing = false
		if event.Button != ui.PointerButtonPrimary || p.hovered != p.focused || p.focused.IsZero() {
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
		if hit.ID != p.focused || hit.Placement == nil || hit.ReadOnly || hit.Disabled {
			continue
		}
		if p.textInputID != p.focused {
			p.syncTextInput()
		}
		if p.textInputID != p.focused {
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
