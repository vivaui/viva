package app

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/diag"
)

// NotificationControl is the cancellable notification an optional Host supplies.
// Close requests native withdrawal and invalidates callbacks. It is idempotent
// and safe from any goroutine. A host returns a non-nil control on success.
type NotificationControl interface{ Close() }

// Application delivers NotificationEvents on its event-loop thread. Hosts may
// signal their supplied callbacks from any thread; Application queues them.
// Both callbacks are optional. OnError reports asynchronous authorization or
// delivery failure; errors without a handler go to the diagnostic sink.
// Activation or failure consumes the handle, so only one terminal event runs.
type NotificationEvents struct {
	OnActivate func()
	OnError    func(error)
}

type notificationHost interface {
	ShowNotification(Notification, NotificationEvents) (NotificationControl, error)
}

// NotificationHandle owns one notification. Close requests withdrawal and discards
// queued callbacks. A callback already running may finish. It is nil-safe,
// idempotent and safe from any goroutine. Applications close handles whose
// targets become stale; Quit and the end of RunApplication close all remaining
// handles automatically. A native dismissal does not promise a terminal event.
type NotificationHandle struct {
	mu          sync.Mutex
	closed      bool
	queued      bool
	control     NotificationControl
	application *Application
	callback    func()
}

func (handle *NotificationHandle) Close() { handle.finish(false) }

// finish claims a terminal event before releasing native ownership. No user or
// host code runs under either mutex, so Close and callbacks may be reentrant.
func (handle *NotificationHandle) finish(deliver bool) func() {
	if handle == nil {
		return nil
	}
	handle.mu.Lock()
	if handle.closed {
		handle.mu.Unlock()
		return nil
	}
	handle.closed = true
	control := handle.control
	owner := handle.application
	callback := handle.callback
	handle.callback = nil
	if !deliver {
		callback = nil
	}
	handle.control = nil
	handle.application = nil
	handle.mu.Unlock()
	if owner != nil {
		owner.mu.Lock()
		delete(owner.notifications, handle)
		owner.mu.Unlock()
	}
	if control != nil {
		control.Close()
	}
	return callback
}

func (handle *NotificationHandle) attach(control NotificationControl) {
	handle.mu.Lock()
	closed := handle.closed
	if !closed {
		handle.control = control
	}
	handle.mu.Unlock()
	if closed {
		control.Close()
	}
}

// post bounds each handle to one queued terminal callback, even for a custom
// host that races activation against failure. Close can still cancel that post.
func (handle *NotificationHandle) post(callback func()) {
	handle.mu.Lock()
	if handle.closed || handle.queued {
		handle.mu.Unlock()
		return
	}
	handle.queued = true
	handle.callback = callback
	owner := handle.application
	handle.mu.Unlock()
	// The queue retains only this handle. Close clears callback and owner even
	// if a custom host stops its loop before processing the queued method.
	owner.Post(handle.deliver)
}

func (handle *NotificationHandle) deliver() {
	if callback := handle.finish(true); callback != nil {
		callback()
	}
}

// ShowNotification requests an interactive system notification. Call it from
// startup or a window callback, like AddStatusItem. The request does not wait
// for authorization UI or user interaction. Success accepts a request, rather than
// promising the OS will display it; system policy may suppress it.
//
// Content must be nonblank valid UTF-8 without NUL. Native limits may truncate
// title/body. The returned handle is application-owned until activation, error
// or Close. Callbacks run on the event loop and may open or restore windows.
// CapabilityNotificationActivation reports native support. Custom/headless
// hosts opt in with ShowNotification(Notification, NotificationEvents)
// (NotificationControl,error); unsupported hosts return ErrUnsupported.
//
// macOS requires an application bundle identifier and system permission. Windows
// uses notification-area balloons, not a persistent toast-activation service.
// Activation works only while this process and its application loop are alive.
// SendNotification remains the compatible title/body-only API.
func (application *Application) ShowNotification(value Notification, events NotificationEvents) (*NotificationHandle, error) {
	if application == nil || application.host == nil {
		return nil, fmt.Errorf("app: application is not running")
	}
	if !utf8.ValidString(value.Title) || !utf8.ValidString(value.Body) || strings.ContainsRune(value.Title, 0) || strings.ContainsRune(value.Body, 0) {
		return nil, fmt.Errorf("app: notification text must be valid UTF-8 without NUL")
	}
	if strings.TrimSpace(value.Title) == "" && strings.TrimSpace(value.Body) == "" {
		return nil, fmt.Errorf("app: notification requires title or body")
	}
	host, ok := application.host.(notificationHost)
	if !ok {
		return nil, fmt.Errorf("%w: notification activation", ErrUnsupported)
	}
	handle := &NotificationHandle{application: application}
	application.mu.Lock()
	if application.notificationsClosed {
		application.mu.Unlock()
		return nil, fmt.Errorf("app: application notifications are closed")
	}
	if application.notifications == nil {
		application.notifications = make(map[*NotificationHandle]struct{})
	}
	application.notifications[handle] = struct{}{}
	application.mu.Unlock()
	control, err := host.ShowNotification(value, NotificationEvents{
		OnActivate: func() { handle.post(events.OnActivate) },
		OnError: func(err error) {
			if err == nil {
				err = fmt.Errorf("app: notification host reported an unspecified failure")
			}
			handle.post(func() {
				if events.OnError != nil {
					events.OnError(err)
				} else {
					reportNotificationError(err)
				}
			})
		},
	})
	if err != nil {
		handle.Close()
		return nil, err
	}
	if control == nil {
		handle.Close()
		return nil, fmt.Errorf("app: notification host returned no control")
	}
	handle.attach(control)
	return handle, nil
}

func (application *Application) closeNotifications() {
	application.mu.Lock()
	application.notificationsClosed = true
	handles := application.notifications
	application.notifications = nil
	application.mu.Unlock()
	for handle := range handles {
		handle.Close()
	}
}

// nativeNotificationControl adapts a platform cancellation without leaking an
// internal type through the optional public Host contract.
type nativeNotificationControl struct {
	once   sync.Once
	cancel func()
}

func (control *nativeNotificationControl) Close() {
	control.once.Do(func() {
		if control.cancel != nil {
			control.cancel()
		}
	})
}

func reportNotificationError(err error) {
	if err != nil {
		diag.Report(diag.Error, "notification", "delivery-failed", err.Error())
	}
}
