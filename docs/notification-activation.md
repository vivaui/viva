# Interactive system notifications

Use `Application.ShowNotification` when a notification needs activation or
withdrawal. The existing comparable `app.Notification{Title, Body}` value and
`app.SendNotification` title/body-only function remain compatible.

```go
handle, err := application.ShowNotification(
    app.Notification{Title: "Download complete", Body: "Click to open the window."},
    app.NotificationEvents{
        OnActivate: func() { window.Show() },
        OnError: func(err error) { log.Printf("notification: %v", err) },
    },
)
if err != nil {
    // The request was rejected before native delivery could be accepted.
    log.Printf("request notification: %v", err)
}
// Later, when this notification is no longer relevant:
handle.Close()
```

Call from startup or an event callback, like `AddStatusItem`. Both event handlers
are optional, and `Close` is nil-safe and safe from any goroutine. The application
marshals native signals onto its existing event loop: callbacks can restore or
open windows without a separate dispatch queue. At most one terminal event is
queued for a handle, and activation or error consumes its ownership. `Close`
invalidates queued events; a callback already running may finish.

The application owns active handles and closes them on `Quit`, startup failure
or the end of `RunApplication`. Applications should also close stale handles
while running. A system dismissal does not promise a terminal event, so retaining
an arbitrary number of handles until process exit is not a resource policy. No
application-specific timeout, deduplication or navigation payload lives in Viva.

## Support and host boundary

`app.Supports(app.CapabilityNotificationActivation)` reports native build-target
support on desktop macOS and Windows. Other native platforms return
`app.ErrUnsupported`. Capabilities describe the build target, not what a custom
runtime host implements.

A custom `app.Host` opts in with the public method:

```go
ShowNotification(app.Notification, app.NotificationEvents) (app.NotificationControl, error)
```

The host returns a non-nil control promptly, reports asynchronous failures through
`OnError`, and may signal callbacks from native threads. Its control requests
withdrawal and invalidates native callbacks with an idempotent, any-goroutine
`Close`. No `internal/platform` import is required. A host without this optional
method returns `ErrUnsupported` instead of accidentally accessing the real OS;
headless applications can supply a synthetic host for notification tests.

## Platform semantics

- macOS uses `UNUserNotificationCenter`, requiring an application bundle
  identifier and permission. The first request can ask for authorization
  asynchronously. Denial is `app.ErrPermissionDenied`, delivered immediately or
  through `OnError` according to when it is discovered. Closing while permission
  is pending prevents late authorization from reinstating the request. A loose
  command-line binary cannot stand in for packaged-app permission testing.
- Windows uses Shell notification-area balloons through the application's
  message-only event-loop window, so closing an ordinary window does not retire
  a notification. Each carrier has an independent full-width identifier;
  callbacks are not routed through the last message sent. The legacy callback
  layout preserves all 32 identifier bits instead of version 4's 16-bit field.
  A title-only request is represented as body text because an empty Shell body
  is a withdrawal request. Closing the loop removes remaining carriers. This is not a persistent toast
  activation service and does not add activation after process exit.

A successful request does not prove that the OS displayed it: notification policy,
quiet modes and Shell state can suppress delivery. Native field limits may
truncate text; UTF-16 truncation preserves surrogate pairs. The title and body
must contain valid UTF-8 without NUL and cannot both be blank. Closing requests
native withdrawal; the OS controls already-presented UI. Unhandled asynchronous
failures enter the structured `notification/delivery-failed` diagnostic stream,
not the recovered-panic fault stream.

## Lifecycle and verification

The framework starts no timer, polling loop or Go worker for interactive
notifications. Application tracking and native callback maps allocate lazily;
active requests retain text, callbacks and native resources until activation,
failure or explicit close. Programs that do not request notifications still carry
some linked code and additional application metadata; binary-size comparisons
should quantify that cost rather than claim it is zero.

`examples/guides/notifications` provides show, withdraw and window-restore actions.
On macOS, package it with the normal Viva application-bundle tooling before
checking permission, notification-center delivery and activation. On Windows,
verify multiple notifications, ordinary-window closure, cancellation and loop
shutdown. Real OS delivery/permission verification is separate from unit tests.

The automated contracts use a synthetic public host to test loop dispatch,
concurrent cancellation, one terminal event, errors, callbacks before host return,
late controls, startup failures and runner cleanup. Native Darwin tests exercise
actual bridge registry consumption and permission-error mapping without requesting
permission or presenting UI. Windows tests exercise callback identities,
wrong-host rejection, duplicate events, carrier cleanup and UTF-16 truncation.
