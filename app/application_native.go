//go:build darwin || windows || linux

package app

import (
	"fmt"
	"runtime"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/keymap"
)

// RunApplication starts the native application loop. start runs once on the
// locked native event-loop thread and must open at least one window. The call
// blocks until the final window closes or a platform error terminates the loop.
func RunApplication(start func(*Application) error) error {
	attendFaults()
	if applicationRunner != nil {
		return applicationRunner(func(host Host) error {
			application := &Application{host: host}
			if err := start(application); err != nil {
				return err
			}
			if !application.hasOpenedWindow() {
				return ErrNoWindows
			}
			return nil
		})
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return runNativeApplication(func(native platform.Application) error {
		application := &Application{host: nativeHost{native}}
		if err := start(application); err != nil {
			return err
		}
		if !application.hasOpenedWindow() {
			return ErrNoWindows
		}
		return nil
	})
}

// nativeHost is the Host over a platform adapter.
type nativeHost struct{ native platform.Application }

func (host nativeHost) OpenWindow(config Config, canvas *gfx.Canvas, closed func()) (WindowControl, error) {
	windowConfig, painter, cleanup, err := prepareNativeWindow(config, canvas)
	if err != nil {
		return nil, err
	}
	applicationClosed := windowConfig.OnClosed
	windowConfig.OnClosed = func() {
		cleanup()
		if applicationClosed != nil {
			applicationClosed()
		}
		closed()
	}
	window, err := host.native.OpenWindow(windowConfig)
	if err != nil {
		cleanup()
		return nil, err
	}
	// A command the runtime did not receive through an event handler, such as a
	// menu action, hands its result here: the painter carries it out and the
	// window's waker asks for the frame that shows it.
	return resultWindow{WindowControl: window, painter: painter, waker: windowConfig.Waker}, nil
}

// resultWindow is the control of a window whose painter can carry out an event
// result from outside an event handler.
type resultWindow struct {
	WindowControl
	painter *painter
	waker   *platform.Waker
}

// The embedded WindowControl intentionally exposes only the methods every
// native window has. Forward optional capabilities explicitly so wrapping a
// window for event-result handling does not hide resize, fullscreen,
// always-on-top, or shortcut capture support.
var _ platform.WindowSizer = resultWindow{}
var _ fullscreenControl = resultWindow{}
var _ alwaysOnTopControl = resultWindow{}
var _ shortcutCapturer = resultWindow{}
var _ titleControl = resultWindow{}
var _ platform.WindowMover = resultWindow{}

// What OpenWindow looks for in a host's window control.
var _ interface {
	applyResult(gfx.EventResult) bool
	requestRedraw()
	runShortcut(keymap.Chord) (gfx.EventResult, bool)
} = resultWindow{}

func (window resultWindow) SetSize(width, height int) error {
	sizer, ok := window.WindowControl.(platform.WindowSizer)
	if !ok {
		return platform.ErrWindowSizeUnsupported
	}
	return sizer.SetSize(width, height)
}

func (window resultWindow) SetFullscreen(fullscreen bool) error {
	control, ok := window.WindowControl.(fullscreenControl)
	if !ok {
		return platform.ErrFullscreenUnsupported
	}
	return control.SetFullscreen(fullscreen)
}

func (window resultWindow) Fullscreen() bool {
	control, ok := window.WindowControl.(fullscreenControl)
	return ok && control.Fullscreen()
}

func (window resultWindow) SetTitle(title string) error {
	control, ok := window.WindowControl.(titleControl)
	if !ok {
		return fmt.Errorf("%w: window titles", ErrUnsupported)
	}
	return control.SetTitle(title)
}

func (window resultWindow) SetAlwaysOnTop(enabled bool) error {
	control, ok := window.WindowControl.(alwaysOnTopControl)
	if !ok {
		return platform.ErrAlwaysOnTopUnsupported
	}
	return control.SetAlwaysOnTop(enabled)
}

func (window resultWindow) MoveTo(origin gfx.Point) {
	if control, ok := window.WindowControl.(platform.WindowMover); ok {
		control.MoveTo(origin)
	}
}

func (window resultWindow) unwrapWindowControl() WindowControl {
	return window.WindowControl
}

func (window resultWindow) CaptureSystemShortcuts(capture bool) error {
	control, ok := window.WindowControl.(shortcutCapturer)
	if !ok {
		return fmt.Errorf("%w: this platform keeps the system's shortcuts", ErrUnsupported)
	}
	return control.CaptureSystemShortcuts(capture)
}

func (window resultWindow) applyResult(result gfx.EventResult) bool {
	return window.painter.applyEventResult(result)
}

func (window resultWindow) requestRedraw() {
	if window.waker != nil {
		window.waker.Wake()
	}
}

// runShortcut offers a menu entry's chord to the shortcut handler with the
// control that holds the keyboard, as the platform offers a key it did not
// claim itself.
func (window resultWindow) runShortcut(chord keymap.Chord) (gfx.EventResult, bool) {
	return window.painter.menuShortcut(chord)
}

// CanPreview says whether the adapter's windows can be a DragPreview.
func (host nativeHost) CanPreview() bool {
	_, ok := host.native.(interface{ SupportsDragPreview() })
	return ok
}

// SetOpenURLHandler passes the handler to adapters whose system delivers links
// to the running program.
func (host nativeHost) SetOpenURLHandler(handler func(string)) {
	if native, ok := host.native.(interface{ SetOpenURLHandler(func(string)) }); ok {
		native.SetOpenURLHandler(handler)
	}
}

// SetOpenFileHandler passes the handler to adapters whose system delivers
// documents to the running program.
func (host nativeHost) SetOpenFileHandler(handler func(string)) {
	if native, ok := host.native.(interface{ SetOpenFileHandler(func(string)) }); ok {
		native.SetOpenFileHandler(handler)
	}
}

func (host nativeHost) SetQuitRequestHandler(handler func()) {
	if native, ok := host.native.(interface{ SetQuitRequestHandler(func()) }); ok {
		native.SetQuitRequestHandler(handler)
	}
}

func (host nativeHost) SetReopenHandler(handler func()) {
	if native, ok := host.native.(interface{ SetReopenHandler(func()) }); ok {
		native.SetReopenHandler(handler)
	}
}

func (host nativeHost) AddStatusItem(config StatusItemConfig) (StatusItemControl, error) {
	var menu func() []platform.MenuItem
	if config.Menu != nil {
		menu = func() []platform.MenuItem { return platformMenu(config.Menu()) }
	}
	item, err := host.native.AddStatusItem(platform.StatusItemConfig{
		Tooltip: config.Tooltip,
		Icon:    config.Icon.platform(),
		Menu:    menu,
		OnClick: func(click platform.StatusClick) {
			if config.OnClick != nil {
				config.OnClick(StatusClick{
					Button:         click.Button,
					Anchor:         StatusAnchor{rect: click.Anchor, set: true},
					PanelDismissed: click.PanelDismissed,
				})
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return nativeStatusItem{item}, nil
}

func (host nativeHost) SetDockBadge(text string) { host.native.SetDockBadge(text) }

func (host nativeHost) SetProgress(fraction float32) { host.native.SetProgress(fraction) }

// SetMenuBar installs the application's own menus where the adapter has an
// application menu bar; the adapters without one report ErrUnsupported.
func (host nativeHost) SetMenuBar(menus []Menu) error {
	setter, ok := host.native.(interface {
		SetMenuBar([]platform.MenuBarMenu) error
	})
	if !ok {
		return fmt.Errorf("%w: the application menu bar", ErrUnsupported)
	}
	return setter.SetMenuBar(platformMenuBar(menus))
}

func (host nativeHost) Quit() { host.native.Quit() }

func (host nativeHost) Post(fn func()) { host.native.Post(fn) }

type nativeStatusItem struct{ item platform.StatusItem }

func (item nativeStatusItem) SetIcon(icon StatusIcon) error {
	return item.item.SetIcon(icon.platform())
}
func (item nativeStatusItem) SetTooltip(tooltip string) { item.item.SetTooltip(tooltip) }
func (item nativeStatusItem) Remove()                   { item.item.Remove() }
