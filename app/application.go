package app

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/vivaui/viva/ui"
)

// ErrNoWindows is returned when an application startup callback returns
// without opening a window. A native application must always have at least one
// explicit owner for its event loop.
var ErrNoWindows = errors.New("app: application opened no windows")

// WindowControl is what a Host gives back for a window it opened. All of them
// are safe to call from any goroutine and act on the event-loop thread.
type WindowControl interface {
	Close()
	Hide()
	Show()
	// Frame is the content area in screen coordinates, false when the window
	// cannot say (closed, or not yet placed). It may be called from any goroutine.
	Frame() (ui.Rect, bool)
}

// Host is the event loop an Application runs on. The native backends provide
// one over their platform adapters; a development runner such as package
// headless provides one to drive an application without a window.
type Host interface {
	// OpenWindow creates and presents a window. closed must be called exactly
	// once, after the window has released its resources.
	OpenWindow(config Config, canvas *ui.Canvas, closed func()) (WindowControl, error)
	// AddStatusItem puts an icon in the system's status area.
	AddStatusItem(config StatusItemConfig) (StatusItemControl, error)
	// Quit closes every window, close requests notwithstanding, and ends the
	// loop. It is safe to call from any goroutine.
	Quit()
}

// Window is one independently retained native window. Close is idempotent and
// may be called from a window callback or another goroutine; the platform
// adapter performs the actual close on its native event-loop thread.
type Window struct {
	closeOnce sync.Once
	control   WindowControl
	closed    atomic.Bool
	hidden    atomic.Bool
	// minimized mirrors what the platform last reported through OnMinimize.
	minimized atomic.Bool
}

// Close requests that the native window close. Closing the final window ends
// RunApplication after all platform resources have been released.
func (window *Window) Close() {
	if window == nil || window.closed.Load() {
		return
	}
	window.closeOnce.Do(func() {
		if window.control != nil {
			window.control.Close()
		}
	})
}

// Hide takes the window off the screen without closing it. Its content, state
// and GPU resources stay, and the application keeps running while it is the
// only window, which is how an application stays in the status area after the
// user closes its main window. Hiding a hidden or closed window does nothing.
func (window *Window) Hide() {
	if window == nil || window.closed.Load() || window.hidden.Swap(true) {
		return
	}
	window.control.Hide()
}

// Show puts the window back on the screen in front of the others and brings
// the application forward. Showing a window already on screen raises it.
func (window *Window) Show() {
	if window == nil || window.closed.Load() {
		return
	}
	window.hidden.Store(false)
	window.control.Show()
}

// Frame returns the window's content area in screen coordinates: logical pixels
// from the top-left corner of the primary display, y down, the coordinates
// ui.PointerEvent.ScreenX and ScreenY use. A drag that ends over another of the
// application's windows finds it by comparing its pointer with their frames. The
// result is false for a closed window. On displays of different scale the
// coordinates of windows on different displays are only approximately comparable.
// It may be called from any goroutine.
func (window *Window) Frame() (ui.Rect, bool) {
	if window == nil || window.control == nil || window.closed.Load() {
		return ui.Rect{}, false
	}
	return window.control.Frame()
}

// Hidden reports whether Hide took the window off the screen and Show has not
// put it back. A macOS Dock reopen also goes through Show.
func (window *Window) Hidden() bool {
	return window != nil && window.hidden.Load()
}

// Minimized reports whether the window manager has the window minimized: in the
// Dock, the taskbar, or the desktop's icon area. It follows the state the
// platform last reported to OnMinimize, so it changes while the window is
// hidden too. It may be called from any goroutine.
func (window *Window) Minimized() bool {
	return window != nil && window.minimized.Load()
}

// Closed reports whether the platform has finished closing the window.
func (window *Window) Closed() bool {
	return window == nil || window.closed.Load()
}

// Application owns the native event loop and all windows created in one
// RunApplication call. OpenWindow must be called from the startup callback or
// a window callback, both of which run on the native event-loop thread.
type Application struct {
	host      Host
	mu        sync.Mutex
	windows   map[*Window]struct{}
	openedAny bool
}

// OpenWindow creates and presents one independent retained window.
func (application *Application) OpenWindow(config Config, canvas *ui.Canvas) (*Window, error) {
	if application == nil || application.host == nil {
		return nil, errors.New("app: application is not running")
	}
	window := &Window{}
	application.mu.Lock()
	if application.windows == nil {
		application.windows = make(map[*Window]struct{})
	}
	application.windows[window] = struct{}{}
	application.mu.Unlock()
	liveWindows.Acquire()
	forget := sync.OnceFunc(func() {
		application.mu.Lock()
		delete(application.windows, window)
		application.mu.Unlock()
		liveWindows.Release()
	})
	closed := func() {
		window.closed.Store(true)
		forget()
	}
	// The runtime tracks the minimize state for every window, so Window.Minimized
	// answers even when the application did not ask for OnMinimize. There is no
	// minimize event during OpenWindow, so the zero value is the initial state.
	onMinimize := config.OnMinimize
	config.OnMinimize = func(minimized bool) {
		window.minimized.Store(minimized)
		if onMinimize != nil {
			onMinimize(minimized)
		}
	}
	setReopenHandler(&config, window)
	control, err := application.host.OpenWindow(config, canvas, closed)
	if err != nil {
		forget()
		return nil, err
	}
	window.control = control
	application.mu.Lock()
	application.openedAny = true
	application.mu.Unlock()
	return window, nil
}

// AddStatusItem puts an icon in the system's status area. It is called from the
// startup callback or a window callback, like OpenWindow.
func (application *Application) AddStatusItem(config StatusItemConfig) (*StatusItem, error) {
	if application == nil || application.host == nil {
		return nil, errors.New("app: application is not running")
	}
	if err := config.Icon.validate(); err != nil {
		return nil, err
	}
	control, err := application.host.AddStatusItem(config)
	if err != nil {
		return nil, err
	}
	return &StatusItem{control: control}, nil
}

// Quit closes every window, whatever their close requests say, and ends
// RunApplication once their resources are released. It may be called from any
// goroutine.
func (application *Application) Quit() {
	if application != nil && application.host != nil {
		application.host.Quit()
	}
}

func (application *Application) hasOpenedWindow() bool {
	application.mu.Lock()
	defer application.mu.Unlock()
	return application.openedAny
}

// ApplicationRunner replaces the native multi-window event loop for a
// development tool. Normal release applications never install one.
type ApplicationRunner func(start func(host Host) error) error

var applicationRunner ApplicationRunner

// UseApplicationRunner makes RunApplication use run instead of native windows.
// Package headless/auto installs this only in explicitly tagged tool builds.
func UseApplicationRunner(run ApplicationRunner) {
	applicationRunner = run
}
