package linux

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// A WindowCustomTitlebar window has no frame from the window manager, so what
// a frame does is done here: the minimize, maximize and close buttons are drawn
// by Viva (app/window_controls.go) and reach the window through
// platform.WindowActions, the edges resize it, and a double click on the strip
// maximizes it. Each of those is a request to the window manager, which stays
// in charge of the window; nothing here moves or resizes it directly.

const (
	xcbEventPropertyChange = 1 << 22
	xcbButton1Mask         = 1 << 8
	xcbPropertyNotify      = 28

	// moveResizeMove is the _NET_WM_MOVERESIZE direction that moves the window;
	// the eight edges are 0 to 7, clockwise from the top left (moveResizeDirection).
	moveResizeMove = 8

	netWMStateToggle = 2
	iconicState      = 3
	sourceApp        = 1

	// doubleClickMillis bounds two presses on the strip that
	// make a double click.
	doubleClickMillis = 400
)

type xcbPropertyEvent struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Window       uint32
	Atom         uint32
	Time         uint32
	State        uint8
}

// frameState is what a custom-titlebar window needs beyond a plain one.
type frameState struct {
	stateAtom, changeStateAtom uint32
	maximizedVertical          uint32
	maximizedHorizontal        uint32
	fullscreen                 uint32
	maximized                  bool
	maximizePending            bool
	maximizeTarget             bool
	lastPress                  uint32
	lastPressX, lastPressY     int16
	pressed                    bool
	// pending is a press on the frame that has not moved yet: the window
	// manager is asked to move or resize the window only once the pointer
	// drags, since a request made at the press can reach it after the button
	// came up, and it would then wait for another press to end the drag.
	pending bool
	// delivered marks a pending press the application has seen too, because it
	// lies on a backdrop that dismisses its overlay on a click: the move, if
	// the pointer drags, cancels that press.
	delivered                  bool
	pendingDirection           uint32
	pendingRootX, pendingRootY int16
}

// openFrame resolves the atoms the frame operations use.
func (win *window) openFrame() error {
	for _, item := range []struct {
		name string
		atom *uint32
	}{
		{"_NET_WM_STATE", &win.frame.stateAtom},
		{"_NET_WM_STATE_MAXIMIZED_VERT", &win.frame.maximizedVertical},
		{"_NET_WM_STATE_MAXIMIZED_HORZ", &win.frame.maximizedHorizontal},
		{"_NET_WM_STATE_FULLSCREEN", &win.frame.fullscreen},
		{"WM_CHANGE_STATE", &win.frame.changeStateAtom},
	} {
		atom, err := win.api.atom(win.connection, item.name)
		if err != nil {
			return err
		}
		*item.atom = atom
	}
	return nil
}

// installActions fills the actions behind Viva's window buttons.
func (win *window) installActions(actions *platform.WindowActions) {
	if actions == nil {
		return
	}
	*actions = platform.WindowActions{
		Minimize:       win.minimize,
		ToggleMaximize: win.toggleMaximize,
		RequestClose:   func() { win.postClientMessage(win.wmProtocols, win.wmDelete) },
		Maximized:      func() bool { return win.frame.maximized },
	}
}

// sendToRoot sends a client message to the root window, where the window
// manager listens for requests about a window it manages.
func (win *window) sendToRoot(atom uint32, data [5]uint32) {
	message := xcbClientEvent{ResponseType: xcbClientMessage, Format: 32, Window: win.id, Type: atom}
	for index, value := range data {
		binary.LittleEndian.PutUint32(message.Data[index*4:], value)
	}
	win.api.sendEvent(win.connection, 0, win.screen.Root, xcbEventSubstructureRedirect|xcbEventSubstructureNotify, (*byte)(unsafe.Pointer(&message)))
	win.api.flush(win.connection)
}

func (win *window) minimize() {
	win.sendToRoot(win.frame.changeStateAtom, [5]uint32{iconicState})
}

func (win *window) toggleMaximize() {
	win.frame.maximizePending = true
	win.frame.maximizeTarget = !win.frame.maximized
	win.sendToRoot(win.frame.stateAtom, [5]uint32{netWMStateToggle, win.frame.maximizedVertical, win.frame.maximizedHorizontal, sourceApp})
}

func (win *window) setMaximized(maximized bool) bool {
	if maximized == win.frame.maximized {
		return false
	}
	win.frame.maximized = maximized
	if win.maximizeHandler != nil {
		win.maximizeHandler(maximized)
	}
	return true
}

// readMaximized reports whether the window manager lists the window as
// maximized in both directions. Fullscreen is a separate native state.
func (win *window) readMaximized() (bool, error) {
	cookie := win.api.getProperty(win.connection, 0, win.id, win.frame.stateAtom, xcbAtomAtom, 0, 64)
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
		if reply != nil {
			win.api.free(unsafe.Pointer(reply))
		}
		return false, fmt.Errorf("linux: read window state")
	}
	if reply == nil {
		return false, fmt.Errorf("linux: read window state returned no reply")
	}
	defer win.api.free(unsafe.Pointer(reply))
	length := win.api.getPropertyLength(reply)
	if reply.Format != 32 || length <= 0 {
		win.observeFullscreen(false)
		return false, nil
	}
	return win.observeWindowState(unsafe.Slice((*uint32)(win.api.getPropertyValue(reply)), int(length)/4)), nil
}

func (win *window) observeWindowState(atoms []uint32) bool {
	var vertical, horizontal, fullscreen bool
	for _, atom := range atoms {
		vertical = vertical || atom == win.frame.maximizedVertical
		horizontal = horizontal || atom == win.frame.maximizedHorizontal
		fullscreen = fullscreen || atom == win.frame.fullscreen
	}
	win.observeFullscreen(fullscreen)
	return vertical && horizontal
}

// propertyChanged follows the window state, and reports whether the maximized
// state changed, which the buttons show.
func (win *window) propertyChanged(event *xcbPropertyEvent) (bool, error) {
	if event.Window != win.id {
		return false, nil
	}
	if win.wmStateAtom != 0 && event.Atom == win.wmStateAtom {
		// WM_STATE is how the window manager says the window is iconic
		// (minimized) or normal again.
		minimized, err := win.readMinimized()
		if err != nil || minimized == win.minimized {
			return false, err
		}
		win.minimized = minimized
		if win.minimizeHandler != nil {
			win.minimizeHandler(minimized)
		}
		return false, nil
	}
	if event.Atom != win.frame.stateAtom {
		return false, nil
	}
	beforeFullscreen := win.fullscreen.Load()
	maximized, err := win.readMaximized()
	if err != nil {
		return false, err
	}
	win.frame.maximizePending = false
	changed := win.setMaximized(maximized)
	return changed || beforeFullscreen != win.fullscreen.Load(), nil
}

// readMinimized reports whether WM_STATE says the window is iconic. A manager
// that does not maintain WM_STATE reads as normal, which is what a window that
// has never been minimized is.
func (win *window) readMinimized() (bool, error) {
	cookie := win.api.getProperty(win.connection, 0, win.id, win.wmStateAtom, 0, 0, 2)
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
		if reply != nil {
			win.api.free(unsafe.Pointer(reply))
		}
		return false, fmt.Errorf("linux: read WM_STATE")
	}
	if reply == nil {
		return false, fmt.Errorf("linux: read WM_STATE returned no reply")
	}
	defer win.api.free(unsafe.Pointer(reply))
	if reply.Format != 32 || win.api.getPropertyLength(reply) < 4 {
		return false, nil
	}
	return *(*uint32)(win.api.getPropertyValue(reply)) == iconicState, nil
}

// moveResizeDirection is the _NET_WM_MOVERESIZE direction that resizes from an
// edge: platform.WindowEdge runs in the same order, from one.
func moveResizeDirection(edge platform.WindowEdge) uint32 { return uint32(edge) - 1 }

// beginWindowMove handles a primary press on a custom-titlebar window that
// belongs to the frame: an edge starts a resize, a move region a move, and a
// second press on a move region maximizes. It reports whether the press was
// consumed. A press on a backdrop over a move region is not: the application
// takes it to dismiss its overlay, and the window moves only if the pointer
// drags.
func (win *window) beginWindowMove(config platform.Config, event *xcbInputEvent) bool {
	if win.fullscreen.Load() {
		return false
	}
	if config.WindowMode != platform.WindowCustomTitlebar || event.Detail != xcbButtonLeft {
		return false
	}
	// The application's coordinates are logical; the event's are device pixels.
	scale := win.displayScale()
	x, y := float32(event.EventX)/scale, float32(event.EventY)/scale
	press := platform.PressClient
	if config.PressAt != nil {
		press = config.PressAt(x, y)
	}
	direction, ok := uint32(moveResizeMove), false
	if !win.frame.maximized {
		// The top edge yields to a control, so the buttons work up to the top.
		edge := platform.EdgeAt(x, y, float32(contentSize(win.width, win.margin))/scale, float32(contentSize(win.height, win.margin))/scale, press == platform.PressControl)
		if ok = edge != platform.EdgeNone; ok {
			direction = moveResizeDirection(edge)
		}
	}
	if !ok {
		if press == platform.PressClient || press == platform.PressControl {
			return false
		}
		if press == platform.PressMoveOnDrag {
			win.frame.pending, win.frame.delivered, win.frame.pendingDirection = true, true, moveResizeMove
			win.frame.pendingRootX, win.frame.pendingRootY = event.RootX, event.RootY
			return false
		}
		if win.doubleClick(event) {
			win.toggleMaximize()
			return true
		}
		direction = moveResizeMove
	}
	win.frame.pending, win.frame.delivered, win.frame.pendingDirection = true, false, direction
	win.frame.pendingRootX, win.frame.pendingRootY = event.RootX, event.RootY
	return true
}

// defaultDragSlop is the base distance in logical pixels before motion on
// the frame becomes a window manager drag or resize.
const defaultDragSlop = 4

func (win *window) dragSlop() int16 {
	scale := float32(1)
	if win != nil {
		scale = max(win.displayScale(), 1)
	}
	return int16(math.Round(float64(defaultDragSlop * scale)))
}

func (win *window) doubleClickSlop() int16 {
	scale := float32(1)
	if win != nil {
		scale = max(win.displayScale(), 1)
	}
	return int16(math.Round(float64(defaultDragSlop * scale)))
}

// dragMotion starts the move or resize a press on the frame asked for once the
// pointer has dragged. consumed reports that a press was waiting and the motion
// is not for the application; cancelled, that the application had the press and
// must be told it ended, since the window manager takes the button release.
func (win *window) dragMotion(event *xcbInputEvent) (consumed, cancelled bool) {
	frame := &win.frame
	if !frame.pending {
		return false, false
	}
	if event.State&xcbButton1Mask == 0 {
		frame.pending = false
		return false, false
	}
	slop := win.dragSlop()
	dx, dy := event.RootX-frame.pendingRootX, event.RootY-frame.pendingRootY
	if dx > -slop && dx < slop && dy > -slop && dy < slop {
		return !frame.delivered, false
	}
	frame.pending = false
	win.api.ungrabPointer(win.connection, event.Time)
	win.sendToRoot(win.moveResizeAtom, [5]uint32{uint32(int32(event.RootX)), uint32(int32(event.RootY)), frame.pendingDirection, xcbButtonLeft, sourceApp})
	return true, frame.delivered
}

// endPress forgets a press on the frame that never dragged.
func (win *window) endPress() { win.frame.pending = false }

// doubleClick records a press on the strip and reports whether it completes a
// double click.
func (win *window) doubleClick(event *xcbInputEvent) bool {
	frame := &win.frame
	slop := win.doubleClickSlop()
	near := func(a, b int16) bool { return a-b <= slop && b-a <= slop }
	double := frame.pressed && event.Time-frame.lastPress <= doubleClickMillis && near(event.EventX, frame.lastPressX) && near(event.EventY, frame.lastPressY)
	frame.pressed, frame.lastPress, frame.lastPressX, frame.lastPressY = !double, event.Time, event.EventX, event.EventY
	return double
}
