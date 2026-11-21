package app

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/framebuild"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/interaction"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/semantic"
	"github.com/vivaui/viva/keymap"
	"github.com/vivaui/viva/text"
)

// painter reconciles immediate canvas paints with the retained scene and the
// native label list, so an unchanged repaint performs no renderer work.
type painter struct {
	fullscreen bool
	paint      func(canvas *gfx.Canvas, size gfx.Size)
	onPointer  func(gfx.PointerEvent) gfx.EventResult
	onKey      func(gfx.KeyEvent) gfx.EventResult
	onShortcut func(gfx.KeyEvent) gfx.EventResult
	onScroll   func(gfx.ScrollEvent) gfx.EventResult
	// onModifierTap and tap recognize a modifier pressed and released on its own.
	onModifierTap func(gfx.ModifierTapEvent) gfx.EventResult
	tap           input.ModifierTap
	onFileDrop    func(gfx.FileDropEvent) gfx.EventResult
	onResize      func(gfx.Size)
	// scale is the display's device pixels per logical pixel, zero when the platform
	// does not report it (see platform.Size): the canvas then keeps the scale it was given.
	scale      float32
	textInput  TextInputProvider
	activeText *TextInput
	// activeEditor remembers the installed owner independently of a provider's
	// reusable TextInput, which the application may mutate between frames.
	activeEditor *text.Editor
	onTextEvent  platform.TextHandler
	textEpoch    uint64
	// placing is a press on a text field that lays its rows out itself being held
	// (see placeCaret), from the rune index placeAnchor. placeUnit is the click
	// unit a multi-click press took (2 a word, 3 a line) and placeUnitRange the
	// selection it covered, which a drag then extends by the same unit.
	placing        bool
	placeAnchor    int
	placeUnit      int
	placeUnitRange text.Range
	nativeText     platform.TextInput
	// keys follows the keys of a terminal that asked for releases (see
	// TextInput.KeyReleases).
	keys            interaction.Keys
	textEnabled     bool
	canvas          *gfx.Canvas
	size            gfx.Size
	focus           interaction.Focus
	pointer         interaction.Pointer
	frame           *framebuild.Builder
	semantics       *semantic.Tree
	semanticIDs     []semantic.ID
	accessibility   []platform.AccessibilityNode
	lastAccess      []platform.AccessibilityNode
	semanticChanges []semantic.Change
	reusedHits      int                   // publishes that kept the previous regions and semantics
	views           []platform.NativeView // the native views of this publish
	lastViews       []platform.NativeView // the ones last sent
	covers          []gfx.Rect            // backing of the views' Covers
	hitScratch      []gfx.Hit
	// publishedHits is the hit list the regions and the semantic tree were last
	// built from. Comparing it byte for byte (the strings keep the previous
	// allocations alive, so equal pointers mean equal bytes) lets a repaint that
	// declared the same hits reuse both instead of mirroring the list again.
	publishedHits      []gfx.Hit
	semanticDirty      []gfx.ID
	publishedFocus     gfx.ID
	publishedTrap      gfx.ID
	publishedInitial   gfx.ID
	publishedEscape    gfx.ID
	publishedRequest   gfx.ID
	publishedReturns   []gfx.ID
	publishedText      string
	publishedSelection TextRange
	publishedCompose   TextRange
	publishedComposing bool
	publishedTextField bool
	hitsPublished      bool
	semanticsDirty     bool
	labelNames         map[gfx.ID]string
	labelNamePeak      int
	// controls draws the window buttons of a custom titlebar, on platforms where
	// Viva does; it is nil otherwise.
	controls *windowControls
	// edge is the window edge the pointer is over, for the resize cursor, on a
	// window whose frame Viva draws (the system supplies the cursor elsewhere).
	edge platform.WindowEdge
	// pointerOutside says the pointer left the window while a button was down.
	// The press keeps the pointer until the button comes up, so the exit waits
	// for the release instead of ending a drag at the window's edge.
	pointerOutside bool
	// onSurface says the pointer is on a surface the page declared with
	// gfx.Canvas.BlockSurface, such as an open menu's panel. It is recorded with
	// the hit test and read back into the next paint.
	onSurface bool
	cursor    Cursor
	scrollX   float32
	scrollY   float32
	// scheduleFrame asks the runtime for one more frame, after at least the
	// delay, while a transition is in flight or the application asked for one.
	scheduleFrame func(delay time.Duration)
	// noteFrameStart records that a frame is beginning, so the timer measures
	// its interval from the start of the paint rather than from its end.
	noteFrameStart func()
	// coalesceInput asks the pointer and wheel paths to leave their repaint to
	// the frame clock, so a burst of events becomes one frame. A window that
	// presents immediately sets it; the headless driver does not.
	coalesceInput    bool
	setClipboardText func(string)
	clipboardText    func() string
	setCursor        func(Cursor) error
	setCursorImage   func(*gfx.CursorImage) error
	// cursorImage is the picture shown while cursor is CursorImage.
	cursorImage *gfx.CursorImage
}

func newPainter(canvas *gfx.Canvas, config *Config) (*painter, error) {
	return newPainterWithClock(canvas, config, monotonicClock())
}

// newPainterWithClock is newPainter with an explicit animation clock, so a
// driver can run time on its own terms from the very first paint.
func newPainterWithClock(canvas *gfx.Canvas, config *Config, clock func() time.Duration) (*painter, error) {
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
		onModifierTap: config.OnModifierTap,
		onFileDrop:    config.OnFileDrop,
		onResize:      config.OnResize,
		textInput:     config.TextInput,
		canvas:        canvas,
		size:          gfx.Size{Width: float32(config.Width), Height: float32(config.Height)},
		frame:         framebuild.New(canvas),
		semantics:     semantic.NewWithCapacity("Viva", len(canvas.Hits())+1),
		semanticIDs:   make([]semantic.ID, 0, len(canvas.Hits())+1),
		accessibility: make([]platform.AccessibilityNode, 0, len(canvas.Hits())+1),
		lastAccess:    make([]platform.AccessibilityNode, 0, len(canvas.Hits())+1),
	}
	painter.onTextEvent = painter.applyTextEvent
	painter.clipboardText = func() string {
		active := painter.activeText
		if active == nil || active.CopyText == nil {
			return ""
		}
		epoch := painter.textEpoch
		override, rectangular := painter.nativeText.CopyOverride, painter.nativeText.CopyRectangular
		value := active.CopyText()
		// A reentrant provider must not copy old text or apply Cut to its replacement.
		if painter.activeText != active || painter.textEpoch != epoch ||
			painter.nativeText.CopyOverride != override || painter.nativeText.CopyRectangular != rectangular {
			return ""
		}
		return value
	}
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
	painter.frame.MarkInitialBitmapUploads()
	painter.lastAccess = append(painter.lastAccess[:0], painter.accessibility...)
	return painter, nil
}

// resize rebuilds the canvas at a new size and returns exactly one frame.
func (painter *painter) resize(size platform.Size) (platform.Frame, error) {
	next := gfx.Size{Width: size.Width, Height: size.Height}
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
	if painter.fullscreen {
		return platform.PressClient
	}
	return painter.focus.Router().PressAt(x, y)
}

// pointer applies one native pointer event to the interaction state machine
// and repaints only when hover or pressed state actually changed.
//
// A secondary press on a backdrop that passes it on (gfx.HitOptions.PassSecondary)
// is two presses in one: the first dismisses the overlay, as a click on the
// backdrop would, and the second is the same press aimed at what the overlay
// was covering, once the paint has taken it away.
func (painter *painter) pointerEvent(event platform.PointerEvent) (platform.Frame, error) {
	if event.Kind == platform.PointerDown {
		painter.tap.Cancel()
		painter.pointerOutside = false
	}
	if event.Kind == platform.PointerCancel {
		// The platform took the press for itself and its release will never
		// arrive, so the application's press ends here. It is told the pointer is
		// gone, which is what a press it can no longer follow amounts to.
		painter.pointerOutside = false
		painter.pointer.Exit()
		event.Kind = platform.PointerExit
	}
	if event.Kind == platform.PointerMove && painter.pointerOutside &&
		event.X >= 0 && event.Y >= 0 && event.X < painter.size.Width && event.Y < painter.size.Height {
		painter.pointerOutside = false
	}
	if event.Kind == platform.PointerExit && !painter.pointer.PressTarget().IsZero() {
		// A press keeps the pointer until its button comes up, as on every
		// desktop. A drag that ended here would stop following the pointer
		// exactly where the user needs it: past the edge of the terminal, the
		// list or the window. The exit is delivered when the button is released.
		painter.pointerOutside = true
		painter.onSurface = false
		if painter.pointer.Hovered().IsZero() {
			return platform.Frame{}, nil
		}
		painter.pointer.SetHovered(gfx.ID{})
		painter.pointer.SetHoveredRole(gfx.Role(0))
		return painter.repaintAfterInput()
	}
	beforeHovered, beforePressed := painter.pointer.Hovered(), painter.pointer.Pressed()
	beforeSurface := painter.onSurface
	beforeFocus, beforeKeyboard := painter.focus.Focused(), painter.focus.KeyboardFocus()
	passes := event.Kind == platform.PointerDown &&
		(event.Button == gfx.PointerButtonSecondary && painter.focus.Router().PassesSecondary(event.X, event.Y) ||
			event.Button == gfx.PointerButtonPrimary && painter.focus.Router().PassesPrimary(event.X, event.Y))
	result, focusChanged, edgeChanged := painter.pointerStep(event)
	if passes {
		if !result.Handled && painter.deliverClick(painter.pointer.Hovered(), pointerClickInfo(event)) {
			result.Handled = true
		}
		painter.paintCanvas()
		painter.focus.Sync(painter.canvas)
		if !painter.focus.Router().PassesSecondary(event.X, event.Y) && !painter.focus.Router().PassesPrimary(event.X, event.Y) {
			var again, edgeAgain bool
			result, again, edgeAgain = painter.pointerStep(event)
			focusChanged, edgeChanged = focusChanged || again, edgeChanged || edgeAgain
		}
		result.Repaint = true
	}

	switch event.Kind {
	case platform.PointerUp:
		if id, ok := painter.pointer.Click(); ok && !result.Handled {
			if painter.isWindowControl(id) {
				painter.controls.activate(id)
				result.Repaint = true
			} else if painter.deliverClick(id, pointerClickInfo(event)) {
				result.Repaint = true
			}
		}
		painter.pointer.Up()
		if painter.pointerOutside {
			// The button came up beyond the window, so the press is over and the
			// pointer is gone: tell the application now that it may clear hover
			// and hover-owned state.
			painter.pointerOutside = false
			if painter.onPointer != nil {
				gone := painter.onPointer(gfx.PointerEvent{Kind: gfx.PointerExit, Viewport: painter.size})
				result.Repaint = result.Repaint || gone.Repaint
			}
		}
	case platform.PointerExit:
		painter.pointer.Exit()
	}

	if painter.pointer.Hovered() == beforeHovered && painter.pointer.Pressed() == beforePressed &&
		painter.onSurface == beforeSurface &&
		painter.focus.Focused() == beforeFocus && painter.focus.KeyboardFocus() == beforeKeyboard &&
		!focusChanged && !result.Repaint {
		if edgeChanged {
			// Only the cursor differs: no repaint for it.
			return platform.Frame{}, painter.syncCursor()
		}
		return platform.Frame{}, nil
	}
	return painter.repaintAfterInput()
}

// pointerStep is one pass of a pointer event through hit testing, the press
// state, the application and the controls' event pass. It returns what the
// application answered and whether focus and the pointer's edge state changed.
func (painter *painter) pointerStep(event platform.PointerEvent) (result gfx.EventResult, focusChanged, edgeChanged bool) {
	painter.pointer.SetHovered(gfx.ID{})
	painter.pointer.SetHoveredRole(gfx.Role(0))
	painter.onSurface = false
	if event.Kind != platform.PointerExit {
		secondaryPress := event.Kind == platform.PointerDown && event.Button == gfx.PointerButtonSecondary
		if target, ok, onSurface := painter.focus.Router().PointerAt(event.X, event.Y, secondaryPress); ok || onSurface {
			painter.pointer.SetHovered(target.Target)
			painter.pointer.SetHoveredRole(target.Role)
			painter.onSurface = onSurface
		}
	}
	edgeChanged = painter.trackEdge(event)

	if event.Kind == platform.PointerDown {
		painter.pointer.Down(event.Button)
		painter.focus.PointerDown(event.X, event.Y, painter.pointer.Hovered())
	}

	target, captured := painter.pointer.Delivery(gfx.PointerEventKind(event.Kind))
	if painter.onPointer != nil {
		result = painter.onPointer(gfx.PointerEvent{
			Kind: gfx.PointerEventKind(event.Kind), Button: event.Button, Modifiers: gfx.Modifiers(event.Modifiers),
			X: event.X, Y: event.Y, ScreenX: event.ScreenX, ScreenY: event.ScreenY, Viewport: painter.size, Target: target, Captured: captured,
			ClickCount: event.ClickCount,
		})
	}
	if !result.Handled && pointerIsDelivered(event.Kind, target, captured) {
		if painter.deliverPointer(gfx.PointerEvent{
			Kind: gfx.PointerEventKind(event.Kind), Button: event.Button, Modifiers: gfx.Modifiers(event.Modifiers),
			X: event.X, Y: event.Y, ScreenX: event.ScreenX, ScreenY: event.ScreenY, Viewport: painter.size, Target: target, Captured: captured,
			ClickCount: event.ClickCount,
		}) {
			result.Handled, result.Repaint = true, true
		}
	}
	painter.pointer.NoteHandled(gfx.PointerEventKind(event.Kind), result.Handled)
	focusChanged = painter.applyEventResult(result)
	if result.Handled {
		// A handler owns this selection; the default drag must not resume.
		painter.placing = false
	} else if painter.placeCaret(event) {
		result.Repaint = true
	}
	return result, focusChanged, edgeChanged
}

// placeCaret puts the caret of the focused text field where a press, or the drag
// of one, lands, when the field lays its rows out itself (gfx.TextPlacement): the
// platform editor knows one line only and leaves such a field to this. A press
// puts the caret there and a drag extends the selection from it. It reports
// whether the selection changed.
func (painter *painter) placeCaret(event platform.PointerEvent) bool {
	switch event.Kind {
	case platform.PointerDown:
		painter.placing = false
		primary := event.Button == gfx.PointerButtonPrimary || event.Button == gfx.PointerButtonNone
		if !primary || painter.focus.Focused().IsZero() || painter.pointer.Hovered() != painter.focus.Focused() {
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
	context, placement, _ := painter.focusedTextField()
	if placement == nil || context.ReadOnly || painter.textInput == nil {
		painter.placing = false
		return false
	}
	offset, ok := placement.OffsetAt(gfx.Point{X: event.X, Y: event.Y})
	input := painter.textInput(context)
	if !ok || input == nil {
		return false
	}
	if input.Editor != nil {
		input.State = TextStateOf(input.Editor)
	}
	value := input.State.Value
	index := utf8.RuneCountInString(value[:max(0, min(offset, len(value)))])
	selection := TextEvent{Kind: TextSetSelection, Selection: TextRange{Start: painter.placeAnchor, End: index}}
	switch event.Kind {
	case platform.PointerDown:
		painter.placing, painter.placeAnchor = true, index
		selection.Selection.Start = index
		painter.placeUnit, painter.placeUnitRange = 0, text.Range{}
		switch {
		case event.ClickCount >= 3:
			painter.placeUnit, painter.placeUnitRange = 3, text.LineRange(value, index)
		case event.ClickCount == 2:
			painter.placeUnit, painter.placeUnitRange = 2, text.WordRange(value, index)
		}
		if painter.placeUnit == 2 && rangeIsSpace(value, painter.placeUnitRange) {
			// A double click on whitespace has no word to take: leave the caret
			// where the press landed instead of selecting the newlines that the
			// next keystroke would use to join two lines.
			painter.placeUnit = 0
		}
		if painter.placeUnit != 0 {
			painter.placeAnchor = painter.placeUnitRange.Start
			selection.Selection = TextRange{Start: painter.placeUnitRange.Start, End: painter.placeUnitRange.End}
		}
	default:
		if painter.placeUnit != 0 {
			selection.Selection = placeUnitSelection(value, index, painter.placeUnit, painter.placeUnitRange)
		}
		if event.Kind == platform.PointerUp {
			painter.placeUnit, painter.placeUnitRange = 0, text.Range{}
		}
	}
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

// placeUnitSelection extends the unit a multi-click press took to the unit
// under the dragging pointer, keeping the pressed unit whole.
func placeUnitSelection(value string, index, unit int, anchor text.Range) TextRange {
	target := text.WordRange(value, index)
	if unit == 3 {
		target = text.LineRange(value, index)
	}
	switch {
	case index >= anchor.Start && index <= anchor.End:
		return TextRange{Start: anchor.Start, End: anchor.End}
	case index > anchor.End:
		return TextRange{Start: anchor.Start, End: target.End}
	default:
		return TextRange{Start: target.Start, End: anchor.End}
	}
}

// rangeIsSpace reports whether a range of value holds only whitespace.
func rangeIsSpace(value string, r text.Range) bool {
	runes := []rune(value)
	start, end := max(0, min(r.Start, len(runes))), max(0, min(r.End, len(runes)))
	for _, r := range runes[start:end] {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// isWindowControl reports whether id is one of the window buttons Viva draws.
func (painter *painter) isWindowControl(id gfx.ID) bool {
	if painter.controls == nil || painter.fullscreen {
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
	painter.tap.Cancel()
	if painter.onShortcut == nil {
		return platform.Frame{}, false, nil
	}
	event.Text = normalizeShortcutText(event.Text)
	response := painter.onShortcut(gfx.KeyEvent{
		Key: gfx.Key(event.Key), Modifiers: gfx.Modifiers(event.Modifiers), Target: painter.focus.Focused(),
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

// menuShortcut offers a menu entry's chord to the window as the platform
// delivers the same key: a character chord to the shortcut handler, a named key
// to the key handler, both with the control that holds the keyboard as the
// target. The application decides whether the chord is its command; the caller
// carries out the result and asks for the repaint, unlike the platform's key
// path here.
func (painter *painter) menuShortcut(chord keymap.Chord) (gfx.EventResult, bool) {
	event := gfx.KeyEvent{Key: chord.Key, Modifiers: chord.Modifiers, Target: painter.focus.Focused()}
	handler := painter.onKey
	if chord.Char != 0 {
		event.Key = gfx.KeyUnknown
		event.Text = normalizeShortcutText(string(chord.Char))
		handler = painter.onShortcut
	}
	if handler == nil || !chord.Valid() {
		return gfx.EventResult{}, false
	}
	response := handler(event)
	return response, response.Handled
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
	if painter.onModifierTap != nil {
		if modifier {
			painter.tap.Press(event.Key, event.Code, event.Modifiers, event.Repeat)
		} else {
			painter.tap.Cancel()
		}
	}
	if modifier && !painter.keysWanted() {
		return platform.Frame{}, nil
	}
	before := painter.focus.Focused()
	beforeKeyboard := painter.focus.KeyboardFocus()
	response := gfx.EventResult{}
	if painter.onKey != nil {
		repeat := event.Repeat
		if event.Code != 0 && painter.keysWanted() {
			again, _ := painter.keys.Press(input.HeldKey{
				Code: event.Code, Key: event.Key, Text: event.Text, Target: painter.focus.Focused(), Win32: event.Win32,
			})
			repeat = repeat || again
		}
		response = painter.onKey(gfx.KeyEvent{Key: gfx.Key(event.Key), Modifiers: gfx.Modifiers(event.Modifiers), Target: painter.focus.Focused(), Repeat: repeat, Text: event.Text, Typed: event.Typed, Win32: event.Win32})
	}
	if modifier {
		return painter.afterKeyResponse(response)
	}
	if response.ContinueTraversal {
		// The application closed an overlay in this handler, so the same key has
		// to traverse the page that is now underneath. Repaint the canvas and
		// rebuild the routing regions from it, but do not publish here: this
		// event still owes the platform exactly one frame, and a frame published
		// now would be dropped by the repaint below. A dropped frame is a lost
		// update, and the update that closes an overlay is a rebuild: the
		// backend would keep the count it had and reject the smaller one.
		painter.applyEventResult(response)
		painter.paintCanvas()
		painter.focus.Sync(painter.canvas)
		response = gfx.EventResult{}
	}
	if !response.Handled && event.Key == input.KeyEscape && !painter.focus.EscapeTarget().IsZero() && painter.canClick() && !painter.nativeText.State.Composing {
		// An input method owns Escape while it is composing: the key cancels
		// the composition instead of dismissing the overlay behind the field.
		painter.deliverClick(painter.focus.EscapeTarget(), gfx.ClickInfo{})
		return painter.repaint()
	}
	result := input.Result{Target: input.Target(painter.focus.Focused())}
	if !response.Handled {
		result = painter.focus.Router().KeyDown(input.KeyEvent{Key: event.Key, Modifiers: event.Modifiers, Repeat: event.Repeat})
	}
	activated := false
	if !response.Handled && result.Activated && painter.deliverClick(result.Target, gfx.ClickInfo{}) {
		activated = true
	}
	painter.focus.Refresh()
	if event.Key == input.KeyTab {
		painter.focus.SetKeyboardFocus(!painter.focus.Focused().IsZero())
	}
	focusChanged := painter.applyEventResult(response)
	if painter.focus.Focused() == before && !activated && painter.focus.KeyboardFocus() == beforeKeyboard && !focusChanged && !response.Repaint {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// afterKeyResponse applies the result of a key handler that has no focus
// traversal of its own to do.
func (painter *painter) afterKeyResponse(response gfx.EventResult) (platform.Frame, error) {
	if !painter.applyEventResult(response) && !response.Repaint {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// keyRelease delivers the end of a key the handler saw go down, with the
// identity the press had. A release of anything else, such as the end of a key
// an input method consumed, is dropped.
func (painter *painter) keyRelease(event platform.KeyEvent) (platform.Frame, error) {
	tapRepaint := false
	if painter.onModifierTap != nil {
		if key, ok := painter.tap.Release(event.Code); ok {
			response := painter.onModifierTap(gfx.ModifierTapEvent{Key: gfx.Key(key), Target: painter.focus.Focused()})
			tapRepaint = painter.applyEventResult(response) || response.Repaint
		}
	}
	frame, painted, err := painter.releaseHeldKey(event)
	if err != nil || painted || !tapRepaint {
		return frame, err
	}
	return painter.repaint()
}

// releaseHeldKey is keyRelease for the key the handler saw go down.
func (painter *painter) releaseHeldKey(event platform.KeyEvent) (frame platform.Frame, painted bool, err error) {
	held, ok := painter.keys.Release(event.Code)
	if !ok || painter.onKey == nil {
		return platform.Frame{}, false, nil
	}
	// A release carries the identity the press had. The modifier state at
	// release may already differ, as when Ctrl is released before C, so the
	// platform's release character can be a different one than the press named.
	win32 := held.Win32
	if win32 == (gfx.Win32Key{}) {
		win32 = event.Win32
	}
	response := painter.onKey(gfx.KeyEvent{
		Key: held.Key, Modifiers: gfx.Modifiers(event.Modifiers), Target: held.Target, Release: true, Text: held.Text, Win32: win32,
	})
	if !painter.applyEventResult(response) && !response.Repaint {
		return platform.Frame{}, false, nil
	}
	frame, err = painter.repaint()
	return frame, true, err
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
		response := painter.onKey(gfx.KeyEvent{Key: held[index].Key, Target: held[index].Target, Release: true, Text: held[index].Text})
		repaint = painter.applyEventResult(response) || response.Repaint || repaint
	}
	return repaint
}

// focusLostHandler is what a window hands the platform for the loss of the
// keyboard: it releases the keys that were down, since their releases will go
// to whatever window has the keyboard now.
func (painter *painter) focusLostHandler() platform.FocusLostHandler {
	if painter.onKey == nil && painter.onModifierTap == nil {
		return nil
	}
	return painter.focusLost
}

func (painter *painter) focusLost() (platform.Frame, error) {
	painter.tap.Cancel()
	if !painter.releaseKeys() {
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

// noteKeyboardTarget records, after a paint, whether the held keys still have a
// terminal that wants them; if not the next repaint ends releases them.
func (painter *painter) noteKeyboardTarget(target gfx.ID, wanted bool) {
	painter.keys.NoteTarget(target, wanted)
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
	target := gfx.ID{}
	if event.Phase != platform.FileDropLeave {
		if hit, ok := painter.focus.Router().HitTest(event.X, event.Y); ok {
			target = hit
		}
	}
	response := painter.onFileDrop(gfx.FileDropEvent{
		Phase: gfx.FileDropPhase(event.Phase), X: event.X, Y: event.Y, Target: target, Paths: event.Paths,
		Files: fileDropFilesFromPaths(event.Paths),
	})
	focusChanged := painter.applyEventResult(response)
	if !response.Repaint && !focusChanged {
		return platform.Frame{}, response.Handled, nil
	}
	frame, err := painter.repaintAfterInput()
	return frame, response.Handled, err
}

func (painter *painter) scroll(event platform.ScrollEvent) (platform.Frame, error) {
	painter.tap.Cancel()
	if event.DeltaY == 0 && event.DeltaX == 0 {
		return platform.Frame{}, nil
	}
	target := gfx.ID{}
	if hit, ok := painter.focus.Router().HitTest(event.X, event.Y); ok {
		target = hit
	}
	response := gfx.EventResult{}
	if painter.onScroll != nil {
		response = painter.onScroll(gfx.ScrollEvent{
			Modifiers: gfx.Modifiers(event.Modifiers), X: event.X, Y: event.Y,
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
	return painter.repaintAfterInput()
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

func (painter *painter) applyEventResult(result gfx.EventResult) bool {
	if result.ClipboardText != "" && painter.setClipboardText != nil {
		painter.setClipboardText(result.ClipboardText)
	}
	changed := painter.focus.Apply(result)
	if !result.Activate.IsZero() && painter.deliverClick(result.Activate, gfx.ClickInfo{}) {
		changed = true // the click's pass changed the application: show it
	}
	return changed
}

// canClick reports whether a click on a control has anywhere to go: whether the
// last paint asked Clicked.
func (painter *painter) canClick() bool {
	return painter.paint != nil && painter.canvas.ListensForClicks()
}

// deliverClick carries out a completed click on id and reports whether anything
// received it. When the last paint asked gfx.Canvas.Clicked about anything, the
// paint runs once more as the event pass of this click, the one pass in which
// Clicked(id) is true and the application may change state. That pass's drawing
// is discarded: the repaint the caller asks for next is the one that shows the
// result. A paint that never asks costs nothing here.
func (painter *painter) deliverClick(id gfx.ID, info gfx.ClickInfo) bool {
	if !painter.canClick() {
		return false
	}
	painter.canvas.BeginEventPass(id, info)
	painter.paintCanvas()
	painter.canvas.EndEventPass()
	painter.canvas.RunAfterPaint()
	painter.focus.Carry(painter.canvas)
	return true
}

// pointerClickInfo is what a pointer event says of the click it delivers: its
// modifier keys and whether it was a double click.
func pointerClickInfo(event platform.PointerEvent) gfx.ClickInfo {
	return gfx.ClickInfo{Modifiers: gfx.Modifiers(event.Modifiers), Clicks: event.ClickCount}
}

// pointerIsDelivered says which pointer events the event pass offers to controls:
// presses and releases aimed at a control, and the moves of a press that is held.
// A move over a control is hover, which a control reads from Canvas.Hovered.
func pointerIsDelivered(kind platform.PointerEventKind, target gfx.ID, captured bool) bool {
	switch kind {
	case platform.PointerDown, platform.PointerUp:
		return !target.IsZero()
	case platform.PointerMove:
		return captured
	}
	return false
}

// deliverPointer runs the paint once more as the event pass of a pointer event when
// the last paint asked gfx.Canvas.PointerOn or PointerIn, and reports whether a
// control consumed the event. Only presses, releases and the moves of a held press
// are offered: hovering is read from Hovered, not delivered. A paint that never
// asks costs nothing here.
func (painter *painter) deliverPointer(event gfx.PointerEvent) bool {
	if painter.paint == nil || !painter.canvas.ListensForPointer() {
		return false
	}
	painter.canvas.BeginPointerPass(event)
	painter.paintCanvas()
	consumed := painter.canvas.EndPointerPass()
	painter.canvas.RunAfterPaint()
	painter.focus.Carry(painter.canvas)
	return consumed
}

// repaint reruns the paint callback with current interaction state and
// synchronizes the result with the retained renderer.
func (painter *painter) repaint() (platform.Frame, error) {
	if painter.noteFrameStart != nil {
		painter.noteFrameStart()
	}
	painter.paintCanvas()
	if !painter.focus.PendingFocus().IsZero() || painter.focus.Carries() {
		before, beforeVisible := painter.focus.Focused(), painter.focus.KeyboardFocus()
		painter.focus.Sync(painter.canvas)
		if painter.focus.Focused() != before || painter.focus.KeyboardFocus() != beforeVisible {
			painter.paintCanvas()
		}
	}
	if painter.paint != nil && painter.canvas.TrappedFocusScope() != painter.focus.Router().ActiveScope() {
		// A modal just opened or closed, which moves focus into it or back
		// out. Settle that now and paint once more so this frame already
		// shows the new focus instead of lagging a repaint behind.
		before := painter.focus.Focused()
		painter.focus.Sync(painter.canvas)
		if painter.focus.Focused() != before {
			painter.paintCanvas()
		}
	}
	frame, err := painter.publish()
	if err != nil {
		return platform.Frame{}, err
	}
	if painter.keys.TakeDue() && painter.releaseKeys() && painter.scheduleFrame != nil {
		painter.scheduleFrame(0)
	}
	painter.scheduleIfAnimating()
	return frame, nil
}

// repaintAfterInput is repaint for a pointer or wheel event: a window that
// coalesces input asks its frame clock for the next frame instead, so a burst
// of events costs one paint and lands on the display's own cadence.
func (painter *painter) repaintAfterInput() (platform.Frame, error) {
	if painter.coalesceInput && painter.scheduleFrame != nil {
		painter.scheduleFrame(0)
		return platform.Frame{}, nil
	}
	return painter.repaint()
}

func (painter *painter) paintCanvas() {
	if painter.paint == nil {
		return
	}
	painter.canvas.Reset()
	if painter.scale > 0 {
		painter.canvas.SetPixelScale(painter.scale)
	}
	focused, visible := painter.focus.PaintTarget()
	painter.canvas.SetInteraction(painter.pointer.Hovered(), painter.pointer.Pressed(), focused, visible)
	painter.canvas.SetHoveredRole(painter.pointer.HoveredRole())
	painter.canvas.SetPointerOnSurface(painter.onSurface)
	painter.canvas.SetScroll(painter.scrollX, painter.scrollY)
	painter.canvas.BeginFrame()
	painter.paint(painter.canvas, painter.size)
	if painter.controls != nil && !painter.fullscreen {
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
	frame, err := painter.frame.Build()
	if err != nil {
		return platform.Frame{}, fmt.Errorf("app: render state is inconsistent: %w", err)
	}

	if views, changed := painter.syncNativeViews(); changed {
		frame.NativeViews = views
	}

	// A paint that asks to take the focus away changes routing without changing a hit.
	// The regions the router keeps are built from a subset of the hit fields: a label,
	// value or description change feeds the semantic payload but leaves every region
	// as it was, so it must not rebuild them.
	hitStructuresUnchanged := painter.hitStructuresUnchanged()
	regionsUnchanged := (hitStructuresUnchanged || painter.regionStructuresUnchanged()) &&
		painter.canvas.FocusRelease().IsZero() && !painter.focus.Carries()
	if regionsUnchanged {
		painter.reusedHits++
	} else {
		painter.focus.Sync(painter.canvas)
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
	structuresUnchanged := painter.structuresUnchanged()
	if !hitStructuresUnchanged || !structuresUnchanged {
		painter.syncSemantics(!hitStructuresUnchanged && structuresUnchanged)
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

// syncNativeViews works out where the native views are this frame, inside their clips, and
// reports them when they differ from the ones the platform has. The slice is never nil, so a frame
// that took the last view away is told from one that says nothing.
func (painter *painter) syncNativeViews() ([]platform.NativeView, bool) {
	painter.views = painter.views[:0]
	painter.covers = painter.covers[:0]
	for _, view := range painter.canvas.NativeViews() {
		bounds := view.Bounds
		if view.HasClip {
			left, top := max(bounds.X, view.ClipOf.X), max(bounds.Y, view.ClipOf.Y)
			right := min(bounds.X+bounds.Width, view.ClipOf.X+view.ClipOf.Width)
			bottom := min(bounds.Y+bounds.Height, view.ClipOf.Y+view.ClipOf.Height)
			if right <= left || bottom <= top {
				continue
			}
			bounds = gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
		}
		native := platform.NativeView{ID: uint32(view.ID), Bounds: bounds, Layer: painter.frame.PlatformLayer(view.Layer)}
		start := len(painter.covers)
		painter.covers = painter.coversOf(view.Layer, bounds, painter.covers)
		native.Covers = painter.covers[start:len(painter.covers):len(painter.covers)]
		painter.views = append(painter.views, native)
	}
	if slices.EqualFunc(painter.views, painter.lastViews, sameNativeView) {
		return nil, false
	}
	if painter.views == nil {
		painter.views = []platform.NativeView{}
	}
	painter.lastViews = painter.lastViews[:0]
	for _, view := range painter.views {
		view.Covers = slices.Clone(view.Covers)
		painter.lastViews = append(painter.lastViews, view)
	}
	return painter.views, true
}

// coversOf lists the hit regions on layers above a view that lie over it, cut to it.
func (painter *painter) coversOf(layer uint16, bounds gfx.Rect, into []gfx.Rect) []gfx.Rect {
	for _, hit := range painter.canvas.Hits() {
		if hit.Layer <= layer {
			continue
		}
		left, top := max(bounds.X, hit.Bounds.X), max(bounds.Y, hit.Bounds.Y)
		right := min(bounds.X+bounds.Width, hit.Bounds.X+hit.Bounds.Width)
		bottom := min(bounds.Y+bounds.Height, hit.Bounds.Y+hit.Bounds.Height)
		if right > left && bottom > top {
			into = append(into, gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top})
		}
	}
	return into
}

func sameNativeView(a, b platform.NativeView) bool {
	return a.ID == b.ID && a.Bounds == b.Bounds && a.Layer == b.Layer && slices.Equal(a.Covers, b.Covers)
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
	painter.frame.Trim()
	const trimAt, trimRatio = 4096, 4
	if slots := painter.semantics.Slots(); slots > trimAt && painter.semantics.Len()*trimRatio < slots {
		painter.semantics = semantic.NewWithCapacity("Viva", len(painter.hitScratch)+1)
		painter.semanticIDs = painter.semanticIDs[:0]
		painter.semanticChanges = painter.semanticChanges[:0]
		painter.semanticsDirty = true
	}
	painter.canvas.Trim()
	painter.focus.Trim()
	painter.hitScratch = framebuild.TrimCapacity(painter.hitScratch)
	painter.publishedHits = framebuild.TrimCapacity(painter.publishedHits)
	painter.semanticDirty = framebuild.TrimCapacity(painter.semanticDirty)
	painter.publishedReturns = framebuild.TrimCapacity(painter.publishedReturns)
	painter.accessibility = framebuild.TrimCapacity(painter.accessibility)
	painter.lastAccess = framebuild.TrimCapacity(painter.lastAccess)
	painter.semanticChanges = framebuild.TrimCapacity(painter.semanticChanges)
	painter.semanticIDs = framebuild.TrimCapacity(painter.semanticIDs)
	if painter.labelNamePeak > 4096 && len(painter.labelNames)*4 < painter.labelNamePeak {
		painter.labelNames = nil
		painter.labelNamePeak = 0
	}
}

func (painter *painter) syncCursor() error {
	next := CursorArrow
	var image *gfx.CursorImage
	if painter.edge != platform.EdgeNone && painter.pointer.PressTarget().IsZero() {
		return painter.applyCursor(cursorForEdge(painter.edge), nil)
	}
	if cursor, picture := interaction.CursorForTarget(painter.canvas.Hits(), painter.pointer.Hovered(), painter.pointer.PressTarget()); cursor != gfx.PointerCursorDefault || picture != nil {
		next, image = cursorFromHit(cursor), picture
	}
	return painter.applyCursor(next, image)
}

// applyCursor makes next the pointer, with image as its picture when it is CursorImage. A picture
// is one pointer by identity, so the same value is not set again.
func (painter *painter) applyCursor(next Cursor, image *gfx.CursorImage) error {
	if next == painter.cursor && image == painter.cursorImage {
		return nil
	}
	if next == CursorImage {
		if painter.setCursorImage != nil {
			if err := painter.setCursorImage(image); err != nil {
				if !errors.Is(err, ErrUnsupported) {
					return err
				}
				// A target that cannot show the picture keeps the arrow; the picture is not
				// asked for again until it changes.
				if painter.setCursor != nil {
					if err := painter.setCursor(CursorArrow); err != nil {
						return err
					}
				}
			}
		}
	} else if painter.setCursor != nil {
		if err := painter.setCursor(next); err != nil {
			return err
		}
	}
	painter.cursor, painter.cursorImage = next, image
	return nil
}

func cursorFromHit(cursor gfx.PointerCursor) Cursor {
	switch cursor {
	case gfx.PointerCursorText:
		return CursorText
	case gfx.PointerCursorHand:
		return CursorHand
	case gfx.PointerCursorCrosshair:
		return CursorCrosshair
	case gfx.PointerCursorWait:
		return CursorWait
	case gfx.PointerCursorResizeHorizontal:
		return CursorResizeHorizontal
	case gfx.PointerCursorResizeVertical:
		return CursorResizeVertical
	case gfx.PointerCursorResizeNWSE:
		return CursorResizeNWSE
	case gfx.PointerCursorResizeNESW:
		return CursorResizeNESW
	case gfx.PointerCursorNotAllowed:
		return CursorNotAllowed
	case gfx.PointerCursorGrabbing:
		return CursorGrabbing
	case gfx.PointerCursorHidden:
		return CursorHidden
	case gfx.PointerCursorImage:
		return CursorImage
	default:
		return CursorArrow
	}
}

// focusedTextField describes the focused text field of the last paint, and how
// it places its caret when it lays its rows out itself.
func (painter *painter) focusedTextField() (TextInputContext, gfx.TextPlacement, int) {
	hit, index, ok := interaction.TextInputHit(painter.canvas, painter.focus.Focused())
	if !ok {
		return TextInputContext{}, nil, -1
	}
	text, _ := painter.canvas.HitText(&hit)
	context := TextInputContext{
		ID: hit.ID, Bounds: hit.Bounds, Layer: hit.Layer, Terminal: hit.Role == gfx.RoleTerminal, TypeAhead: hit.TypeAhead && hit.Role != gfx.RoleTerminal,
		Multiline: hit.Multiline, ReadOnly: hit.ReadOnly, Secure: hit.Secure,
		NavigationKeys: hit.NavigationKeys,
		TextBounds:     text.Bounds, FontSize: text.Size, FontWeight: text.Weight, TextColor: text.Color, Alignment: text.Alignment,
		HideNativeOverlays: hit.HideNativeOverlays,
	}
	return context, text.Placement, index
}

func (painter *painter) syncTextInput() platform.TextInputUpdate {
	if painter.textInput == nil {
		return painter.disableTextInput()
	}
	context, placement, hitIndex := painter.focusedTextField()
	previousEditor := painter.activeEditor
	var input *TextInput
	if !context.ID.IsZero() && !context.ReadOnly {
		input = painter.textInput(context)
	}
	if input == nil {
		placement = nil
	}
	var nextEditor *text.Editor
	if input != nil {
		nextEditor = input.Editor
	}
	handoff := previousEditor != nil && painter.textEnabled &&
		(previousEditor != nextEditor || painter.nativeText.Target != context.ID)
	if handoff {
		painter.settleTextInput()
	}
	if input != nil && input.Editor != nil {
		input.State = TextStateOf(input.Editor)
	}
	painter.noteKeyboardTarget(context.ID, input != nil && context.Terminal && input.KeyReleases)
	if input == nil {
		return painter.disableTextInput()
	}
	// What the application leaves unset comes from the component's declaration.
	bounds, fontSize, weight, tint := input.Bounds, input.FontSize, input.Weight, input.Color
	alignment := input.Alignment
	if bounds == (gfx.Rect{}) {
		bounds = context.TextBounds
		if bounds.Width <= 0 || bounds.Height <= 0 {
			bounds = context.Bounds
		}
	}
	if transformed, moving := painter.canvas.HitPresenceRect(hitIndex, bounds); moving {
		bounds = framebuild.RoundPresenceRect(transformed)
	}
	if fontSize == 0 {
		fontSize = context.FontSize
	}
	if weight == 0 {
		weight = context.FontWeight
	}
	if tint == (gfx.Color{}) {
		tint = context.TextColor
	}
	if alignment == gfx.AlignLeft {
		alignment = context.Alignment
	}
	if !(fontSize > 0) || math.IsInf(float64(fontSize), 0) {
		// Every platform refuses an editor without a font size. What neither the
		// application nor the declaration set is the body text's.
		fontSize = gfx.DefaultMetrics().FontSizeM
	}
	if bounds.Width <= 0 || bounds.Height <= 0 ||
		framebuild.RoundPixel(bounds.Width) <= 0 || framebuild.RoundPixel(bounds.Height) <= 0 {
		return painter.disableTextInput()
	}
	var cellWidth, baseline float32
	if context.Terminal || context.TypeAhead {
		cellWidth, _ = painter.canvas.MonoCellSize(fontSize, input.MonoFace)
		baseline = painter.canvas.MonoBaseline(fontSize, input.MonoFace, bounds.Height)
	}
	if context.Terminal || context.TypeAhead || placement != nil {
		// Terminal and self-placed inputs keep the fixed origin they had before
		// alignment existed.
		alignment = gfx.AlignLeft
	} else {
		alignment = resolveTextInputAlignment(alignment, input.State.Value, input.State.Selection.End)
	}
	native := platform.TextInput{
		Target: context.ID,
		X:      bounds.X, Y: bounds.Y,
		Width: bounds.Width, Height: bounds.Height,
		FontSize: fontSize, Weight: weight, FontGeneration: painter.canvas.FontGeneration(),
		Alignment: alignment, Layer: painter.frame.PlatformLayer(context.Layer), Multiline: context.Multiline, Secure: context.Secure, Terminal: context.Terminal || context.TypeAhead,
		AltSendsEscape: input.AltSendsEscape, KeyReleases: context.Terminal && input.KeyReleases, DisableIME: input.DisableIME, MonoFace: input.MonoFace, CellWidth: cellWidth, Baseline: baseline,
		NavigationKeys:     input.NavigationKeys || context.NavigationKeys,
		CopyOverride:       input.CopyOverride,
		CopyRectangular:    input.CopyRectangular,
		CopyText:           painter.clipboardText,
		HideNativeOverlays: input.HideNativeOverlays || context.HideNativeOverlays,
		Color: platform.Color{
			Red: float64(tint.Red), Green: float64(tint.Green),
			Blue: float64(tint.Blue), Alpha: float64(tint.Alpha),
		},
		State:   platformTextState(input.State).WithoutRedraw(),
		OnEvent: painter.onTextEvent,
	}
	if placement != nil {
		native.Placed = true
		if caret, ok := placement.CaretBounds(); ok {
			if transformed, moving := painter.canvas.HitPresenceRect(hitIndex, caret); moving {
				caret = framebuild.RoundPresenceRect(transformed)
			}
			native.CaretX, native.CaretY, native.CaretWidth, native.CaretHeight = caret.X, caret.Y, caret.Width, caret.Height
		}
	}
	painter.activeText = input
	painter.activeEditor = nextEditor
	if !painter.textEnabled || previousEditor != nextEditor || painter.nativeText.Target != native.Target ||
		painter.nativeText.Multiline != native.Multiline || painter.nativeText.Secure != native.Secure ||
		painter.nativeText.Terminal != native.Terminal || painter.nativeText.Placed != native.Placed ||
		painter.nativeText.DisableIME != native.DisableIME || painter.nativeText.State != native.State {
		painter.textEpoch++
	}
	if painter.textEnabled && samePlatformTextInput(painter.nativeText, native) {
		return platform.TextInputUpdate{}
	}
	painter.nativeText = native
	painter.textEnabled = true
	return platform.TextInputUpdate{Changed: true, Input: &painter.nativeText}
}

// settleTextInput ends the installed owner's composition before a handoff or
// disable. Documents keep the visible text; terminal preedit was never sent
// to the application and is discarded. State/OnEvent providers own settlement.
func (painter *painter) settleTextInput() {
	editor := painter.activeEditor
	painter.activeEditor = nil
	if editor == nil || !painter.textEnabled {
		return
	}
	if editor.TentativeCompositionActive() {
		phase := text.CompositionCommit
		if painter.nativeText.Terminal {
			phase = text.CompositionCancel
		}
		editor.ApplyComposition(text.CompositionEvent{Phase: phase})
		if phase == text.CompositionCommit && painter.nativeText.Secure {
			editor.ClearHistory()
		}
	}
	editor.BreakUndoGroup()
}

func (painter *painter) disableTextInput() platform.TextInputUpdate {
	painter.settleTextInput()
	painter.noteKeyboardTarget(gfx.ID{}, false)
	painter.activeText = nil
	if !painter.textEnabled {
		return platform.TextInputUpdate{}
	}
	painter.textEpoch++
	painter.textEnabled = false
	return platform.TextInputUpdate{Changed: true}
}

func (painter *painter) applyTextEvent(event platform.TextEvent) (platform.TextState, error) {
	painter.tap.Cancel()
	active := painter.activeText
	// The installed owner is the editor the platform has been told about, not
	// whatever the last published TextInput names: a cached provider can hand
	// back a different Editor before the next sync. Keep activeText for the
	// provider's own state and CopyText.
	editor := painter.activeEditor
	if editor == nil && active != nil {
		editor = active.Editor
	}
	if active == nil || editor == nil && active.OnEvent == nil {
		return platform.TextState{}, fmt.Errorf("app: native text event without an active handler")
	}
	epoch := painter.textEpoch
	appEvent := appTextEvent(event)
	explicit := event.CompositionPhase != text.CompositionNone
	if explicit {
		if editor != nil {
			explicit = editor.TentativeCompositionEnabled()
		} else {
			explicit = active.State.TentativeComposition
		}
	}
	if !painter.nativeText.Multiline && !painter.nativeText.Terminal &&
		(event.Kind == platform.TextPaste || event.Kind == platform.TextInsert) && event.Text != "" {
		appEvent.Text = text.SingleLine(event.Text)
		if appEvent.Text != event.Text {
			// Folding removed the line structure, so the column shape the
			// clipboard declared no longer describes the text the app receives.
			appEvent.Rectangular = false
		}
		if appEvent.Text == "" && !explicit {
			return painter.nativeText.State, nil
		}
	}
	if !explicit && !painter.nativeText.Multiline && !painter.nativeText.Terminal &&
		event.Kind == platform.TextCommandEvent &&
		(event.Command == platform.TextInsertNewline || event.Command == platform.TextInsertLineBreak) {
		// A single-line field holds no line break. While an input method is
		// composing, AppKit answers Return with insertNewline: even though the
		// field has one line, and the editor would otherwise take the newline.
		return painter.nativeText.State, nil
	}
	var next TextState
	if editor != nil {
		accepted := appEvent.Apply(editor)
		if painter.nativeText.Secure && accepted && !editor.TentativeCompositionActive() {
			editor.ClearHistory()
		}
		next = TextStateOf(editor)
	} else {
		var err error
		if next, err = active.OnEvent(appEvent); err != nil {
			return platform.TextState{}, err
		}
	}
	if painter.textEpoch != epoch {
		// Nested input or provider replacement has already installed newer state.
		if painter.activeText == nil {
			return platform.TextState{Redraw: true}, nil
		}
		native := painter.nativeText.State
		native.Redraw = true
		return native, nil
	}
	painter.textEpoch++
	painter.activeText.State = next
	native := platformTextState(next)
	// What the platform mirrors back never carries the one-shot request.
	painter.nativeText.State = native.WithoutRedraw()
	painter.activeText.State.Redraw = false
	return native, nil
}

func samePlatformTextInput(left, right platform.TextInput) bool {
	return left.Target == right.Target && left.X == right.X && left.Y == right.Y && left.Width == right.Width && left.Height == right.Height &&
		left.FontSize == right.FontSize && left.Weight == right.Weight && left.FontGeneration == right.FontGeneration &&
		left.Alignment == right.Alignment && left.Layer == right.Layer && left.Multiline == right.Multiline && left.Secure == right.Secure && left.Terminal == right.Terminal && left.MonoFace == right.MonoFace && left.CellWidth == right.CellWidth && left.Baseline == right.Baseline &&
		left.AltSendsEscape == right.AltSendsEscape && left.KeyReleases == right.KeyReleases && left.DisableIME == right.DisableIME &&
		left.NavigationKeys == right.NavigationKeys &&
		left.CopyOverride == right.CopyOverride && left.CopyRectangular == right.CopyRectangular &&
		left.HideNativeOverlays == right.HideNativeOverlays &&
		left.Placed == right.Placed && left.CaretX == right.CaretX && left.CaretY == right.CaretY &&
		left.CaretWidth == right.CaretWidth && left.CaretHeight == right.CaretHeight &&
		left.Color == right.Color && left.State == right.State
}

// hitStructuresUnchanged reports whether the paint produced exactly the input
// the semantic tree and the accessibility payload were last built from, so both
// can be kept. It compares every field those structures read, plus the focus,
// scope, escape and text-selection state they carry outside the hit list. A page
// that only changes colors, or does not change at all, therefore costs no
// per-hit bookkeeping. Regions are gated separately by sameRegionHit, because a
// changed label rebuilds the payload without moving a region.
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

// regionStructuresUnchanged reports whether the paint produced the same input
// the input router's regions are built from. Focus.Sync copies a subset of each
// hit (see internal/interaction/focus.go); the accessible name, value, hint and
// description are not in it. A repaint that only changes those words must not
// rebuild every region on a large page, so this check compares that subset
// instead of the whole hit list.
func (painter *painter) regionStructuresUnchanged() bool {
	if !painter.hitsPublished || !painter.focus.PendingFocus().IsZero() {
		return false
	}
	if painter.focus.Focused() != painter.publishedFocus ||
		painter.canvas.TrappedFocusScope() != painter.publishedTrap ||
		painter.canvas.InitialFocus() != painter.publishedInitial ||
		painter.canvas.EscapeTarget() != painter.publishedEscape ||
		painter.canvas.FocusRequest() != painter.publishedRequest ||
		!slices.Equal(painter.canvas.FocusReturns(), painter.publishedReturns) {
		return false
	}
	hits := painter.canvas.Hits()
	if len(hits) != len(painter.publishedHits) {
		return false
	}
	for index := range hits {
		if !sameRegionHit(&painter.publishedHits[index], &hits[index]) {
			return false
		}
	}
	return true
}

// sameRegionHit compares the hit fields Focus.Sync copies into an input.Region.
// It reads the hits by pointer: a hit is large, and a page has thousands of them.
func sameRegionHit(published, current *gfx.Hit) bool {
	return published.ID == current.ID &&
		published.Bounds == current.Bounds &&
		published.Scope == current.Scope &&
		published.Layer == current.Layer &&
		published.Role == current.Role &&
		published.Focusable == current.Focusable &&
		published.Disabled == current.Disabled &&
		published.FocusableDisabled == current.FocusableDisabled &&
		published.SecondaryWhenDisabled == current.SecondaryWhenDisabled &&
		published.SkipTab == current.SkipTab &&
		published.Block == current.Block &&
		published.BlockSurface == current.BlockSurface &&
		published.Container == current.Container &&
		published.IsLabel == current.IsLabel &&
		published.Backdrop == current.Backdrop &&
		published.MovesWindow == current.MovesWindow &&
		published.PassSecondary == current.PassSecondary &&
		published.PassPrimary == current.PassPrimary &&
		published.Autofocus == current.Autofocus
}

// structuresUnchanged is hitStructuresUnchanged without the hit comparison: the
// focus, trap, escape and text state that also feed the semantic tree. The
// publish uses it for the second check, after syncTextInput has had its say;
// the hits cannot change in between, so the expensive memcmp runs once.
func (painter *painter) structuresUnchanged() bool {
	if !painter.hitsPublished || painter.semanticsDirty || !painter.focus.PendingFocus().IsZero() {
		return false
	}
	if painter.focus.Focused() != painter.publishedFocus ||
		painter.canvas.TrappedFocusScope() != painter.publishedTrap ||
		painter.canvas.InitialFocus() != painter.publishedInitial ||
		painter.canvas.EscapeTarget() != painter.publishedEscape ||
		painter.canvas.FocusRequest() != painter.publishedRequest ||
		!slices.Equal(painter.canvas.FocusReturns(), painter.publishedReturns) {
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

// sameHits reports whether two hit lists are identical in the fields the
// semantic tree and the accessibility payload read. The text-editor geometry
// lives beside the hits (Canvas.HitText) and feeds neither, so it is not part of
// the comparison. The structs are compared as bytes: Go strings are immutable,
// and the published list keeps the previous string allocations reachable, so a
// matching pointer cannot name different bytes. Padding, and the index into the
// text table, can only make the comparison stricter (an unnecessary publish),
// never more permissive.
func sameHits(published, current []gfx.Hit) bool {
	if len(published) != len(current) {
		return false
	}
	if len(published) == 0 {
		return true
	}
	size := int(unsafe.Sizeof(gfx.Hit{}))
	left := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(published))), len(published)*size)
	right := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(current))), len(current)*size)
	return bytes.Equal(left, right)
}

// recordHitStructures keeps the state that lets the next repaint skip the
// region and semantic mirror when nothing that feeds them changed.
func (painter *painter) recordHitStructures() {
	painter.publishedHits = append(painter.publishedHits[:0], painter.canvas.Hits()...)
	painter.publishedFocus = painter.focus.Focused()
	painter.publishedTrap = painter.canvas.TrappedFocusScope()
	painter.publishedInitial = painter.canvas.InitialFocus()
	painter.publishedEscape = painter.canvas.EscapeTarget()
	painter.publishedRequest = painter.canvas.FocusRequest()
	painter.publishedReturns = append(painter.publishedReturns[:0], painter.canvas.FocusReturns()...)
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
func (painter *painter) syncSemantics(incremental bool) {
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
	// incremental is set when only the hit list changed: the focus, text state
	// and view are the same, so a control whose hit and label are unchanged
	// keeps its accessibility node. Compare the raw hits first and remember the
	// few ids whose entry may have moved; a structural change, a label-map
	// fallback or too many dirty ids falls back to the full mirror below.
	fast := incremental && len(canvasHits) == len(painter.publishedHits)
	dirty := painter.semanticDirty[:0]
	if fast {
		for index := range canvasHits {
			current, previous := &canvasHits[index], &painter.publishedHits[index]
			if current.ID != previous.ID || current.IsLabel != previous.IsLabel ||
				current.Block != previous.Block || current.MovesWindow != previous.MovesWindow ||
				current.Backdrop != previous.Backdrop {
				fast = false
				break
			}
			if *current != *previous && !current.ID.IsZero() {
				known := false
				for _, id := range dirty {
					if id == current.ID {
						known = true
						break
					}
				}
				if !known {
					dirty = append(dirty, current.ID)
					if len(dirty) > 64 {
						fast = false
						break
					}
				}
			}
		}
	}
	painter.semanticDirty = dirty
	for index := range canvasHits {
		hit := &canvasHits[index]
		switch {
		case hit.Block, hit.MovesWindow:
		case hit.IsLabel:
			labelled = true
			switch {
			case hit.ID.IsZero() || hit.Label == "":
				// nameFromLabels ignores an anonymous label, and one without
				// words, too: the control keeps its own name.
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

	if fast && labelled && (needsLabelMap || pendingCount > 0) {
		fast = false
	}
	if fast && len(hits) != len(painter.semanticIDs) {
		fast = false
	}
	if fast {
		focused := painter.focus.Focused()
		previous := painter.accessibility
		for index := range hits {
			dirtyHit := false
			for _, id := range dirty {
				if id == hits[index].ID {
					dirtyHit = true
					break
				}
			}
			if !dirtyHit {
				continue
			}
			node := painter.accessibilityNode(&hits[index], focused)
			if index < len(previous) && previous[index] == node {
				continue
			}
			painter.accessibility[index] = node
			painter.semantics.Set(painter.semanticIDs[index], semanticNode(hits[index], focused))
		}
		painter.semanticChanges = painter.semantics.DrainChanges(painter.semanticChanges[:0])
		return
	}
	common := len(painter.semanticIDs)
	if len(hits) < common {
		common = len(hits)
	}
	// The accessible payload is compared entry by entry: a repaint that changes
	// one label must not rewrite the semantic tree or a thousand identical
	// accessibility nodes. The retained payload is also the previous entry's
	// copy, so an equal node means neither structure has anything to update.
	focused := painter.focus.Focused()
	previous := painter.accessibility
	if len(hits) < len(previous) {
		clear(previous[len(hits):])
	}
	if cap(previous) >= len(hits) {
		painter.accessibility = previous[:len(hits)]
	} else {
		painter.accessibility = make([]platform.AccessibilityNode, len(hits))
		copy(painter.accessibility, previous)
	}
	for index := 0; index < common; index++ {
		node := painter.accessibilityNode(&hits[index], focused)
		if index < len(previous) && previous[index] == node {
			continue
		}
		painter.accessibility[index] = node
		painter.semantics.Set(painter.semanticIDs[index], semanticNode(hits[index], focused))
	}
	for index := common; index < len(hits); index++ {
		node := painter.accessibilityNode(&hits[index], focused)
		painter.accessibility[index] = node
		id, _ := painter.semantics.Add(root, semanticNode(hits[index], focused))
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
}

// accessibilityNode is the payload entry for one hit.
func (painter *painter) accessibilityNode(hit *gfx.Hit, focused gfx.ID) platform.AccessibilityNode {
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
		Focused:     !hit.ID.IsZero() && hit.ID == focused,
		Disabled:    hit.Disabled,
		Checked:     hit.Checked,
		Mixed:       hit.Mixed,
		Selected:    hit.Selected,
		Expanded:    hit.Expanded,
		Multiline:   hit.Multiline,
		ReadOnly:    hit.ReadOnly,
		Invalid:     hit.Invalid,
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
		(hit.Role == gfx.RoleTextField || hit.Role == gfx.RoleCombobox) && painter.activeText.State.Value == hit.Value {
		selection := painter.activeText.State.Selection
		node.Selection = platform.AccessibilitySelection{Start: int32(selection.Start), End: int32(selection.End), Valid: true}
	}
	return node
}

// labelPair is a label hit waiting for the control it names.
type labelPair struct {
	id    gfx.ID
	words string
}

// nameFromLabels gives each control that has a label hit the words of the first
// one as its accessible name, as `<label for>` names an input. A label without
// words names nothing: the control keeps its own name, as an empty label falls
// through in HTML. It builds the label map once per paint instead of scanning
// every hit for every control: a page with many labelled fields used to cost
// O(controls*hits), which made a 4,000-field page take tens of milliseconds
// per repaint.
func (painter *painter) nameFromLabels(controls []gfx.Hit) {
	all := painter.canvas.Hits()
	if painter.labelNames == nil {
		painter.labelNames = make(map[gfx.ID]string, 8)
	}
	clear(painter.labelNames)
	for index := range all {
		label := &all[index]
		if !label.IsLabel || label.ID.IsZero() || label.Label == "" {
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

func semanticNode(hit gfx.Hit, focused gfx.ID) semantic.Node {
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
		Invalid:   hit.Invalid,
		Secure:    hit.Secure,
	}
}

func semanticRole(role gfx.Role) semantic.Role {
	switch role {
	case gfx.RoleLink:
		return semantic.RoleLink
	case gfx.RoleCheckbox:
		return semantic.RoleCheckbox
	case gfx.RoleRadio:
		return semantic.RoleRadio
	case gfx.RoleTab:
		return semantic.RoleTab
	case gfx.RoleMenuItem:
		return semantic.RoleMenuItem
	case gfx.RoleCombobox:
		return semantic.RoleCombobox
	case gfx.RoleSwitch:
		return semantic.RoleSwitch
	case gfx.RoleTextField:
		return semantic.RoleTextField
	case gfx.RoleSlider:
		return semantic.RoleSlider
	case gfx.RoleTerminal:
		return semantic.RoleTextField
	case gfx.RoleGroup:
		return semantic.RoleGroup
	case gfx.RoleMenu:
		return semantic.RoleMenu
	case gfx.RoleMenubar:
		return semantic.RoleMenubar
	case gfx.RoleListBox:
		return semantic.RoleListBox
	case gfx.RoleOption:
		return semantic.RoleOption
	case gfx.RoleGrid:
		return semantic.RoleGrid
	case gfx.RoleGridCell:
		return semantic.RoleGridCell
	case gfx.RoleHeading:
		return semantic.RoleHeading
	case gfx.RoleStatus:
		return semantic.RoleStatus
	case gfx.RoleSeparator:
		return semantic.RoleSeparator
	case gfx.RoleProgressBar:
		return semantic.RoleProgress
	default:
		return semantic.RoleButton
	}
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

// windowActions makes the set of operations the window buttons need, for the
// platform adapter to fill, or nil when the system draws the buttons.
func (painter *painter) windowActions() *platform.WindowActions {
	if painter.controls == nil {
		return nil
	}
	painter.controls.actions = &platform.WindowActions{}
	return painter.controls.actions
}
