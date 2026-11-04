# System Integration Support

The `app` package exposes a small synchronous system-integration surface. An
operation returns an error when the target cannot complete it with the declared
semantics; adapters never report a fabricated success.

| Capability | macOS | Windows | Linux/X11 | WebAssembly browser |
| --- | --- | --- | --- | --- |
| Read/write text clipboard | `NSPasteboard` | Unicode Win32 clipboard | X11 `CLIPBOARD`, `UTF8_STRING` with `STRING` read fallback; text above 128 KiB is served and read in pieces (INCR); handed to the clipboard manager when the owning window closes, if the desktop has one | `ErrAsyncRequired`; Clipboard API returns Promises |
| Copied files (`ClipboardFiles`, returns `[]string`) | the pasteboard's file URLs, file reference URLs resolved to paths; promised files have no path and are not listed | the `CF_HDROP` list (UTF-16 form) | X11 `CLIPBOARD`: `x-special/gnome-copied-files` (Nautilus, Nemo, Caja, Thunar) or `text/uri-list` (Dolphin, most toolkits); the GTK shell asks GTK for `text/uri-list` | `ErrUnsupported` |
| Primary selection (`SetPrimarySelection`, `PrimarySelection`) | `ErrUnsupported` | `ErrUnsupported` | X11 `PRIMARY`: selected text, pasted with a middle click | `ErrUnsupported` |
| Cursor shape | AppKit cursor; wait unsupported, diagonal resize requires macOS 15 | Win32 system cursors; running window required | XCB cursor font; running window required | CSS cursor on `documentElement` |
| Open files | `NSOpenPanel` | `IFileOpenDialog` | the desktop portal's `FileChooser` over D-Bus; installed `zenity` or `kdialog` without one | `ErrAsyncRequired` |
| Save file | `NSSavePanel` | `IFileSaveDialog` | the desktop portal's `FileChooser`; `zenity` or `kdialog` without one | `ErrAsyncRequired` |
| Open external URL | `NSWorkspace` | `ShellExecuteW` | installed `xdg-open` or `gio` | `window.open`; popup rejection is `ErrUserGestureRequired` |
| System sound | AppKit system sounds | `MessageBeep` | `canberra-gtk-play` or X11 bell | `ErrUnsupported` |
| Notification | UserNotifications; app bundle ID and permission required | shell notification icon; running window required | the desktop's `org.freedesktop.Notifications` service, under the executable's name; installed `notify-send` without one; no click callback | only when Notification permission is already granted |
| Dock badge (`Application.SetDockBadge`) | `NSDockTile.badgeLabel`: the red label macOS draws at the tile's top corner, and an empty label removes it | nothing to do: the taskbar badge a program can set is a picture, not a label | nothing to do: a launcher entry has no badge | nothing to do |
| File icon (`FileIcon`, returns `ui.Bitmap`) | `NSWorkspace.iconForFile:`: the icon Finder shows, an application bundle's own included | `ErrUnsupported` | `ErrUnsupported` | `ErrUnsupported` |
| Preferred languages (`PreferredLanguages`, returns `[]string`) | `NSLocale.preferredLanguages` | `GetUserPreferredUILanguages` | `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG`; the C locale reports nothing | `navigator.languages` |
| System appearance (`SystemAppearance`, `WatchSystemAppearance`; returns `Appearance`) | the application's effective appearance, watched by KVO | the `AppsUseLightTheme` setting, changes heard as `WM_SETTINGCHANGE` | the settings portal's `org.freedesktop.appearance` `color-scheme`, else the XSETTINGS manager, `GTK_THEME`, GTK's `settings.ini` and KDE's `ColorScheme`; changes heard as the portal's `SettingChanged` (one session-bus connection the event loop polls, open only while something watches) and as the XSETTINGS manager's property change; unknown when none says | `prefers-color-scheme`, watched with `matchMedia` |
| Native look (`SetNativeAppearance`) | `NSApplication.appearance`: window buttons, menus, file dialogs and native text editing follow it | nothing to do: Viva draws the window buttons | nothing to do: Viva draws the window buttons | nothing to do |
| Reduced motion (`ReduceMotion`, returns `bool`) | `NSWorkspace.accessibilityDisplayShouldReduceMotion` | `SPI_GETCLIENTAREAANIMATION` | the settings portal's `reduced-motion` or `enable-animations`, else XSETTINGS `Gtk/EnableAnimations`, else KDE's `AnimationDurationFactor` of 0; a change is applied to a running canvas | reports `false`; the browser backend runs no transitions |

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

Clipboard operations and file pickers are intentionally not started from the
synchronous API: returning before their Promise settles would lie about success,
and waiting would deadlock the WASM event loop. Notification permission is not
requested synchronously for the same reason. A typed completion API is proposed
for this boundary.
