---
title: System integration
summary: Clipboard, file dialogs, notifications, secrets and the rest of the operating system, and how failures are reported.
order: 11
group: guides
---

The `app` package exposes a small, synchronous set of system integrations. An operation either completes with the semantics it declares or returns an error. An adapter never reports a success it did not achieve.

| Capability | Functions |
| --- | --- |
| Clipboard text | `app.ClipboardText`, `app.SetClipboardText` |
| File dialogs | `app.OpenFiles`, `app.OpenDirectory`, `app.SaveFile` |
| Open a URL | `app.OpenURL` |
| Open a local document in its default application (never a program) | `app.OpenPath` |
| Notifications and sounds | `app.SendNotification`, `app.PlaySystemSound` |
| Cursor | `app.SetCursor` |
| Preferred languages | `app.PreferredLanguages` |
| System appearance | `app.SystemAppearance`, `app.WatchSystemAppearance` |
| Look of what the system draws | `app.SetNativeAppearance` |
| Reduced motion | `app.ReduceMotion` |
| Secrets | `app.StoreSecret`, `app.LoadSecret`, `app.DeleteSecret` |
| Status-area icons and panels | `Application.AddStatusItem`, `app.WindowPanel` |
| Fixed-pitch fonts | `app.MonospaceFamilies` |
| Dock badge | `Application.SetDockBadge` |
| The icon the file manager shows for a file or folder | `app.FileIcon` |

## What each platform uses

Each function maps to the platform's own API: `NSPasteboard`, `NSOpenPanel` and `NSWorkspace` on macOS; the Win32 clipboard, `IFileOpenDialog` and `ShellExecuteW` on Windows; X11 selections on Linux, with the desktop portal's file chooser and the notification service over D-Bus (`zenity`, `kdialog` and `notify-send` when a desktop has neither), and `xdg-open` or `gio` for URLs. Secrets go to the Keychain, Credential Manager or Secret Service.

The full table, including the cases where a platform cannot do something, is the repository's `docs/system-integrations.md`.

## Errors

Callers compare with `errors.Is`, never with strings:

| Error | Meaning |
| --- | --- |
| `ErrUnsupported` | This platform or desktop environment cannot provide the capability. |
| `ErrCanceled` | The user dismissed a file dialog. |
| `ErrPermissionDenied` | The user or the system denied it. |
| `ErrAsyncRequired` | The browser only offers a Promise. The synchronous API will not wait for it. |
| `ErrUserGestureRequired` | A browser rejected the call outside a direct user gesture. |

## A web page among the controls

A page of the system's web engine can sit beside Viva's own controls. The application opens it with `app.WebHost` once the window is ready, reserves its room with a `flex` slot, and declares where it stands with `Canvas.NativeView`. The page can be served by the application itself under its own scheme, from memory, and it talks to Viva with `viva.postMessage` and `viva.onmessage`. A popup on a higher layer, such as the menu below, stands over the page and takes the pointer where it covers it.

<!-- example: webview/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// A page of the system's web engine sits among Viva's own controls. The application opens it when
// the window is ready, reserves its room with a flex Slot, and declares the place with
// Canvas.NativeView. The menu is a popup on a higher layer: it stands over the page and takes the
// pointer where it covers it.
var (
	tree = ui.Tree{}
	home = tree.Feature("browser")

	back        = tree.ID(home, "back")
	forward     = tree.ID(home, "forward")
	reload      = tree.ID(home, "reload")
	menu        = tree.ID(home, "menu")
	menuDismiss = tree.ID(home, "menu-dismiss")
	menuHello   = tree.ID(home, "menu-hello")
)

// The page is served by the application itself, from memory, under its own scheme: no server, no files.
const page = `<!doctype html><meta charset=utf-8>
<body style="font:16px system-ui;margin:24px">
<h2><img src="logo.svg" width=28 height=28 style="vertical-align:middle"> A page in a Viva window</h2>
<p><input placeholder="type here"> <button onclick="viva.postMessage('hello from the page')">Tell Viva</button></p>
<p id=says>Viva has not said anything yet.</p>
<script>viva.onmessage = e => says.textContent = 'Viva says: ' + e.data</script>`

type model struct {
	web      app.WebHost
	view     *app.WebView
	flex     flex.View
	menuOpen bool
	status   string
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Web view", Width: 760, Height: 520, Web: &m.web}

	m.web.OnReady = func() {
		view, err := m.web.Open(app.WebViewConfig{
			URL: "viva-app://page/index.html",
			Scheme: &app.WebScheme{Name: "viva-app", Serve: func(path string) (string, []byte, bool) {
				switch path {
				case "index.html":
					return "text/html; charset=utf-8", []byte(page), true
				case "logo.svg":
					return "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 28 28"><circle cx="14" cy="14" r="13" fill="#2563eb"/></svg>`), true
				}
				return "", nil, false
			}},
			OnMessage: func(text string) { m.status = "page: " + text }, // a paint follows
		})
		if err != nil {
			m.status = err.Error() // no engine installed, or a platform without pages
			return
		}
		m.view = view
	}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		var content flex.Box
		m.flex.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.PopupLayer(1) // the menu stands over the page, which is on layer 0
			f.Column().Fill().Pad(8).Gap(8).Children(func() {
				f.Row().Gap(8).Items(flex.Center).Children(func() {
					if f.Button(back, "Back").Outline().Clicked() && m.view != nil {
						m.view.Back()
					}
					if f.Button(forward, "Forward").Outline().Clicked() && m.view != nil {
						m.view.Forward()
					}
					if f.Button(reload, "Reload").Outline().Clicked() && m.view != nil {
						m.view.Reload()
					}
					menuButton := f.Button(menu, "Menu").Outline()
					if menuButton.Clicked() {
						m.menuOpen = !m.menuOpen
					}
					f.Text(m.status).Muted()

					if m.menuOpen {
						canvas.SetEscapeTarget(menuDismiss)
						f.Popup().At(0, 0).Fill().Hit(menuDismiss, "Close menu").Children(func() {})
						if canvas.Clicked(menuDismiss) {
							m.menuOpen = false
						}
						f.Popup().BelowBox(menuButton, 4, flex.Start).Width(200).Pad(4).Background(theme.Popover).
							Border(1, theme.Border).Radius(theme.Metrics.RadiusControl).Shadow(ui.ShadowMD).Children(func() {
							if f.Button(menuHello, "Say hello to the page").Ghost().Clicked() && m.view != nil {
								m.view.PostMessage("hello from Viva")
								m.menuOpen = false
							}
						})
					}
				})
				content = f.Slot().Grow(1)
			})
		})
		if m.view != nil {
			canvas.NativeView(m.view.ID(), content.Rect())
		}
	}
	return config
}

func main() {
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

## Boundaries worth knowing

- Linux file dialogs and notifications use the desktop's portal and notification service over D-Bus, and start an installed desktop utility only on a desktop without them; URL dispatch always does. Viva does not bundle GTK or Qt.
- In the browser, the clipboard and file pickers return `ErrAsyncRequired` by design. Returning early would lie about success, and waiting would deadlock the WebAssembly event loop.
- To follow the system's light or dark look, read `app.SystemAppearance` while painting and call `app.WatchSystemAppearance` once with a function that requests a redraw. It is called on the event-loop thread, only when the look really changed. Linux listens to the desktop portal's signal while something watches, falls back to the XSETTINGS manager, `GTK_THEME` and the GTK and KDE configuration files, and answers `AppearanceUnknown` when none gives a preference; a headless `Driver` answers `AppearanceUnknown` until `SetSystemAppearance` says otherwise.
- If the user can pick light or dark regardless of the system, call `app.SetNativeAppearance` with that pick so macOS's own window buttons, menus, file dialogs and native text editing match the window; `AppearanceUnknown` hands them back to the system, and `SystemAppearance` keeps reporting the system's look meanwhile.
- The Dock badge is macOS's own: a label in the red badge at the corner of the application's Dock tile, which the system draws and sizes. Windows and Linux have no Dock tile and do nothing with it; an empty label removes the badge.
- Linux clipboard reads have a three-second deadline and add no idle worker or polling loop.
