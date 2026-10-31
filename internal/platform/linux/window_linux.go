package linux

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

type window struct {
	minimumSize               *minimumSizeCache
	mu                        sync.Mutex
	api                       *xcbAPI
	connection                uintptr
	screen                    *xcbScreen
	id, wmProtocols, wmDelete uint32
	redrawAtom                uint32
	caretAtom                 uint32
	closeAtom                 uint32
	commandAtom               uint32
	moveResizeAtom            uint32
	cursor                    uint32
	// themed is the cursor theme's loader, tried once.
	themed        *themedCursors
	cursorsTried  bool
	clipboard     string
	clipboardAtom uint32
	utf8Atom      uint32
	textAtom      uint32
	targetsAtom   uint32
	ownsClipboard bool
	// selections is the PRIMARY selection and the streams of data too large for one
	// property (selection_linux.go).
	selections selectionState
	// width and height are the window's size in device pixels, as X reports it;
	// the application sees them divided by the display scale.
	width, height uint32
	// scaleBits is the display scale as float32 bits (zero is 1): the event loop
	// changes it and Frame reads it from any goroutine.
	scaleBits atomic.Uint32
	// tracker finds the scale and notices when it changes.
	tracker *scaleTracker
	keymap  xcbKeymap
	frame   frameState
	xdnd    xdndAtoms
	// wmStateAtom is WM_STATE, resolved when the application asked for
	// minimize changes; minimized is what it last said.
	wmStateAtom uint32
	minimized   bool
	// minimizeHandler is the application's callback, run on the loop.
	minimizeHandler platform.MinimizeHandler
	// maximizeHandler is the application's callback, run on the loop.
	maximizeHandler platform.MaximizeHandler
	// lastFrame is the content rectangle last reported to OnFrameChange.
	lastFrame ui.Rect
	// pingAtom and syncRequestAtom are the WM_PROTOCOLS messages the window
	// manager checks it is alive with and paces a resize with; syncCounter is
	// nil without the XSYNC extension (window_sync_linux.go).
	pingAtom, syncRequestAtom uint32
	syncCounter               *syncCounter
	syncPending               syncRequest
	// margin is the transparent margin around the content of a window whose frame
	// Viva draws, in device pixels: 0 for any other, and for such a window while
	// it is maximized. frameWanted says the window has a drawn frame at all
	// (window_frame.go).
	margin      uint32
	frameWanted bool
	colormap    uint32
	// shapeRectangles and shapeLibrary set the input region (window_margin_linux.go).
	shapeRectangles func(uintptr, uint8, uint8, uint8, uint32, int16, int16, uint32, unsafe.Pointer) xcbVoidCookie
	shapeLibrary    uintptr
}

var activeWindow atomic.Pointer[window]

// displayScale is the device pixels per logical pixel of this window's display.
func (win *window) displayScale() float32 {
	if bits := win.scaleBits.Load(); bits != 0 {
		return math.Float32frombits(bits)
	}
	return 1
}

func (win *window) storeScale(scale float32) { win.scaleBits.Store(math.Float32bits(scale)) }

// logicalSize is the window's size in logical pixels.
func (win *window) logicalSize() platform.Size {
	scale := win.displayScale()
	return platform.Size{Width: float32(contentSize(win.width, win.margin)) / scale, Height: float32(contentSize(win.height, win.margin)) / scale, Scale: scale}
}

func Run(config platform.Config) error {
	return RunApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

type linuxWindowState struct {
	application             *linuxApplication
	config                  platform.Config
	win                     *window
	text                    *linuxTextInput
	renderer                *vulkanRenderer
	router                  eventRouter
	pendingKeyReleaseDetail uint8
	pendingKeyReleaseTime   uint32
	// held is an event read ahead for key autorepeat or drag motion compression;
	// the loop handles it next.
	held         unsafe.Pointer
	drag         xdndDrag
	resizeEvents uint64
	profile      bool
	started      time.Time
	// frameGeometryDirty means a maximization property changed the client-side
	// margin before its matching ConfigureNotify supplied the final dimensions.
	frameGeometryDirty bool
	// hidden is set by Hide: a compositor drops the contents of an unmapped
	// window, so the first Expose after the window is shown again presents once.
	// The same goes for the first one after the window opened: what was drawn
	// before the window manager mapped it is gone.
	hidden  bool
	exposed bool
}

type linuxApplication struct {
	windows    []*linuxWindowState
	pollFDs    []pollFD
	pollStates []*linuxWindowState // parallel to pollFDs; nil for the status bus
	firstErr   error
	status     *statusNotifier
	quit       bool

	// atspi serves the windows' accessibility trees to a screen reader, once
	// the desktop has accessibility on (see atspiHost).
	atspi *atspiHost

	// ibus is the connection to the input method daemon, shared by the windows
	// that take text; ibusTried says it has been looked for.
	ibus      *ibus.Bus
	ibusTried bool

	// desktop is the connection that delivers the desktop's settings while someone
	// watches them. Retry state is owned by this event loop.
	desktop               *desktopWatch
	desktopRetry          desktopRetry
	desktopRetryRequested atomic.Bool
	now                   func() time.Time

	// live is every window that can still be posted to, for requests that come
	// from another goroutine (see post); the loop's own slice is not safe to read
	// from there.
	liveMu sync.Mutex
	live   []*window
}

// Commands another goroutine posts to a window's event loop through
// win.commandAtom, in the first word of the client message.
const (
	commandHide uint32 = iota + 1
	commandShow
	commandQuit
	commandStatus
	commandDesktop
)

type nativeWindow struct {
	state  *linuxWindowState
	closed atomic.Bool
}

func (window *nativeWindow) Close() {
	if window != nil && !window.closed.Swap(true) {
		window.state.win.postClientMessage(window.state.win.closeAtom, 0)
	}
}

// errnoINTR is the error poll reports when a signal interrupts it.
const errnoINTR = 4

func RunApplication(start platform.ApplicationStart) error {
	backend, err := selectBackend()
	if err != nil {
		return err
	}
	return backend.Run(start)
}

func (application *linuxApplication) Run(start platform.ApplicationStart) error {
	// errno belongs to the thread; the loop reads it right after poll fails, and
	// must still be on the thread that failed.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	application.quit = false
	defer application.closeStatus()
	defer application.atspiClose()
	defer application.closeDesktop()
	// Whoever starts to watch the appearance, from any goroutine, wakes the loop to open the
	// connection that hears the desktop.
	platform.SetWatchHook(func() {
		application.desktopRetryRequested.Store(true)
		application.post(commandDesktop)
	})
	defer platform.SetWatchHook(nil)
	if err := start(application); err != nil {
		application.closeAll()
		return err
	}
	for len(application.windows) > 0 {
		// Whoever started to watch the desktop's settings during the last events is
		// heard from now on.
		application.syncDesktopWatch()
		fds := application.pollFDs
		states := application.pollStates
		if len(fds) == 0 {
			break
		}
		if result := application.windows[0].win.api.poll(&fds[0], uintptr(len(fds)), application.pollTimeout()); result < 0 {
			// A signal that reaches the thread while it sleeps ends the wait
			// early without anything being wrong: wait again.
			if errno := *application.windows[0].win.api.errnoLocation(); errno == errnoINTR {
				continue
			}
			application.fail(fmt.Errorf("linux: poll X11 connections failed (errno %d)", *application.windows[0].win.api.errnoLocation()))
			break
		}
		if application.ibus != nil {
			// A key the input method never answered gives up here, not before.
			application.ibus.Expire()
			application.checkInputMethod()
		}
		for index, descriptor := range fds {
			if descriptor.REvents == 0 {
				continue
			}
			state := states[index]
			if state == nil {
				switch {
				case application.ibus != nil && int(descriptor.FD) == application.ibus.FD():
					application.serveInputMethod()
				case application.desktop != nil && int(descriptor.FD) == application.desktop.conn.FD():
					application.serveDesktop()
				case application.atspi.fd() >= 0 && int(descriptor.FD) == application.atspi.fd():
					application.serveAtspi()
				default:
					application.serveStatus()
				}
				continue
			}
			for application.contains(state) {
				event := state.nextEvent()
				if event == nil {
					if code := state.win.api.connectionHasError(state.win.connection); code != 0 {
						application.fail(fmt.Errorf("linux: XCB connection failed with code %d", code))
					}
					break
				}
				closeWindow, err := state.handleEvent(event)
				state.win.api.free(event)
				if err != nil {
					application.fail(err)
					break
				}
				if closeWindow {
					application.closeWindow(state)
					break
				}
			}
		}
		if application.quit {
			application.closeAll()
		}
	}
	return application.firstErr
}

func (application *linuxApplication) OpenWindow(config platform.Config) (platform.Window, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	state := &linuxWindowState{application: application, config: config, profile: os.Getenv("VIVA_PROFILE_STARTUP") != ""}
	if state.profile {
		state.started = time.Now()
	}
	win, err := openWindow(config)
	if err != nil {
		return nil, err
	}
	state.win = win
	textInput, err := newLinuxTextInput(&config)
	if err != nil {
		win.close()
		return nil, err
	}
	state.text = textInput
	textInput.bindCaretWaker(win.postCaretBlink)
	initialRectangles, initialLabels := textInput.initial()
	renderer, err := newVulkanRenderer(newXcbSurfaceProvider(win.connection, win.id), win.width, win.height, win.displayScale(), config.ClearColor, initialRectangles, initialLabels, win.margin)
	if err != nil {
		textInput.close()
		win.close()
		return nil, err
	}
	state.renderer = renderer
	if win.margin > 0 && !renderer.frame.alpha {
		// The surface cannot composite with alpha, and a transparent margin would
		// show black: the window goes without its drawn frame.
		win.setMargin(0)
		win.frameWanted = false
		if err := renderer.setFrameMargin(0); err != nil {
			renderer.close()
			textInput.close()
			win.close()
			return nil, err
		}
	}
	var ime *linuxIME
	state.router = eventRouter{
		config: config, text: textInput, scale: win.displayScale, modifierKeys: newModifierTracker(),
		clicks: &input.ClickCounter{Interval: 500 * time.Millisecond, Distance: 4},
		present: func(frame platform.Frame) error {
			if ime != nil {
				defer ime.sync()
			}
			if frame.Accessibility != nil {
				application.atspiFrame(state, frame.Accessibility)
			}
			merged, dirty, err := textInput.prepare(frame)
			if err != nil || !dirty {
				return err
			}
			changed, err := renderer.apply(merged)
			if err != nil || !changed {
				return err
			}
			return renderer.present()
		},
	}
	if err := renderer.present(); err != nil {
		renderer.close()
		textInput.close()
		win.close()
		return nil, err
	}
	if config.DynamicTextInput || config.TextInput != nil {
		if ime = application.newIME(win, textInput, state.router); ime != nil {
			state.router.ime = ime
		}
	}
	if config.Waker != nil && config.OnRedraw != nil {
		config.Waker.Bind(win.postRedraw)
	}
	application.windows = append(application.windows, state)
	application.liveMu.Lock()
	application.live = append(application.live, win)
	application.liveMu.Unlock()
	application.rebuildPoll()
	application.atspiOpened(state)
	activeWindow.Store(win)
	if state.profile {
		fmt.Fprintf(os.Stderr, "viva_first_frame_ms=%.3f\n", float64(time.Since(state.started).Microseconds())/1000)
	}
	if os.Getenv("VIVA_TEST_AUTOCLOSE") != "" {
		win.postClientMessage(win.closeAtom, 0)
	}
	application.retryDesktopWatch()
	return &nativeWindow{state: state}, nil
}

func (application *linuxApplication) rebuildPoll() {
	count := len(application.windows)
	if application.status != nil {
		count++
	}
	if application.ibus != nil {
		count++
	}
	if application.desktop != nil {
		count++
	}
	if application.atspi.fd() >= 0 {
		count++
	}
	application.pollFDs = make([]pollFD, count)
	application.pollStates = make([]*linuxWindowState, count)
	for index, state := range application.windows {
		application.pollFDs[index] = pollFD{FD: state.win.api.fileDescriptor(state.win.connection), Events: 1}
		application.pollStates[index] = state
	}
	next := len(application.windows)
	if application.status != nil {
		application.pollFDs[next] = pollFD{FD: int32(application.status.conn.FD()), Events: 1}
		next++
	}
	if application.ibus != nil {
		application.pollFDs[next] = pollFD{FD: int32(application.ibus.FD()), Events: 1}
		next++
	}
	if application.desktop != nil {
		application.pollFDs[next] = pollFD{FD: int32(application.desktop.conn.FD()), Events: 1}
		next++
	}
	if fd := application.atspi.fd(); fd >= 0 {
		application.pollFDs[next] = pollFD{FD: int32(fd), Events: 1}
	}
}

// pollTimeout is how long the loop may sleep: forever, unless the input method
// or a desktop retry has a bounded deadline.
func (application *linuxApplication) pollTimeout() int32 {
	timeout := int32(-1)
	if application.ibus != nil {
		if deadline, waiting := application.ibus.Deadline(); waiting {
			timeout = int32(max(time.Until(deadline).Milliseconds()+1, 0))
		}
	}
	if deadline, waiting := application.desktopRetry.deadline(platform.DesktopWatched(), application.desktop != nil); waiting {
		retryTimeout := int32(max(deadline.Sub(application.timeNow()).Milliseconds()+1, 0))
		if timeout < 0 || retryTimeout < timeout {
			timeout = retryTimeout
		}
	}
	return timeout
}

// takeHeld returns the event read ahead, once.
func (state *linuxWindowState) takeHeld() unsafe.Pointer {
	event := state.held
	state.held = nil
	return event
}

// maxQueuedDragMotions bounds the queued drag positions one dispatch may
// discard, so a queue under continuous input cannot keep the loop inside a
// single window's connection: every batch ends and the outer loop serves the
// other descriptors and the window-closed and error checks between batches.
const maxQueuedDragMotions = 256

// nextEvent returns the next event of the window, with the queued positions of
// a primary-button drag collapsed to their latest one: an application only
// paints the pointer where it ended up, so an obsolete position would cost a
// frame and a present for nothing. Every other event, and a motion that moves
// to another window, root, child, screen or set of held buttons and modifiers,
// is a boundary and is delivered as it was read, in order.
func (state *linuxWindowState) nextEvent() unsafe.Pointer {
	event := state.takeHeld()
	if event == nil {
		event = state.win.api.pollForEvent(state.win.connection)
	}
	if event == nil || (*xcbGenericEvent)(event).ResponseType&0x7f != xcbMotionNotify {
		return event
	}
	motion := (*xcbInputEvent)(event)
	if motion.State&xcbButton1Mask == 0 {
		return event
	}
	// Skip stale drag positions, bounding each read-ahead batch under continuous input.
	for range maxQueuedDragMotions {
		next := state.win.api.pollForEvent(state.win.connection)
		if next == nil {
			break
		}
		if (*xcbGenericEvent)(next).ResponseType&0x7f != xcbMotionNotify {
			state.held = next
			break
		}
		newer := (*xcbInputEvent)(next)
		if newer.Event != motion.Event || newer.Root != motion.Root || newer.Child != motion.Child || newer.State != motion.State || newer.SameScreen != motion.SameScreen {
			state.held = next
			break
		}
		*motion = *newer
		state.win.api.free(next)
	}
	return event
}

// syncMaximized reads the window's maximized state from the window manager
// and notifies the application if it changed.
func (state *linuxWindowState) syncMaximized() (bool, error) {
	win := state.win
	maximized, err := win.readMaximized()
	if err != nil {
		return false, err
	}
	if win.frame.maximizePending {
		if maximized == win.frame.maximizeTarget {
			win.frame.maximizePending = false
		} else {
			maximized = win.frame.maximizeTarget
		}
	}
	return win.setMaximized(maximized), nil
}

func (application *linuxApplication) contains(target *linuxWindowState) bool {
	for _, state := range application.windows {
		if state == target {
			return true
		}
	}
	return false
}

func (application *linuxApplication) closeWindow(target *linuxWindowState) {
	index := -1
	for candidate, state := range application.windows {
		if state == target {
			index = candidate
			break
		}
	}
	if index < 0 {
		return
	}
	copy(application.windows[index:], application.windows[index+1:])
	application.windows[len(application.windows)-1] = nil
	application.windows = application.windows[:len(application.windows)-1]
	application.liveMu.Lock()
	for at, live := range application.live {
		if live == target.win {
			application.live = append(application.live[:at], application.live[at+1:]...)
			break
		}
	}
	application.liveMu.Unlock()
	if target.config.Waker != nil && target.config.OnRedraw != nil {
		target.config.Waker.Unbind()
	}
	if target.profile {
		fmt.Fprintf(os.Stderr, "viva_gpu_submissions=%d\nviva_gpu_full_rebuilds=%d\nviva_gpu_partial_updates=%d\nviva_resize_events=%d\nviva_goroutines=%d\n", target.renderer.submissions, target.renderer.rebuilds, target.renderer.partials, target.resizeEvents, runtime.NumGoroutine())
	}
	if target.router.ime != nil {
		target.router.ime.close()
	}
	application.atspiClosed(target)
	if target.held != nil && target.win.api != nil {
		target.win.api.free(target.held)
		target.held = nil
	}
	target.win.saveClipboardToManager()
	target.renderer.close()
	target.text.close()
	target.win.close()
	application.rebuildPoll()
	if len(application.windows) == 0 {
		activeWindow.CompareAndSwap(target.win, nil)
	} else if activeWindow.Load() == target.win {
		activeWindow.Store(application.windows[len(application.windows)-1].win)
	}
	if target.config.OnClosed != nil {
		target.config.OnClosed()
	}
}

func (application *linuxApplication) closeAll() {
	for len(application.windows) > 0 {
		application.closeWindow(application.windows[len(application.windows)-1])
	}
}

func (application *linuxApplication) fail(err error) {
	if application.firstErr == nil {
		application.firstErr = err
	}
	application.closeAll()
}

// reportFrame tells the application where its content area is now, in logical
// pixels. It asks the X server for the window's origin inside the frame the
// window manager may have put it in, and reports only changes.
// reportFrame reports the content rectangle to OnFrameChange. It returns the
// content's root position when it had to read it, so a caller that also needs
// the position (the minimum-size constraint) shares one query.
func (state *linuxWindowState) reportFrame() (int32, int32, bool) {
	win, config := state.win, state.config
	if config.OnFrameChange == nil || win.frame.maximized {
		return 0, 0, false
	}
	x, y := win.contentOrigin()
	scale := win.displayScale()
	frame := ui.Rect{
		X: float32(x) / scale, Y: float32(y) / scale,
		Width: float32(contentSize(win.width, win.margin)) / scale, Height: float32(contentSize(win.height, win.margin)) / scale,
	}
	if frame == win.lastFrame {
		return x, y, true
	}
	win.lastFrame = frame
	config.OnFrameChange(frame)
	return x, y, true
}

func (state *linuxWindowState) handleEvent(event unsafe.Pointer) (bool, error) {
	win, config, textInput, renderer, router := state.win, state.config, state.text, state.renderer, state.router
	activeWindow.Store(win)
	kind := (*xcbGenericEvent)(event).ResponseType & 0x7f
	if win.margin != 0 {
		win.shiftPointer(kind, event)
	}
	if kind != xcbKeyPress && kind != xcbKeyRelease {
		state.pendingKeyReleaseDetail, state.pendingKeyReleaseTime = 0, 0
	}
	switch kind {
	case xcbDestroyNotify:
		return true, nil
	case xcbClientMessage:
		message := (*xcbClientEvent)(event)
		switch {
		case message.Window == win.id && win.isDragMessage(message.Type):
			return false, state.handleDragMessage(message)
		case message.Format == 32 && message.Window == win.id && message.Type == win.closeAtom:
			return true, nil
		case message.Format == 32 && message.Window == win.id && message.Type == win.commandAtom:
			return false, state.command(binary.LittleEndian.Uint32(message.Data[:4]))
		case message.Format == 32 && message.Window == win.id && message.Type == win.caretAtom:
			generation := binary.LittleEndian.Uint32(message.Data[:4])
			if frame, changed := textInput.blinkCaret(generation); changed {
				dirty, err := renderer.apply(frame)
				if err == nil && dirty {
					err = renderer.present()
				}
				return false, err
			}
		case message.Format == 32 && message.Window == win.id && message.Type == win.redrawAtom && config.OnRedraw != nil:
			frame, err := config.OnRedraw()
			if err == nil {
				err = router.present(frame)
			}
			return false, err
		case message.Format == 32 && message.Window == win.id && message.Type == win.wmProtocols && win.protocolMessage(message):
			// A ping or a sync request: handled.
		case message.Format == 32 && message.Window == win.id && message.Type == win.wmProtocols && binary.LittleEndian.Uint32(message.Data[:4]) == win.wmDelete:
			if config.OnCloseRequest != nil && config.WindowMode != platform.WindowPanel {
				config.OnCloseRequest(func() { win.postClientMessage(win.closeAtom, 0) })
				return false, nil
			}
			return true, nil
		}
	case xcbPropertyNotify:
		if win.incrEvent((*xcbPropertyEvent)(event)) {
			return false, nil
		}
		win.minimumSizeProperty(config, (*xcbPropertyEvent)(event))
		if win.tracker != nil && win.tracker.settingsEvent((*xcbPropertyEvent)(event)) {
			// The desktop's settings changed: the user may have chosen another scale.
			return false, state.refreshScale(true)
		}
		changed, err := win.propertyChanged((*xcbPropertyEvent)(event))
		if err != nil {
			return false, err
		}
		if changed {
			marginChanged := false
			if win.frameWanted {
				marginChanged, err = state.syncMargin()
				if err != nil {
					return false, err
				}
				state.frameGeometryDirty = state.frameGeometryDirty || marginChanged
			}
			if !marginChanged && config.OnRedraw != nil {
				// The buttons show whether the window is maximized.
				var frame platform.Frame
				if frame, err = config.OnRedraw(); err == nil {
					err = router.present(frame)
				}
			}
		}
		return false, err
	case xcbConfigureNotify:
		configure := (*xcbConfigureEvent)(event)
		width, height := uint32(configure.Width), uint32(configure.Height)
		marginChanged := false
		maximizedChanged := false
		if win.frameWanted || win.maximizeHandler != nil {
			var err error
			maximizedChanged, err = state.syncMaximized()
			if err != nil {
				return false, err
			}
		}
		if win.frameWanted {
			var err error
			marginChanged, err = state.syncMargin()
			if err != nil {
				return false, err
			}
		}
		resized := false
		if width > 0 && height > 0 && (width != win.width || height != win.height || marginChanged || state.frameGeometryDirty) {
			win.width, win.height = width, height
			if err := renderer.resize(width, height); err != nil {
				return false, err
			}
			if err := state.resized(); err != nil {
				return false, err
			}
			win.updateInputRegion()
			state.frameGeometryDirty = false
			resized = true
		}
		if !resized && maximizedChanged && config.OnRedraw != nil {
			if frame, err := config.OnRedraw(); err == nil {
				_ = router.present(frame)
			}
		}
		if win.tracker != nil && win.tracker.followsPosition() {
			// A window dragged to another monitor may be on another scale.
			if err := state.refreshScale(false); err != nil {
				return false, err
			}
		}
		x, y, frameKnown := state.reportFrame()
		win.answerSync()
		if frameKnown {
			win.updateMinimumSizeAt(config, x, y)
		} else {
			win.updateMinimumSize(config)
		}
	case xcbButtonPress:
		inputEvent := (*xcbInputEvent)(event)
		if win.beginWindowMove(config, inputEvent) {
			break
		}
		handledScroll, err := router.scroll(inputEvent)
		if err == nil && !handledScroll && xcbPointerButton(inputEvent.Detail) != 0 {
			err = router.pointer(platform.PointerDown, inputEvent)
		}
		return false, err
	case xcbButtonRelease:
		inputEvent := (*xcbInputEvent)(event)
		win.endPress()
		if xcbPointerButton(inputEvent.Detail) != 0 {
			return false, router.pointer(platform.PointerUp, inputEvent)
		}
	case xcbMotionNotify, xcbEnterNotify:
		consumed, cancelled := win.dragMotion((*xcbInputEvent)(event))
		if cancelled {
			if err := router.pointerExit(); err != nil {
				return false, err
			}
		}
		if consumed {
			return false, nil
		}
		return false, router.pointer(platform.PointerMove, (*xcbInputEvent)(event))
	case xcbLeaveNotify:
		return false, router.pointerExit()
	case xcbKeyPress:
		inputEvent := (*xcbInputEvent)(event)
		repeat := state.pendingKeyReleaseDetail == inputEvent.Detail && state.pendingKeyReleaseTime == inputEvent.Time
		state.pendingKeyReleaseDetail, state.pendingKeyReleaseTime = 0, 0
		return false, router.key(&win.keymap, inputEvent, repeat)
	case xcbKeyRelease:
		inputEvent := (*xcbInputEvent)(event)
		state.pendingKeyReleaseDetail, state.pendingKeyReleaseTime = inputEvent.Detail, inputEvent.Time
		if state.keyReleased(inputEvent) {
			return false, router.release(&win.keymap, inputEvent)
		}
	case xcbMappingNotify:
		mapping := (*xcbMappingEvent)(event)
		if config.OnKey != nil && mapping.Request == xcbMappingKeyboard {
			if err := win.reloadKeymap(); err != nil {
				return false, err
			}
			// A held key can change class with the layout: AltGr may become Alt.
			state.refreshModifierKeys()
			return false, nil
		}
	case xcbFocusIn:
		state.application.atspiActive(state, true)
		platform.NotifyAppearanceChanged()
		if win.tracker != nil {
			// A desktop whose manager was not running when the window opened, or
			// changed it without our hearing, is asked again.
			if err := state.refreshScale(true); err != nil {
				return false, err
			}
		}
		focus := (*xcbFocusEvent)(event)
		realFocus := focus.Mode != xcbNotifyGrab && focus.Mode != xcbNotifyUngrab && focus.Detail != xcbNotifyInferior
		// Only a focus that reached this window starts a new span. NotifyPointer
		// and the virtual details accompany a change that happened elsewhere and
		// must not forget keys held here.
		keyboardFocus := focus.Detail == xcbNotifyAncestor || focus.Detail == xcbNotifyNonlinear
		if keyboardFocus {
			// A key pressed or released while another window or a grab held the
			// keyboard left no event here. The server's bitmap says which keys
			// are really down, including ones still held across the change.
			if keys, ok := win.keysDown(); ok {
				router.modifierKeys.seed(keys, func(scancode uint32) input.Modifiers {
					if scancode > 255 {
						return 0
					}
					key := win.keymap.key(uint8(scancode))
					return modifierBit(win.keymap.keysym(uint8(scancode), 0), key)
				})
			} else {
				router.modifierKeys.reset()
			}
		}
		if router.ime != nil && realFocus {
			router.ime.setFocused(true)
		}
	case xcbFocusOut:
		state.application.atspiActive(state, false)
		focus := (*xcbFocusEvent)(event)
		focusLeft := focus.Detail != xcbNotifyInferior
		// Moving into or out of a grab is not the user leaving.
		realFocusLoss := focusLeft && focus.Mode != xcbNotifyGrab && focus.Mode != xcbNotifyUngrab
		if focusLeft && config.OnFocusLost != nil {
			// A grab (a window manager's Alt+Tab) takes the keys' releases too.
			frame, err := config.OnFocusLost()
			if err := router.presentFrame(frame, err); err != nil {
				return false, err
			}
		}
		if router.ime != nil && realFocusLoss {
			router.ime.setFocused(false)
		}
		// The held key set is left for the next FocusIn, which starts a new
		// focus span; dropping it here would lose a pair that is still down
		// across a grab.
		if config.WindowMode == platform.WindowPanel && realFocusLoss {
			panelDismissedAt.Store(monotonicNow())
			return true, nil
		}
	case xcbExpose:
		if (state.hidden || !state.exposed) && (*xcbExposeEvent)(event).Count == 0 {
			state.hidden, state.exposed = false, true
			return false, renderer.present()
		}
	case xcbSelectionRequest:
		return false, win.handleSelectionRequest((*xcbSelectionRequestEvent)(event))
	case xcbSelectionClear:
		win.handleSelectionClear((*xcbSelectionClearEvent)(event))
	}
	return false, nil
}

// resized tells the application the window's logical size and presents the frame
// it answers with. The renderer already has the device size.
func (state *linuxWindowState) resized() error {
	win, config, renderer := state.win, state.config, state.renderer
	if config.OnResize != nil {
		frame, err := config.OnResize(win.logicalSize())
		if err == nil {
			var merged platform.Frame
			var dirty bool
			merged, dirty, err = state.text.prepare(frame)
			if err == nil && dirty {
				_, err = renderer.apply(merged)
			}
		}
		if err != nil {
			return err
		}
	}
	if err := renderer.present(); err != nil {
		return err
	}
	state.resizeEvents++
	return nil
}

// refreshScale looks for the display scale again, after the desktop's settings
// changed (reload) or the window moved, and applies it when it differs.
func (state *linuxWindowState) refreshScale(reload bool) error {
	win := state.win
	if win.tracker == nil {
		return nil
	}
	if reload {
		win.tracker.reload()
		// The look, the motion preference or the fonts may be what changed.
		state.application.announce(win.tracker.publish())
	}
	x, y := win.rootOrigin()
	if win.tracker.followsPosition() || reload {
		win.tracker.reloadMonitors()
	}
	scale := win.tracker.scale(int(x)+int(win.width)/2, int(y)+int(win.height)/2)
	if scale == win.displayScale() {
		return nil
	}
	return state.applyScale(scale)
}

// applyScale moves the window to a new scale. The logical size is kept: the
// window asks for the device size that gives it, and what the window manager
// grants (a maximized window stays as it is) is what ConfigureNotify then
// reports. Until that arrives the window is drawn at the new scale on the old
// size, and the application is told the logical size that gives.
func (state *linuxWindowState) applyScale(scale float32) error {
	win := state.win
	old := win.displayScale()
	logicalWidth := int(math.Round(float64(contentSize(win.width, win.margin)) / float64(old)))
	logicalHeight := int(math.Round(float64(contentSize(win.height, win.margin)) / float64(old)))
	win.storeScale(scale)
	if err := state.renderer.setScale(scale); err != nil {
		return err
	}
	margin := uint32(0)
	if win.margin > 0 {
		// The margin is a whole number of device pixels at every scale.
		margin = frameMargin(scale)
		win.setMargin(margin)
		if err := state.renderer.setFrameMargin(margin); err != nil {
			return err
		}
	}
	win.updateMinimumSize(state.config)
	size := [2]uint32{deviceLength(logicalWidth, scale) + 2*margin, deviceLength(logicalHeight, scale) + 2*margin}
	win.api.configureWindow(win.connection, win.id, xcbConfigWindowWidth|xcbConfigWindowHeight, &size[0])
	win.api.flush(win.connection)
	return state.resized()
}

func validateConfig(config platform.Config) error {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return fmt.Errorf("linux: only amd64 and arm64 are supported")
	}
	if config.Width < 1 || config.Height < 1 || config.Width > math.MaxUint16 || config.Height > math.MaxUint16 {
		return fmt.Errorf("linux: X11 window dimensions must be between 1 and %d, got %dx%d", uint32(math.MaxUint16), config.Width, config.Height)
	}
	if config.WindowMode > platform.WindowPanel {
		return fmt.Errorf("linux: invalid window mode %d", config.WindowMode)
	}
	if err := validateRectangles(config.Rectangles); err != nil {
		return err
	}
	if err := validateInitialTextLabels(config.TextLabels); err != nil {
		return err
	}
	return nil
}

func openWindow(config platform.Config) (*window, error) {
	api, err := loadXCB()
	if err != nil {
		return nil, err
	}
	var screenIndex int32
	connection := api.connect(nil, &screenIndex)
	if connection == 0 || api.connectionHasError(connection) != 0 {
		if connection != 0 {
			api.disconnect(connection)
		}
		api.close()
		return nil, fmt.Errorf("linux: connect to X server")
	}
	iterator := api.rootsIterator(api.getSetup(connection))
	for index := int32(0); index < screenIndex && iterator.Remaining > 0; index++ {
		api.screenNext(&iterator)
	}
	if iterator.Data == nil || iterator.Remaining == 0 {
		api.disconnect(connection)
		api.close()
		return nil, fmt.Errorf("linux: X server has no screen")
	}
	win := &window{api: api, connection: connection, screen: iterator.Data}
	// The scale decides the window's size in device pixels. Where the window
	// will be is not known yet; the requested origin is the best guess of which
	// monitor, and the first ConfigureNotify corrects it.
	win.tracker = newScaleTracker(api, connection, iterator.Data, screenIndex)
	win.tracker.publish()
	guessX, guessY := -1, -1
	if config.HasOrigin {
		guessX, guessY = int(config.Origin.X), int(config.Origin.Y)
	}
	scale := win.tracker.scale(guessX, guessY)
	win.storeScale(scale)
	win.width, win.height = deviceLength(config.Width, scale), deviceLength(config.Height, scale)
	frameVisual := uint32(0)
	if config.WindowMode == platform.WindowCustomTitlebar {
		// The application draws the frame of this window, and on a desktop that
		// composites it draws the shadow and the round corners too, in a margin.
		if visual, framed := win.wantsFrame(screenIndex); framed {
			frameVisual, win.frameWanted = visual, true
			win.margin = frameMargin(scale)
			win.width, win.height = win.width+2*win.margin, win.height+2*win.margin
		}
	}
	if win.width > math.MaxUint16 || win.height > math.MaxUint16 {
		win.close()
		return nil, fmt.Errorf("linux: window of %dx%d at %gx is beyond the %d pixels X11 allows", config.Width, config.Height, scale, uint32(math.MaxUint16))
	}
	if config.OnKey != nil || config.OnShortcut != nil {
		if err := win.reloadKeymap(); err != nil {
			win.close()
			return nil, err
		}
	}
	win.id = api.generateID(connection)
	eventMask := uint32(xcbEventExposure | xcbEventStructure)
	// Focus changes are heard by every window: a panel dismisses itself when it
	// loses focus, a window that takes text tells the input method when the
	// keyboard is its own, and gaining it is when a desktop that does not
	// announce its light or dark switch is asked again which it is.
	eventMask |= xcbEventFocusChange
	if config.WindowMode == platform.WindowCustomTitlebar || config.OnMinimize != nil || config.OnMaximize != nil || config.MinWidth > 0 || config.MinHeight > 0 {
		eventMask |= xcbEventPropertyChange
	}
	if config.OnPointer != nil {
		eventMask |= xcbEventButtonPress | xcbEventButtonRelease | xcbEventEnterWindow | xcbEventLeaveWindow | xcbEventPointerMotion
	}
	if config.OnScroll != nil {
		eventMask |= xcbEventButtonPress
	}
	if config.OnKey != nil || config.OnShortcut != nil {
		eventMask |= xcbEventKeyPress | xcbEventKeyRelease
	}
	values := [2]uint32{win.screen.BlackPixel, eventMask}
	x, y := win.panelOrigin(config)
	if win.frameWanted {
		win.createFrameWindow(frameVisual, eventMask, x, y)
		win.setFrameExtents(win.margin)
		win.updateInputRegion()
	} else {
		api.createWindow(connection, xcbCopyFromParent, win.id, win.screen.Root, x, y, uint16(win.width), uint16(win.height), 0, xcbWindowClassInput, win.screen.RootVisual, xcbCWBackPixel|xcbCWEventMask, &values[0])
	}
	if config.WindowMode != platform.WindowTitled {
		motifHints, atomErr := api.atom(connection, "_MOTIF_WM_HINTS")
		if atomErr != nil {
			win.close()
			return nil, atomErr
		}
		hints := [5]uint32{2, 0, 0, 0, 0}
		api.changeProperty(connection, xcbPropertyReplace, win.id, motifHints, motifHints, 32, uint32(len(hints)), unsafe.Pointer(&hints[0]))
	}
	if config.WindowMode == platform.WindowCustomTitlebar || config.OnMaximize != nil {
		win.moveResizeAtom, err = api.atom(connection, "_NET_WM_MOVERESIZE")
		if err == nil {
			err = win.openFrame()
		}
		if err != nil {
			win.close()
			return nil, err
		}
		if config.WindowMode == platform.WindowCustomTitlebar {
			win.installActions(config.Actions)
		}
	}
	if config.OnMaximize != nil {
		win.maximizeHandler = config.OnMaximize
	}
	if config.OnMinimize != nil {
		win.wmStateAtom, err = api.atom(connection, "WM_STATE")
		if err != nil {
			win.close()
			return nil, err
		}
		win.minimizeHandler = config.OnMinimize
	}
	if config.HasOrigin && config.WindowMode != platform.WindowPanel {
		// The user-specified position hint: without it a window manager places the
		// window as it likes and ignores where it was created.
		const usPosition = 1
		hints := [18]uint32{usPosition, uint32(int32(x)), uint32(int32(y))}
		if config.MinWidth > 0 || config.MinHeight > 0 {
			win.minimumSize = &minimumSizeCache{normalHints: hints}
		}
		api.changeProperty(connection, xcbPropertyReplace, win.id, xcbAtomWMNormalHints, xcbAtomWMSizeHints, 32, uint32(len(hints)), unsafe.Pointer(&hints[0]))
	}
	win.updateMinimumSize(config)
	if class := wmClass(config.Class); class != nil {
		api.changeProperty(connection, xcbPropertyReplace, win.id, xcbAtomWMClass, xcbAtomString, 8, uint32(len(class)), unsafe.Pointer(&class[0]))
	}
	if icon := netWMIcon(config.Icon); len(icon) > 0 {
		if atom, err := api.atom(connection, "_NET_WM_ICON"); err == nil {
			api.changeProperty(connection, xcbPropertyReplace, win.id, atom, xcbAtomCardinal, 32, uint32(len(icon)), unsafe.Pointer(&icon[0]))
		}
	}
	win.setIdentity(config.Title)
	win.wmProtocols, err = api.atom(connection, "WM_PROTOCOLS")
	if err != nil {
		win.close()
		return nil, err
	}
	win.wmDelete, err = api.atom(connection, "WM_DELETE_WINDOW")
	if err != nil {
		win.close()
		return nil, err
	}
	win.closeAtom, err = api.atom(connection, "VIVA_CLOSE_WINDOW")
	if err != nil {
		win.close()
		return nil, err
	}
	win.commandAtom, err = api.atom(connection, "VIVA_COMMAND")
	if err != nil {
		win.close()
		return nil, err
	}
	if config.WindowMode == platform.WindowPanel {
		if err := win.markPanel(config, x, y); err != nil {
			win.close()
			return nil, err
		}
	}
	if config.Waker != nil && config.OnRedraw != nil {
		win.redrawAtom, err = api.atom(connection, "VIVA_REDRAW")
		if err != nil {
			win.close()
			return nil, err
		}
	}
	if config.TextInput != nil || config.DynamicTextInput {
		win.caretAtom, err = api.atom(connection, "VIVA_CARET_BLINK")
		if err != nil {
			win.close()
			return nil, err
		}
	}
	protocols := append([]uint32{win.wmDelete}, win.openSync()...)
	api.changeProperty(connection, xcbPropertyReplace, win.id, win.wmProtocols, xcbAtomAtom, 32, uint32(len(protocols)), unsafe.Pointer(&protocols[0]))
	if config.OnFileDrop != nil {
		if err := win.setupFileDrop(); err != nil {
			win.close()
			return nil, err
		}
	}
	api.mapWindow(connection, win.id)
	if api.flush(connection) <= 0 || api.connectionHasError(connection) != 0 {
		win.close()
		return nil, fmt.Errorf("linux: create X window")
	}
	return win, nil
}

func (win *window) postRedraw() {
	win.postClientMessage(win.redrawAtom, 0)
}

func (win *window) postCaretBlink(generation uint32) {
	win.postClientMessage(win.caretAtom, generation)
}

func (win *window) postClientMessage(atom, value uint32) {
	win.mu.Lock()
	defer win.mu.Unlock()
	if win.connection == 0 || atom == 0 {
		return
	}
	event := xcbClientEvent{
		ResponseType: xcbClientMessage,
		Format:       32,
		Window:       win.id,
		Type:         atom,
	}
	binary.LittleEndian.PutUint32(event.Data[:4], value)
	win.api.sendEvent(win.connection, 0, win.id, 0, (*byte)(unsafe.Pointer(&event)))
	win.api.flush(win.connection)
}

func (win *window) close() {
	win.mu.Lock()
	defer win.mu.Unlock()
	if win.tracker != nil {
		win.tracker.close()
		win.tracker = nil
	}
	win.themed.close()
	win.themed = nil
	if win.connection != 0 {
		win.closeSync()
		win.closeFrame()
		if win.cursor != 0 {
			win.api.freeCursor(win.connection, win.cursor)
			win.cursor = 0
		}
		if win.id != 0 {
			win.api.destroyWindow(win.connection, win.id)
			win.api.flush(win.connection)
		}
		win.api.disconnect(win.connection)
		win.connection = 0
	}
	if win.api != nil {
		win.api.close()
		win.api = nil
	}
}

// refreshModifierKeys re-reads the bit each held key sets under the keymap the
// server just installed.
func (state *linuxWindowState) refreshModifierKeys() {
	tracker := state.router.modifierKeys
	if tracker == nil || state.win == nil {
		return
	}
	for scancode := range tracker.down {
		if scancode > 255 {
			continue
		}
		key := state.win.keymap.key(uint8(scancode))
		tracker.down[scancode] = modifierBit(state.win.keymap.keysym(uint8(scancode), 0), key)
	}
}

// keyReleased handles a key coming up. X reports an autorepeating key as a
// release and a press at the same instant, which is not a release, so the next
// event is read first: when it is that press, nothing is sent anywhere. The
// event is kept for the loop to handle next. It tells the input method and
// reports whether the key is really up, for a terminal that wants releases.
func (state *linuxWindowState) keyReleased(release *xcbInputEvent) bool {
	ime := state.router.ime
	wanted := state.router.wantsKeyReleases()
	if (ime == nil || !ime.active) && !wanted {
		// No delivery path runs for this release; forget the key here so it
		// cannot linger as held when a terminal takes over.
		state.router.modifierKeys.release(uint32(release.Detail))
		return false
	}
	if next := state.win.api.pollForEvent(state.win.connection); next != nil {
		state.held = next
		if (*xcbGenericEvent)(next).ResponseType&0x7f == xcbKeyPress {
			press := (*xcbInputEvent)(next)
			if press.Detail == release.Detail && press.Time == release.Time {
				return false
			}
		}
	}
	if ime != nil && ime.active {
		ime.release(&state.win.keymap, release)
	}
	if !wanted {
		// The input method took this release; release() is not called for it.
		state.router.modifierKeys.release(uint32(release.Detail))
	}
	return wanted
}
