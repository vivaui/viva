//go:build android

// Package android runs a Viva program in an Android process. The program is a shared
// library loaded by the host's Activity (hostsrc): the host owns the UI thread, the
// surface and the input method, and the program draws on the surface with Vulkan from
// a thread of its own. What happens on the UI thread reaches the program as events on
// a queue, and the program is idle, with no timer and no wake-up, until one comes.
package android

/*
#include <android/native_window.h>
#include "jni_android.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/vulkan"
)

// eventKind is what the host reports.
type eventKind uint8

const (
	eventSurface         eventKind = iota + 1 // a surface, of a size, on a display of a density
	eventSurfaceLost                          // the surface is going away: let go of it, then answer
	eventTouch                                // a finger, a pen or a mouse
	eventVisible                              // the activity is in front or behind
	eventLowMemory                            // the system is short of memory
	eventClose                                // the user asked to close the window
	eventWake                                 // something was posted
	eventRedraw                               // the application asked for a frame
	eventText                                 // the input method edits the text input
	eventKey                                  // a key of a keyboard
	eventHover                                // a mouse over the surface
	eventHoverExit                            // the mouse left it
	eventScroll                               // a wheel
	eventAppearance                           // the theme, the language or the animation setting may have changed
	eventAccessibility                        // a screen reader started or stopped
	eventAccessible                           // a screen reader asks something of a node
	eventInsets                               // the system bars, the cutout or the keyboard cover other edges
	eventLifecycle                            // the activity went to the background or came back
	eventSelectionHandle                      // a selection handle is held or dragged
	eventSelectionAction                      // an item of the selection's menu
	eventRedrawNeeded                         // the system shows the surface once a frame is on it: draw one, then answer
)

// event is one thing that happened, small enough to queue by value.
type event struct {
	kind    eventKind
	surface *C.ANativeWindow
	width   int32
	height  int32
	density float32
	// A touch: the Android action, the pointer's id, where, the tool, the buttons
	// held, the modifier state and when.
	action, pointer, tool, buttons, modifiers int32
	x, y                                      float32
	time                                      int64
	// ack is answered once the program no longer uses the surface it is told to let go of,
	// or has applied the edit it was sent.
	ack chan struct{}
	// edit is eventText's operation.
	edit edit
	// A key: its Android code, the character it types and whether it repeats; the modifiers
	// are modifiers above. A scroll's distances are dx and dy.
	keyCode, unicode int32
	repeat           bool
	dx, dy           float32
	// An accessibility request: the node (an index of the tree last sent), the action (above), the
	// text and the span; the number is dx.
	index      int32
	text       string
	start, end int32
	// visible is eventVisible's state.
	visible bool
}

// queue is the events from the host, in order. Moves of a finger may be dropped when
// the program falls behind; nothing else is.
var queue = make(chan event, 1024)

// send queues an event for the program.
func send(ev event) { queue <- ev }

// sendMove queues a finger's move, unless the program is so far behind that there
// are no more places: the next move says where the finger is.
func sendMove(ev event) {
	select {
	case queue <- ev:
	default:
	}
}

// Run enters the program's event loop with one window.
func Run(config platform.Config) error {
	return RunApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

// RunApplication runs the event loop on the calling goroutine, which stays on one OS
// thread for as long as it does. It returns when the window is closed.
func RunApplication(start platform.ApplicationStart) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if C.viva_attach_thread() == 0 {
		return errors.New("android: the process has no Java virtual machine; the program must run as the host's library")
	}
	app := &application{}
	loopApplication.Store(app)
	defer loopApplication.Store(nil)
	if err := start(app); err != nil {
		app.closeWindow()
		return err
	}
	return app.loop()
}

// loopApplication is the application whose loop runs, for what the host asks of it
// from other threads.
var loopApplication atomic.Pointer[application]

// application is the one window a phone's application has.
type application struct {
	window *window
	done   bool
	err    error

	postMu  sync.Mutex
	posts   []func()
	pending atomic.Bool // a wake is queued
	redraw  atomic.Bool // a redraw is queued
}

var errOneWindow = errors.New("android: an Android application has one window; its surface is the Activity's")

func (application *application) OpenWindow(config platform.Config) (platform.Window, error) {
	if application.window != nil && !application.window.closed {
		return nil, errOneWindow
	}
	win := &window{application: application, config: config, visible: true}
	application.window = win
	if config.Waker != nil && config.OnRedraw != nil {
		config.Waker.Bind(application.wakeRedraw)
	}
	if config.Title != "" {
		setTitle(config.Title)
	}
	// A surface the host made before the window was asked for waits for it.
	if pending := takePendingSurface(); pending != nil {
		application.surfaceChanged(*pending)
	}
	return win, nil
}

func (application *application) AddStatusItem(platform.StatusItemConfig) (platform.StatusItem, error) {
	return nil, platform.ErrStatusAreaUnavailable
}

func (application *application) SetDockBadge(string) {}

func (application *application) SetProgress(float32) {}

// Quit closes the window and ends the loop.
func (application *application) Quit() {
	application.Post(func() { application.closeWindow() })
}

// Post runs fn on the loop's thread.
func (application *application) Post(fn func()) {
	application.postMu.Lock()
	application.posts = append(application.posts, fn)
	application.postMu.Unlock()
	if !application.pending.Swap(true) {
		send(event{kind: eventWake})
	}
}

// wakeRedraw is what the application's waker calls from any goroutine.
func (application *application) wakeRedraw() {
	if !application.redraw.Swap(true) {
		send(event{kind: eventRedraw})
	}
}

// loop waits for the host's events and handles them, until the window is closed.
func (application *application) loop() error {
	for !application.done {
		ev := <-queue
		application.handle(ev)
	}
	return application.err
}

func (application *application) handle(ev event) {
	switch ev.kind {
	case eventSurface:
		application.surfaceChanged(ev)
	case eventSurfaceLost:
		application.surfaceLost(ev)
	case eventTouch:
		application.touch(ev)
	case eventVisible:
		application.visibility(ev.visible)
	case eventClose:
		application.closeRequested()
	case eventWake:
		application.runPosts()
	case eventRedraw:
		application.redrawNow()
	case eventText:
		application.textEdit(ev)
	case eventKey:
		application.keyEvent(ev)
	case eventHover, eventHoverExit:
		application.hover(ev)
	case eventScroll:
		application.scroll(ev)
	case eventAppearance:
		platform.NotifyAppearanceChanged()
	case eventAccessibility:
		application.accessibilityState(ev.visible)
	case eventAccessible:
		application.accessible(ev)
	case eventLowMemory:
		application.lifecycle(platform.LifecycleMemoryWarning)
	case eventSelectionHandle:
		application.selectionHandle(ev)
	case eventSelectionAction:
		application.selectionAction(ev)
	case eventRedrawNeeded:
		application.redrawNeeded(ev)
	case eventLifecycle:
		application.lifecycle(platform.Lifecycle(ev.action))
	case eventInsets:
		application.insetsChanged(ev)
	}
}

// lifecycle tells the application the system moved it between the foreground and the
// background, or asked for memory.
func (application *application) lifecycle(event platform.Lifecycle) {
	win := application.window
	if win == nil || win.closed || win.config.OnLifecycle == nil {
		return
	}
	defer application.guard()
	win.config.OnLifecycle(event)
}

// insetsChanged follows the system covering other edges of the window: the page lays out
// again inside what is left. The host gives them in device pixels; they are kept as the
// window's own, in logical pixels, and so are known when the first frame is painted.
func (application *application) insetsChanged(ev event) {
	left, top, right, bottom := ev.x, ev.y, ev.dx, ev.dy
	hostInsets.Lock()
	hostInsets.left, hostInsets.top, hostInsets.right, hostInsets.bottom, hostInsets.known = left, top, right, bottom, true
	hostInsets.Unlock()
	win := application.window
	if win == nil || win.closed || win.density <= 0 {
		return
	}
	defer application.guard()
	insets := insetsIn(win.density)
	if insets == win.insets {
		return
	}
	win.insets = insets
	if err := win.resized(false); err != nil {
		application.fail(err)
	}
}

// hostInsets are the insets the host last gave, in device pixels. They arrive before the
// window exists and again whenever they change; the window takes them with its density.
var hostInsets struct {
	sync.Mutex
	left, top, right, bottom float32
	known                    bool
}

// insetsIn is the host's insets in logical pixels at a density.
func insetsIn(density float32) gfx.Insets {
	hostInsets.Lock()
	defer hostInsets.Unlock()
	if !hostInsets.known || density <= 0 {
		return gfx.Insets{}
	}
	return gfx.Insets{Top: hostInsets.top / density, Left: hostInsets.left / density, Bottom: hostInsets.bottom / density, Right: hostInsets.right / density}
}

func (application *application) runPosts() {
	application.postMu.Lock()
	posts := application.posts
	application.posts = nil
	application.pending.Store(false)
	application.postMu.Unlock()
	for _, fn := range posts {
		fn()
	}
}

// fail ends the window with err.
func (application *application) fail(err error) {
	logf(logError, "the window ends: %v", err)
	if application.err == nil {
		application.err = err
	}
	application.closeWindow()
}

// guard turns a panic in the application's handlers into the end of the window, as the
// other adapters do.
func (application *application) guard() {
	if recovered := recover(); recovered != nil {
		platform.ReportFault("android: event handler", recovered)
		application.fail(fmt.Errorf("android: a handler panicked: %v", recovered))
	}
}

// window is the Activity's surface and what draws on it.
type window struct {
	application *application
	config      platform.Config
	closed      bool
	visible     bool

	native    *C.ANativeWindow // the surface the host gave, nil when it has none
	width     uint32           // its size in device pixels
	height    uint32
	density   float32 // device pixels per logical pixel
	renderer  *vulkan.Renderer
	text      *textInput  // the application's focused text input, if it has one
	selection selectionUI // the handles and the menu the host shows over a selection
	insets    gfx.Insets  // how far the system covers each edge, in logical pixels
	hovering  bool        // a mouse is over the surface
	a11y      bool        // a screen reader reads the window
	treeIDs   []gfx.ID    // the nodes of the tree last sent to the host, by place
}

func (win *window) Close() {
	win.application.Post(func() { win.application.closeWindow() })
}

func (win *window) Hide() {}
func (win *window) Show() {}

// Frame is the content area: the whole surface, from the screen's corner.
func (win *window) Frame() (gfx.Rect, bool) {
	density := win.density
	if density <= 0 {
		return gfx.Rect{}, false
	}
	return gfx.Rect{Width: float32(win.width) / density, Height: float32(win.height) / density}, true
}

func (win *window) size() platform.Size {
	density := win.density
	return platform.Size{Width: float32(win.width) / density, Height: float32(win.height) / density, Scale: density, SafeArea: win.insets}
}

// closeWindow ends the window and the loop.
func (application *application) closeWindow() {
	win := application.window
	if win == nil || win.closed {
		application.done = true
		return
	}
	win.closed = true
	if win.config.Waker != nil && win.config.OnRedraw != nil {
		win.config.Waker.Unbind()
	}
	if win.renderer != nil {
		win.renderer.Close()
		win.renderer = nil
	}
	win.setTextInput(platform.TextInputUpdate{Changed: true})
	win.releaseNative()
	application.done = true
	if win.config.OnClosed != nil {
		win.config.OnClosed()
	}
}

func (win *window) releaseNative() {
	if win.native != nil {
		C.ANativeWindow_release(win.native)
		win.native = nil
	}
}

// pendingSurface is a surface that came before the window was opened.
var pendingSurface struct {
	sync.Mutex
	event *event
}

func takePendingSurface() *event {
	pendingSurface.Lock()
	defer pendingSurface.Unlock()
	ev := pendingSurface.event
	pendingSurface.event = nil
	return ev
}

// surfaceChanged takes the surface the host gave: the first makes the renderer, a later
// one for the same window is a resize, and a different window a surface to move to.
func (application *application) surfaceChanged(ev event) {
	win := application.window
	if win == nil || win.closed {
		pendingSurface.Lock()
		if old := pendingSurface.event; old != nil && old.surface != nil && old.surface != ev.surface {
			C.ANativeWindow_release(old.surface)
		}
		kept := ev
		pendingSurface.event = &kept
		pendingSurface.Unlock()
		return
	}
	defer application.guard()
	if ev.width <= 0 || ev.height <= 0 || ev.density <= 0 {
		return
	}
	density := ev.density
	win.insets = insetsIn(density)
	switch {
	case win.renderer == nil:
		win.native, win.width, win.height, win.density = ev.surface, uint32(ev.width), uint32(ev.height), density
		source, err := win.surfaceSource()
		if err == nil {
			win.renderer, err = vulkan.New(vulkan.Config{
				Surface: source, Width: win.width, Height: win.height, Scale: density, Clear: win.config.ClearColor,
				Rectangles: win.config.Rectangles, Labels: win.config.TextLabels, Rasterizer: newRasterizer,
			})
		}
		if err != nil {
			application.fail(err)
			return
		}
		if win.config.SetFrameInterval != nil {
			// A FIFO present blocks until the display takes the frame, so the display
			// paces the window; the runtime asks for the next frame as soon as the last
			// one is shown.
			win.config.SetFrameInterval(0)
		}
		if err := win.resized(false); err != nil {
			application.fail(err)
		}
	case ev.surface != win.native:
		// A new surface: the activity was recreated, or came back from behind another.
		win.releaseNative()
		win.native, win.width, win.height, win.density = ev.surface, uint32(ev.width), uint32(ev.height), density
		if err := win.renderer.SetScale(density); err != nil {
			application.fail(err)
			return
		}
		if err := win.renderer.ReplaceSurface(win.width, win.height); err != nil {
			application.fail(err)
			return
		}
		if err := win.resized(true); err != nil {
			application.fail(err)
		}
	default:
		// The same surface at another size (a rotation, a split screen) or density.
		C.ANativeWindow_release(ev.surface) // the host's reference to a window we already hold
		changedDensity := density != win.density
		win.width, win.height, win.density = uint32(ev.width), uint32(ev.height), density
		if changedDensity {
			if err := win.renderer.SetScale(density); err != nil {
				application.fail(err)
				return
			}
		}
		if err := win.resized(true); err != nil {
			application.fail(err)
		}
	}
}

// resized shows the application at the surface's size: it paints for the size and the
// frame it answers with is drawn. A whole frame is what a surface that came back
// needs, and what the application's resize handler answers with.
func (win *window) resized(swapchain bool) error {
	defer win.application.guard()
	renderer := win.renderer
	if renderer == nil || win.native == nil {
		return nil
	}
	if win.config.OnResize != nil {
		frame, err := win.config.OnResize(win.size())
		if err != nil {
			return err
		}
		if swapchain {
			if _, err := renderer.Stage(frame); err != nil {
				return err
			}
			if err := renderer.Resize(win.width, win.height); err != nil {
				return err
			}
		} else if _, err := renderer.Apply(frame); err != nil {
			return err
		}
		win.setTextInput(frame.TextInput)
		win.syncTree(frame)
	}
	if err := renderer.Present(); err != nil {
		return err
	}
	if win.config.FramePresented != nil {
		win.config.FramePresented()
	}
	return nil
}

// surfaceLost lets go of the surface before the host takes it away.
func (application *application) surfaceLost(ev event) {
	if win := application.window; win != nil && !win.closed {
		if win.renderer != nil {
			win.renderer.ReleaseSurface()
		}
		win.releaseNative()
	} else {
		pendingSurface.Lock()
		if old := pendingSurface.event; old != nil && old.surface != nil {
			C.ANativeWindow_release(old.surface)
		}
		pendingSurface.event = nil
		pendingSurface.Unlock()
	}
	if ev.ack != nil {
		close(ev.ack)
	}
}

// redrawNeeded draws the window on the surface the system is about to show, and answers
// once the frame is presented (SurfaceHolder.Callback2.surfaceRedrawNeeded): the system
// shows a new surface only after it, so the user never sees one without the window on it.
// The frame is presented even when nothing in it changed, as the surface may hold none.
func (application *application) redrawNeeded(ev event) {
	defer close(ev.ack)
	win := application.window
	if win == nil || win.closed || win.renderer == nil || win.native == nil {
		return
	}
	defer application.guard()
	if win.config.OnRedraw != nil {
		frame, err := win.config.OnRedraw()
		if err == nil {
			_, err = win.renderer.Apply(frame)
		}
		if err == nil {
			win.setTextInput(frame.TextInput)
			win.syncTree(frame)
		}
		if err != nil {
			application.fail(err)
			return
		}
	}
	if err := win.renderer.Present(); err != nil {
		application.fail(err)
		return
	}
	if win.config.FramePresented != nil {
		win.config.FramePresented()
	}
}

// present draws a frame the application answered with.
func (win *window) present(frame platform.Frame) error {
	if win.renderer == nil || win.native == nil || !win.visible {
		return nil
	}
	changed, err := win.renderer.Apply(frame)
	if err == nil && changed {
		err = win.renderer.Present()
	}
	if err == nil {
		win.setTextInput(frame.TextInput)
		win.syncTree(frame)
		win.syncSelection()
	}
	if err == nil && win.config.FramePresented != nil {
		win.config.FramePresented()
	}
	return err
}

// redrawNow answers the application's request for a frame.
func (application *application) redrawNow() {
	application.redraw.Store(false)
	win := application.window
	if win == nil || win.closed || win.config.OnRedraw == nil || win.renderer == nil || win.native == nil || !win.visible {
		return
	}
	defer application.guard()
	frame, err := win.config.OnRedraw()
	if err == nil {
		err = win.present(frame)
	}
	if err != nil {
		application.fail(err)
	}
}

// visibility follows the activity going behind another and coming back.
func (application *application) visibility(visible bool) {
	win := application.window
	if win == nil || win.closed {
		return
	}
	defer application.guard()
	was := win.visible
	win.visible = visible
	if win.config.OnVisibility != nil && visible != was {
		win.config.OnVisibility(visible)
	}
	if visible && !was && win.renderer != nil && win.native != nil {
		// What was drawn last may have gone with the surface: the application
		// paints for the size again.
		if err := win.resized(true); err != nil {
			application.fail(err)
		}
	}
}

// closeRequested is the user's back gesture at the root: the application may keep the
// window, as it may a window's close button.
func (application *application) closeRequested() {
	win := application.window
	if win == nil || win.closed {
		return
	}
	defer application.guard()
	if win.config.OnCloseRequest != nil {
		win.config.OnCloseRequest(func() { application.Post(application.closeWindow) })
		return
	}
	application.closeWindow()
}

// setTitle names the task in the system's list of recent apps.
func setTitle(title string) {
	units := utf16Units(title)
	if len(units) == 0 {
		return
	}
	C.viva_host_set_title((*C.uint16_t)(unsafe.Pointer(&units[0])), C.int(len(units)))
}

// syncTree gives the host the accessibility tree a frame carries, when a screen reader reads.
func (win *window) syncTree(frame platform.Frame) {
	if win.a11y && frame.Accessibility != nil {
		win.pushTree(frame.Accessibility)
	}
}
