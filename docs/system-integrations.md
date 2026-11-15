# System Integration Support

The `app` package exposes a small system-integration surface. An operation
returns an error when the target cannot complete it with the declared
semantics; adapters never report a fabricated success. Integrations a browser
can only settle through a Promise also have a typed asynchronous API; their
synchronous forms report `ErrAsyncRequired`, `ErrSecretStoreUnavailable` or
`ErrUnsupported` instead of blocking the WASM event loop.

| Capability | macOS | Windows | Linux/X11 | WebAssembly browser |
| --- | --- | --- | --- | --- |
| Read/write text clipboard | `NSPasteboard` | Unicode Win32 clipboard | X11 `CLIPBOARD`, `UTF8_STRING` with `STRING` read fallback; text above 128 KiB is served and read in pieces (INCR); handed to the clipboard manager when the owning window closes, if the desktop has one | synchronous calls return `ErrAsyncRequired`; `ClipboardTextAsync` and `SetClipboardTextAsync` use the Clipboard API and require a user gesture |
| Rich copy (`SetClipboardHTML`, html + plain alt) | `NSPasteboard`: `public.html` beside `public.utf8-plain-text` | registered `HTML Format` (CF_HTML, with its header of byte offsets) beside `CF_UNICODETEXT` | X11 `CLIPBOARD`: `text/html` target beside `UTF8_STRING`; the GTK shell serves the same pair through `gtk_clipboard_set_with_data` | synchronous calls return `ErrAsyncRequired`; `SetClipboardHTMLAsync` writes one `ClipboardItem` with both MIME flavours and needs a user gesture |
| Copied files (`ClipboardFiles`, returns `[]string`) | the pasteboard's file URLs, file reference URLs resolved to paths; promised files have no path and are not listed | the `CF_HDROP` list (UTF-16 form) | X11 `CLIPBOARD`: `x-special/gnome-copied-files` (Nautilus, Nemo, Caja, Thunar) or `text/uri-list` (Dolphin, most toolkits); the GTK shell asks GTK for `text/uri-list` | `ErrUnsupported` |
| Primary selection (`SetPrimarySelection`, `PrimarySelection`) | `ErrUnsupported` | `ErrUnsupported` | X11 `PRIMARY`: selected text, pasted with a middle click | `ErrUnsupported` |
| Cursor shape | AppKit cursor; wait unsupported, diagonal resize requires macOS 15 | Win32 system cursors; running window required | XCB cursor font; running window required | CSS cursor on `documentElement` |
| Open files | `NSOpenPanel` | `IFileOpenDialog` | the desktop portal's `FileChooser` over D-Bus; installed `zenity` or `kdialog` without one | synchronous call returns `ErrAsyncRequired`; `OpenFilesAsync` returns `FileRef` values, using File System Access or an `<input type=file>` fallback |
| Save file | `NSSavePanel` | `IFileSaveDialog` | the desktop portal's `FileChooser`; `zenity` or `kdialog` without one | synchronous call returns `ErrAsyncRequired`; `SaveFileAsync` returns a `FileSink`, using File System Access or a Blob download fallback |
| Open directory | `NSOpenPanel` | `IFileOpenDialog` | the desktop portal's `FileChooser` | `ErrUnsupported` |
| File drop (`OnFileDrop`) | AppKit drag destination, with paths | `WM_DROPFILES`, with paths | XDND / GTK drop, with paths | DOM drag events; `gfx.FileDropEvent.Files` carries the browser files and `Paths` is empty |
| Open external URL | `NSWorkspace` | `ShellExecuteW` | installed `xdg-open` or `gio` | `window.open`; popup rejection is `ErrUserGestureRequired` |
| System sound | AppKit system sounds | `MessageBeep` | `canberra-gtk-play` or X11 bell | `ErrUnsupported` |
| Notification | UserNotifications; app bundle ID and permission required | shell notification icon; running window required | the desktop's `org.freedesktop.Notifications` service, under the executable's name; installed `notify-send` without one; no click callback | only when Notification permission is already granted; `RequestNotificationPermission` asks for it asynchronously |
| Secret store (`StoreSecretAsync`, `LoadSecretAsync`, `DeleteSecretAsync`) | Keychain | Credential Manager | Secret Service (GNOME Keyring, KWallet) | AES-GCM through WebCrypto with a non-extractable key held in IndexedDB; synchronous calls return `ErrSecretStoreUnavailable` |
| Font registration (`RegisterFontAsync`) | CoreText process registration | DirectWrite process registration | Fontconfig process registration | `FontFace` loaded from the application's data; family, weight and italic come from the sfnt tables |
| Font enumeration (`FontFamilies`, `MonospaceFamilies`) | system list | system list | fontconfig list | empty: browsers do not expose installed fonts |
| Dock badge (`Application.SetDockBadge`) | `NSDockTile.badgeLabel`: the red label macOS draws at the tile's top corner, and an empty label removes it | nothing to do: the taskbar badge a program can set is a picture, not a label | nothing to do: a launcher entry has no badge | nothing to do |
| File icon (`FileIcon`, returns `gfx.Bitmap`) | `NSWorkspace.iconForFile:`: the icon Finder shows, an application bundle's own included; a missing path answers from the type its extension names | `SHGetFileInfoW`: the icon the shell gives, an existing file or folder's own included; a missing path answers from its extension | `ErrUnsupported` | `ErrUnsupported` |
| Preferred languages (`PreferredLanguages`, returns `[]string`) | `NSLocale.preferredLanguages` | `GetUserPreferredUILanguages` | `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG`; the C locale reports nothing | `navigator.languages` |
| System appearance (`SystemAppearance`, `WatchSystemAppearance`; returns `Appearance`) | the application's effective appearance, watched by KVO | the `AppsUseLightTheme` setting, changes heard as `WM_SETTINGCHANGE` | the settings portal's `org.freedesktop.appearance` `color-scheme`, else the XSETTINGS manager, `GTK_THEME`, GTK's `settings.ini` and KDE's `ColorScheme`; changes heard as the portal's `SettingChanged` (one session-bus connection the event loop polls, open only while something watches) and as the XSETTINGS manager's property change; unknown when none says | `prefers-color-scheme`, watched with `matchMedia` |
| Native look (`SetNativeAppearance`) | `NSApplication.appearance`: window buttons, menus, file dialogs and native text editing follow it | nothing to do: Viva draws the window buttons | nothing to do: Viva draws the window buttons | nothing to do |
| Reduced motion (`ReduceMotion`, returns `bool`) | `NSWorkspace.accessibilityDisplayShouldReduceMotion` | `SPI_GETCLIENTAREAANIMATION` | the settings portal's `reduced-motion` or `enable-animations`, else XSETTINGS `Gtk/EnableAnimations`, else KDE's `AnimationDurationFactor` of 0; a change is applied to a running canvas | `prefers-reduced-motion`, read at mount and watched with `matchMedia` |
| Keep awake (`PreventSleep`) | an `IOPMAssertion` of type `PreventUserIdleSystemSleep`, named by the reason; `pmset -g assertions` lists it | a power request of type `SystemRequired`, created with the reason; `powercfg /requests` lists it | logind's `Manager.Inhibit("idle")` on the system bus, held by the returned descriptor; `systemd-inhibit --list` shows it. `ErrSleepPreventionUnavailable` without logind | `ErrSleepPreventionUnavailable`: a page cannot keep the machine awake |
| Application menu bar (`Application.SetMenuBar`) | the NSMenu bar's application and Window menus stay; the application's own menus sit between them, with their key equivalents, enablement, check marks, submenus and loop-thread actions | `ErrUnsupported`: a window's own menu bar is `components.Menubar` | `ErrUnsupported`: `components.Menubar` | `ErrUnsupported`: `components.Menubar` |
| System proxy (`app/proxy.SystemProxy`) | CFNetwork's settings for the URL: the static HTTP, HTTPS or SOCKS proxy with its bypass list. A PAC script is reported as `ErrSystemProxyUnavailable`, not silently bypassed | WinHTTP's automatic proxy: auto-detection (WPAD), the PAC script, or the static per-user settings | the environment only (`HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, `NO_PROXY`); a proxy kept in GSettings is not read | `ErrSystemProxyUnavailable` |
| Fullscreen (`RequestFullscreen`, `OnFullscreen`) | `NSWindow.toggleFullScreen:` | `SetWindowPos` / window style | window-manager fullscreen | `requestFullscreen` / `exitFullscreen`, reported by `fullscreenchange` |
| Page visibility (`OnMinimize`) | window minimized/restored | window minimized/restored | window minimized/restored | `visibilitychange`; a hidden tab counts as minimized |
| Window activation (`OnActivate`) | `NSWindowDidBecomeKeyNotification` | `WM_ACTIVATE`, when not `WA_INACTIVE` | an X11 `FocusIn` whose detail is `Ancestor` or `Nonlinear` outside a grab, or GTK's `focus-in-event` | the window `focus` event |

## Opening registered documents

Declare the documents an application opens in `viva.toml`:

```toml
[[app.file_types]]
name = "Markdown document"
extensions = ["md", "markdown"]
mime = "text/markdown"
uti = "net.daringfireball.markdown"
```

The package registers the extensions with the operating system. Windows adds
the application to **Open with** and **Default apps**, Linux adds it to the
desktop MIME associations, and macOS adds the extensions and optional UTI to
the bundle document types. The operating system still asks the user to choose a
default; applications cannot silently replace Windows' per-user `UserChoice`.

Call `Application.OnOpenFile` during startup to receive each document. macOS
delivers open documents to the running application through AppKit. Windows and
Linux start the program with paths as arguments, so an application that keeps a
single running copy should pass those arguments through
`app/singleinstance`; the handler itself is not invoked on those systems.

## Error Semantics

- `ErrUnsupported` means the target or its available desktop environment cannot
  provide the capability.
- `ErrCanceled` means the user dismissed a file dialog.
- `ErrAsyncRequired` means the target exposes only a Promise or later DOM event,
  which the synchronous API will not wait for.
- `ErrUserGestureRequired` means a browser rejected an operation outside an
  accepted direct-input gesture.
- `ErrPermissionDenied` means the user or system denied the capability.

Callers should use `errors.Is` rather than matching error strings.

## Pasting a picture into a text field

A text field cannot hold a picture, so a paste (Command or Control+V, Shift+Insert)
with no text on the clipboard changes nothing. The field is told: its
`TextInput.OnEvent` receives a `TextPasteNoText` event and may then read the
picture with `ClipboardImagePNG`, as a chat box that accepts a screenshot does.
`TextEvent.Apply` ignores the event, so an ordinary editor needs no code for it,
and a terminal never receives it. macOS, Windows, Linux and the headless driver
send it; the browser sends it from the `paste` event when the clipboard data has
no `text/plain` (not yet exercised in a browser).

On Linux a focused text field that is not a terminal also takes Control+C, Control+X
and Control+V (and the Insert and Delete forms) itself, ahead of `OnShortcut`, as
it does on Windows; before, those chords reached only the application.

## Reading the files the user copied

`ClipboardFiles` returns the absolute paths of the files a file manager put on
the clipboard, or `ErrNoFiles`. It reads nothing but the list: the files are
not opened, so a path can name a file that is gone, and a source that promises a
file it has not written (a browser's dragged image, a macOS file promise) lists
nothing. The list is cut at `MaxClipboardFiles` (4096) paths. Finder, Explorer
and the Linux file managers also leave the files' names on the clipboard as
text, so an application that shares the clipboard asks for the files first.

Nothing watches for files separately: `OnClipboardChange` says the clipboard
changed, and the application reads when it acts on that. The headless driver
holds a private list (`Driver.SetClipboardFiles`) that, like the text and the
image, is replaced by whatever else is copied.

## Linux Boundary

Linux clipboard and cursor support use the already-loaded XCB system library.
Clipboard reads use a separate bounded XCB request with a three-second deadline;
clipboard ownership remains on the application's window connection so it can
serve selection requests through the normal event loop. No idle worker or poll
loop is added.

File dialogs and notifications talk to the desktop's portal and notification
service over the session bus with Viva's own small D-Bus client (`internal/dbus`);
a dialog blocks its caller on a connection of its own and starts no thread. URL
dispatch and rich named sounds are explicit user actions and may start an
installed desktop utility, which is also what a dialog or notification falls
back to on a desktop without the portal or the service. Viva does not bundle
GTK, Qt, or those utilities.

The desktop's look, its preference for reduced motion and its font rendering
change while an application runs. The window already hears the XSETTINGS
manager's property change, which covers fonts, the GTK theme and animations on
desktops that have one. The settings portal's signal needs a connection to the
session bus: the event loop polls it with the windows' connections, opens it
only while the application watches the system appearance
(`WatchSystemAppearance`), and closes it when the last watcher stops. A change
of the desktop's fonts drops the shaped lines the text engine cached and draws
the labels again. No goroutine and no timer is added.

An application that selects text should call `SetPrimarySelection` as the
selection changes and read `PrimarySelection` on a middle click; the native text
editors do not do this themselves yet.

## Browser Boundary

Clipboard operations, file pickers, secret storage and font loading settle
through Promises. They are intentionally not started from the synchronous API:
returning before the Promise settles would lie about success, and waiting would
deadlock the WASM event loop. The typed asynchronous counterparts are
`ClipboardTextAsync`, `SetClipboardTextAsync`, `SetClipboardHTMLAsync`, `RequestClipboardImage`,
`OpenFilesAsync`, `SaveFileAsync`, `StoreSecretAsync`, `LoadSecretAsync`,
`DeleteSecretAsync`, `RegisterFontAsync` and `RequestNotificationPermission`.
Each calls its completion exactly once; native targets complete before the call
returns, browsers complete on the event loop.

The browser has no path for an application to reopen. `FileRef` and `FileSink`
name contents the user granted, not a location; `OnFileDrop` carries `Files`
instead of `Paths`. Code shared with a native build should branch on
`app.Supports(app.CapabilityFileDialogsAsync)` and on `Paths`/`Files`, not
invent a temporary path.

The page is a single window. Multi-window, tray items, native menus, web views,
custom URL schemes, directory pickers and clipboard-file lists are unsupported
rather than approximated. `OnCloseRequest` maps to `beforeunload` and cannot
guarantee cancellation; save state before the user leaves. `OnClosed` maps to
`pagehide`/`unload` and is not called when the page enters the back/forward
cache. See [web-deployment.md](web-deployment.md) for deployment, headers and
the full lifecycle table.

`app.Run` mounts that single page as the window on the web target;
`app.RunApplication` reports `ErrUnsupported` because it exists to open several
windows. Terminal text input keeps the native contract: `TextInput.KeyReleases`
delivers matching press/release pairs, `AltSendsEscape` is honoured, and the
page releases held keys when it loses the keyboard or becomes hidden.
