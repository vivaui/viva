---
title: Windows and the status area
summary: Window styles, a custom title bar, several windows, asking before a window closes, and an icon in the menu bar or tray.
order: 3
group: guides
---

## One window or several

`app.RunCanvas` opens a single window and returns when it closes. For anything with more than one window, or a window that can hide while the application keeps running, use `app.RunApplication`. Its start callback runs once on the native event-loop thread and opens the first window with `Application.OpenWindow`; later windows are opened from that callback or from another window's callbacks, which run on the same thread.

Every window has its own canvas, paint callback, focus, input state and close handling. The loop ends when the last window closes, or when something calls `Application.Quit`, which closes every window whatever their close handlers say.

## Window styles

`Config.WindowMode` picks the chrome:

| Mode | Window |
| --- | --- |
| `WindowTitled` | The standard titled window. This is the default. |
| `WindowFrameless` | No chrome at all, traffic lights included. |
| `WindowCustomTitlebar` | Content runs full height behind a transparent title bar. The window moves from the regions you declare with `MovesWindow` (or `Canvas.MoveRegion`), and from nowhere else. |
| `WindowPanel` | A transient popup with no chrome, above other windows, meant to open beside a status icon. |

With a custom title bar the window buttons are the system's on macOS and are drawn by Viva on Windows and Linux. Either way they cover part of the strip, so ask `app.TitlebarInsets` how much and keep your own content clear of it. `Config.TitlebarHeight` sets the strip's height (where Viva puts its buttons and the traffic lights); the window moves from the row you mark `MovesWindow`, controls inside it keep their own clicks, and a menu or dialog open over it is dismissed by a click there, with the window moving once the pointer drags. `Config.WindowControls` styles the buttons Viva draws so they follow your theme.

## Fullscreen

Call `window.SetFullscreen(true)` to enter native fullscreen and
`window.SetFullscreen(false)` to leave. The call is safe from any goroutine;
a nil error means the request was accepted, not that a native animation has
finished. A host without fullscreen support returns an error wrapping
`app.ErrUnsupported`.

`Config.OnFullscreen func(bool)` runs on the window thread for application
requests and system transitions (including macOS's green button and fullscreen
shortcut). `window.Fullscreen()` is already updated when the callback runs.
The callback precedes the transition's resize/frame notifications, and Viva
repaints automatically afterward. Use the callback to switch application
layout; applications still decide which content to display. Repeated requests
for the same state do not produce another transition callback.

Windows uses the whole monitor, not the taskbar's work area, and saves the
previous placement and maximized state. macOS uses AppKit fullscreen Spaces
and native animations. Linux uses `_NET_WM_STATE_FULLSCREEN` on X11 and GTK's
native fullscreen API on GTK/Wayland. The window manager owns Linux placement
restoration. macOS status-area panels are not fullscreen windows. The browser
host does not implement this capability.

Viva hides its caption buttons and disables titlebar move/resize behavior in
fullscreen. Native macOS fullscreen chrome remains managed by AppKit.
Normal window placement is not reported through `OnFrameChange` while
fullscreen, so persisting normal bounds does not accidentally save a monitor
rectangle. `Frame()` still describes the current content bounds.

For tests, `Driver.SetFullscreen(bool) error` uses the same callbacks and
repaints immediately. `Driver.SetFullscreenMonitor(ui.Rect) error` defines
full monitor bounds in logical pixels (default `(0,0,1920,1080)`); negative
origins represent monitors left of the primary display. Changing the monitor
or resizing while fullscreen does not replace the placement restored on exit.
`Driver.Fullscreen()` reports the simulated state. Driver operations, unlike
`Window` methods, run on the caller's thread and are not concurrent.

The headless application's `Window.SetFullscreen` queues work for the driver's
next pump, without a worker or polling. Its command protocol also provides
`fullscreen on|off` and `fullscreenmonitor x y width height`.

## A complete example

The program below opens a custom-title-bar window with a Settings button that opens a second window, and refuses to close while a note is unsaved. The strip along the top is an ordinary `Row` padded by `app.TitlebarInsets`, so the title never runs under the window buttons, and the confirmation is an `f.Dialog` declared only while a close is pending:

<!-- example: windows/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree    = ui.Tree{}
	page    = tree.Feature("notes")
	confirm = tree.Sub(page, tree.Name("confirm"))

	settingsButton = tree.ID(page, "settings")
	confirmScope   = tree.ID(confirm, "scope")
	keep           = tree.ID(confirm, "keep")
	discard        = tree.ID(confirm, "discard")
)

type notes struct {
	unsaved        bool
	confirmClosing bool
	allowClose     func()
	view           flex.View
}

func main() {
	theme := ui.LightTheme()
	state := &notes{unsaved: true}

	err := app.RunApplication(func(application *app.Application) error {
		var main, settings ui.Canvas

		config := app.Config{
			Tree: &tree, Title: "Notes", Width: 720, Height: 480,
			// Content runs full height under a transparent title bar; the window
			// moves from the row marked MovesWindow. Keep your own content clear of
			// the window buttons.
			WindowMode: app.WindowCustomTitlebar,
		}
		leading, trailing := app.TitlebarInsets()
		config.Paint = func(canvas *ui.Canvas, size ui.Size) {
			bounds := ui.Rect{Width: size.Width, Height: size.Height}
			canvas.Fill(bounds, theme.Background)

			state.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
				f.Column().Fill().Children(func() {
					// The bar is as tall as the title bar and leaves room for the
					// window buttons on whichever side the platform puts them.
					f.Row().Height(28).PadSides(0, trailing, 0, leading).Items(flex.Center).MovesWindow().Children(func() {
						f.Text("Notes").Muted()
					})
					f.Column().Grow(1).Pad(24).Items(flex.Start).Children(func() {
						if f.Button(settingsButton, "Settings").Outline().Clicked() {
							// Windows are opened from the start callback or from another
							// window's callbacks, which run on the native event-loop thread.
							_, _ = application.OpenWindow(app.Config{Title: "Settings", Width: 420, Height: 320}, &settings)
						}
					})
					if state.confirmClosing {
						d := f.Dialog(flex.DialogSpec{
							Title: "Discard changes?", Description: "The note has not been saved.", Width: 400,
							Scope: confirmScope, DismissID: keep, NoClose: true,
						})
						d.Footer(func() {
							f.Button(keep, "Keep editing").Outline()
							if f.Button(discard, "Discard").Destructive().Clicked() {
								state.confirmClosing = false
								state.unsaved = false
								state.allowClose()
							}
						})
						if d.Dismissed() {
							state.confirmClosing = false
						}
					}
				})
			})
		}
		// The close button asks. Call allow now, later, or never to keep the window.
		config.OnCloseRequest = func(allow func()) {
			if !state.unsaved {
				allow()
				return
			}
			state.confirmClosing, state.allowClose = true, allow
		}

		_, err := application.OpenWindow(config, &main)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`OnCloseRequest` receives an `allow` function instead of returning an answer, so the decision can wait for the user. Call `allow` now, later or never; never calling it keeps the window open. Here the window shows a confirmation dialog, and the Discard button calls `allow`.

## Hiding a window

`Window.Hide` and `Window.Show` keep the window and its state while taking it off screen, and `Window.Hidden` and `Window.Closed` report where it is. A hidden window still receives `Invalidator` wake-ups, so an application that lives in the status area can keep its icon up to date from a window nobody can see. `Window.Close` is idempotent and safe to call from any goroutine.

## Where windows are on the screen

`Config.Origin` asks where a window's content area opens, and `Window.Frame` says where it is, both in screen coordinates: logical pixels from the primary display's top-left corner, y down. `ui.PointerEvent` carries `ScreenX` and `ScreenY` as well, and a held press keeps reporting them when the pointer leaves the window, so a drag that ends over another of your windows finds it by comparing its pointer with their frames. Windows and macOS put the content area at the origin; an X11 window manager puts the frame there, so the content area sits below its title bar. On displays of different scale the coordinates of windows on different displays are only approximately comparable.

`Config.OnFrameChange` runs on the window thread when that rectangle moves or changes size on screen, with the rectangle `Frame` would return at that moment; `OnResize` still runs first on a size change, for layout. An application saves the rectangle there (debouncing a live drag) and passes it back as `Config.Origin`, `Config.Width` and `Config.Height` when it opens its next window.

`Config.OnMinimize` runs when the window manager minimizes or restores the window, whoever asked for it, and `Window.Minimized` reports the state it last reported. `Window.Hide` is the application's own request and is not reported there. A minimized window still exists and still receives wakes, so an application that streams a picture stops asking for one instead of closing anything. Occlusion by another window is not reported: the platforms cannot all tell.

## An icon in the status area

`Application.AddStatusItem` puts an icon in the menu bar on macOS, the notification area on Windows and the StatusNotifier tray on Linux. Give it a square RGBA icon, a tooltip and either an `OnClick` handler or a `Menu`:

<!-- example: windows/tray.go -->
```go
package main

import "github.com/vivaui/viva/app"

// addTray keeps an icon in the menu bar, notification area or tray. The icon is
// square RGBA; a template icon has only alpha and is tinted to suit the bar.
func addTray(application *app.Application, window *app.Window, icon app.StatusIcon) (*app.StatusItem, error) {
	return application.AddStatusItem(app.StatusItemConfig{
		Tooltip: "Notes",
		Icon:    icon,
		Menu: func() []app.MenuItem {
			// Built each time the menu is about to open, so it shows current state.
			return []app.MenuItem{
				{Title: "Show Notes", OnSelect: window.Show},
				{Title: "Hide Notes", OnSelect: window.Hide, Disabled: window.Hidden()},
				{Separator: true},
				{Title: "Quit", OnSelect: application.Quit},
			}
		},
	})
}
```

When a `Menu` is set, a press opens the system's own native menu, built by calling your function each time it is about to open, so it always shows current state. Without one, `OnClick` receives a `StatusClick` with the button, and an `Anchor` you can assign to `Config.Anchor` to open a `WindowPanel` beside the icon. A panel closes itself when it loses focus; `StatusClick.PanelDismissed` tells your handler that the very press that arrived is the one that closed it, so a click that toggles a panel does not reopen it.

`StatusItem.SetIcon`, `SetTooltip` and `Remove` change or remove the icon later.

## What differs between platforms

The same call can only say what the platform can say. A status icon reports its rectangle on macOS and Windows but only the click point on Linux. A panel is dismissed by losing key status, by deactivation or by `FocusOut` respectively. A new icon in a full macOS menu bar can be hidden by the system, notably next to the notch, and Windows 11 shows a new icon in the overflow flyout until the user promotes it. `docs/conformance.md` in the repository lists these, and what must be checked by hand on each platform.
