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
worker or native hotkey resource. Each live Windows binding has a blocking
Win32 message queue; it does not poll or invalidate frames. `stop` releases the
registration and is idempotent. Callbacks may execute off the window thread;
use thread-safe Window methods or store an event and call an Invalidator.

Keys are ASCII letters/digits, F1–F24, Space, Enter, Escape or Tab; at least one
modifier is required. Windows reserves F12 for debuggers and reports an error.
Conflicts are returned to the caller, so it can retain the previous setting.
Other native backends currently return `app.ErrUnsupported`.

Driver runs keep registrations private. `driver.PressGlobalShortcut(binding)`
tests the callback without registering anything with the host OS. Closing the
Driver clears those bindings. The opt-in native Windows registration/release
test uses `VIVA_NATIVE_GLOBAL_SHORTCUT_TEST=1`; it does not synthesize user input.
