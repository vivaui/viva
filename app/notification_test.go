//go:build darwin || windows || linux

package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vivaui/viva/gfx"
)

type notificationTestControl struct{ closed atomic.Int32 }

func (control *notificationTestControl) Close() { control.closed.Add(1) }

type notificationTestHost struct {
	fakeHost
	mu       sync.Mutex
	posts    []func()
	events   []NotificationEvents
	controls []*notificationTestControl
	show     func(NotificationEvents)
	err      error
}

func (host *notificationTestHost) Post(fn func()) {
	host.mu.Lock()
	host.posts = append(host.posts, fn)
	host.mu.Unlock()
}
func (host *notificationTestHost) drain() {
	for {
		host.mu.Lock()
		if len(host.posts) == 0 {
			host.mu.Unlock()
			return
		}
		fn := host.posts[0]
		host.posts = host.posts[1:]
		host.mu.Unlock()
		fn()
	}
}
func (host *notificationTestHost) ShowNotification(_ Notification, events NotificationEvents) (NotificationControl, error) {
	host.events = append(host.events, events)
	if host.err != nil {
		return nil, host.err
	}
	control := &notificationTestControl{}
	host.controls = append(host.controls, control)
	if host.show != nil {
		host.show(events)
	}
	return control, nil
}

func TestNotificationDispatchesOnLoopAndConsumesOnlyOneEvent(t *testing.T) {
	host := &notificationTestHost{}
	application := &Application{host: host}
	calls := 0
	handle, err := application.ShowNotification(Notification{Title: "测试通知"}, NotificationEvents{OnActivate: func() { calls++; application.Post(func() { calls++ }) }})
	if err != nil {
		t.Fatal(err)
	}
	host.events[0].OnActivate()
	host.events[0].OnActivate()
	host.events[0].OnError(errors.New("late failure"))
	if calls != 0 {
		t.Fatal("native callback changed UI state before the event loop")
	}
	host.drain()
	if calls != 2 || host.controls[0].closed.Load() != 1 || len(application.notifications) != 0 {
		t.Fatal("terminal event did not consume ownership exactly once", calls)
	}
	handle.Close()
	if host.controls[0].closed.Load() != 1 || handle.application != nil || handle.callback != nil {
		t.Fatal("consumed control retained its owner or closed twice")
	}
}

func TestNotificationCloseCancelsQueuedActivationConcurrently(t *testing.T) {
	host := &notificationTestHost{}
	application := &Application{host: host}
	calls := 0
	handle, err := application.ShowNotification(Notification{Body: "body"}, NotificationEvents{OnActivate: func() { calls++ }})
	if err != nil {
		t.Fatal(err)
	}
	host.events[0].OnActivate()
	var jobs sync.WaitGroup
	for range 32 {
		jobs.Add(1)
		go func() { defer jobs.Done(); handle.Close() }()
	}
	jobs.Wait()
	host.drain()
	if calls != 0 || host.controls[0].closed.Load() != 1 {
		t.Fatal("Close did not discard queued callbacks")
	}
	var empty *NotificationHandle
	empty.Close()
	(&NotificationHandle{}).Close()
}

func TestNotificationErrorsValidationAndOptionalCallbacks(t *testing.T) {
	host := &notificationTestHost{}
	application := &Application{host: host}
	for _, value := range []Notification{{}, {Title: "\x00"}, {Body: string([]byte{0xff})}} {
		if _, err := application.ShowNotification(value, NotificationEvents{}); err == nil {
			t.Fatal("invalid content reached host")
		}
	}
	if len(host.events) != 0 {
		t.Fatal("invalid request reached host")
	}
	failure := errors.New("authorization denied")
	got := error(nil)
	handle, err := application.ShowNotification(Notification{Title: "test"}, NotificationEvents{OnError: func(err error) { got = err }})
	if err != nil {
		t.Fatal(err)
	}
	host.events[0].OnError(failure)
	host.drain()
	handle.Close()
	if got != failure || len(application.notifications) != 0 {
		t.Fatal("asynchronous error lost ownership or identity", got)
	}
	host.err = failure
	if _, err := application.ShowNotification(Notification{Title: "test"}, NotificationEvents{}); err != failure || len(application.notifications) != 0 {
		t.Fatal("immediate failure retained a handle", err)
	}
	host.err = nil
	handle, err = application.ShowNotification(Notification{Body: "callbacks optional"}, NotificationEvents{})
	if err != nil {
		t.Fatal(err)
	}
	handle.Close()
	unsupported := &Application{host: &fakeHost{}}
	if _, err := unsupported.ShowNotification(Notification{Title: "test"}, NotificationEvents{}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("host support was inferred from the OS", err)
	}
}

func TestNotificationSynchronousHostEventAndCloseBeforeAttach(t *testing.T) {
	host := &notificationTestHost{}
	application := &Application{host: host}
	calls := 0
	host.show = func(events NotificationEvents) { events.OnActivate(); application.Quit() }
	_, err := application.ShowNotification(Notification{Title: "test"}, NotificationEvents{OnActivate: func() { calls++ }})
	if err != nil {
		t.Fatal(err)
	}
	host.drain()
	if calls != 0 || host.controls[0].closed.Load() != 1 || len(application.notifications) != 0 {
		t.Fatal("late control escaped application shutdown")
	}
	if _, err := application.ShowNotification(Notification{Title: "after quit"}, NotificationEvents{}); err == nil {
		t.Fatal("quit application accepted notification")
	}
}

func TestNotificationRunApplicationCleanup(t *testing.T) {
	host := &notificationTestHost{}
	previous := applicationRunner
	defer func() { applicationRunner = previous }()
	applicationRunner = func(start func(Host) error) error { return start(host) }
	calls := 0
	err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{Title: "test"}, &gfx.Canvas{}); err != nil {
			return err
		}
		_, err := application.ShowNotification(Notification{Title: "test"}, NotificationEvents{OnActivate: func() { calls++ }})
		return err
	})
	defer func() {
		for _, window := range host.windows {
			window.Close()
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	host.events[0].OnActivate()
	host.drain()
	if calls != 0 || host.controls[0].closed.Load() != 1 {
		t.Fatal("notification survived the application runner")
	}
}

func TestNotificationConcurrentCallbacksAndCancellation(t *testing.T) {
	host := &notificationTestHost{}
	application := &Application{host: host}
	calls := atomic.Int32{}
	handle, err := application.ShowNotification(Notification{Title: "test"}, NotificationEvents{OnActivate: func() { calls.Add(1) }, OnError: func(error) { calls.Add(1) }})
	if err != nil {
		t.Fatal(err)
	}
	events := host.events[0]
	var jobs sync.WaitGroup
	for index := range 48 {
		jobs.Add(1)
		go func() {
			defer jobs.Done()
			switch index % 3 {
			case 0:
				handle.Close()
			case 1:
				events.OnActivate()
			case 2:
				events.OnError(errors.New("failed"))
			}
		}()
	}
	jobs.Wait()
	host.drain()
	if calls.Load() != 0 || host.controls[0].closed.Load() != 1 {
		t.Fatal("cancellation lost the callback race")
	}
}

func TestNotificationStartupFailureCleansUp(t *testing.T) {
	for _, failStartup := range []bool{true, false} {
		t.Run(map[bool]string{true: "start error", false: "no window"}[failStartup], func(t *testing.T) {
			host := &notificationTestHost{}
			previous := applicationRunner
			defer func() { applicationRunner = previous }()
			applicationRunner = func(start func(Host) error) error { return start(host) }
			sentinel := errors.New("startup failed")
			err := RunApplication(func(application *Application) error {
				if _, err := application.ShowNotification(Notification{Title: "test"}, NotificationEvents{}); err != nil {
					return err
				}
				if failStartup {
					return sentinel
				}
				return nil
			})
			want := sentinel
			if !failStartup {
				want = ErrNoWindows
			}
			if err != want || host.controls[0].closed.Load() != 1 {
				t.Fatal("startup failure leaked notification", err)
			}
		})
	}
}
