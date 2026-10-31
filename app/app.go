// Package app owns the small public lifecycle surface for Viva applications.
package app

import (
	"fmt"

	"github.com/vivaui/viva/ui"
)

const (
	defaultTitle  = "Viva"
	defaultWidth  = 960
	defaultHeight = 600
)

// Color is a linear RGBA color.
type Color struct {
	Red   float64
	Green float64
	Blue  float64
	Alpha float64
}

// WindowMode selects native window chrome.
type WindowMode uint8

const (
	// WindowTitled is the standard titled window with traffic lights.
	WindowTitled WindowMode = iota
	// WindowFrameless hides all window chrome including traffic lights.
	WindowFrameless
	// WindowCustomTitlebar keeps traffic lights but extends content full
	// height behind a transparent titlebar. The window is moved from the move
	// regions the page declares (ui.Canvas.MoveRegion, flex Box.MovesWindow),
	// and from nowhere else.
	WindowCustomTitlebar
	// WindowPanel is a transient popup beside a status item: no chrome, above
	// other windows, taking keyboard focus without bringing the rest of the
	// application forward. Set Config.Anchor to the StatusClick's anchor. It
	// closes itself when it loses focus; OnClosed reports that, and OnCloseRequest
	// is not asked.
	WindowPanel
)

// Config describes the initial native window.
type Config struct {
	Title string
	// Class is the name a Linux desktop uses to match the window to its launcher
	// entry (the X11 WM_CLASS, which a .desktop file names in StartupWMClass), so
	// the taskbar shows the application's icon. Empty means the executable's file
	// name, which is what viva package writes into the entry.
	Class string
	// Icon is the application's picture where the desktop shows one for a window:
	// the taskbar and Alt+Tab on Windows, the taskbar and title bar on X11. A
	// packaged application has its icon in its bundle, installer or launcher
	// entry; Icon is for the program run without one, and for Windows, where a
	// running window's taskbar button asks the window and not the shortcut. On
	// macOS it sets the Dock icon only when the program is not in an .app bundle,
	// whose own icon stays. Zero means none.
	Icon   WindowIcon
	Width  int
	Height int
	// MinWidth and MinHeight limit user resizing of the content area, in logical
	// pixels, independently of display DPI. Zero leaves that axis unrestricted.
	// Native adapters cap these limits to the current screen's work area when it
	// is smaller. Initial dimensions below the requested minimum are enlarged.
	MinWidth  int
	MinHeight int
	// Origin, when set, is where the top-left of the content area opens, in
	// screen coordinates (see Window.Frame). Nil leaves the placement to the
	// system. It is a request: on Windows and macOS the content area lands there
	// (kept on a screen); an X11 window manager places the window's frame there,
	// so the content area sits below the title bar. Window.Frame reports where it
	// really is.
	Origin     *ui.Point
	WindowMode WindowMode
	// TitlebarHeight is the height in logical pixels of the strip the
	// application paints under a custom titlebar's traffic lights, where the
	// window buttons Viva draws sit and on macOS the traffic lights are centered.
	// It does not make the strip move the window: declare a move region for
	// that. Zero means 28,
	// the system titlebar's own height. It only applies to WindowCustomTitlebar.
	TitlebarHeight float32
	// WindowControls styles the minimize, maximize and close buttons Viva draws at
	// the trailing end of a WindowCustomTitlebar's strip on Windows and Linux,
	// where the system draws none for a window without a titlebar; macOS has its
	// own and ignores this. It runs at every paint, so the buttons follow a theme
	// change. Nil, or a zero field, takes a color that suits ClearColor. Use
	// TitlebarInsets to keep your own content clear of the buttons.
	WindowControls func() WindowControlsStyle
	// Anchor is where a WindowPanel opens: beside the status item whose click
	// supplied it. Without one the panel is centered on the screen.
	Anchor     StatusAnchor
	ClearColor Color
	// WindowMaterials opts into native materials at window creation. Its zero
	// value keeps the ordinary renderer; no build flags are required.
	WindowMaterials
	// Paint, when set, repaints the whole canvas whenever the window size
	// changes. It runs on the window thread and must not block.
	Paint func(canvas *ui.Canvas, size ui.Size)
	// Tree is the identity tree the application's ui.IDs are named in. Setting
	// it makes the window's canvas use it, so identities print as their paths in
	// logs and in the headless driver's replies. An application with only a few
	// controls may leave it nil.
	Tree *ui.Tree
	// OnPointer receives normalized down, move, up, and exit events with the
	// current hit target and logical window coordinates. The moves of a drag may
	// be coalesced to the latest queued position, as the platform does natively,
	// so a handler follows the coordinates rather than the number of calls.
	// Returning Repaint asks for one synchronous paint; Handled suppresses the
	// click that press would complete.
	OnPointer func(event ui.PointerEvent) ui.EventResult
	// OnKey receives normalized keyboard events for the focused hit target.
	// Handled suppresses built-in traversal, activation, and Escape dismissal.
	OnKey func(event ui.KeyEvent) ui.EventResult
	// OnShortcut receives Control- or Meta-modified printable chords before
	// the native text system. Return Handled only for chords the application
	// owns; an unhandled chord continues through normal text and IME handling.
	// Named keys remain OnKey events. On Windows an ordinary (non-terminal,
	// non-Placed) text field that has focus takes the standard editing and
	// clipboard chords first, so the application does not see them there.
	// This callback runs on the window thread.
	OnShortcut func(event ui.KeyEvent) ui.EventResult
	// OnScroll receives normalized pixel deltas and the hovered hit target.
	// Handled suppresses the window's default canvas scroll offset.
	OnScroll func(event ui.ScrollEvent) ui.EventResult
	// OnFileDrop receives files dragged from the system over this window: the
	// pointer entering and moving with them (FileDropOver), leaving
	// (FileDropLeave), and the drop. Each event carries the hit under the point;
	// returning Handled says that hit takes files, which the system shows to the
	// user. On Windows only the drop is reported. Unset, the window accepts no
	// files and the system refuses the drag.
	OnFileDrop func(event ui.FileDropEvent) ui.EventResult
	// OnResize runs on the window thread each time the window's content size
	// changes, right before the repaint it causes, with the new size in
	// logical pixels. Paint may only read state, so derive anything the new
	// size implies (a terminal's rows and columns, a pty's window size) here.
	// It is not called for the initial size, which is Width and Height, and a
	// development reload worker does not receive it.
	OnResize func(size ui.Size)
	// OnFrameChange runs on the window thread when the window's content
	// rectangle moves or changes size on screen, with the same rectangle
	// Window.Frame reports at that moment. OnResize still runs first on a
	// size change, for layout. An application saves this rectangle, together
	// with the window's current size, to open its next window where this one
	// was; a live drag reports every intermediate position. On Linux and
	// Windows, reports are suppressed while the window is maximized so the
	// saved rectangle preserves the floating geometry for the next launch.
	OnFrameChange func(frame ui.Rect)
	// OnMinimize runs on the window thread when the window is minimized or
	// restored, including by the system (the Dock, the taskbar, a window
	// manager key binding), not only by Viva's own window controls. It lets an
	// application stop work nobody can see, such as a remote desktop stream.
	// Hide is the application's own request and is not reported here.
	OnMinimize func(minimized bool)
	// OnMaximize runs on the window thread when the window is maximized or
	// restored on Windows and Linux, including by the system (the window
	// manager, the taskbar, or a keyboard shortcut), not only by Viva's own
	// window controls. On macOS, windows zoom or enter full screen instead,
	// and this callback is not invoked.
	OnMaximize func(maximized bool)
	// TextInput resolves the currently focused text-field hit into one
	// application-owned native editor. Returning nil disables native editing.
	// The callback runs on the window thread after painting.
	TextInput TextInputProvider
	// OnCloseRequest, when set, intercepts window close (the red traffic
	// light). Call allow to close the window immediately or later; never
	// calling it keeps the window open.
	OnCloseRequest func(allow func())
	// OnClosed runs exactly once on the window thread after native resources
	// stop receiving events. Release window-owned application resources here.
	OnClosed func()
	// Invalidator, when set, lets other goroutines request a repaint with
	// Invalidate. Pair it with OnWake to apply what they produced. Unset, no
	// wake-up machinery exists. A development reload worker ignores it.
	Invalidator *Invalidator
	// OnWake runs on the window thread right before each repaint that
	// Invalidate or a running transition requested. Move the results of
	// background work into application state here, so Paint only reads state.
	OnWake func()
	// Reload contains optional development-only state migration hooks. Normal
	// release runs never call them and do not link the reload coordinator.
	Reload ReloadConfig
	// onReopen routes a native reopen request through the owning Window.Show.
	onReopen func()
}

// ReloadConfig explicitly snapshots application-owned state across a
// development worker replacement. The runtime preserves window, focus,
// pointer, and scroll state independently.
type ReloadConfig struct {
	Snapshot func() ([]StateSlot, error)
	Restore  func([]StateSlot) error
}

// StateSlot is one bounded, versioned application state value. Keys must be
// stable and unique. Data encoding is application-owned and should be kept
// small; the development protocol rejects oversized aggregate state.
type StateSlot struct {
	Key     uint64
	Version uint32
	Data    []byte
}

func (config Config) normalize() (Config, error) {
	if config.Title == "" {
		config.Title = defaultTitle
	}
	if config.Width == 0 {
		config.Width = defaultWidth
	}
	if config.Height == 0 {
		config.Height = defaultHeight
	}
	if config.Width < 1 || config.Height < 1 {
		return Config{}, fmt.Errorf("app: window dimensions must be positive, got %dx%d", config.Width, config.Height)
	}
	if config.MinWidth < 0 || config.MinHeight < 0 {
		return Config{}, fmt.Errorf("app: minimum window dimensions must not be negative")
	}
	config.Width = max(config.Width, config.MinWidth)
	config.Height = max(config.Height, config.MinHeight)

	if err := config.Icon.validate(); err != nil {
		return Config{}, err
	}

	color := config.ClearColor
	if color == (Color{}) {
		config.ClearColor = Color{Red: 0.055, Green: 0.065, Blue: 0.09, Alpha: 1}
	} else if !validColor(color) {
		return Config{}, fmt.Errorf("app: clear color components must be between 0 and 1")
	}

	return config, nil
}

func validColor(color Color) bool {
	return validComponent(color.Red) &&
		validComponent(color.Green) &&
		validComponent(color.Blue) &&
		validComponent(color.Alpha)
}

func validComponent(value float64) bool {
	return value >= 0 && value <= 1
}

// originPoint is Origin's value, or the zero point when none is set.
func (config Config) originPoint() ui.Point {
	if config.Origin == nil {
		return ui.Point{}
	}
	return *config.Origin
}
