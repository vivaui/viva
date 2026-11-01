//go:build linux && vivagtk

package linux

import (
	"math"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// wheelStep is how many logical pixels one notch of a mouse wheel scrolls, as the XCB backend has
// it. A touchpad reports fractions of a notch with a precise source, one notch being about this
// many device pixels of finger travel.
const (
	gtkWheelStep   = 48
	gtkPreciseStep = 10
)

// gtkPointer is the state of the pointer on a window.
type gtkPointer struct {
	clicks *input.ClickCounter
	// pending is a press on the window's frame waiting to become a move of the window once the
	// pointer drags: GTK asks for the move with the press that started it.
	pending         bool
	pendingDelivery bool // the application has the press too
	pendingX        float64
	pendingY        float64
	pendingTime     uint32
	pendingButton   int32
	over            bool
}

// gtkKeys is the keyboard state of a window.
type gtkKeys struct {
	down map[uint16]bool
}

// handleEvent routes one GDK event of the window's widgets. The pointer events are the drawing
// area's, the rest the toplevel's.
func (w *gtkWindow) handleEvent(widget, event uintptr) uintptr {
	if w.closed {
		return 0
	}
	switch eventType(event) {
	case gdkMotionNotify:
		e := (*gdkEventMotion)(unsafePointer(event))
		dx, dy := w.eventOffset(widget)
		w.pointerMove(e.X+dx, e.Y+dy, e.XRoot, e.YRoot, e.State, e.Time)
		return 1
	case gdkEnterNotify:
		e := (*gdkEventCrossing)(unsafePointer(event))
		if e.Mode != gdkCrossingGrab && e.Mode != gdkCrossingUngrab {
			dx, dy := w.eventOffset(widget)
			w.pointerMove(e.X+dx, e.Y+dy, e.XRoot, e.YRoot, e.State, e.Time)
		}
		return 0
	case gdkLeaveNotify:
		e := (*gdkEventCrossing)(unsafePointer(event))
		if e.Mode != gdkCrossingGrab && e.Mode != gdkCrossingUngrab {
			w.pointerExit()
		}
		return 0
	case gdkButtonPress, gdkButtonRelease:
		dx, dy := w.eventOffset(widget)
		return w.pointerButton((*gdkEventButton)(unsafePointer(event)), dx, dy)
	case gdk2ButtonPress, gdk3ButtonPress:
		return 1 // counted by the window
	case gdkScroll:
		dx, dy := w.eventOffset(widget)
		return w.scrollEvent(event, (*gdkEventScroll)(unsafePointer(event)), dx, dy)
	case gdkKeyPress, gdkKeyRelease:
		return w.keyEvent(event)
	case gdkFocusChange:
		in := (*gdkEventFocus)(unsafePointer(event)).In != 0
		if widget != w.toplevel {
			w.webFocusChanged(widget, in)
			return 0
		}
		w.focusChanged(in)
	case gdkWindowState:
		w.stateChanged((*gdkEventWindowState)(unsafePointer(event)))
	case gdkConfigure:
		w.reportFrame()
	}
	return 0
}

// logical converts a position in widget units to logical pixels.
func (w *gtkWindow) logical(x, y float64) (float32, float32) {
	return float32(x) / w.ratio, float32(y) / w.ratio
}

// pointerMoveAt, pointerButtonAt and scrollAt are the events of the Wayland subsurface, whose
// positions are already logical pixels of the window.
func (w *gtkWindow) pointerMoveAt(x, y float32, modifiers input.Modifiers) {
	w.deliverPointer(platform.PointerEvent{Kind: platform.PointerMove, Modifiers: modifiers, X: x, Y: y})
}

func (w *gtkWindow) pointerButtonAt(x, y float32, button uint32, down bool, eventTime uint32, modifiers input.Modifiers) {
	w.buttonAt(x, y, 0, 0, button, down, eventTime, modifiers)
}

func (w *gtkWindow) scrollAt(scroll platform.ScrollEvent) {
	if w.config.OnScroll == nil {
		return
	}
	frame, err := w.config.OnScroll(scroll)
	w.present(frame, err)
}

func (w *gtkWindow) pointerMove(x, y, rootX, rootY float64, state uint32, eventTime uint32) {
	lx, ly := w.logical(x, y)
	if w.pointer.pending {
		if state&gdkButton1Mask == 0 {
			w.pointer.pending = false
		} else if w.dragPassedSlop(rootX, rootY) {
			w.beginMove()
			return
		}
	}
	w.deliverPointer(platform.PointerEvent{Kind: platform.PointerMove, Modifiers: gdkModifiers(state), X: lx, Y: ly, ScreenX: float32(rootX) / w.ratio, ScreenY: float32(rootY) / w.ratio})
}

func (w *gtkWindow) pointerExit() {
	w.pointer.over = false
	if w.config.OnPointer != nil {
		frame, err := w.config.OnPointer(platform.PointerEvent{Kind: platform.PointerExit})
		w.present(frame, err)
	}
}

func (w *gtkWindow) pointerButton(e *gdkEventButton, dx, dy float64) uintptr {
	down := e.Type == gdkButtonPress
	x, y := w.logical(e.X+dx, e.Y+dy)
	if down && e.Button == 1 && w.frameDrag(e, x, y) {
		return 1
	}
	w.buttonAt(x, y, float32(e.XRoot)/w.ratio, float32(e.YRoot)/w.ratio, e.Button, down, e.Time, gdkModifiers(e.State))
	return 1
}

// buttonAt gives a button press or release to the application.
func (w *gtkWindow) buttonAt(x, y, screenX, screenY float32, gdkButton uint32, down bool, eventTime uint32, modifiers input.Modifiers) {
	if !down && gdkButton == 1 {
		w.pointer.pending = false
	}
	button := gdkPointerButton(gdkButton)
	if button == ui.PointerButtonNone {
		return
	}
	if down {
		w.takeKeys()
		w.pointer.clicks.Interval, w.pointer.clicks.Distance = clickTimingGTK(w)
		w.pointer.clicks.Press(time.Duration(eventTime)*time.Millisecond, uint8(gdkButton), x, y)
	}
	kind := platform.PointerUp
	if down {
		kind = platform.PointerDown
	}
	w.deliverPointer(platform.PointerEvent{Kind: kind, Button: button, Modifiers: modifiers, X: x, Y: y, ScreenX: screenX, ScreenY: screenY, ClickCount: w.pointer.clicks.Current()})
}

// deliverPointer gives an event to the application, then to the text field under it.
func (w *gtkWindow) deliverPointer(event platform.PointerEvent) {
	w.app.active = w
	w.pointer.over = true
	if w.config.OnPointer != nil {
		frame, err := w.config.OnPointer(event)
		w.present(frame, err)
	}
	if w.closed {
		return
	}
	if event.Kind == platform.PointerDown && w.ime != nil {
		w.ime.setFocused(true)
	}
	frame, handled, err := w.text.pointer(event)
	if handled || err != nil {
		w.present(frame, err)
	}
}

// clickTimingGTK is the interval and distance of a double click, from the desktop's settings.
func clickTimingGTK(w *gtkWindow) (time.Duration, float32) {
	return clickTiming(w.app.settings, w.scale)
}

func (w *gtkWindow) scrollEvent(event uintptr, e *gdkEventScroll, dx, dy float64) uintptr {
	if w.config.OnScroll == nil {
		return 0
	}
	scroll := platform.ScrollEvent{Modifiers: gdkModifiers(e.State)}
	scroll.X, scroll.Y = w.logical(e.X+dx, e.Y+dy)
	switch e.Direction {
	case gdkScrollUp:
		scroll.DeltaY = -gtkWheelStep
	case gdkScrollDown:
		scroll.DeltaY = gtkWheelStep
	case gdkScrollLeft:
		scroll.DeltaX = -gtkWheelStep
	case gdkScrollRight:
		scroll.DeltaX = gtkWheelStep
	case gdkScrollSmooth:
		if e.DeltaX == 0 && e.DeltaY == 0 {
			return 1 // the end of a scroll gesture
		}
		step := float64(gtkWheelStep)
		if w.preciseSource(event) {
			step, scroll.Precise = gtkPreciseStep, true
		}
		scroll.DeltaX, scroll.DeltaY = float32(e.DeltaX*step), float32(e.DeltaY*step)
	default:
		return 1
	}
	frame, err := w.config.OnScroll(scroll)
	w.present(frame, err)
	return 1
}

// preciseSource reports whether the device behind a scroll event scrolls in fractions of a notch: a
// touchpad or a touch screen, as opposed to a wheel.
func (w *gtkWindow) preciseSource(event uintptr) bool {
	device := w.g.gdkEventSourceDevice(event)
	if device == 0 {
		return false
	}
	switch w.g.gdkDeviceSource(device) {
	case 5 /* GDK_SOURCE_TOUCHSCREEN */, 6 /* GDK_SOURCE_TOUCHPAD */ :
		return true
	}
	return false
}

// frameDrag handles a press on the frame of a window whose frame Viva draws: it begins a resize
// from an edge, or prepares the move of the window a press on its title bar means. It reports that
// the press is the window manager's and not the application's.
func (w *gtkWindow) frameDrag(e *gdkEventButton, x, y float32) bool {
	if w.config.WindowMode != platform.WindowCustomTitlebar {
		return false
	}
	press := platform.PressClient
	if w.config.PressAt != nil {
		press = w.config.PressAt(x, y)
	}
	if !w.maximized {
		// The top edge yields to a control, so the buttons work up to the top.
		if edge := platform.EdgeAt(x, y, w.width, w.height, press == platform.PressControl); edge != platform.EdgeNone {
			w.g.windowBeginResize(w.toplevel, gdkEdge(edge), 1, int32(e.XRoot), int32(e.YRoot), e.Time)
			return true
		}
	}
	switch press {
	case platform.PressClient, platform.PressControl:
		return false
	case platform.PressMoveOnDrag:
		w.pointer.pending, w.pointer.pendingDelivery = true, true
		w.pointer.pendingX, w.pointer.pendingY, w.pointer.pendingTime, w.pointer.pendingButton = e.XRoot, e.YRoot, e.Time, int32(e.Button)
		return false
	}
	w.pointer.clicks.Press(time.Duration(e.Time)*time.Millisecond, uint8(e.Button), x, y)
	if w.pointer.clicks.Current() == 2 {
		w.actionToggleMaximize()
		return true
	}
	w.pointer.pending, w.pointer.pendingDelivery = true, false
	w.pointer.pendingX, w.pointer.pendingY, w.pointer.pendingTime, w.pointer.pendingButton = e.XRoot, e.YRoot, e.Time, int32(e.Button)
	return true
}

func (w *gtkWindow) actionToggleMaximize() {
	if w.g.windowIsMaximized(w.toplevel) != 0 {
		w.g.windowUnmaximize(w.toplevel)
	} else {
		w.g.windowMaximize(w.toplevel)
	}
}

// dragPassedSlop reports whether the pointer moved far enough from the press to be a drag.
func (w *gtkWindow) dragPassedSlop(rootX, rootY float64) bool {
	slop := float64(defaultDragSlop)
	return math.Abs(rootX-w.pointer.pendingX) > slop || math.Abs(rootY-w.pointer.pendingY) > slop
}

// beginMove hands the window to the window manager to move, as the drag of its title bar asked.
func (w *gtkWindow) beginMove() {
	pointer := &w.pointer
	pointer.pending = false
	if pointer.pendingDelivery {
		// The application had the press; the window manager takes the release.
		w.pointerExit()
	}
	w.g.windowBeginMove(w.toplevel, pointer.pendingButton, int32(pointer.pendingX), int32(pointer.pendingY), pointer.pendingTime)
}

// gdkEdge is GdkWindowEdge for an edge of the window (north west, north, north east, west, east,
// south west, south, south east).
func gdkEdge(edge platform.WindowEdge) int32 {
	switch edge {
	case platform.EdgeTopLeft:
		return 0
	case platform.EdgeTop:
		return 1
	case platform.EdgeTopRight:
		return 2
	case platform.EdgeLeft:
		return 3
	case platform.EdgeRight:
		return 4
	case platform.EdgeBottomLeft:
		return 5
	case platform.EdgeBottom:
		return 6
	}
	return 7 // bottom right
}

// focusChanged is the keyboard going to the window or away from it.
func (w *gtkWindow) focusChanged(focused bool) {
	if w.focused == focused {
		return
	}
	w.focused = focused
	w.app.atspiActive(w, focused)
	if focused {
		w.app.active = w
	}
	if w.ime != nil {
		w.ime.setFocused(focused)
	}
	if focused {
		platform.NotifyAppearanceChanged()
		return
	}
	clear(w.keys.down)
	if w.config.OnFocusLost != nil {
		frame, err := w.config.OnFocusLost()
		w.present(frame, err)
	}
	if w.config.WindowMode == platform.WindowPanel {
		// A panel is a transient popup: losing the focus dismisses it.
		panelDismissedAt.Store(monotonicNow())
		w.destroy()
	}
}

// stateChanged follows the window being minimized and maximized.
func (w *gtkWindow) stateChanged(e *gdkEventWindowState) {
	if e.Changed&gdkWindowStateMaximized != 0 {
		w.maximized = e.New&gdkWindowStateMaximized != 0
		w.regionsDirty = true
		if w.wl != nil {
			w.wl.updateRegions(w, int32(w.width+0.5), int32(w.height+0.5))
		}
		if w.config.OnMaximize != nil {
			w.config.OnMaximize(w.maximized)
		}
		if w.config.OnRedraw != nil {
			// The controls show whether the window is maximized.
			w.redraw()
		}
	}
	if e.Changed&gdkWindowStateIconified != 0 {
		w.minimized = e.New&gdkWindowStateIconified != 0
		if w.config.OnMinimize != nil {
			w.config.OnMinimize(w.minimized)
		}
	}
}

// wantsKeyReleases reports whether the focused terminal asked for presses and releases of every key.
func (w *gtkWindow) wantsKeyReleases() bool {
	t := w.text
	return t != nil && t.enabled && t.input.Terminal && t.input.KeyReleases
}

// keyEvent routes one key press or release of the toplevel. It reports 1 when Viva took the key.
func (w *gtkWindow) keyEvent(event uintptr) uintptr {
	if w.nativeKeys {
		return 0 // the page that has the keyboard gets the key from GTK
	}
	if w.config.OnKey == nil && w.config.OnShortcut == nil && w.ime == nil {
		return 0
	}
	e := (*gdkEventKey)(unsafePointer(event))
	if e.Type == gdkKeyRelease {
		w.keyRelease(e, event)
		return 1
	}
	if w.keys.down == nil {
		w.keys.down = map[uint16]bool{}
	}
	repeat := w.keys.down[e.HardwareKeycode]
	w.keys.down[e.HardwareKeycode] = true
	key := normalizedKeysym(e.Keyval)
	modifiers := gdkModifiers(e.State)
	keyEvent := platform.KeyEvent{Key: key, Modifiers: modifiers, Repeat: repeat}
	if modifiers&(input.ModifierControl|input.ModifierMeta) != 0 {
		// A chord is the same on every layout: Ctrl+C is the key that types c.
		keyEvent.Text = w.shortcutText(e, e.State&gdkShiftMask != 0)
	} else if r, ok := keysymRune(e.Keyval); ok {
		keyEvent.Text = string(r)
	}
	if w.wantsKeyReleases() {
		keyEvent.Code = uint32(e.HardwareKeycode)
		keyEvent.Text = w.shortcutText(e, false)
		if r, ok := keysymRune(e.Keyval); ok {
			keyEvent.Typed = string(r)
		}
		if key.IsModifier() {
			keyEvent.Modifiers |= key.Modifier()
			w.deliverKey(keyEvent)
			return 1
		}
	} else if key.IsModifier() {
		return 1
	}
	if w.config.OnShortcut != nil && platform.ShortcutCandidate(keyEvent) && !w.text.ownsChord(keyEvent) {
		frame, handled, err := w.config.OnShortcut(keyEvent)
		w.present(frame, err)
		if handled {
			return 1
		}
	}
	// The input method gets the key first; one it takes comes back as a commit or a preedit change,
	// and one it hands back arrives again as an ordinary key event.
	if w.ime != nil && w.ime.filtering() && w.ime.filter(event) {
		return 1
	}
	w.deliverKey(keyEvent)
	return 1
}

func (w *gtkWindow) keyRelease(e *gdkEventKey, event uintptr) {
	delete(w.keys.down, e.HardwareKeycode)
	if w.ime != nil && w.ime.filtering() && w.ime.filter(event) {
		return
	}
	if !w.wantsKeyReleases() || w.config.OnKey == nil {
		return
	}
	key := normalizedKeysym(e.Keyval)
	modifiers := gdkModifiers(e.State)
	if key.IsModifier() {
		modifiers &^= key.Modifier()
	}
	frame, err := w.config.OnKey(platform.KeyEvent{Key: key, Modifiers: modifiers, Release: true, Code: uint32(e.HardwareKeycode)})
	w.present(frame, err)
}

// deliverKey hands a key to the focused text field, then to the application.
func (w *gtkWindow) deliverKey(keyEvent platform.KeyEvent) {
	if !(keyEvent.Code != 0 && w.wantsKeyReleases()) {
		frame, handled, err := w.text.key(keyEvent)
		if handled || err != nil {
			w.present(frame, err)
			return
		}
	}
	if w.config.OnKey == nil || keyEvent.Key == input.KeyUnknown && keyEvent.Text == "" {
		return
	}
	frame, err := w.config.OnKey(keyEvent)
	w.present(frame, err)
}

// shortcutText is the character a key stands for in a shortcut: the Latin one, whatever layout is
// active (see xcbKeymap.shortcutText). A key no layout gives an ASCII character stands for what it
// types.
func (w *gtkWindow) shortcutText(e *gdkEventKey, shifted bool) string {
	if e.Keyval >= 0x20 && e.Keyval <= 0x7e && shifted == (e.State&gdkShiftMask != 0) {
		return string(rune(e.Keyval))
	}
	var keys, keyvals uintptr
	var count int32
	if w.g.gdkKeymapEntries(w.g.gdkKeymapDefault(), uint32(e.HardwareKeycode), &keys, &keyvals, &count) != 0 {
		defer w.g.gFree(keys)
		defer w.g.gFree(keyvals)
		level := int32(0)
		if shifted {
			level = 1
		}
		for group := int32(0); group < 2; group++ {
			for index := int32(0); index < count; index++ {
				// GdkKeymapKey: keycode uint32, group int32, level int32.
				entry := (*[3]int32)(unsafe.Add(unsafePointer(keys), uintptr(index)*12))
				if entry[1] != group || entry[2] != level {
					continue
				}
				if keyval := *(*uint32)(unsafe.Add(unsafePointer(keyvals), uintptr(index)*4)); keyval >= 0x20 && keyval <= 0x7e {
					return string(rune(keyval))
				}
			}
		}
	}
	if r, ok := keysymRune(e.Keyval); ok {
		return string(r)
	}
	return ""
}
