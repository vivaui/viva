# Optional global shortcuts

`Config.OnShortcut` handles keys delivered to a focused window. A tray application
can also offer an explicit system-wide show/hide binding:

```go
stop, err := app.RegisterGlobalShortcut(app.GlobalShortcut{
    Modifiers: ui.ModifierControl | ui.ModifierAlt,
    Key: "G",
}, func() {
    if window.Hidden() { window.Show() } else { window.Hide() }
})
if err != nil { /* show the conflict or unsupported-platform error */ }
if stop != nil { defer stop() }
```

Registration is optional. An application that never registers one starts no
worker or native hotkey resource. Windows uses a blocking Win32 message queue.
macOS uses Carbon application hotkey events. Linux/X11 uses a private XCB
connection and a blocking wait; Caps Lock and Num Lock don't change the binding.
`stop` releases the registration and is idempotent. An in-flight callback may
finish after `stop`; native bindings coalesce presses when a callback is still
running. Callbacks execute off the
window thread; use thread-safe Window methods or store an event and call an
Invalidator. On macOS, register and release bindings on the main thread or
while the main event loop is running.

Keys are ASCII letters/digits, F1–F24, Space, Enter, Escape or Tab; at least one
modifier is required. Windows reserves F12 for debuggers and reports an error.
Conflicts are returned to the caller, so it can retain the previous setting.
macOS supports F1–F20 and uses ANSI key positions for letters and digits.
Linux resolves keycodes at registration using the current X keyboard map and
uses the usual X11 Alt (Mod1) and Super (Mod4) masks. Native Wayland and other
backends return `app.ErrUnsupported`. XWayland cannot provide desktop-wide
bindings for native Wayland clients.

Driver runs keep registrations private. `driver.PressGlobalShortcut(binding)`
tests the callback without registering anything with the host OS. Closing the
Driver clears those bindings. Opt-in native lifecycle tests use
`VIVA_NATIVE_GLOBAL_SHORTCUT_TEST=1`. The Linux test also delivers an actual
X11 key via xdotool under a dedicated Xvfb display. The macOS test dispatches
Carbon events to the app's handler without injecting desktop input.
