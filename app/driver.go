//go:build darwin || windows || linux

package app

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// ErrNoTextInput is returned by Driver.Type and the editing shortcuts when no
// text field has keyboard focus.
var ErrNoTextInput = errors.New("app: no focused text field")

// ErrNoTerminal is returned by Driver.Chord when no terminal has the keyboard.
var ErrNoTerminal = errors.New("app: no focused terminal")

// Driver runs an application without a window: the same painter, input router,
// focus rules and text-input path a native window drives, fed by explicit
// calls instead of operating-system events, with an animation clock that only
// moves when told to. It exists so tools and tests can operate an application
// and look at it without opening anything on the user's screen.
//
// Output is a Snapshot: the retained primitives, labels, native-editor overlay,
// hit regions and focus. Turning a Snapshot into pixels is the job of the
// platform's offscreen renderer (see package headless).
//
// A Driver is not safe for concurrent use. Background goroutines reach it only
// through Config.Invalidator, exactly as with a window; the wake is applied the
// next time the driver is called (or by Pump).
type Driver struct {
	config  Config
	canvas  *ui.Canvas
	painter *painter

	// origin is where Config.Origin put the content area; MoveWindow changes it
	// as a window manager would.
	origin    ui.Point
	minimized bool
	maximized bool

	now       time.Duration
	animating bool
	frameDue  time.Duration // virtual time the next requested frame is due
	wake      atomic.Bool
	wakeSent  chan struct{}

	// modifierKeys is the modifier keys KeyDown holds, one bit per
	// ui.KeyShiftLeft..ui.KeyMetaRight.
	modifierKeys uint8

	clipboard      string
	openedURL      string
	hasOpenedURL   bool
	openedPath     string
	hasOpenedPath  bool
	releases       []func()
	closed         bool
	appearanceOpen bool

	system                   driverSystem
	openFilesResponses       [driverOpenFilesQueueSize]driverOpenFilesResponse
	openFilesHead            int
	openFilesCount           int
	lastOpenFilesRequest     OpenFileOptions
	hasOpenFilesRequest      bool
	openDirectoryResponses   [driverOpenDirectoryQueueSize]driverOpenDirectoryResponse
	openDirectoryHead        int
	openDirectoryCount       int
	lastOpenDirectoryRequest OpenDirectoryOptions
	hasOpenDirectoryRequest  bool
	saveFileResponses        [driverSaveFileQueueSize]driverSaveFileResponse
	saveFileHead             int
	saveFileCount            int
	lastSaveFileRequest      SaveFileOptions
	hasSaveFileRequest       bool
}

// NewDriver creates a driver for config, painting into canvas (a fresh canvas
// when nil). The first frame has been painted when it returns.
func NewDriver(config Config, canvas *ui.Canvas) (*Driver, error) {
	normalized, err := config.normalize()
	if err != nil {
		return nil, err
	}
	if canvas == nil {
		canvas = &ui.Canvas{}
	}
	driver := &Driver{config: normalized, canvas: canvas, wakeSent: make(chan struct{}, 1),
		origin: normalized.originPoint()}

	liveDrivers.Acquire()
	// Motion preferences are the user's, not the test's: run animations.
	canvas.SetReduceMotion(false)

	driver.painter, err = newPainterWithClock(canvas, &driver.config, func() time.Duration { return driver.now })
	if err != nil {
		driver.Close()
		return nil, err
	}
	driver.painter.setClipboardText = func(value string) { driver.clipboard, driver.system.clipboardImage = value, nil }
	driver.painter.setCursor = func(Cursor) error { return nil }
	driver.painter.scheduleFrame = func(delay time.Duration) {
		due := driver.now + delay
		if !driver.animating || due < driver.frameDue {
			driver.frameDue = due
		}
		driver.animating = true
	}
	driver.painter.scheduleIfAnimating()
	if err := driver.registerSystem(); err != nil {
		driver.Close()
		return nil, err
	}
	enterDriverAppearance()
	driver.appearanceOpen = true
	if invalidator := driver.config.Invalidator; invalidator != nil {
		invalidator.waker.Bind(driver.requestWake)
	}
	return driver, nil
}

// Close detaches the driver from its Invalidator and releases its resources.
func (driver *Driver) Close() {
	if driver.closed {
		return
	}
	driver.closed = true
	liveDrivers.Release()
	if driver.appearanceOpen {
		leaveDriverAppearance()
	}
	driver.unregisterSystem()
	if driver.config.Invalidator != nil {
		driver.config.Invalidator.waker.Unbind()
	}
	for _, release := range driver.releases {
		release()
	}
	driver.releases = nil
	if driver.config.OnClosed != nil {
		driver.config.OnClosed()
	}
}

func (driver *Driver) requestWake() {
	driver.wake.Store(true)
	select {
	case driver.wakeSent <- struct{}{}:
	default:
	}
}

// Wakes receives a value whenever a background goroutine asked for a repaint
// since the last Pump. Tools use it to sleep until there is something to apply.
func (driver *Driver) Wakes() <-chan struct{} { return driver.wakeSent }

// Pump applies a pending background repaint request, the way a window would on
// its next loop turn: OnWake runs, then the application repaints. Every input
// method pumps first, so state published by another goroutine is visible to the
// event that follows.
func (driver *Driver) Pump() error {
	driver.activateSystem()
	if !driver.wake.Swap(false) {
		return nil
	}
	return driver.redraw()
}

// redraw is the window's redraw handler: OnWake, then a repaint.
func (driver *Driver) redraw() error {
	if driver.config.OnWake != nil {
		driver.config.OnWake()
	}
	_, err := driver.painter.repaint()
	return err
}

// Transitioning reports whether a transition is in flight: the last paint asked
// for the next frame at display cadence. A display that changes slowly (a
// spinner stepping every 125 ms) asks later and does not count, so waiting for
// the application to come to rest ends even with one on screen.
func (driver *Driver) Transitioning() bool {
	return driver.animating && driver.frameDue <= driver.now
}

// Now is the driver's virtual time, which starts at zero.
func (driver *Driver) Now() time.Duration { return driver.now }

// Advance moves virtual time forward by duration, repainting at frame cadence
// while an animation is in flight and applying background wakes. An idle
// application repaints nothing, as in a window.
func (driver *Driver) Advance(duration time.Duration) error {
	end := driver.now + duration
	for driver.now < end {
		step := min(frameInterval, end-driver.now)
		driver.now += step
		if err := driver.Pump(); err != nil {
			return err
		}
		if driver.animating && driver.now >= driver.frameDue {
			driver.animating = false
			// A due frame reaches the application the way the window's timer
			// does: OnWake first, then the repaint.
			if err := driver.redraw(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Resize changes the content size in logical pixels and repaints.
func (driver *Driver) Resize(width, height int) error {
	if width < 1 || height < 1 {
		return fmt.Errorf("app: window dimensions must be positive, got %dx%d", width, height)
	}
	if err := driver.Pump(); err != nil {
		return err
	}
	_, err := driver.painter.resize(platform.Size{Width: float32(width), Height: float32(height)})
	if err == nil && driver.config.OnFrameChange != nil {
		driver.config.OnFrameChange(driver.Frame())
	}
	return err
}

func (driver *Driver) pointer(kind platform.PointerEventKind, x, y float32, button ui.PointerButton, clicks int) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	origin := driver.config.originPoint()
	_, err := driver.painter.pointer(platform.PointerEvent{
		Kind: kind, Button: button, Modifiers: driver.heldModifiers(), X: x, Y: y, ScreenX: origin.X + x, ScreenY: origin.Y + y, ClickCount: clicks,
	})
	return err
}

// Frame is the window's content area in screen coordinates: where Config.Origin
// or the last MoveWindow put it (the screen's corner when neither), at the
// window's current size.
func (driver *Driver) Frame() ui.Rect {
	return ui.Rect{X: driver.origin.X, Y: driver.origin.Y, Width: driver.painter.size.Width, Height: driver.painter.size.Height}
}

// MoveWindow places the content area at a screen point, as dragging the window
// would, and reports the new frame to OnFrameChange.
func (driver *Driver) MoveWindow(x, y float32) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	driver.origin = ui.Point{X: x, Y: y}
	if driver.config.OnFrameChange != nil {
		driver.config.OnFrameChange(driver.Frame())
	}
	return nil
}

// SetMinimized reports the window as minimized or restored, as the window
// manager would, and calls OnMinimize.
func (driver *Driver) SetMinimized(minimized bool) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if driver.minimized == minimized {
		return nil
	}
	driver.minimized = minimized
	if driver.config.OnMinimize != nil {
		driver.config.OnMinimize(minimized)
	}
	return nil
}

// Minimized reports the state SetMinimized last reported.
func (driver *Driver) Minimized() bool { return driver.minimized }

// SetMaximized reports the window as maximized or restored, as the window
// manager would, and calls OnMaximize.
func (driver *Driver) SetMaximized(maximized bool) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if driver.maximized == maximized {
		return nil
	}
	driver.maximized = maximized
	if driver.config.OnMaximize != nil {
		driver.config.OnMaximize(maximized)
	}
	return nil
}

// Maximized reports the state SetMaximized last reported.
func (driver *Driver) Maximized() bool { return driver.maximized }

// HitTest returns the front-most enabled interaction target at a logical-pixel
// point. It applies the same focus-scope and blocking-region rules as pointer
// input without moving the pointer or repainting.
func (driver *Driver) HitTest(x, y float32) (ui.ID, bool) {
	return driver.painter.router.HitTest(x, y)
}

// PressAt reports what a press at a point would do in a custom-titlebar window:
// whether the platform moves the window from there, now or once the pointer
// drags, or the application takes the press. It applies the same rules as the
// platform's own question, so a test can assert that a title bar moves the window
// and that a menu's backdrop over it still takes the press.
func (driver *Driver) PressAt(x, y float32) PressKind {
	return driver.painter.pressAt(x, y)
}

// Move moves the pointer to a point in logical pixels.
func (driver *Driver) Move(x, y float32) error {
	return driver.pointer(platform.PointerMove, x, y, ui.PointerButtonNone, 0)
}

// Leave moves the pointer out of the window.
func (driver *Driver) Leave() error {
	return driver.pointer(platform.PointerExit, 0, 0, ui.PointerButtonNone, 0)
}

// Press presses a pointer button at a point (moving there first) as a lone
// click.
func (driver *Driver) Press(x, y float32, button ui.PointerButton) error {
	return driver.PressCount(x, y, button, 1)
}

// PressCount presses a button as the clicks-th press of a click sequence (2
// for the second press of a double click), the way the operating system
// reports it.
func (driver *Driver) PressCount(x, y float32, button ui.PointerButton, clicks int) error {
	if err := driver.Move(x, y); err != nil {
		return err
	}
	return driver.pointer(platform.PointerDown, x, y, button, max(clicks, 1))
}

// Release releases a pointer button at a point, as the lone click's release.
func (driver *Driver) Release(x, y float32, button ui.PointerButton) error {
	return driver.ReleaseCount(x, y, button, 1)
}

// ReleaseCount releases a button with the count of the press it ends.
func (driver *Driver) ReleaseCount(x, y float32, button ui.PointerButton, clicks int) error {
	return driver.pointer(platform.PointerUp, x, y, button, max(clicks, 1))
}

// Click moves to a point and presses and releases the primary button.
func (driver *Driver) Click(x, y float32) error {
	return driver.ClickCount(x, y, 1)
}

// ClickButton moves to a point and presses and releases the given button once:
// ui.PointerButtonSecondary is the right button, whose press opens context menus.
func (driver *Driver) ClickButton(x, y float32, button ui.PointerButton) error {
	if err := driver.PressCount(x, y, button, 1); err != nil {
		return err
	}
	return driver.ReleaseCount(x, y, button, 1)
}

// DoubleClick clicks twice at a point in quick succession: the second press
// and release carry a click count of 2.
func (driver *Driver) DoubleClick(x, y float32) error {
	return driver.ClickCount(x, y, 2)
}

// ClickCount performs a click sequence of the given length (1 for a click, 2
// for a double click, 3 for a triple click); each press and release carries
// its position in the sequence.
func (driver *Driver) ClickCount(x, y float32, clicks int) error {
	for press := 1; press <= max(clicks, 1); press++ {
		if err := driver.PressCount(x, y, ui.PointerButtonPrimary, press); err != nil {
			return err
		}
		if err := driver.ReleaseCount(x, y, ui.PointerButtonPrimary, press); err != nil {
			return err
		}
	}
	return nil
}

// Scroll delivers a scroll gesture at a logical-pixel window point. Positive
// deltas reveal content below and to the right. Target and Viewport are derived
// from the current frame; the other fields are delivered as supplied.
func (driver *Driver) Scroll(event ui.ScrollEvent) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	_, err := driver.painter.scroll(platform.ScrollEvent{
		Modifiers: event.Modifiers | driver.heldModifiers(), X: event.X, Y: event.Y,
		DeltaX: event.DeltaX, DeltaY: event.DeltaY, Precise: event.Precise,
	})
	return err
}

// Drop delivers a drag of files from the system: a hover (FileDropOver) at a
// point, its end (FileDropLeave), or the drop with its paths. It reports whether
// the hit under the point took the files.
func (driver *Driver) Drop(event ui.FileDropEvent) (bool, error) {
	if err := driver.Pump(); err != nil {
		return false, err
	}
	_, accepted, err := driver.painter.fileDrop(platform.FileDropEvent{
		Phase: platform.FileDropPhase(event.Phase), X: event.X, Y: event.Y, Paths: event.Paths,
	})
	return accepted, err
}

// editing reports whether a native text editor is active.
func (driver *Driver) editing() bool {
	return driver.painter.textEnabled && driver.painter.activeText != nil
}

// heldModifiers is the modifiers KeyDown is holding.
func (driver *Driver) heldModifiers() ui.Modifiers {
	var modifiers ui.Modifiers
	for key := ui.KeyShiftLeft; key <= ui.KeyMetaRight; key++ {
		if driver.modifierKeys&(1<<(key-ui.KeyShiftLeft)) != 0 {
			modifiers |= key.Modifier()
		}
	}
	return modifiers
}

// HoldsKeys reports whether the focused terminal asked for presses and releases
// (TextInput.KeyReleases), the case in which KeyDown and KeyUp deliver key
// events with their Release flag.
func (driver *Driver) HoldsKeys() bool { return driver.painter.keysWanted() }

// driverKeyCode is the Code a driven key has: stable between its press and release.
func driverKeyCode(key ui.Key, character string) uint32 {
	if key != ui.KeyUnknown {
		return 0x10000 + uint32(key)
	}
	for _, r := range strings.ToLower(character) {
		return uint32(r)
	}
	return 0
}

// driverTyped is the text a driven printable key types: the character, in capitals
// while Shift is held. Shift with a symbol key has no layout to consult here.
func driverTyped(key ui.Key, character string, modifiers ui.Modifiers) string {
	if key != ui.KeyUnknown || modifiers&(ui.ModifierControl|ui.ModifierMeta|ui.ModifierAlt) != 0 {
		return ""
	}
	if modifiers&ui.ModifierShift != 0 {
		return strings.ToUpper(character)
	}
	return character
}

// KeyDown presses a key and keeps it down until KeyUp, as a person holding it
// would. Modifier keys (ui.KeyShiftLeft and the like) stay held for every pointer,
// scroll and key the driver delivers meanwhile, so Shift-click and Ctrl-click
// work. A focused terminal that asked for releases (HoldsKeys) gets the press as
// a KeyEvent: a named key by Key, a printable key by Text, which character
// names; any other focus gets the key as Key or Chord would deliver it, and
// KeyUp then does nothing.
func (driver *Driver) KeyDown(key ui.Key, character string) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if key.IsModifier() {
		driver.modifierKeys |= 1 << (key - ui.KeyShiftLeft)
	}
	modifiers := driver.heldModifiers()
	if !driver.HoldsKeys() {
		switch {
		case key.IsModifier():
			return nil
		case key != ui.KeyUnknown:
			return driver.Key(key, modifiers)
		case modifiers&(ui.ModifierControl|ui.ModifierMeta|ui.ModifierAlt) != 0:
			return driver.Chord(modifiers, character)
		}
		return driver.Type(character)
	}
	text := strings.ToLower(character)
	if key == ui.KeyUnknown && modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0 {
		_, handled, err := driver.painter.shortcut(platform.KeyEvent{Modifiers: modifiers, Text: text})
		if err != nil || handled {
			return err
		}
	}
	if key == ui.KeyUnknown && modifiers&ui.ModifierAlt != 0 && modifiers&(ui.ModifierControl|ui.ModifierMeta) == 0 &&
		!driver.painter.nativeText.AltSendsEscape {
		// Without Alt-as-Meta the text system owns the chord, as in a window.
		return driver.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: character})
	}
	_, err := driver.painter.key(platform.KeyEvent{
		Key: key, Modifiers: modifiers, Text: text, Code: driverKeyCode(key, character),
		Typed: driverTyped(key, character, modifiers),
	})
	return err
}

// KeyUp releases a key KeyDown pressed. A key that is not down is ignored.
func (driver *Driver) KeyUp(key ui.Key, character string) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if key.IsModifier() {
		driver.modifierKeys &^= 1 << (key - ui.KeyShiftLeft)
	}
	if !driver.HoldsKeys() {
		return nil
	}
	_, err := driver.painter.key(platform.KeyEvent{
		Release: true, Modifiers: driver.heldModifiers(), Code: driverKeyCode(key, character),
	})
	return err
}

// LoseFocus makes the window lose the keyboard, as switching to another
// application does: keys held down are released first. Modifiers held by
// KeyDown are forgotten with them.
func (driver *Driver) LoseFocus() error {
	if err := driver.Pump(); err != nil {
		return err
	}
	driver.modifierKeys = 0
	_, err := driver.painter.focusLost()
	return err
}

// terminal reports whether the focused text input is a terminal's raw keyboard.
func (driver *Driver) terminal() bool {
	return driver.editing() && driver.painter.nativeText.Terminal
}

// Key delivers a key press with its modifiers. While a text field is being
// edited the key goes where a native window would send it: Tab and Escape (and
// Enter in a single-line field) to the application, everything else to the
// text system as an editing command. A terminal gets every named key raw, as
// ui.KeyEvents.
func (driver *Driver) Key(key ui.Key, modifiers ui.Modifiers) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	modifiers |= driver.heldModifiers()
	if driver.terminal() {
		return driver.terminalKey(key, modifiers, "")
	}
	if driver.editing() && key != ui.KeyTab && key != ui.KeyEscape && (key != ui.KeyEnter || driver.painter.nativeText.Multiline) {
		if driver.navigationKey(key) || driver.rowKey(key, modifiers) {
			_, err := driver.painter.key(platform.KeyEvent{Key: key, Modifiers: modifiers})
			return err
		}
		return driver.textKey(key, modifiers)
	}
	_, err := driver.painter.key(platform.KeyEvent{Key: key, Modifiers: modifiers})
	return err
}

// navigationKey reports whether a key moves between choices outside the focused
// field (such as a dropdown, autocomplete list or command palette) when the field
// requested NavigationKeys.
func (driver *Driver) navigationKey(key ui.Key) bool {
	text := &driver.painter.nativeText
	if !text.NavigationKeys || text.State.Composing {
		return false
	}
	switch key {
	case ui.KeyUp, ui.KeyDown, ui.KeyPageUp, ui.KeyPageDown:
		return true
	}
	return false
}

// rowKey reports whether a key moves between the rows of the focused field,
// which lays them out itself: the platform's text system knows one line, so
// these reach the application as key events unless an input method is composing
// (platform.TextInput.Placed).
func (driver *Driver) rowKey(key ui.Key, modifiers ui.Modifiers) bool {
	text := &driver.painter.nativeText
	if !text.Placed || text.State.Composing {
		return false
	}
	switch key {
	case ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown:
		return true
	case ui.KeyLeft, ui.KeyRight:
		return modifiers&ui.ModifierMeta != 0
	}
	return false
}

// terminalKey delivers a key to a focused terminal the way a native window
// does: a plain space is text unless the terminal requested Alt-as-Meta;
// everything else is a raw key event.
func (driver *Driver) terminalKey(key ui.Key, modifiers ui.Modifiers, text string) error {
	altRaw := modifiers&ui.ModifierAlt != 0 && driver.painter.nativeText.AltSendsEscape &&
		!driver.painter.nativeText.State.Composing
	if key == ui.KeySpace && modifiers&(ui.ModifierControl|ui.ModifierMeta) == 0 && !altRaw {
		return driver.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: " "})
	}
	_, err := driver.painter.key(platform.KeyEvent{Key: key, Modifiers: modifiers, Text: text})
	return err
}

// Chord delivers a modified printable character. Control and Meta chords are
// raw key events whose Text is the base character (Control+C is Key unknown,
// Text "c"). Alt is raw only when the focused terminal requested
// AltSendsEscape and no input-method composition is active. It fails unless a
// terminal has the keyboard. Characters typed with only Shift are text.
func (driver *Driver) Chord(modifiers ui.Modifiers, character string) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0 {
		_, handled, err := driver.painter.shortcut(platform.KeyEvent{Modifiers: modifiers, Text: strings.ToLower(character)})
		if err != nil || handled {
			return err
		}
	}
	if !driver.terminal() {
		return ErrNoTerminal
	}
	raw := modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0
	if modifiers&ui.ModifierAlt != 0 && driver.painter.nativeText.AltSendsEscape &&
		!driver.painter.nativeText.State.Composing {
		raw = true
	}
	if !raw {
		return driver.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: character})
	}
	return driver.terminalKey(ui.KeyUnknown, modifiers, strings.ToLower(character))
}

func (driver *Driver) textKey(key ui.Key, modifiers ui.Modifiers) error {
	editKey := platform.KeyEvent{Key: input.Key(key), Modifiers: input.Modifiers(modifiers)}
	if action := platform.TextClipboardAction(editKey); action != 0 {
		return driver.clipboardEdit(action)
	}
	if command, ok := platform.TextEditingCommand(editKey); ok {
		return driver.textEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
	}
	if command, ok := platform.AppleTextEditingCommand(editKey); ok {
		return driver.textEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
	}
	extend := modifiers&ui.ModifierShift != 0
	var event platform.TextEvent
	switch key {
	case ui.KeyBackspace:
		event = platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextDeleteBackward}
	case ui.KeyDelete:
		event = platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextDeleteForward}
	case ui.KeyLeft:
		command := platform.TextMoveLeft
		if extend {
			command = platform.TextMoveLeftAndModifySelection
		}
		event = platform.TextEvent{Kind: platform.TextCommandEvent, Command: command}
	case ui.KeyRight:
		command := platform.TextMoveRight
		if extend {
			command = platform.TextMoveRightAndModifySelection
		}
		event = platform.TextEvent{Kind: platform.TextCommandEvent, Command: command}
	case ui.KeyEnter:
		command := platform.TextInsertNewline
		if extend {
			command = platform.TextInsertLineBreak
		}
		event = platform.TextEvent{Kind: platform.TextCommandEvent, Command: command}
	case ui.KeySpace:
		event = platform.TextEvent{Kind: platform.TextInsert, Text: " "}
	default:
		return nil // the text system has no command for this key
	}
	return driver.textEvent(event)
}

// textEvent applies one text-system event and redraws when its native
// presentation changed.
func (driver *Driver) textEvent(event platform.TextEvent) error {
	previous := driver.painter.nativeText.State
	next, err := driver.painter.applyTextEvent(event)
	if err != nil {
		return err
	}
	if !platform.TextInputPresentationChanged(driver.painter.nativeText.Terminal, previous, next) {
		return nil
	}
	return driver.redraw()
}

// Type inserts text into the focused text field, replacing its selection.
func (driver *Driver) Type(value string) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if !driver.editing() {
		return ErrNoTextInput
	}
	return driver.textEvent(platform.TextEvent{Kind: platform.TextInsert, Text: value})
}

// Compose sets the input-method composition of the focused text field to value,
// as an input method does while the user types a pinyin syllable before choosing
// a candidate; an empty value ends the composition without committing anything.
// The caret sits at the end of the composition. Type commits text.
func (driver *Driver) Compose(value string) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if !driver.editing() {
		return ErrNoTextInput
	}
	if value == "" {
		return driver.textEvent(platform.TextEvent{Kind: platform.TextUnmark})
	}
	end := utf8.RuneCountInString(value)
	return driver.textEvent(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: text.Range{Start: end, End: end}})
}

// SelectText selects a rune range of the focused text field.
func (driver *Driver) SelectText(start, end int) error {
	if err := driver.Pump(); err != nil {
		return err
	}
	if !driver.editing() {
		return ErrNoTextInput
	}
	return driver.textEvent(platform.TextEvent{Kind: platform.TextSetSelection, Selection: text.Range{Start: start, End: end}})
}

// Shortcut delivers a Command-key shortcut. It is retained as the compact
// editing helper; use ShortcutChord when the complete modifier set matters.
func (driver *Driver) Shortcut(character rune) (bool, error) {
	return driver.ShortcutChord(ui.ModifierMeta, character)
}

// ShortcutChord handles ordinary text editing before offering other chords to
// the application and applying the terminal fallback. The complete
// modifier set is preserved, which lets headless tests exercise application
// shortcuts such as Command+Alt+X without bypassing the native text path.
func (driver *Driver) ShortcutChord(modifiers ui.Modifiers, character rune) (bool, error) {
	if err := driver.Pump(); err != nil {
		return false, err
	}
	shortcut := shortcutText(character)
	if driver.editing() && !driver.terminal() {
		event := platform.KeyEvent{Modifiers: input.Modifiers(modifiers), Text: shortcut}
		if command, ok := platform.TextEditingCommand(event); ok {
			return true, driver.textEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command})
		}
		if action := platform.TextClipboardAction(event); action != 0 {
			return true, driver.clipboardEdit(action)
		}
	}
	if shortcut != "" {
		_, handled, err := driver.painter.shortcut(platform.KeyEvent{Modifiers: modifiers, Text: shortcut})
		if err != nil || handled {
			return handled, err
		}
	}
	if !driver.editing() {
		return false, nil
	}
	if driver.painter.nativeText.Terminal {
		// A terminal owns every modified chord except an unadorned primary
		// paste, which the text system delivers with the driver's private
		// clipboard.
		if character != 'v' || modifiers&(ui.ModifierAlt|ui.ModifierShift) != 0 {
			return true, driver.terminalKey(ui.KeyUnknown, modifiers, shortcut)
		}
		if driver.clipboard == "" {
			return true, nil
		}
		return true, driver.textEvent(platform.TextEvent{Kind: platform.TextPaste, Text: driver.clipboard})
	}
	return false, nil
}

func (driver *Driver) clipboardEdit(action byte) error {
	event, changed, err := platform.TextClipboardEvent(driver.painter.nativeText.State, driver.painter.nativeText.Secure, action,
		func() (string, error) { return driver.clipboard, nil },
		func(value string) error { driver.SetClipboard(value); return nil })
	if err != nil || !changed {
		return err
	}
	return driver.textEvent(event)
}

func shortcutText(character rune) string {
	const characters = "0123456789abcdefghijklmnopqrstuvwxyz"
	switch {
	case character >= '0' && character <= '9':
		index := int(character - '0')
		return characters[index : index+1]
	case character >= 'A' && character <= 'Z':
		index := 10 + int(character-'A')
		return characters[index : index+1]
	case character >= 'a' && character <= 'z':
		index := 10 + int(character-'a')
		return characters[index : index+1]
	default:
		return string(character)
	}
}

// SetSystemAppearance changes what SystemAppearance reports while this driver
// runs and tells the watchers, as the system would.
func (driver *Driver) SetSystemAppearance(appearance Appearance) {
	driverAppearance.Lock()
	driverAppearance.value = appearance
	driverAppearance.Unlock()
	appearanceMaybeChanged()
}

// NativeAppearance is what the application last asked SetNativeAppearance for,
// AppearanceUnknown (the system's own) if it has not.
func (driver *Driver) NativeAppearance() Appearance {
	driverAppearance.Lock()
	defer driverAppearance.Unlock()
	return driverAppearance.native
}

// Clipboard is the driver's private clipboard, including text an application
// returns through ui.EventResult.ClipboardText.
func (driver *Driver) Clipboard() string { return driver.clipboard }

// SetClipboard replaces the driver's private clipboard.
func (driver *Driver) SetClipboard(value string) {
	driver.clipboard, driver.system.clipboardImage = value, nil
}

// LastOpenedURL returns the most recent target passed to OpenURL while this
// driver was active. Driver-backed runs record the request without asking the
// operating system to open anything.
func (driver *Driver) LastOpenedURL() (string, bool) {
	return driver.openedURL, driver.hasOpenedURL
}

// LastOpenedPath returns the most recent file passed to OpenPath while this
// driver was active. Like LastOpenedURL, driver-backed runs record the request
// and open nothing.
func (driver *Driver) LastOpenedPath() (string, bool) {
	return driver.openedPath, driver.hasOpenedPath
}

// PrimitiveKind says what a Primitive is.
type PrimitiveKind uint8

const (
	// RectanglePrimitive is an axis-aligned rectangle, rounded when Radius > 0.
	RectanglePrimitive PrimitiveKind = iota
	// TrianglePrimitive is one anti-aliased triangle of a vector shape.
	TrianglePrimitive
	// ShadowPrimitive is the soft shadow cast by the rounded rectangle Bounds and
	// Radius, placed and blurred by Shadow.
	ShadowPrimitive
)

// Primitive is one retained instance to draw. Instances are in paint order
// within a layer; higher layers paint over lower ones.
type Primitive struct {
	Kind   PrimitiveKind
	Bounds ui.Rect
	Points [3]ui.Point
	Color  ui.Color
	Radius float32
	Layer  uint16
	// EdgeMask selects the boundary edges feathered for TrianglePrimitive.
	EdgeMask uint8
	// Shaded gives a TrianglePrimitive the colors Shade[0] and Shade[1] at
	// Points[1] and Points[2], Color being that of Points[0]; the color varies
	// linearly between them.
	Shaded bool
	Shade  [2]ui.Color
	// Shadow places and blurs a ShadowPrimitive.
	Shadow ui.ShadowStyle
	// Clipped reports that the primitive is drawn only inside Clip, a region of
	// the window in logical pixels that cuts it; Bounds and Points are whole.
	Clipped bool
	Clip    ui.Rect
}

// Label is one line of text to draw.
type Label struct {
	Value  string
	Bounds ui.Rect
	// OriginX and OriginY are platform.TextLabel's: the text's sub-pixel position
	// within Bounds.
	OriginX, OriginY float32
	FontSize         float32
	Weight           float32
	Color            ui.Color
	Layer            uint16
	// Mono, MonoFace, CellWidth and Runs describe a fixed-pitch grid line (see
	// ui.Canvas.LabelRuns); Runs is empty for an ordinary label.
	Mono      bool
	MonoFace  ui.MonoFace
	CellWidth float32
	Runs      []ui.TextRun
	// Clip is the region the label may paint into when Clipped (see
	// platform.TextLabel.Clip).
	Clip          ui.Rect
	Clipped       bool
	Bitmap        []byte
	BitmapWidth   int
	BitmapHeight  int
	BitmapStride  int
	BitmapID      uint64
	BitmapVersion uint32
	BitmapUpload  bool
}

// Editor is the native text editor's overlay, drawn above everything else
// while a text field has keyboard focus. Selection and Composition are rune
// ranges into Value.
type Editor struct {
	Bounds      ui.Rect
	FontSize    float32
	Layer       uint16
	MonoFace    ui.MonoFace
	Color       ui.Color
	Value       string
	Selection   [2]int
	Composition [2]int
	Composing   bool
	Multiline   bool
	Secure      bool
	// Terminal is a terminal's preedit surface: the composition only, laid on the
	// terminal's cells in its fixed-pitch face (MonoFace), with no caret or selection.
	Terminal bool
}

// Snapshot is everything visible and interactive at one moment.
type Snapshot struct {
	Title         string
	Width, Height int
	Clear         Color
	Primitives    []Primitive
	Labels        []Label
	// Editor is nil unless a text field is being edited natively.
	Editor *Editor
	// Texts is every text the application painted this frame, before clipping,
	// for queries; Labels holds what is actually drawn.
	Texts []ui.Text
	// Runs are the text runs Texts refer to; see TextRuns.
	Runs []ui.TextRun
	Hits []ui.Hit
	// Tree is the identity tree the hits are named in, for printing them.
	Tree *ui.Tree
	// Focused and Hovered are hit identities (0 for none).
	Focused ui.ID
	Hovered ui.ID
	Cursor  Cursor
	Now     time.Duration
}

// TextRuns returns the runs of one of the snapshot's Texts.
func (snapshot Snapshot) TextRuns(text ui.Text) []ui.TextRun {
	if text.RunCount == 0 || int(text.RunFirst+text.RunCount) > len(snapshot.Runs) {
		return nil
	}
	return snapshot.Runs[text.RunFirst : text.RunFirst+text.RunCount]
}

// Snapshot captures the current frame. The result is independent of later
// driver calls.
func (driver *Driver) Snapshot() Snapshot {
	painter := driver.painter
	snapshot := Snapshot{
		Title: driver.config.Title, Width: int(painter.size.Width), Height: int(painter.size.Height),
		Clear:   driver.config.ClearColor,
		Texts:   append([]ui.Text(nil), painter.canvas.Texts()...),
		Runs:    append([]ui.TextRun(nil), painter.canvas.AllTextRuns()...),
		Hits:    append([]ui.Hit(nil), painter.canvas.Hits()...),
		Tree:    painter.canvas.Tree(),
		Focused: painter.focused, Hovered: painter.hovered, Cursor: painter.cursor, Now: driver.now,
	}
	for _, rectangle := range painter.renderer.Rectangles() {
		primitive := Primitive{
			Bounds: ui.Rect{X: rectangle.Bounds.X, Y: rectangle.Bounds.Y, Width: rectangle.Bounds.Width, Height: rectangle.Bounds.Height},
			Color:  ui.Color{Red: rectangle.Color.Red, Green: rectangle.Color.Green, Blue: rectangle.Color.Blue, Alpha: rectangle.Color.Alpha},
			Radius: rectangle.Radius, Layer: rectangle.Layer,
		}
		if rectangle.Clipped {
			primitive.Clipped = true
			primitive.Clip = ui.Rect{X: rectangle.Clip.X, Y: rectangle.Clip.Y, Width: rectangle.Clip.Width, Height: rectangle.Clip.Height}
		}
		if rectangle.Kind == scene.KindShadow {
			primitive.Kind = ShadowPrimitive
			primitive.Shadow = ui.ShadowStyle(rectangle.Shadow)
		}
		if rectangle.Kind == scene.KindTriangle {
			primitive.Kind = TrianglePrimitive
			primitive.EdgeMask = rectangle.EdgeMask
			if rectangle.Shade.On {
				primitive.Shaded = true
				for index, color := range rectangle.Shade.Colors {
					primitive.Shade[index] = ui.Color(color)
				}
			}
			for index, point := range rectangle.Points {
				primitive.Points[index] = ui.Point{X: point.X, Y: point.Y}
			}
		}
		snapshot.Primitives = append(snapshot.Primitives, primitive)
	}
	for _, label := range painter.labels {
		captured := Label{
			Value:   label.Value,
			Bounds:  ui.Rect{X: label.X, Y: label.Y, Width: label.Width, Height: label.Height},
			OriginX: label.OriginX, OriginY: label.OriginY,
			FontSize: label.FontSize, Weight: label.Weight,
			Color: ui.Color{Red: float32(label.Color.Red), Green: float32(label.Color.Green), Blue: float32(label.Color.Blue), Alpha: float32(label.Color.Alpha)},
			Layer: label.Layer,
			Mono:  label.Mono, MonoFace: label.MonoFace, CellWidth: label.CellWidth, Runs: append([]ui.TextRun(nil), label.Runs...),
			Clip: label.Clip, Clipped: label.Clipped,
			BitmapWidth: label.BitmapWidth, BitmapHeight: label.BitmapHeight, BitmapStride: label.BitmapStride,
			BitmapID: label.BitmapID, BitmapVersion: label.BitmapVersion,
			BitmapUpload: label.BitmapUpload,
		}
		captured.Bitmap = append([]byte(nil), label.Bitmap...)
		snapshot.Labels = append(snapshot.Labels, captured)
	}
	if driver.editing() {
		input := painter.nativeText
		snapshot.Editor = &Editor{
			Bounds:   ui.Rect{X: input.X, Y: input.Y, Width: input.Width, Height: input.Height},
			FontSize: input.FontSize,
			// Layer keeps a field inside a popup, dialog or sheet above that
			// surface's own paint in an offscreen render, as it already is in a
			// window.
			Layer:    input.Layer,
			MonoFace: input.MonoFace,
			Color:    ui.Color{Red: float32(input.Color.Red), Green: float32(input.Color.Green), Blue: float32(input.Color.Blue), Alpha: float32(input.Color.Alpha)},
			Value:    input.State.Value, Selection: [2]int{input.State.Selection.Start, input.State.Selection.End},
			Composition: [2]int{input.State.Composition.Start, input.State.Composition.End},
			Composing:   input.State.Composing, Multiline: input.Multiline, Secure: input.Secure,
			Terminal: input.Terminal,
		}
	}
	return snapshot
}
