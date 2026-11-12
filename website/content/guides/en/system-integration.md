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

A page of the system's web engine can sit beside Viva's own controls. The application opens it with `app.WebHost` once the window is ready and gives it the room of an element with `Box.NativeView`: the page fills that element wherever the layout puts it, inside the clips and on the layer the element is painted on, so it scrolls with a scroll area and stands on a dialog's layer inside a dialog. The page can be served by the application itself under its own scheme, from memory, and it talks to Viva with `viva.postMessage` and `viva.onmessage`. The page's popovers, popups and dialogs go on a higher layer than the page, so the menu below stands over it and takes the pointer where it covers it. While a popover is open, a press on the page only closes the popover: the page does not receive that press.

<!-- example: webview/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// A page of the system's web engine sits among Viva's own controls. The application opens it when
// the window is ready and gives it the room of an element with Box.NativeView: the page fills the
// element where the layout puts it. The menu is a popover: it stands over the page and takes the
// pointer where it covers it.

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
	menuOpen bool
	status   string
}

// open opens the page once the window can host one.
func (m *model) open() {
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

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(8).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			if ui.Button(c, "Back").Outline().Clicked() && m.view != nil {
				m.view.Back()
			}
			if ui.Button(c, "Forward").Outline().Clicked() && m.view != nil {
				m.view.Forward()
			}
			if ui.Button(c, "Reload").Outline().Clicked() && m.view != nil {
				m.view.Reload()
			}
			menu := ui.Button(c, "Menu").Outline()
			if menu.Clicked() {
				m.menuOpen = !m.menuOpen
			}
			ui.Text(c, m.status).Muted()
			ui.PopoverWith(c, menu, ui.PopoverSpec{Width: 220, Align: ui.Start}, &m.menuOpen, func() {
				if ui.Button(c, "Say hello to the page").Ghost().Clicked() && m.view != nil {
					m.view.PostMessage("hello from Viva")
					m.menuOpen = false
				}
			})
		})
		content := ui.Slot(c).Grow(1)
		if m.view != nil {
			content.NativeView(m.view.ID())
		}
	})
}

func main() {
	m := &model{}
	m.web.OnReady = m.open
	config := app.Config{Title: "Web view", Width: 760, Height: 520, Web: &m.web}
	if err := ui.Run(config, m.page); err != nil {
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
