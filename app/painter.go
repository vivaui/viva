package app

import (
	"bytes"
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/internal/semantic"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// painter reconciles immediate canvas paints with the retained scene and the
// native label list, so an unchanged repaint performs no renderer work.
type painter struct {
	paint      func(canvas *ui.Canvas, size ui.Size)
	onPointer  func(ui.PointerEvent) ui.EventResult
	onKey      func(ui.KeyEvent) ui.EventResult
	onShortcut func(ui.KeyEvent) ui.EventResult
	onScroll   func(ui.ScrollEvent) ui.EventResult
	onFileDrop func(ui.FileDropEvent) ui.EventResult
	onResize   func(ui.Size)
	// scale is the display's device pixels per logical pixel, zero when the platform
	// does not report it (see platform.Size): the canvas then keeps the scale it was given.
	scale       float32
	textInput   TextInputProvider
	activeText  *TextInput
	onTextEvent platform.TextHandler
	// placing is a press on a text field that lays its rows out itself being held
	// (see placeCaret), from the rune index placeAnchor.
	placing     bool
	placeAnchor int
	nativeText  platform.TextInput
	// keys follows the keys of a terminal that asked for releases (see
	// TextInput.KeyReleases); heldTarget is the hit they were pressed on.
	keys input.KeyTracker
	// releaseDue is set when something happened that ends the keyboard's use by
	// the keys in keys: the next repaint releases them.
	releaseDue bool
	// focusClaim is the non-modal focus request already honored, so a popover
	// painted every frame does not pull focus back after the user leaves it.
	focusClaim      ui.ID
	keysTarget      ui.ID
	textEnabled     bool
	canvas          *ui.Canvas
	size            ui.Size
	router          input.Router
	regions         []input.Region
	graph           *scene.Scene
	renderer        render.Store
	semantics       *semantic.Tree
	semanticIDs     []semantic.ID
	accessibility   []platform.AccessibilityNode
	lastAccess      []platform.AccessibilityNode
	changes         []scene.Change
	semanticChanges []semantic.Change
	resyncs         int        // times the renderer had to be rebuilt after a failed batch
	reusedHits      int        // publishes that kept the previous regions and semantics
	shapeIDs        []scene.ID // one scene node per entry of order, in paint order
	labels          []platform.TextLabel
	paintedIDs      []ui.ID // hit IDs of the previous paint, for Autofocus
	paintedScratch  []ui.ID // the next paint's IDs, so one pass can read the old ones
	lastTexts       []ui.Text
	lastRuns        []ui.TextRun // the runs lastTexts refer to, copied out of the application's memory
	lastImages      []ui.Image
	stateBySlot     []instanceState
	// layerShift is 1 while the frame has a backdrop, which takes platform layer
	// zero; every canvas layer then travels one higher. It is 0 otherwise.
	layerShift uint16
	hitScratch []ui.Hit
	// publishedHits is the hit list the regions and the semantic tree were last
	// built from. Comparing it byte for byte (the strings keep the previous
	// allocations alive, so equal pointers mean equal bytes) lets a repaint that
	// declared the same hits reuse both instead of mirroring the list again.
	publishedHits      []ui.Hit
	publishedFocus     ui.ID
	publishedTrap      ui.ID
	publishedInitial   ui.ID
	publishedEscape    ui.ID
	publishedRequest   ui.ID
	publishedText      string
	publishedSelection TextRange
	publishedCompose   TextRange
	publishedComposing bool
	publishedTextField bool
	hitsPublished      bool
	semanticsDirty     bool
	labelNames         map[ui.ID]string
	labelNamePeak      int
	clipRects          [4]ui.Rect
	clipped            []ui.Rectangle
	clippedPaths       []ui.Triangle
	clippedSoft        []ui.Shadow
	order              []shapeRef     // clipped, clippedPaths and clippedSoft interleaved in paint order
	blocks             []ui.Rectangle // scratch for appendBlockElements
	clippedTexts       []ui.Text
	clippedImages      []ui.Image
	// controls draws the window buttons of a custom titlebar, on platforms where
	// Viva does; it is nil otherwise.
	controls *windowControls
	// edge is the window edge the pointer is over, for the resize cursor, on a
	// window whose frame Viva draws (the system supplies the cursor elsewhere).
	edge           platform.WindowEdge
	hovered        ui.ID
	pressed        ui.ID
	pressTarget    ui.ID
	pressButton    ui.PointerButton
	pressHandled   bool
	cursor         Cursor
	focused        ui.ID
	keyboardFocus  bool
	pendingFocus   ui.ID
	pendingVisible bool
	escapeTarget   ui.ID
	scrollX        float32
	scrollY        float32
	// scheduleFrame asks the runtime for one more frame, after at least the
	// delay, while a transition is in flight or the application asked for one.
	scheduleFrame    func(delay time.Duration)
	setClipboardText func(string)
	setCursor        func(Cursor) error
}

func newPainter(canvas *ui.Canvas, config *Config) (*painter, error) {
	return newPainterWithClock(canvas, config, monotonicClock())
}

// newPainterWithClock is newPainter with an explicit animation clock, so a
// driver can run time on its own terms from the very first paint.
func newPainterWithClock(canvas *ui.Canvas, config *Config, clock func() time.Duration) (*painter, error) {
	canvas.SetClock(clock)
	if config.Tree != nil {
		canvas.SetTree(config.Tree)
	}
	painter := &painter{
		paint:         config.Paint,
		onPointer:     config.OnPointer,
		onKey:         config.OnKey,
		onShortcut:    config.OnShortcut,
		onScroll:      config.OnScroll,
		onFileDrop:    config.OnFileDrop,
		onResize:      config.OnResize,
		textInput:     config.TextInput,
		canvas:        canvas,
		size:          ui.Size{Width: float32(config.Width), Height: float32(config.Height)},
		graph:         scene.NewWithCapacity(len(canvas.Rectangles()) + len(canvas.Triangles()) + len(canvas.Shadows()) + 1),
		semantics:     semantic.NewWithCapacity("Viva", len(canvas.Hits())+1),
		semanticIDs:   make([]semantic.ID, 0, len(canvas.Hits())+1),
		accessibility: make([]platform.AccessibilityNode, 0, len(canvas.Hits())+1),
		lastAccess:    make([]platform.AccessibilityNode, 0, len(canvas.Hits())+1),
		shapeIDs:      make([]scene.ID, 0, len(canvas.Rectangles())+len(canvas.Triangles())+len(canvas.Shadows())),
		labels:        make([]platform.TextLabel, 0, len(canvas.Texts())+len(canvas.Images())+1),
		regions:       make([]input.Region, 0, len(canvas.Hits())+1),
	}
	painter.onTextEvent = painter.applyTextEvent
	if config.WindowMode == WindowCustomTitlebar && controlsDrawnByViva {
		painter.controls = newWindowControls(config)
	}
	if _, err := painter.publish(); err != nil {
		return nil, err
	}
	if config.Paint != nil {
		// Repaint once with the injected measurer so the initial frame and
		// every later repaint agree on text widths; a pre-painted canvas from
		// the caller would otherwise embed estimated widths.
		if _, err := painter.repaint(); err != nil {
			return nil, err
		}
	}
	for index := range painter.labels {
		painter.labels[index].BitmapUpload = len(painter.labels[index].Bitmap) > 0
	}
	painter.saveContent()
	painter.lastAccess = append(painter.lastAccess[:0], painter.accessibility...)
	return painter, nil
}

// resize rebuilds the canvas at a new size and returns exactly one frame.
func (painter *painter) resize(size platform.Size) (platform.Frame, error) {
	next := ui.Size{Width: size.Width, Height: size.Height}
	changed := next != painter.size
	painter.size, painter.scale = next, size.Scale
	if changed && painter.onResize != nil {
		painter.onResize(next)
	}
	return painter.repaint()
}

// pressAt tells a custom-titlebar window what a press at a window point does:
// whether the platform moves the window from there, now or once the pointer
// drags, or the application takes it (see input.Router.PressAt).
func (painter *painter) pressAt(x, y float32) platform.PressKind {
	return painter.router.PressAt(x, y)
}

// pointer applies one native pointer event to the interaction state machine
// and repaints only when hover or pressed state actually changed.
//
// A secondary press on a backdrop that passes it on (ui.HitOptions.PassSecondary)
// is two presses in one: the first dismisses the overlay, as a click on the
// backdrop would, and the second is the same press aimed at what the overlay
// was covering, once the paint has taken it away.
func (painter *painter) pointer(event platform.PointerEvent) (platform.Frame, error) {
	beforeHovered, beforePressed := painter.hovered, painter.pressed
	beforeFocus, beforeKeyboard := painter.focused, painter.keyboardFocus
	passes := event.Kind == platform.PointerDown &&
		(event.Button == ui.PointerButtonSecondary && painter.router.PassesSecondary(event.X, event.Y) ||
			event.Button == ui.PointerButtonPrimary && painter.router.PassesPrimary(event.X, event.Y))
	result, focusChanged, edgeChanged := painter.pointerStep(event)
	if passes {
		if !result.Handled && painter.deliverClick(painter.hovered) {
			result.Handled = true
		}
		painter.paintCanvas()
		painter.syncRegions()
		if !painter.router.PassesSecondary(event.X, event.Y) && !painter.router.PassesPrimary(event.X, event.Y) {
			var again, edgeAgain bool
			result, again, edgeAgain = painter.pointerStep(event)
			focusChanged, edgeChanged = focusChanged || again, edgeChanged || edgeAgain
		}
		result.Repaint = true
	}

	switch event.Kind {
	case platform.PointerUp:
		if painter.pressButton == ui.PointerButtonPrimary && !painter.pressHandled && !result.Handled && !painter.pressed.IsZero() && painter.pressed == painter.hovered &&
			painter.pressed == painter.pressTarget {
			if painter.isWindowControl(painter.pressed) {
				painter.controls.activate(painter.pressed)
				result.Repaint = true
			} else if painter.deliverClick(painter.pressed) {
				result.Repaint = true
			}
		}
		painter.pressed = ui.ID{}
		painter.pressTarget = ui.ID{}
		painter.pressButton = ui.PointerButtonNone
		painter.pressHandled = false
	case platform.PointerExit:
		painter.pressed = ui.ID{}
		painter.pressTarget = ui.ID{}
		painter.pressButton = ui.PointerButtonNone
		painter.pressHandled = false
	}

	if painter.hovered == beforeHovered && painter.pressed == beforePressed && painter.focused == beforeFocus && painter.keyboardFocus == beforeKeyboard && !focusChanged && !result.Repaint {
		if edgeChanged {
			// Only the cursor differs: no repaint for it.
			return platform.Frame{}, painter.syncCursor()
		}
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// pointerStep is one pass of a pointer event through hit testing, the press
// state, the application and the controls' event pass. It returns what the
// application answered and whether focus and the pointer's edge state changed.
func (painter *painter) pointerStep(event platform.PointerEvent) (result ui.EventResult, focusChanged, edgeChanged bool) {
	painter.hovered = ui.ID{}
	if event.Kind != platform.PointerExit {
		if target, ok := painter.router.HitTest(event.X, event.Y); ok {
			painter.hovered = target
		}
	}
	edgeChanged = painter.trackEdge(event)

	switch event.Kind {
	case platform.PointerDown:
		painter.pressed = painter.hovered
		painter.pressTarget = painter.hovered
		painter.pressButton = event.Button
		if painter.pressButton == ui.PointerButtonNone {
			painter.pressButton = ui.PointerButtonPrimary
		}
		painter.router.PointerDown(event.X, event.Y)
		if painter.hovered.IsZero() && painter.router.ActiveScope().IsZero() {
			painter.router.Blur()
		}
		if focused, ok := painter.router.Focused(); ok {
			painter.focused = focused
		} else {
			painter.focused = ui.ID{}
		}
		painter.keyboardFocus = false
	}

	target := painter.hovered
	captured := false
	if event.Kind != platform.PointerDown && !painter.pressTarget.IsZero() {
		target = painter.pressTarget
		captured = true
	}
	if painter.onPointer != nil {
		result = painter.onPointer(ui.PointerEvent{
			Kind: ui.PointerEventKind(event.Kind), Button: event.Button, Modifiers: ui.Modifiers(event.Modifiers),
			X: event.X, Y: event.Y, ScreenX: event.ScreenX, ScreenY: event.ScreenY, Viewport: painter.size, Target: target, Captured: captured,
			ClickCount: event.ClickCount,
		})
	}
	if !result.Handled && pointerIsDelivered(event.Kind, target, captured) {
		if painter.deliverPointer(ui.PointerEvent{
			Kind: ui.PointerEventKind(event.Kind), Button: event.Button, Modifiers: ui.Modifiers(event.Modifiers),
			X: event.X, Y: event.Y, ScreenX: event.ScreenX, ScreenY: event.ScreenY, Viewport: painter.size, Target: target, Captured: captured,
			ClickCount: event.ClickCount,
		}) {
			result.Handled, result.Repaint = true, true
		}
	}
	if event.Kind == platform.PointerDown {
		painter.pressHandled = result.Handled
	} else if !painter.pressTarget.IsZero() && result.Handled {
		painter.pressHandled = true
	}
	focusChanged = painter.applyEventResult(result)
	if painter.placeCaret(event) {
		result.Repaint = true
	}
	return result, focusChanged, edgeChanged
}

// placeCaret puts the caret of the focused text field where a press, or the drag
// of one, lands, when the field lays its rows out itself (ui.TextPlacement): the
// platform editor knows one line only and leaves such a field to this. A press
// puts the caret there and a drag extends the selection from it. It reports
// whether the selection changed.
func (painter *painter) placeCaret(event platform.PointerEvent) bool {
	switch event.Kind {
	case platform.PointerDown:
		painter.placing = false
		primary := event.Button == ui.PointerButtonPrimary || event.Button == ui.PointerButtonNone
		if !primary || painter.focused.IsZero() || painter.hovered != painter.focused {
			return false
		}
	case platform.PointerMove:
		if !painter.placing {
			return false
		}
	case platform.PointerUp:
		placing := painter.placing
		painter.placing = false
		if !placing {
			return false
		}
	default:
		return false
	}
	context, placement := painter.focusedTextField()
	if placement == nil || context.ReadOnly || painter.textInput == nil {
		painter.placing = false
		return false
	}
	offset, ok := placement.OffsetAt(ui.Point{X: event.X, Y: event.Y})
	input := painter.textInput(context)
	if !ok || input == nil {
		return false
	}
	if input.Editor != nil {
		input.State = TextStateOf(input.Editor)
	}
	value := input.State.Value
	index := utf8.RuneCountInString(value[:max(0, min(offset, len(value)))])
	if event.Kind == platform.PointerDown {
		painter.placing, painter.placeAnchor = true, index
	}
	selection := TextEvent{Kind: TextSetSelection, Selection: TextRange{Start: painter.placeAnchor, End: index}}
	if input.Editor != nil {
		selection.Apply(input.Editor)
		input.State = TextStateOf(input.Editor)
		return true
	}
	if input.OnEvent == nil {
		return false
	}
	next, err := input.OnEvent(selection)
	if err != nil {
		return false
	}
	input.State = next
	return true
}

// isWindowControl reports whether id is one of the window buttons Viva draws.
func (painter *painter) isWindowControl(id ui.ID) bool {
	if painter.controls == nil {
		return false
	}
	_, ok := painter.controls.control(id)
	return ok
}

// shortcut offers one modified printable chord to the application before the
// native text system. An unhandled chord must remain available to that system.
// shortcutHandler is what a window hands the platform for modified chords: the
// painter's, when the application asked for chords, and none otherwise. Both ways
// of opening a window use it, so neither can forget to.
func (painter *painter) shortcutHandler(config *Config) platform.ShortcutHandler {
	if config.OnShortcut == nil {
		return nil
	}
	return painter.shortcut
}

func (painter *painter) shortcut(event platform.KeyEvent) (platform.Frame, bool, error) {
	if painter.onShortcut == nil {
		return platform.Frame{}, false, nil
	}
	event.Text = normalizeShortcutText(event.Text)
	response := painter.onShortcut(ui.KeyEvent{
		Key: ui.Key(event.Key), Modifiers: ui.Modifiers(event.Modifiers), Target: painter.focused,
		Repeat: event.Repeat, Text: event.Text, Win32: event.Win32,
	})
	focusChanged := painter.applyEventResult(response)
	if !focusChanged && !response.Repaint {
		return platform.Frame{}, response.Handled, nil
	}
	frame, err := painter.repaint()
	return frame, response.Handled, err
}

func normalizeShortcutText(value string) string {
	if len(value) == 1 && value[0] >= 'A' && value[0] <= 'Z' {
		const letters = "abcdefghijklmnopqrstuvwxyz"
		index := int(value[0] - 'A')
		return letters[index : index+1]
	}
	return value
}

// keysWanted reports whether the focused terminal asked for presses and
// releases of every key (TextInput.KeyReleases).
func (painter *painter) keysWanted() bool {
	return painter.textEnabled && painter.nativeText.Terminal && painter.nativeText.KeyReleases
}

// key applies one keyboard event: Tab traversal and Enter/Space activation.
// Releases and modifier keys exist only for a terminal that asked for them;
// they never reach a handler otherwise.
func (painter *painter) key(event platform.KeyEvent) (platform.Frame, error) {
	if event.Release {
		return painter.keyRelease(event)
	}
	modifier := event.Key.IsModifier()
	if modifier && !painter.keysWanted() {
		return platform.Frame{}, nil
	}
	before := painter.focused
	beforeKeyboard := painter.keyboardFocus
	response := ui.EventResult{}
	if painter.onKey != nil {
		repeat := event.Repeat
		if event.Code != 0 && painter.keysWanted() {
			again, tracked := painter.keys.Press(input.HeldKey{
				Code: event.Code, Key: event.Key, Text: event.Text, Target: painter.focused, Win32: event.Win32,
			})
			if tracked && !again && painter.keys.Len() == 1 {
				painter.keysTarget = painter.focused
			}
			repeat = repeat || again
		}
		response = painter.onKey(ui.KeyEvent{Key: ui.Key(event.Key), Modifiers: ui.Modifiers(event.Modifiers), Target: painter.focused, Repeat: repeat, Text: event.Text, Typed: event.Typed, Win32: event.Win32})
	}
	if modifier {
		return painter.afterKeyResponse(response)
	}
	if response.ContinueTraversal {
		// The application closed an overlay in this handler. Show that frame
		// first, then let the same key traverse the page that is now underneath.
		painter.applyEventResult(response)
		if _, err := painter.repaint(); err != nil {
			return platform.Frame{}, err
		}
		response = ui.EventResult{}
	}
	if !response.Handled && event.Key == input.KeyEscape && !painter.escapeTarget.IsZero() && painter.canClick() && !painter.nativeText.State.Composing {
		// An input method owns Escape while it is composing: the key cancels
		// the composition instead of dismissing the overlay behind the field.
		painter.deliverClick(painter.escapeTarget)
		return painter.repaint()
	}
	result := input.Result{Target: input.Target(painter.focused)}
	if !response.Handled {
		result = painter.router.KeyDown(input.KeyEvent{Key: event.Key, Modifiers: event.Modifiers, Repeat: event.Repeat})
	}
	activated := false
	if !response.Handled && result.Activated && painter.deliverClick(result.Target) {
		activated = true
	}
	painter.focused = result.Target
	if event.Key == input.KeyTab {
		painter.keyboardFocus = !painter.focused.IsZero()
	}
	focusChanged := painter.applyEventResult(response)
	if painter.focused == before && !activated && painter.keyboardFocus == beforeKeyboard && !focusChanged && !response.Repaint {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// afterKeyResponse applies the result of a key handler that has no focus
// traversal of its own to do.
func (painter *painter) afterKeyResponse(response ui.EventResult) (platform.Frame, error) {
	if !painter.applyEventResult(response) && !response.Repaint {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// keyRelease delivers the end of a key the handler saw go down, with the
// identity the press had. A release of anything else, such as the end of a key
// an input method consumed, is dropped.
func (painter *painter) keyRelease(event platform.KeyEvent) (platform.Frame, error) {
	held, ok := painter.keys.Release(event.Code)
	if !ok || painter.onKey == nil {
		return platform.Frame{}, nil
	}
	// A release carries the identity the press had. The modifier state at
	// release may already differ, as when Ctrl is released before C, so the
	// platform's release character can be a different one than the press named.
	win32 := held.Win32
	if win32 == (ui.Win32Key{}) {
		win32 = event.Win32
	}
	response := painter.onKey(ui.KeyEvent{
		Key: held.Key, Modifiers: ui.Modifiers(event.Modifiers), Target: held.Target, Release: true, Text: held.Text, Win32: win32,
	})
	return painter.afterKeyResponse(response)
}

// releaseKeys releases every key still down, last pressed first, the way the
// keyboard would have if it had stayed with them. It reports whether a handler
// asked for a repaint.
func (painter *painter) releaseKeys() bool {
	if painter.keys.Len() == 0 {
		return false
	}
	var storage [input.MaxHeldKeys]input.HeldKey
	held := painter.keys.Take(storage[:0])
	repaint := false
	for index := len(held) - 1; index >= 0 && painter.onKey != nil; index-- {
		response := painter.onKey(ui.KeyEvent{Key: held[index].Key, Target: held[index].Target, Release: true, Text: held[index].Text})
		repaint = painter.applyEventResult(response) || response.Repaint || repaint
	}
	return repaint
}

// focusLostHandler is what a window hands the platform for the loss of the
// keyboard: it releases the keys that were down, since their releases will go
// to whatever window has the keyboard now.
func (painter *painter) focusLostHandler() platform.FocusLostHandler {
	if painter.onKey == nil {
		return nil
	}
	return painter.focusLost
}

func (painter *painter) focusLost() (platform.Frame, error) {
	if !painter.releaseKeys() {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// noteKeyboardTarget records, after a paint, whether the held keys still have a
// terminal that wants them; if not the next repaint ends releases them.
func (painter *painter) noteKeyboardTarget(target ui.ID, wanted bool) {
	if painter.keys.Len() > 0 && (!wanted || target != painter.keysTarget) {
		painter.releaseDue = true
	}
}

// scroll applies one scroll gesture to the vertical offset and repaints.
// Positive native deltas scroll content up, revealing content below.
// fileDrop takes one step of a drag of files from the system. The point is
// resolved to the hit under it, like a pointer's, and the answer is whether
// that hit takes files.
func (painter *painter) fileDrop(event platform.FileDropEvent) (platform.Frame, bool, error) {
	if painter.onFileDrop == nil {
		return platform.Frame{}, false, nil
	}
	target := ui.ID{}
	if event.Phase != platform.FileDropLeave {
		if hit, ok := painter.router.HitTest(event.X, event.Y); ok {
			target = hit
		}
	}
	response := painter.onFileDrop(ui.FileDropEvent{
		Phase: ui.FileDropPhase(event.Phase), X: event.X, Y: event.Y, Target: target, Paths: event.Paths,
	})
	focusChanged := painter.applyEventResult(response)
	if !response.Repaint && !focusChanged {
		return platform.Frame{}, response.Handled, nil
	}
	frame, err := painter.repaint()
	return frame, response.Handled, err
}

func (painter *painter) scroll(event platform.ScrollEvent) (platform.Frame, error) {
	if event.DeltaY == 0 && event.DeltaX == 0 {
		return platform.Frame{}, nil
	}
	target := ui.ID{}
	if hit, ok := painter.router.HitTest(event.X, event.Y); ok {
		target = hit
	}
	response := ui.EventResult{}
	if painter.onScroll != nil {
		response = painter.onScroll(ui.ScrollEvent{
			Modifiers: ui.Modifiers(event.Modifiers), X: event.X, Y: event.Y,
			DeltaX: event.DeltaX, DeltaY: event.DeltaY, Viewport: painter.size, Target: target, Precise: event.Precise,
		})
	}
	focusChanged := painter.applyEventResult(response)
	beforeX, beforeY := painter.scrollX, painter.scrollY
	if !response.Handled {
		painter.scrollX = max(0, painter.scrollX+event.DeltaX)
		painter.scrollY = max(0, painter.scrollY+event.DeltaY)
	}
	if painter.scrollX == beforeX && painter.scrollY == beforeY && !focusChanged && !response.Repaint {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// initialTextInput is the native editor the first paint already calls for: a
// text field or terminal that has the keyboard from the start (HitOptions.
// Autofocus, or an initial focus). The platform has to be told when its window
// opens: the painter has already recorded the editor as enabled, so no later
// repaint would announce it.
func (painter *painter) initialTextInput() *platform.TextInput {
	if !painter.textEnabled || painter.activeText == nil {
		return nil
	}
	input := painter.nativeText
	return &input
}

func (painter *painter) applyEventResult(result ui.EventResult) bool {
	if result.ClipboardText != "" && painter.setClipboardText != nil {
		painter.setClipboardText(result.ClipboardText)
	}
	before, beforeVisible := painter.focused, painter.keyboardFocus
	if result.Blur {
		painter.pendingFocus = ui.ID{}
		painter.pendingVisible = false
		painter.router.Blur()
	} else if !result.Focus.IsZero() {
		if painter.router.Focus(input.Target(result.Focus)) {
			painter.pendingFocus = ui.ID{}
			painter.pendingVisible = false
		} else if focused, ok := painter.router.Focused(); !ok || focused != result.Focus {
			// The event may have revealed this hit. Resolve it against the hit
			// list produced by the repaint instead of making the application
			// stage focus through a second event.
			painter.pendingFocus = result.Focus
			painter.pendingVisible = result.FocusVisible
		}
	}
	if focused, ok := painter.router.Focused(); ok {
		painter.focused = focused
	} else {
		painter.focused = ui.ID{}
	}
	if painter.focused != before {
		painter.keyboardFocus = result.FocusVisible && !painter.focused.IsZero()
	} else if !result.Focus.IsZero() || result.Blur {
		painter.keyboardFocus = result.FocusVisible && !painter.focused.IsZero()
	}
	return painter.focused != before || painter.keyboardFocus != beforeVisible || !painter.pendingFocus.IsZero()
}

// canClick reports whether a click on a control has anywhere to go: whether the
// last paint asked Clicked.
func (painter *painter) canClick() bool {
	return painter.paint != nil && painter.canvas.ListensForClicks()
}

// deliverClick carries out a completed click on id and reports whether anything
// received it. When the last paint asked ui.Canvas.Clicked about anything, the
// paint runs once more as the event pass of this click, the one pass in which
// Clicked(id) is true and the application may change state. That pass's drawing
// is discarded: the repaint the caller asks for next is the one that shows the
// result. A paint that never asks costs nothing here.
func (painter *painter) deliverClick(id ui.ID) bool {
	if !painter.canClick() {
		return false
	}
	painter.canvas.BeginEventPass(id)
	painter.paintCanvas()
	painter.canvas.EndEventPass()
	painter.canvas.RunAfterPaint()
	return true
}

// pointerIsDelivered says which pointer events the event pass offers to controls:
// presses and releases aimed at a control, and the moves of a press that is held.
// A move over a control is hover, which a control reads from Canvas.Hovered.
func pointerIsDelivered(kind platform.PointerEventKind, target ui.ID, captured bool) bool {
	switch kind {
	case platform.PointerDown, platform.PointerUp:
		return !target.IsZero()
	case platform.PointerMove:
		return captured
	}
	return false
}

// deliverPointer runs the paint once more as the event pass of a pointer event when
// the last paint asked ui.Canvas.PointerOn or PointerIn, and reports whether a
// control consumed the event. Only presses, releases and the moves of a held press
// are offered: hovering is read from Hovered, not delivered. A paint that never
// asks costs nothing here.
func (painter *painter) deliverPointer(event ui.PointerEvent) bool {
	if painter.paint == nil || !painter.canvas.ListensForPointer() {
		return false
	}
	painter.canvas.BeginPointerPass(event)
	painter.paintCanvas()
	return painter.canvas.EndPointerPass()
}

// repaint reruns the paint callback with current interaction state and
// synchronizes the result with the retained renderer.
func (painter *painter) repaint() (platform.Frame, error) {
	painter.paintCanvas()
	if !painter.pendingFocus.IsZero() {
		before, beforeVisible := painter.focused, painter.keyboardFocus
		painter.syncRegions()
		if painter.focused != before || painter.keyboardFocus != beforeVisible {
			painter.paintCanvas()
		}
	}
	if painter.paint != nil && painter.canvas.TrappedFocusScope() != painter.router.ActiveScope() {
		// A modal just opened or closed, which moves focus into it or back
		// out. Settle that now and paint once more so this frame already
		// shows the new focus instead of lagging a repaint behind.
		before := painter.focused
		painter.syncRegions()
		if painter.focused != before {
			painter.paintCanvas()
		}
	}
	frame, err := painter.publish()
	if err != nil {
		return platform.Frame{}, err
	}
	if painter.releaseDue {
		painter.releaseDue = false
		if painter.releaseKeys() && painter.scheduleFrame != nil {
			painter.scheduleFrame(0)
		}
	}
	painter.scheduleIfAnimating()
	return frame, nil
}

func (painter *painter) paintCanvas() {
	if painter.paint == nil {
		return
	}
	painter.canvas.Reset()
	if painter.scale > 0 {
		painter.canvas.SetPixelScale(painter.scale)
	}
	focused, visible := painter.focused, painter.keyboardFocus
	if !painter.pendingFocus.IsZero() {
		// A virtualized list needs the requested focus while laying out the
		// reveal. Painting the old focus can scroll it back into view before
		// the pending target has had a chance to enter the hit list.
		focused, visible = painter.pendingFocus, painter.pendingVisible
	}
	painter.canvas.SetInteraction(painter.hovered, painter.pressed, focused, visible)
	painter.canvas.SetScroll(painter.scrollX, painter.scrollY)
	painter.canvas.BeginFrame()
	painter.paint(painter.canvas, painter.size)
	if painter.controls != nil {
		painter.controls.paint(painter.canvas, painter.size)
	}
	painter.canvas.EndFrame()
}

// scheduleIfAnimating asks for exactly one more frame when the last paint left
// a transition in flight or asked for a later one. A settled canvas schedules
// nothing, which is what keeps an idle application at zero frames, timers, and
// GPU submissions.
func (painter *painter) scheduleIfAnimating() {
	if painter.scheduleFrame == nil {
		return
	}
	if delay, ok := painter.canvas.NextFrame(); ok {
		painter.scheduleFrame(delay)
	}
}

// publish reconciles the canvas with the scene, router, and label list and
// returns the resulting renderer frame.
func (painter *painter) publish() (platform.Frame, error) {
	painter.applyClipping()
	painter.syncShapes()
	painter.changes = painter.graph.DrainChanges(painter.changes[:0])
	update, err := painter.renderer.Apply(painter.changes)
	if err != nil {
		// The scene has already absorbed this frame, so the failed batch will
		// never be sent again: leaving the store as it is would freeze every
		// node it skipped at its old value for good. Rebuild both from the
		// canvas instead.
		if update, err = painter.resyncRenderer(); err != nil {
			return platform.Frame{}, fmt.Errorf("app: render state is inconsistent: %w", err)
		}
	}
	if painter.stampState() && !update.Rebuild {
		update = render.Update{Count: len(painter.renderer.Rectangles()), Rebuild: true}
	}

	frame := platform.Frame{Update: update, Rectangles: painter.renderer.Rectangles()}
	if !painter.sameAsLastContent() {
		painter.syncLabels()
		painter.saveContent()
		frame.TextLabels = painter.labels
	}

	regionsUnchanged := painter.hitStructuresUnchanged()
	if regionsUnchanged {
		painter.reusedHits++
	} else {
		painter.syncRegions()
	}
	if err := painter.syncCursor(); err != nil {
		return platform.Frame{}, err
	}
	// The text input comes first: the caret a text field reports to assistive
	// technology is the one the editor holds now.
	frame.TextInput = painter.syncTextInput()
	// The semantic payload follows the text input, because the caret a text
	// field reports to assistive technology is the one the editor holds now.
	// A repaint that changed neither the hits, the focus, nor the text state
	// keeps the payload from the previous publish.
	if !regionsUnchanged || !painter.structuresUnchanged() {
		painter.syncSemantics()
		painter.recordHitStructures()
		if !sameAccessibility(painter.accessibility, painter.lastAccess) {
			painter.lastAccess = append(painter.lastAccess[:0], painter.accessibility...)
			frame.Accessibility = painter.accessibility
		}
	}
	// The frame is built; the platform will consume it before the next paint.
	// Release capacities left behind by a previous, larger page so a one-off
	// peak does not become the application's permanent footprint.
	painter.trim()
	return frame, nil
}

// trim gives back the backing arrays of the painter's per-frame buffers when
// they are far larger than the current frame. canvas.Trim does the same for
// the primitives the application painted. The slices keep their contents, so
// the frame that was just returned remains valid; its old arrays are released
// once the platform has consumed it.
func (painter *painter) trim() {
	// A scene and semantic tree keep their free slots for reuse. That is right
	// for a page that stays large, but after a large page is replaced by a
	// small one the slots are pure retention. Rebuild both from the small
	// frame instead of carrying the peak for the rest of the session.
	const trimAt, trimRatio = 4096, 4
	if slots := painter.graph.Slots(); slots > trimAt && painter.graph.Len()*trimRatio < slots {
		if _, err := painter.resyncRenderer(); err == nil {
			painter.stateBySlot = nil
		}
	}
	if slots := painter.semantics.Slots(); slots > trimAt && painter.semantics.Len()*trimRatio < slots {
		painter.semantics = semantic.NewWithCapacity("Viva", len(painter.hitScratch)+1)
		painter.semanticIDs = painter.semanticIDs[:0]
		painter.semanticChanges = painter.semanticChanges[:0]
		painter.semanticsDirty = true
	}
	painter.canvas.Trim()
	painter.regions = trimPainterCapacity(painter.regions)
	painter.shapeIDs = trimPainterCapacity(painter.shapeIDs)
	painter.labels = trimPainterCapacity(painter.labels)
	painter.paintedIDs = trimPainterCapacity(painter.paintedIDs)
	painter.paintedScratch = trimPainterCapacity(painter.paintedScratch)
	painter.lastTexts = trimPainterCapacity(painter.lastTexts)
	painter.lastRuns = trimPainterCapacity(painter.lastRuns)
	painter.lastImages = trimPainterCapacity(painter.lastImages)
	painter.hitScratch = trimPainterCapacity(painter.hitScratch)
	painter.publishedHits = trimPainterCapacity(painter.publishedHits)
	painter.clipped = trimPainterCapacity(painter.clipped)
	painter.clippedPaths = trimPainterCapacity(painter.clippedPaths)
	painter.clippedSoft = trimPainterCapacity(painter.clippedSoft)
	painter.order = trimPainterCapacity(painter.order)
	painter.blocks = trimPainterCapacity(painter.blocks)
	painter.clippedTexts = trimPainterCapacity(painter.clippedTexts)
	painter.clippedImages = trimPainterCapacity(painter.clippedImages)
	painter.accessibility = trimPainterCapacity(painter.accessibility)
	painter.lastAccess = trimPainterCapacity(painter.lastAccess)
	painter.changes = trimPainterCapacity(painter.changes)
	painter.semanticChanges = trimPainterCapacity(painter.semanticChanges)
	painter.semanticIDs = trimPainterCapacity(painter.semanticIDs)
	if painter.labelNamePeak > 4096 && len(painter.labelNames)*4 < painter.labelNamePeak {
		painter.labelNames = nil
		painter.labelNamePeak = 0
	}
}

// trimPainterCapacity is trimCapacity for the painter's buffers. The same
// threshold is used so a page that stays near its capacity never reallocates.
func trimPainterCapacity[T any](items []T) []T {
	const minCapacity = 4096
	const ratio = 4
	if cap(items) <= minCapacity || len(items)*ratio >= cap(items) {
		return items
	}
	next := make([]T, len(items), max(len(items)*2, minCapacity/64))
	copy(next, items)
	return next
}

func (painter *painter) syncCursor() error {
	target := painter.hovered
	if !painter.pressTarget.IsZero() {
		target = painter.pressTarget
	}
	next := CursorArrow
	if painter.edge != platform.EdgeNone && painter.pressTarget.IsZero() {
		return painter.applyCursor(cursorForEdge(painter.edge))
	}
	if !target.IsZero() {
		hits := painter.canvas.Hits()
		for index := len(hits) - 1; index >= 0; index-- {
			if hits[index].ID == target && !hits[index].Disabled && !hits[index].IsLabel {
				next = cursorFromHit(hits[index].Cursor)
				// Text fields remain selectable when read-only. An explicit cursor
				// still wins, and disabled fields keep the ordinary pointer.
				if hits[index].Cursor == ui.PointerCursorDefault && hits[index].Role == ui.RoleTextField {
					next = CursorText
				}
				break
			}
		}
	}
	return painter.applyCursor(next)
}

func (painter *painter) applyCursor(next Cursor) error {
	if next == painter.cursor {
		return nil
	}
	if painter.setCursor != nil {
		if err := painter.setCursor(next); err != nil {
			return err
		}
	}
	painter.cursor = next
	return nil
}

func cursorFromHit(cursor ui.PointerCursor) Cursor {
	switch cursor {
	case ui.PointerCursorText:
		return CursorText
	case ui.PointerCursorHand:
		return CursorHand
	case ui.PointerCursorCrosshair:
		return CursorCrosshair
	case ui.PointerCursorWait:
		return CursorWait
	case ui.PointerCursorResizeHorizontal:
		return CursorResizeHorizontal
	case ui.PointerCursorResizeVertical:
		return CursorResizeVertical
	case ui.PointerCursorResizeNWSE:
		return CursorResizeNWSE
	case ui.PointerCursorResizeNESW:
		return CursorResizeNESW
	case ui.PointerCursorNotAllowed:
		return CursorNotAllowed
	default:
		return CursorArrow
	}
}

// resyncRenderer discards the retained scene and render store and rebuilds
// both from the current canvas, returning the full-rebuild update.
func (painter *painter) resyncRenderer() (render.Update, error) {
	painter.resyncs++
	painter.graph = scene.NewWithCapacity(len(painter.clipped) + len(painter.clippedPaths) + len(painter.clippedSoft) + 1)
	painter.renderer = render.Store{}
	painter.shapeIDs = painter.shapeIDs[:0]
	painter.syncShapes()
	painter.changes = painter.graph.DrainChanges(painter.changes[:0])
	return painter.renderer.Apply(painter.changes)
}

// focusedTextField describes the focused text field of the last paint, and how
// it places its caret when it lays its rows out itself.
func (painter *painter) focusedTextField() (TextInputContext, ui.TextPlacement) {
	for _, hit := range painter.canvas.Hits() {
		// A combobox owns the native editor only when it names the text
		// geometry: one that paints its own query (components.Command) declares
		// no TextBounds and keeps what it draws.
		if hit.ID == painter.focused && !hit.Disabled &&
			(hit.Role == ui.RoleTextField || hit.Role == ui.RoleTerminal ||
				hit.Role == ui.RoleCombobox && hit.TextBounds != (ui.Rect{})) {
			return TextInputContext{
				ID: hit.ID, Bounds: hit.Bounds, Layer: hit.Layer, Terminal: hit.Role == ui.RoleTerminal,
				Multiline: hit.Multiline, ReadOnly: hit.ReadOnly, Secure: hit.Secure,
				NavigationKeys: hit.NavigationKeys,
				TextBounds:     hit.TextBounds, FontSize: hit.TextSize, TextColor: hit.TextColor,
			}, hit.Placement
		}
	}
	return TextInputContext{}, nil
}

func (painter *painter) syncTextInput() platform.TextInputUpdate {
	if painter.textInput == nil {
		painter.noteKeyboardTarget(ui.ID{}, false)
		return platform.TextInputUpdate{}
	}
	context, placement := painter.focusedTextField()
	var previousEditor *text.Editor
	if painter.activeText != nil {
		previousEditor = painter.activeText.Editor
	}
	var input *TextInput
	if !context.ID.IsZero() && !context.ReadOnly {
		input = painter.textInput(context)
	}
	if input == nil {
		placement = nil
	}
	if input != nil && input.Editor != nil {
		input.State = TextStateOf(input.Editor)
	}
	var nextEditor *text.Editor
	if input != nil {
		nextEditor = input.Editor
	}
	if previousEditor != nil && previousEditor != nextEditor {
		previousEditor.BreakUndoGroup()
	}
	painter.noteKeyboardTarget(context.ID, input != nil && context.Terminal && input.KeyReleases)
	if input == nil {
		painter.activeText = nil
		if !painter.textEnabled {
			return platform.TextInputUpdate{}
		}
		painter.textEnabled = false
		return platform.TextInputUpdate{Changed: true}
	}
	// What the application leaves unset comes from the component's declaration.
	bounds, fontSize, tint := input.Bounds, input.FontSize, input.Color
	if bounds == (ui.Rect{}) {
		bounds = context.TextBounds
		if bounds == (ui.Rect{}) {
			bounds = context.Bounds
		}
	}
	if fontSize == 0 {
		fontSize = context.FontSize
	}
	if tint == (ui.Color{}) {
		tint = context.TextColor
	}
	if bounds.Width <= 0 || bounds.Height <= 0 ||
		roundPixel(bounds.Width) <= 0 || roundPixel(bounds.Height) <= 0 {
		painter.activeText = nil
		if !painter.textEnabled {
			return platform.TextInputUpdate{}
		}
		painter.textEnabled = false
		return platform.TextInputUpdate{Changed: true}
	}
	native := platform.TextInput{
		Target: context.ID,
		X:      bounds.X, Y: bounds.Y,
		Width: bounds.Width, Height: bounds.Height,
		FontSize: fontSize, Layer: painter.platformLayer(context.Layer), Multiline: context.Multiline, Secure: context.Secure, Terminal: context.Terminal,
		AltSendsEscape: input.AltSendsEscape, KeyReleases: context.Terminal && input.KeyReleases, MonoFace: input.MonoFace,
		NavigationKeys: input.NavigationKeys || context.NavigationKeys,
		Color: platform.Color{
			Red: float64(tint.Red), Green: float64(tint.Green),
			Blue: float64(tint.Blue), Alpha: float64(tint.Alpha),
		},
		State: platform.TextState{
			Value:             input.State.Value,
			Selection:         platformTextRange(input.State.Selection),
			SelectionReversed: input.State.SelectionReversed,
			Composition:       platformTextRange(input.State.Composition),
			Composing:         input.State.Composing,
		},
		OnEvent: painter.onTextEvent,
	}
	if placement != nil {
		native.Placed = true
		if caret, ok := placement.CaretBounds(); ok {
			native.CaretX, native.CaretY, native.CaretWidth, native.CaretHeight = caret.X, caret.Y, caret.Width, caret.Height
		}
	}
	painter.activeText = input
	if painter.textEnabled && samePlatformTextInput(painter.nativeText, native) {
		return platform.TextInputUpdate{}
	}
	painter.nativeText = native
	painter.textEnabled = true
	return platform.TextInputUpdate{Changed: true, Input: &painter.nativeText}
}

func (painter *painter) applyTextEvent(event platform.TextEvent) (platform.TextState, error) {
	active := painter.activeText
	if active == nil || active.Editor == nil && active.OnEvent == nil {
		return platform.TextState{}, fmt.Errorf("app: native text event without an active handler")
	}
	appEvent := TextEvent{
		Kind: TextEventKind(event.Kind), Text: event.Text,
		Selection:       appTextRange(event.Selection),
		MarkedSelection: appTextRange(event.MarkedSelection),
		Replacement:     appTextRange(event.Replacement), HasReplacement: event.HasReplacement,
		Command: TextCommand(event.Command),
	}
	if !painter.nativeText.Multiline && !painter.nativeText.Terminal &&
		(event.Kind == platform.TextPaste || event.Kind == platform.TextInsert) && event.Text != "" {
		appEvent.Text = text.SingleLine(event.Text)
		if appEvent.Text == "" {
			return painter.nativeText.State, nil
		}
	}
	var next TextState
	if active.Editor != nil {
		appEvent.Apply(active.Editor)
		if painter.nativeText.Secure {
			active.Editor.ClearHistory()
		}
		next = TextStateOf(active.Editor)
	} else {
		var err error
		if next, err = active.OnEvent(appEvent); err != nil {
			return platform.TextState{}, err
		}
	}
	painter.activeText.State = next
	native := platform.TextState{
		Value:             next.Value,
		Selection:         platformTextRange(next.Selection),
		SelectionReversed: next.SelectionReversed,
		Composition:       platformTextRange(next.Composition),
		Composing:         next.Composing,
		Redraw:            next.Redraw,
	}
	// What the platform mirrors back never carries the one-shot request.
	painter.nativeText.State = native.WithoutRedraw()
	painter.activeText.State.Redraw = false
	return native, nil
}

func samePlatformTextInput(left, right platform.TextInput) bool {
	return left.Target == right.Target && left.X == right.X && left.Y == right.Y && left.Width == right.Width && left.Height == right.Height &&
		left.FontSize == right.FontSize && left.Layer == right.Layer && left.Multiline == right.Multiline && left.Secure == right.Secure && left.Terminal == right.Terminal && left.MonoFace == right.MonoFace &&
		left.AltSendsEscape == right.AltSendsEscape && left.KeyReleases == right.KeyReleases &&
		left.NavigationKeys == right.NavigationKeys &&
		left.Placed == right.Placed && left.CaretX == right.CaretX && left.CaretY == right.CaretY &&
		left.CaretWidth == right.CaretWidth && left.CaretHeight == right.CaretHeight &&
		left.Color == right.Color && left.State == right.State
}

// shapeRef names one clipped shape: an entry of painter.clippedPaths when
// triangle is set, of painter.clippedSoft when shadow is set, of painter.clipped
// otherwise.
type shapeRef struct {
	index    int32
	triangle bool
	shadow   bool
}

// applyClipping drops every shape and text its canvas clip hides and notes, on
// the others, whether the clip cuts them: a shape left with HasClip set is drawn
// through a scissor rectangle by the renderer, which is how a partial shape is
// cut, so no geometry is cut here. It also merges the rectangles, triangles and
// shadows back into the order they were painted in.
func (painter *painter) applyClipping() {
	painter.clipped = painter.clipped[:0]
	painter.clippedPaths = painter.clippedPaths[:0]
	painter.clippedSoft = painter.clippedSoft[:0]
	painter.clippedTexts = painter.clippedTexts[:0]
	painter.clippedImages = painter.clippedImages[:0]
	painter.order = painter.order[:0]
	triangles := painter.canvas.Triangles()
	shadows := painter.canvas.Shadows()
	next := 0       // the next canvas triangle to place
	nextShadow := 0 // the next canvas shadow to place
	// placeTrianglesBefore places the triangles and shadows painted before
	// rectangle number rectangles, so they enter the order ahead of it. A shadow
	// follows the triangles painted before it and precedes the others.
	placeTrianglesBefore := func(rectangles int) {
		for {
			triangleDue := next < len(triangles) && int(triangles[next].RectsBefore) <= rectangles
			shadowDue := nextShadow < len(shadows) && int(shadows[nextShadow].RectsBefore) <= rectangles
			if shadowDue && (!triangleDue || int(shadows[nextShadow].TrianglesBefore) <= next) {
				shadow := shadows[nextShadow]
				nextShadow++
				if shadow.HasClip {
					visible, cut := clipVisibility(shadowBounds(shadow), shadow.ClipOf)
					if !visible {
						continue
					}
					shadow.HasClip = cut
				}
				painter.order = append(painter.order, shapeRef{index: int32(len(painter.clippedSoft)), shadow: true})
				painter.clippedSoft = append(painter.clippedSoft, shadow)
				continue
			}
			if !triangleDue {
				return
			}
			triangle := triangles[next]
			next++
			if triangle.HasClip {
				bounds := triangleBounds(triangle.Points)
				if triangle.EdgeMask != 0 {
					// Native renderers also cover half a device pixel outside the
					// silhouette. Keep the scissor when that fringe meets a clip.
					half := 0.5 / painter.canvas.PixelScale()
					bounds.X -= half
					bounds.Y -= half
					bounds.Width += 2 * half
					bounds.Height += 2 * half
				}
				visible, cut := clipVisibility(bounds, triangle.ClipOf)
				if !visible {
					continue
				}
				triangle.HasClip = cut
			}
			painter.order = append(painter.order, shapeRef{index: int32(len(painter.clippedPaths)), triangle: true})
			painter.clippedPaths = append(painter.clippedPaths, triangle)
		}
	}
	for number, rectangle := range painter.canvas.Rectangles() {
		placeTrianglesBefore(number)
		if rectangle.HasClip {
			visible, cut := clipVisibility(rectangle.Bounds, rectangle.ClipOf)
			if !visible {
				continue
			}
			rectangle.HasClip = cut
		}
		painter.order = append(painter.order, shapeRef{index: int32(len(painter.clipped))})
		painter.clipped = append(painter.clipped, rectangle)
	}
	placeTrianglesBefore(len(painter.canvas.Rectangles()))
	for _, text := range painter.canvas.Texts() {
		// A label with no area or no font size paints nothing, and the native
		// label backends require positive values. Transient layouts (a closing
		// drawer, a pane squeezed to nothing) can produce one; drop it here
		// rather than failing the whole frame.
		if roundPixel(text.Bounds.Width) <= 0 || roundPixel(text.Bounds.Height) <= 0 || text.Style.Size <= 0 {
			continue
		}
		if text.HasClip {
			if _, visible := intersectVisible(text.Bounds, text.ClipOf); !visible {
				continue
			}
		}
		painter.clippedTexts = append(painter.clippedTexts, text)
		if text.RunCount > 0 {
			painter.appendBlockElements(text)
		}
	}
	painter.layerShift = 0
	for _, image := range painter.canvas.Images() {
		if roundPixel(image.Bounds.Width) <= 0 || roundPixel(image.Bounds.Height) <= 0 {
			continue
		}
		if image.Backdrop {
			painter.layerShift = 1
			painter.clippedImages = append(painter.clippedImages, image)
			continue
		}
		if !image.HasClip {
			painter.clippedImages = append(painter.clippedImages, image)
			continue
		}
		visible, ok := intersectVisible(image.Bounds, image.ClipOf)
		if !ok {
			continue
		}
		if image.DevicePixels {
			// The clip may end between device pixels; the picture is cut at one.
			scale := painter.canvas.PixelScale()
			if visible = deviceBounds(visible, scale); visible.Width <= 0 || visible.Height <= 0 {
				continue
			}
		}
		if visible != image.Bounds {
			sx := image.Source.Width / image.Bounds.Width
			sy := image.Source.Height / image.Bounds.Height
			image.Source.X += (visible.X - image.Bounds.X) * sx
			image.Source.Y += (visible.Y - image.Bounds.Y) * sy
			image.Source.Width = visible.Width * sx
			image.Source.Height = visible.Height * sy
			image.Bounds = visible
			if image.DevicePixels {
				image.Source = wholePixels(image.Source)
			}
		}
		painter.clippedImages = append(painter.clippedImages, image)
	}
}

// appendBlockElements paints the Block Elements of a fixed-pitch line as rectangles after
// the canvas's own, as the line's text is drawn after them (see ui.Canvas.BlockRectangles).
func (painter *painter) appendBlockElements(text ui.Text) {
	painter.blocks = painter.canvas.BlockRectangles(painter.blocks[:0], text)
	for _, rectangle := range painter.blocks {
		if rectangle.HasClip {
			visible, cut := clipVisibility(rectangle.Bounds, rectangle.ClipOf)
			if !visible {
				continue
			}
			rectangle.HasClip = cut
		}
		painter.order = append(painter.order, shapeRef{index: int32(len(painter.clipped))})
		painter.clipped = append(painter.clipped, rectangle)
	}
}

// clipVisibility reports whether any of bounds lies inside clip and whether the
// clip cuts it, that is, whether part of bounds lies outside.
func clipVisibility(bounds, clip ui.Rect) (visible, cut bool) {
	if bounds.X+bounds.Width <= clip.X || bounds.Y+bounds.Height <= clip.Y ||
		bounds.X >= clip.X+clip.Width || bounds.Y >= clip.Y+clip.Height {
		return false, false
	}
	return true, !containsRect(clip, bounds)
}

// shadowBounds is the region a shadow paints in: the rectangle that casts it
// grown by how far its blur, spread and offset reach.
func shadowBounds(shadow ui.Shadow) ui.Rect {
	extent := shadow.Style.Extent()
	return ui.Rect{
		X: shadow.Bounds.X - extent, Y: shadow.Bounds.Y - extent,
		Width: shadow.Bounds.Width + 2*extent, Height: shadow.Bounds.Height + 2*extent,
	}
}

func triangleBounds(points [3]ui.Point) ui.Rect {
	left := min32(points[0].X, min32(points[1].X, points[2].X))
	top := min32(points[0].Y, min32(points[1].Y, points[2].Y))
	right := max32(points[0].X, max32(points[1].X, points[2].X))
	bottom := max32(points[0].Y, max32(points[1].Y, points[2].Y))
	return ui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func intersectVisible(bounds, clip ui.Rect) (ui.Rect, bool) {
	left := max32(bounds.X, clip.X)
	top := max32(bounds.Y, clip.Y)
	right := min32(bounds.X+bounds.Width, clip.X+clip.Width)
	bottom := min32(bounds.Y+bounds.Height, clip.Y+clip.Height)
	if right <= left || bottom <= top {
		return ui.Rect{}, false
	}
	return ui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}, true
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// drawnBy reports whether a stored instance already draws a clipped shape.
// Both sides are compared field by field, as the scene stores them.
func (painter *painter) drawnBy(stored *render.Rectangle, ref shapeRef) bool {
	if ref.triangle {
		triangle := &painter.clippedPaths[ref.index]
		return stored.Kind == scene.KindTriangle && stored.Color == scene.Color(triangle.Color) &&
			stored.EdgeMask == triangle.EdgeMask && stored.Shade == sceneShade(*triangle) &&
			stored.Points[0] == scene.Point(triangle.Points[0]) &&
			stored.Points[1] == scene.Point(triangle.Points[1]) &&
			stored.Points[2] == scene.Point(triangle.Points[2])
	}
	if ref.shadow {
		shadow := &painter.clippedSoft[ref.index]
		return stored.Kind == scene.KindShadow && stored.Radius == shadow.Radius &&
			stored.Color == scene.Color(shadow.Color) && stored.Bounds == scene.Rect(shadow.Bounds) &&
			stored.Shadow == scene.Shadow(shadow.Style)
	}
	rectangle := &painter.clipped[ref.index]
	return stored.Kind == scene.KindRectangle && stored.Radius == rectangle.Radius &&
		stored.Color == scene.Color(rectangle.Color) && stored.Bounds == scene.Rect(rectangle.Bounds)
}

// syncShapes aligns the scene's nodes with the clipped shapes, rectangles and
// triangles together in paint order, so the render store, which paints in node
// order, always equals the canvas.
//
// A repaint usually changes one stretch of the shapes: a highlight appears, a
// tab closes, a page swaps. The shapes before and after that stretch are left
// alone; inside it the nodes are rewritten in place (a node that held a
// rectangle may become a triangle and keeps its place), and the difference in
// length is inserted or removed at the end of the stretch. Cost follows the
// size of the change, not of the page, and no node is ever painted out of
// order.
func (painter *painter) syncShapes() {
	root := painter.graph.Root()
	stored := painter.renderer.Rectangles() // what the previous frame painted
	oldCount, newCount := len(painter.shapeIDs), len(painter.order)
	if len(stored) != oldCount {
		// Not reachable while the store follows the scene; if it ever does not,
		// rewrite every node rather than trust the comparison below.
		stored = nil
	}

	prefix := 0
	for prefix < min(len(stored), newCount) && painter.drawnBy(&stored[prefix], painter.order[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < min(oldCount, newCount)-prefix && len(stored) == oldCount &&
		painter.drawnBy(&stored[oldCount-1-suffix], painter.order[newCount-1-suffix]) {
		suffix++
	}
	oldMiddle, newMiddle := oldCount-prefix-suffix, newCount-prefix-suffix

	for index := prefix; index < prefix+min(oldMiddle, newMiddle); index++ {
		painter.setShape(painter.shapeIDs[index], painter.order[index])
	}
	switch {
	case newMiddle > oldMiddle:
		at := prefix + oldMiddle
		add := newMiddle - oldMiddle
		var previous scene.ID
		if at > 0 {
			previous = painter.shapeIDs[at-1]
		}
		for range add {
			painter.shapeIDs = append(painter.shapeIDs, 0)
		}
		copy(painter.shapeIDs[at+add:], painter.shapeIDs[at:oldCount])
		for index := at; index < at+add; index++ {
			previous = painter.insertShape(root, previous, painter.order[index])
			painter.shapeIDs[index] = previous
		}
	case oldMiddle > newMiddle:
		from := prefix + newMiddle
		for _, id := range painter.shapeIDs[from : prefix+oldMiddle] {
			painter.graph.Remove(id)
		}
		painter.shapeIDs = slices.Delete(painter.shapeIDs, from, prefix+oldMiddle)
	}
}

// setShape makes an existing node draw a shape, whatever it drew before.
func (painter *painter) setShape(id scene.ID, ref shapeRef) {
	if ref.triangle {
		value := sceneTriangle(painter.clippedPaths[ref.index])
		painter.graph.SetTriangle(id, value.Points, value.Color, value.EdgeMask, value.Shade)
		return
	}
	if ref.shadow {
		shadow := &painter.clippedSoft[ref.index]
		painter.graph.SetShadow(id, scene.Rect(shadow.Bounds), scene.Color(shadow.Color), shadow.Radius, scene.Shadow(shadow.Style))
		return
	}
	painter.setRectangle(id, painter.clipped[ref.index])
}

// insertShape adds a node drawing a shape right after previous.
func (painter *painter) insertShape(root, previous scene.ID, ref shapeRef) scene.ID {
	if ref.triangle {
		value := sceneTriangle(painter.clippedPaths[ref.index])
		id, _ := painter.graph.InsertTriangle(root, previous, value.Points, value.Color, value.EdgeMask, value.Shade)
		return id
	}
	if ref.shadow {
		shadow := &painter.clippedSoft[ref.index]
		id, _ := painter.graph.InsertShadow(root, previous, scene.Rect(shadow.Bounds), scene.Color(shadow.Color), shadow.Radius, scene.Shadow(shadow.Style))
		return id
	}
	rectangle := painter.clipped[ref.index]
	value := sceneRectangle(rectangle)
	id, _ := painter.graph.InsertRoundedRectangle(root, previous, value.Bounds, value.Color, rectangle.Radius)
	return id
}

func (painter *painter) setRectangle(id scene.ID, rectangle ui.Rectangle) {
	value := sceneRectangle(rectangle)
	painter.graph.SetRoundedRectangle(id, value.Bounds, value.Color, rectangle.Radius)
}

// applyAutofocus moves the keyboard to the first hit that asks for it
// (HitOptions.Autofocus) and was not painted last time. A trapped focus scope
// keeps its own say: an autofocus outside the active scope cannot take focus.
func (painter *painter) applyAutofocus() {
	hits := painter.canvas.Hits()
	next := painter.paintedScratch[:0]
	focused := false
	for index := range hits {
		hit := &hits[index]
		if !focused && hit.Autofocus && !hit.ID.IsZero() && !painter.wasPainted(hit.ID) {
			if painter.router.Focus(input.Target(hit.ID)) {
				focused = true
			}
		}
		if !hit.ID.IsZero() {
			next = append(next, hit.ID)
		}
	}
	painter.paintedScratch = next
	painter.paintedIDs, painter.paintedScratch = next, painter.paintedIDs[:0]
}

// wasPainted reports whether the previous paint declared a hit with this ID.
func (painter *painter) wasPainted(id ui.ID) bool {
	for _, previous := range painter.paintedIDs {
		if previous == id {
			return true
		}
	}
	return false
}

func compareRegionLayers(left, right input.Region) int {
	return cmp.Compare(left.Layer, right.Layer)
}

// syncRegions feeds the canvas hit list into the retained input router.
func (painter *painter) syncRegions() {
	painter.regions = painter.regions[:0]
	hits := painter.canvas.Hits()
	for index := range hits {
		hit := &hits[index]
		if hit.Container {
			continue // semantic regions are not interaction targets
		}
		painter.regions = append(painter.regions, input.Region{
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
	// Hits are declared in the order the code paints, which is not the order
	// the layers composite in: a popup on a higher layer is often painted
	// before the field it covers. The front-most region must be the one on the
	// highest layer, and within a layer the last declared.
	if !slices.IsSortedFunc(painter.regions, compareRegionLayers) {
		slices.SortStableFunc(painter.regions, compareRegionLayers)
	}
	painter.router.SetRegions(painter.regions)
	painter.router.SetActiveScope(painter.canvas.TrappedFocusScope(), input.Target(painter.canvas.InitialFocus()))
	painter.applyAutofocus()
	if request := painter.canvas.FocusRequest(); !request.IsZero() {
		if request != painter.focusClaim {
			if focused, ok := painter.router.Focused(); ok && focused == input.Target(request) {
				painter.focusClaim = request
			} else if painter.router.Focus(input.Target(request)) {
				painter.focusClaim = request
			}
		}
	} else {
		painter.focusClaim = ui.ID{}
	}
	if !painter.pendingFocus.IsZero() {
		target := input.Target(painter.pendingFocus)
		painter.router.Focus(target)
		if focused, ok := painter.router.Focused(); ok && focused == target {
			painter.keyboardFocus = painter.pendingVisible
		}
		painter.pendingFocus = ui.ID{}
		painter.pendingVisible = false
	}
	painter.escapeTarget = painter.canvas.EscapeTarget()
	if focused, ok := painter.router.Focused(); ok {
		painter.focused = focused
	} else {
		painter.focused = ui.ID{}
	}
}

// hitStructuresUnchanged reports whether the paint produced exactly the input
// the regions and the semantic tree were last built from, so both can be kept.
// It compares every field those structures read, plus the focus, scope, escape
// and text-selection state they carry outside the hit list. A page that only
// changes colors, or does not change at all, therefore costs no per-hit
// bookkeeping; any semantic change opens the gate and the full mirror runs.
func (painter *painter) hitStructuresUnchanged() bool {
	if !painter.structuresUnchanged() {
		return false
	}
	hits := painter.canvas.Hits()
	if len(hits) != len(painter.publishedHits) {
		return false
	}
	return sameHits(painter.publishedHits, hits)
}

// structuresUnchanged is hitStructuresUnchanged without the hit comparison: the
// focus, trap, escape and text state that also feed the semantic tree. The
// publish uses it for the second check, after syncTextInput has had its say;
// the hits cannot change in between, so the expensive memcmp runs once.
func (painter *painter) structuresUnchanged() bool {
	if !painter.hitsPublished || painter.semanticsDirty || !painter.pendingFocus.IsZero() {
		return false
	}
	if painter.focused != painter.publishedFocus ||
		painter.canvas.TrappedFocusScope() != painter.publishedTrap ||
		painter.canvas.InitialFocus() != painter.publishedInitial ||
		painter.canvas.EscapeTarget() != painter.publishedEscape ||
		painter.canvas.FocusRequest() != painter.publishedRequest {
		return false
	}
	if painter.activeText == nil {
		if painter.publishedTextField {
			return false
		}
	} else if !painter.publishedTextField ||
		painter.activeText.State.Value != painter.publishedText ||
		painter.activeText.State.Selection != painter.publishedSelection ||
		painter.activeText.State.Composition != painter.publishedCompose ||
		painter.activeText.State.Composing != painter.publishedComposing {
		return false
	}
	return true
}

// sameHits reports whether two hit lists are field for field identical. It
// compares the structs as bytes: Go strings are immutable, and the published
// list keeps the previous string allocations reachable, so a matching pointer
// cannot name different bytes. Padding bytes can only make the comparison
// stricter (an unnecessary publish), never more permissive.
func sameHits(published, current []ui.Hit) bool {
	if len(published) != len(current) {
		return false
	}
	if len(published) == 0 {
		return true
	}
	size := int(unsafe.Sizeof(ui.Hit{}))
	left := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(published))), len(published)*size)
	right := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(current))), len(current)*size)
	return bytes.Equal(left, right)
}

// recordHitStructures keeps the state that lets the next repaint skip the
// region and semantic mirror when nothing that feeds them changed.
func (painter *painter) recordHitStructures() {
	painter.publishedHits = append(painter.publishedHits[:0], painter.canvas.Hits()...)
	painter.publishedFocus = painter.focused
	painter.publishedTrap = painter.canvas.TrappedFocusScope()
	painter.publishedInitial = painter.canvas.InitialFocus()
	painter.publishedEscape = painter.canvas.EscapeTarget()
	painter.publishedRequest = painter.canvas.FocusRequest()
	painter.publishedTextField = painter.activeText != nil
	if painter.activeText != nil {
		painter.publishedText = painter.activeText.State.Value
		painter.publishedSelection = painter.activeText.State.Selection
		painter.publishedCompose = painter.activeText.State.Composition
		painter.publishedComposing = painter.activeText.State.Composing
	}
	painter.hitsPublished = true
	painter.semanticsDirty = false
}

// syncSemantics mirrors the canvas hit list into the retained semantic tree
// and the flattened accessibility payload.
func (painter *painter) syncSemantics() {
	root := painter.semantics.Root()
	// Blocks are occluders, not targets: they have no accessible meaning. Labels
	// are no nodes either; their words name the control they label.
	hits := painter.hitScratch[:0]
	labelled := false
	needsLabelMap := false
	patchedLabel := false
	lastAppended := -1
	// A label declared before its control is held here until that control is
	// appended. Forms paint a title label, then the field, so this is the common
	// order; the map fallback is only for labels that never meet their control,
	// a second label for the same control, or more pending labels than fit.
	var pending [8]labelPair
	pendingCount := 0
	trap := painter.canvas.TrappedFocusScope()
	canvasHits := painter.canvas.Hits()
	for index := range canvasHits {
		hit := &canvasHits[index]
		switch {
		case hit.Block, hit.MovesWindow:
		case hit.IsLabel:
			labelled = true
			switch {
			case hit.ID.IsZero():
				// nameFromLabels ignores an anonymous label too.
			case len(hits) > 0 && lastAppended == index-1 && !patchedLabel && hits[len(hits)-1].ID == hit.ID:
				// The label follows its control: patch the words in place.
				hits[len(hits)-1].Label = hit.Label
				patchedLabel = true
			case len(hits) > 0 && lastAppended == index-1 && patchedLabel && hits[len(hits)-1].ID == hit.ID:
				// A second label for one control: the first wins, as before.
				needsLabelMap = true
			case pendingCount == len(pending):
				needsLabelMap = true
			default:
				known := false
				for pendingIndex := 0; pendingIndex < pendingCount; pendingIndex++ {
					if pending[pendingIndex].id == hit.ID {
						known = true // keep the first label, as nameFromLabels does
						break
					}
				}
				if !known {
					pending[pendingCount] = labelPair{id: hit.ID, words: hit.Label}
					pendingCount++
				}
			}
		case hit.Backdrop:
			// A dismiss backdrop is not a control, and it must not be announced
			// as a window-sized button.
		case !trap.IsZero() && hit.Scope != trap:
			// A modal makes the page behind it inert to assistive technology,
			// as Base UI's aria-hidden does, rather than only to the pointer.
		default:
			current := *hit
			for pendingIndex := 0; pendingIndex < pendingCount; pendingIndex++ {
				if pending[pendingIndex].id == current.ID {
					current.Label = pending[pendingIndex].words
					pending[pendingIndex] = pending[pendingCount-1]
					pendingCount--
					break
				}
			}
			hits = append(hits, current)
			lastAppended = index
			patchedLabel = false
		}
	}
	if labelled && (needsLabelMap || pendingCount > 0) {
		painter.nameFromLabels(hits)
	}
	painter.hitScratch = hits

	common := len(painter.semanticIDs)
	if len(hits) < common {
		common = len(hits)
	}
	for index := 0; index < common; index++ {
		painter.semantics.Set(painter.semanticIDs[index], semanticNode(hits[index], painter.focused))
	}
	for index := len(painter.semanticIDs); index < len(hits); index++ {
		id, _ := painter.semantics.Add(root, semanticNode(hits[index], painter.focused))
		painter.semanticIDs = append(painter.semanticIDs, id)
	}
	if len(hits) < len(painter.semanticIDs) {
		for _, id := range painter.semanticIDs[len(hits):] {
			painter.semantics.Remove(id)
		}
		painter.semanticIDs = painter.semanticIDs[:len(hits)]
	}
	// The tree is only mirrored natively below; reusing the scratch avoids
	// allocating a discarded change list every time the hit set changes.
	painter.semanticChanges = painter.semantics.DrainChanges(painter.semanticChanges[:0])

	painter.accessibility = painter.accessibility[:0]
	for _, hit := range hits {
		node := platform.AccessibilityNode{
			ID:          hit.ID,
			Role:        uint8(semanticRole(hit.Role)),
			X:           hit.Bounds.X,
			Y:           hit.Bounds.Y,
			Width:       hit.Bounds.Width,
			Height:      hit.Bounds.Height,
			Label:       hit.Label,
			Value:       hit.Value,
			Hint:        hit.Hint,
			Focused:     !hit.ID.IsZero() && hit.ID == painter.focused,
			Disabled:    hit.Disabled,
			Checked:     hit.Checked,
			Mixed:       hit.Mixed,
			Selected:    hit.Selected,
			Expanded:    hit.Expanded,
			Multiline:   hit.Multiline,
			ReadOnly:    hit.ReadOnly,
			Secure:      hit.Secure,
			Focusable:   hit.Focusable,
			Expandable:  hit.Expandable,
			Description: hit.Description,
			Level:       hit.Level,
			PosInSet:    hit.PosInSet,
			SetSize:     hit.SetSize,
			Live:        hit.Live,
			Range:       hit.Range,
		}
		if node.Focused && !hit.ID.IsZero() && painter.activeText != nil && !hit.Secure &&
			(hit.Role == ui.RoleTextField || hit.Role == ui.RoleCombobox) && painter.activeText.State.Value == hit.Value {
			selection := painter.activeText.State.Selection
			node.Selection = platform.AccessibilitySelection{Start: int32(selection.Start), End: int32(selection.End), Valid: true}
		}
		painter.accessibility = append(painter.accessibility, node)
	}
}

// labelPair is a label hit waiting for the control it names.
type labelPair struct {
	id    ui.ID
	words string
}

// nameFromLabels gives each control that has a label hit the words of the first
// one as its accessible name, as `<label for>` names an input. It builds the
// label map once per paint instead of scanning every hit for every control:
// a page with many labelled fields used to cost O(controls*hits), which made
// a 4,000-field page take tens of milliseconds per repaint.
func (painter *painter) nameFromLabels(controls []ui.Hit) {
	all := painter.canvas.Hits()
	if painter.labelNames == nil {
		painter.labelNames = make(map[ui.ID]string, 8)
	}
	clear(painter.labelNames)
	for index := range all {
		label := &all[index]
		if !label.IsLabel || label.ID.IsZero() {
			continue
		}
		if _, exists := painter.labelNames[label.ID]; !exists {
			painter.labelNames[label.ID] = label.Label
		}
	}
	for index := range controls {
		if label, ok := painter.labelNames[controls[index].ID]; ok {
			controls[index].Label = label
		}
	}
	if len(painter.labelNames) > painter.labelNamePeak {
		painter.labelNamePeak = len(painter.labelNames)
	}
}

func semanticNode(hit ui.Hit, focused ui.ID) semantic.Node {
	return semantic.Node{
		Role:      semanticRole(hit.Role),
		Label:     hit.Label,
		Value:     hit.Value,
		Hint:      hit.Hint,
		Bounds:    semantic.Rect{X: hit.Bounds.X, Y: hit.Bounds.Y, Width: hit.Bounds.Width, Height: hit.Bounds.Height},
		Focusable: hit.Focusable,
		Focused:   !hit.ID.IsZero() && hit.ID == focused,
		Disabled:  hit.Disabled,
		Checked:   hit.Checked,
		Mixed:     hit.Mixed,
		Selected:  hit.Selected,
		Expanded:  hit.Expanded,
		Multiline: hit.Multiline,
		ReadOnly:  hit.ReadOnly,
		Secure:    hit.Secure,
	}
}

func semanticRole(role ui.Role) semantic.Role {
	switch role {
	case ui.RoleLink:
		return semantic.RoleLink
	case ui.RoleCheckbox:
		return semantic.RoleCheckbox
	case ui.RoleRadio:
		return semantic.RoleRadio
	case ui.RoleTab:
		return semantic.RoleTab
	case ui.RoleMenuItem:
		return semantic.RoleMenuItem
	case ui.RoleCombobox:
		return semantic.RoleCombobox
	case ui.RoleSwitch:
		return semantic.RoleSwitch
	case ui.RoleTextField:
		return semantic.RoleTextField
	case ui.RoleSlider:
		return semantic.RoleSlider
	case ui.RoleTerminal:
		return semantic.RoleTextField
	case ui.RoleGroup:
		return semantic.RoleGroup
	case ui.RoleMenu:
		return semantic.RoleMenu
	case ui.RoleMenubar:
		return semantic.RoleMenubar
	case ui.RoleListBox:
		return semantic.RoleListBox
	case ui.RoleOption:
		return semantic.RoleOption
	case ui.RoleGrid:
		return semantic.RoleGrid
	case ui.RoleGridCell:
		return semantic.RoleGridCell
	case ui.RoleHeading:
		return semantic.RoleHeading
	case ui.RoleStatus:
		return semantic.RoleStatus
	case ui.RoleSeparator:
		return semantic.RoleSeparator
	default:
		return semantic.RoleButton
	}
}

// syncLabels rebuilds the label list. A bitmap label carries its pixels across
// the native boundary only when that bitmap is not exactly what the same place
// in the previous frame showed, so a picture made of many bitmaps (see
// ui.Surface) stages just the ones that changed.
func (painter *painter) syncLabels() {
	texts := painter.clippedTexts
	painter.labels = painter.labels[:0]
	for index, image := range painter.clippedImages {
		bitmap := image.Bitmap
		left := max(int(image.Source.X), 0)
		top := max(int(image.Source.Y), 0)
		right := min(int(math.Ceil(float64(image.Source.X+image.Source.Width))), bitmap.Width)
		bottom := min(int(math.Ceil(float64(image.Source.Y+image.Source.Height))), bitmap.Height)
		if left >= right || top >= bottom {
			continue
		}
		offset := top*bitmap.Stride + left*4
		length := (bottom-top-1)*bitmap.Stride + (right-left)*4
		if offset < 0 || length <= 0 || offset > len(bitmap.Pix) || length > len(bitmap.Pix)-offset {
			continue
		}
		width := roundPixel(image.Bounds.Width)
		height := roundPixel(image.Bounds.Height)
		if width <= 0 || height <= 0 {
			continue
		}
		layer := painter.platformLayer(image.Layer)
		if image.Backdrop {
			layer = 0
		}
		// Rounded to logical pixels, unless the image is laid out in device pixels.
		x, y, w, h := float32(roundPixel(image.Bounds.X)), float32(roundPixel(image.Bounds.Y)), float32(width), float32(height)
		if image.DevicePixels {
			x, y, w, h = image.Bounds.X, image.Bounds.Y, image.Bounds.Width, image.Bounds.Height
		}
		painter.labels = append(painter.labels, platform.TextLabel{
			X: x, Y: y,
			Width: w, Height: h,
			Layer:  layer,
			Bitmap: bitmap.Pix[offset : offset+length], BitmapWidth: right - left, BitmapHeight: bottom - top,
			BitmapStride: bitmap.Stride, BitmapID: bitmap.ID, BitmapVersion: bitmap.Version,
			BitmapUpload: index >= len(painter.lastImages) || !sameImageBitmap(image, painter.lastImages[index]),
		})
	}
	for _, label := range texts {
		width := roundPixel(label.Bounds.Width)
		height := roundPixel(label.Bounds.Height)
		if width <= 0 || height <= 0 {
			continue
		}
		// Integer logical-pixel bounds keep the texture exactly one texel per
		// physical pixel so linear sampling stays crisp on scaled displays.
		entry := platform.TextLabel{
			Value: label.Value,
			X:     float32(roundPixel(label.Bounds.X)), Y: float32(roundPixel(label.Bounds.Y)),
			Width: float32(width), Height: float32(height),
			OriginX: roundingRemainder(label.Bounds.X), OriginY: roundingRemainder(label.Bounds.Y),
			FontSize: label.Style.Size, Weight: label.Style.Weight,
			Color: platform.Color{
				Red: float64(label.Style.Color.Red), Green: float64(label.Style.Color.Green),
				Blue: float64(label.Style.Color.Blue), Alpha: float64(label.Style.Color.Alpha),
			},
			Layer: painter.platformLayer(label.Layer),
			Mono:  label.Style.Mono, MonoFace: label.Style.MonoFace, CellWidth: label.CellWidth, Runs: painter.canvas.TextRuns(label),
		}
		if label.RunCount > 0 {
			// The font does not draw Block Elements; appendBlockElements did.
			entry.Value, entry.Runs = ui.ReplaceBlockElements(entry.Value, entry.Runs)
		}
		if label.HasClip && !containsRect(label.ClipOf, label.Bounds) {
			// The clip cuts the label partway: the renderers scissor to it.
			entry.Clip, entry.Clipped = label.ClipOf, true
		}
		painter.labels = append(painter.labels, entry)
	}
}

// deviceBounds rounds the edges of bounds to whole device pixels of scale.
func deviceBounds(bounds ui.Rect, scale float32) ui.Rect {
	edge := func(value float32) float32 { return float32(math.Floor(float64(value)*float64(scale) + 0.5)) }
	left, top := edge(bounds.X), edge(bounds.Y)
	right, bottom := edge(bounds.X+bounds.Width), edge(bounds.Y+bounds.Height)
	return ui.Rect{X: left / scale, Y: top / scale, Width: (right - left) / scale, Height: (bottom - top) / scale}
}

// wholePixels rounds the edges of a bitmap region to whole bitmap pixels.
func wholePixels(region ui.Rect) ui.Rect {
	left, top := math.Floor(float64(region.X)+0.5), math.Floor(float64(region.Y)+0.5)
	right, bottom := math.Floor(float64(region.X+region.Width)+0.5), math.Floor(float64(region.Y+region.Height)+0.5)
	return ui.Rect{X: float32(left), Y: float32(top), Width: float32(right - left), Height: float32(bottom - top)}
}

// containsRect reports whether inner lies wholly inside outer.
func containsRect(outer, inner ui.Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}

// roundingRemainder is what roundPixel took off value: the sub-pixel position backends
// that place text at device pixels draw it at (platform.TextLabel.OriginX). Positions
// roundPixel clamps to zero have none.
func roundingRemainder(value float32) float32 {
	if value < 0 {
		return 0
	}
	return value - float32(roundPixel(value))
}

func roundPixel(value float32) int32 {
	if value < 0 {
		return 0
	}
	return int32(value + 0.5)
}

func sceneRectangle(rectangle ui.Rectangle) render.Rectangle {
	return render.Rectangle{
		Kind: scene.KindRectangle,
		Bounds: scene.Rect{
			X: rectangle.Bounds.X, Y: rectangle.Bounds.Y,
			Width: rectangle.Bounds.Width, Height: rectangle.Bounds.Height,
		},
		Color: scene.Color{
			Red: rectangle.Color.Red, Green: rectangle.Color.Green,
			Blue: rectangle.Color.Blue, Alpha: rectangle.Color.Alpha,
		},
		Radius: rectangle.Radius,
		Layer:  rectangle.Layer,
	}
}

func sceneTriangle(triangle ui.Triangle) render.Rectangle {
	return render.Rectangle{
		Kind: scene.KindTriangle,
		Points: [3]scene.Point{
			{X: triangle.Points[0].X, Y: triangle.Points[0].Y},
			{X: triangle.Points[1].X, Y: triangle.Points[1].Y},
			{X: triangle.Points[2].X, Y: triangle.Points[2].Y},
		},
		EdgeMask: triangle.EdgeMask,
		Color: scene.Color{
			Red: triangle.Color.Red, Green: triangle.Color.Green,
			Blue: triangle.Color.Blue, Alpha: triangle.Color.Alpha,
		},
		Shade: sceneShade(triangle),
		Layer: triangle.Layer,
	}
}

// sceneShade is the vertex colors of a shaded triangle, and the zero Shade for a
// flat one, whatever stale colors its Shade array holds.
func sceneShade(triangle ui.Triangle) scene.Shade {
	if !triangle.Shaded {
		return scene.Shade{}
	}
	return scene.Shade{On: true, Colors: [2]scene.Color{scene.Color(triangle.Shade[0]), scene.Color(triangle.Shade[1])}}
}

// instanceState is what a render instance knows about its place on screen that
// the scene does not carry: the layer it composes in and the clip that cuts it.
type instanceState struct {
	layer   uint16
	clipped bool
	clip    scene.Rect
}

// stampState copies each primitive's canvas layer and clip onto its render
// instance and reports whether either changed anywhere, which a partial update
// would not carry to the GPU. Instances are listed by the scene's tree order,
// which syncShapes keeps equal to paint order, so the shape at index i is the
// instance at index i; should the store ever disagree, the state travels with
// the node's storage slot instead.
func (painter *painter) stampState() (changed bool) {
	instances := painter.renderer.Rectangles()
	if len(instances) != len(painter.shapeIDs) {
		return painter.stampBySlot()
	}
	for index, id := range painter.shapeIDs {
		instance := &instances[index]
		if instance.ID != id {
			return painter.stampBySlot()
		}
		state := painter.shapeState(painter.order[index])
		if instance.Layer != state.layer || instance.Clipped != state.clipped || instance.Clip != state.clip {
			instance.Layer, instance.Clipped, instance.Clip = state.layer, state.clipped, state.clip
			changed = true
		}
	}
	return changed
}

// stampBySlot is stampState for a store whose order is not the paint order.
func (painter *painter) stampBySlot() (changed bool) {
	for index, id := range painter.shapeIDs {
		slot, ok := id.Slot()
		if !ok {
			continue
		}
		for int(slot) >= len(painter.stateBySlot) {
			painter.stateBySlot = append(painter.stateBySlot, instanceState{})
		}
		painter.stateBySlot[slot] = painter.shapeState(painter.order[index])
	}
	instances := painter.renderer.Rectangles()
	for index := range instances {
		slot, _ := instances[index].ID.Slot()
		state := painter.stateBySlot[slot]
		if instances[index].Layer != state.layer || instances[index].Clipped != state.clipped || instances[index].Clip != state.clip {
			instances[index].Layer, instances[index].Clipped, instances[index].Clip = state.layer, state.clipped, state.clip
			changed = true
		}
	}
	return changed
}

// shapeState is the layer and clip the platform draws a shape with.
func (painter *painter) shapeState(ref shapeRef) instanceState {
	var state instanceState
	if ref.shadow {
		shadow := &painter.clippedSoft[ref.index]
		state = instanceState{layer: shadow.Layer, clipped: shadow.HasClip}
		if shadow.HasClip {
			state.clip = scene.Rect(shadow.ClipOf)
		}
	} else if ref.triangle {
		triangle := &painter.clippedPaths[ref.index]
		state = instanceState{layer: triangle.Layer, clipped: triangle.HasClip}
		if triangle.HasClip {
			state.clip = scene.Rect(triangle.ClipOf)
		}
	} else {
		rectangle := &painter.clipped[ref.index]
		state = instanceState{layer: rectangle.Layer, clipped: rectangle.HasClip}
		if rectangle.HasClip {
			state.clip = scene.Rect(rectangle.ClipOf)
		}
	}
	state.layer = painter.platformLayer(state.layer)
	return state
}

// platformLayer is the layer a canvas layer takes on the platform. The topmost
// layer stays topmost.
func (painter *painter) platformLayer(layer uint16) uint16 {
	if painter.layerShift == 0 || layer == math.MaxUint16 {
		return layer
	}
	return layer + painter.layerShift
}

// sameAsLastTexts reports whether the texts to draw are exactly those last
// sent to the platform.
func (painter *painter) sameAsLastContent() bool {
	if !sameImages(painter.clippedImages, painter.lastImages) {
		return false
	}
	left, right := painter.clippedTexts, painter.lastTexts
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a.Value != b.Value || a.Style != b.Style || a.Layer != b.Layer {
			return false
		}
		if a.Bounds != b.Bounds || a.CellWidth != b.CellWidth || a.RunCount != b.RunCount {
			return false
		}
		if a.HasClip != b.HasClip || a.HasClip && a.ClipOf != b.ClipOf {
			return false
		}
		if a.RunCount == 0 {
			continue
		}
		current := painter.canvas.TextRuns(a)
		if len(current) != int(a.RunCount) || b.RunFirst < 0 || b.RunCount < 0 || int(b.RunFirst+b.RunCount) > len(painter.lastRuns) {
			return false
		}
		saved := painter.lastRuns[b.RunFirst : b.RunFirst+b.RunCount]
		for run := range current {
			if current[run] != saved[run] {
				return false
			}
		}
	}
	return true
}

func sameImages(left, right []ui.Image) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a.Bounds != b.Bounds || a.Source != b.Source || a.Layer != b.Layer || a.Backdrop != b.Backdrop || a.Bitmap.Width != b.Bitmap.Width ||
			a.Bitmap.Height != b.Bitmap.Height || a.Bitmap.Stride != b.Bitmap.Stride || a.Bitmap.ID != b.Bitmap.ID ||
			a.Bitmap.Version != b.Bitmap.Version || len(a.Bitmap.Pix) != len(b.Bitmap.Pix) {
			return false
		}
		if len(a.Bitmap.Pix) > 0 && &a.Bitmap.Pix[0] != &b.Bitmap.Pix[0] {
			return false
		}
	}
	return true
}

// sameImageBitmap reports whether two draws show the very same pixels: the same
// identity and version, source region and storage. A bitmap with no identity
// (ID 0) is never the same: nothing says its pixels did not change.
func sameImageBitmap(a, b ui.Image) bool {
	if a.Source != b.Source || a.Bitmap.Width != b.Bitmap.Width || a.Bitmap.Height != b.Bitmap.Height ||
		a.Bitmap.Stride != b.Bitmap.Stride || a.Bitmap.ID != b.Bitmap.ID || a.Bitmap.Version != b.Bitmap.Version ||
		len(a.Bitmap.Pix) != len(b.Bitmap.Pix) || a.Bitmap.ID == 0 {
		return false
	}
	return len(a.Bitmap.Pix) == 0 || &a.Bitmap.Pix[0] == &b.Bitmap.Pix[0]
}

func sameAccessibility(left, right []platform.AccessibilityNode) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// saveTexts remembers this frame's texts for the next comparison. Their runs
// live in the canvas, which is rewritten by the next paint, so they are copied.
func (painter *painter) saveContent() {
	painter.lastImages = append(painter.lastImages[:0], painter.clippedImages...)
	painter.lastTexts = append(painter.lastTexts[:0], painter.clippedTexts...)
	painter.lastRuns = painter.lastRuns[:0]
	for index := range painter.lastTexts {
		text := &painter.lastTexts[index]
		runs := painter.canvas.TextRuns(*text)
		text.RunFirst = int32(len(painter.lastRuns))
		painter.lastRuns = append(painter.lastRuns, runs...)
	}
}

// windowActions makes the set of operations the window buttons need, for the
// platform adapter to fill, or nil when the system draws the buttons.
func (painter *painter) windowActions() *platform.WindowActions {
	if painter.controls == nil {
		return nil
	}
	painter.controls.actions = &platform.WindowActions{}
	return painter.controls.actions
}
