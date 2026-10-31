package linux

import (
	"errors"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

type fakeDesktopConnection struct {
	fd         int
	readErr    error
	receiveErr error
	closes     int
}

func (conn *fakeDesktopConnection) Call(_, _, _, member, _ string, _ []byte) (*dbus.Message, error) {
	if member == "AddMatch" {
		return &dbus.Message{}, nil
	}
	if conn.readErr != nil {
		return nil, conn.readErr
	}
	var body dbus.Encoder
	body.Signature("v")
	body.Signature("u")
	body.Uint32(1)
	return &dbus.Message{Body: body.Bytes()}, nil
}

func (conn *fakeDesktopConnection) Receive() ([]*dbus.Message, error) {
	return nil, conn.receiveErr
}

func (conn *fakeDesktopConnection) FD() int { return conn.fd }
func (conn *fakeDesktopConnection) Close()  { conn.closes++ }

type desktopRetryHarness struct {
	sync        func()
	retry       func()
	serve       func()
	close       func()
	connected   func() bool
	sources     func() int
	retryState  func() desktopRetry
	pollTimeout func() int32
}

func prepareDesktopRetryTest(t *testing.T) {
	t.Helper()
	resetDesktopState(t)
	platform.SetAppearanceObserver(func() {})
	t.Cleanup(func() { platform.SetAppearanceObserver(nil) })
	previous := dialDesktop
	t.Cleanup(func() { dialDesktop = previous })
}

func exerciseDesktopRetry(t *testing.T, now *time.Time, harness desktopRetryHarness) {
	t.Helper()
	dials := 0
	var transient, first, second *fakeDesktopConnection
	dialDesktop = func() (desktopConnection, error) {
		dials++
		switch dials {
		case 1:
			return nil, errors.New("session bus is restarting")
		case 2:
			transient = &fakeDesktopConnection{fd: 20, readErr: &dbus.Error{Name: "org.freedesktop.DBus.Error.NoReply"}}
			return transient, nil
		case 3:
			first = &fakeDesktopConnection{fd: 21}
			return first, nil
		case 4:
			second = &fakeDesktopConnection{fd: 22}
			return second, nil
		default:
			t.Fatalf("unexpected desktop dial %d", dials)
			return nil, nil
		}
	}

	harness.sync()
	if dials != 1 || harness.connected() || harness.sources() != 0 {
		t.Fatalf("dial failure: dials %d connected %v sources %d", dials, harness.connected(), harness.sources())
	}
	retry := harness.retryState()
	if retry.failures != 1 || !retry.next.Equal(now.Add(desktopRetryMin)) {
		t.Fatalf("first retry = %+v, now %v", retry, *now)
	}
	if timeout := harness.pollTimeout(); timeout < 0 || timeout > 101 {
		t.Fatalf("retry poll timeout = %d, want a bounded wake-up", timeout)
	}
	harness.sync()
	if dials != 1 {
		t.Fatal("a loop wake-up retried before the backoff deadline")
	}

	// Observer re-registration and new-window creation both use this immediate path.
	harness.retry()
	if dials != 2 || transient.closes != 1 {
		t.Fatalf("immediate retry: dials %d closes %d", dials, transient.closes)
	}
	if desktopPortalGone() {
		t.Fatal("a transient portal read failure became permanent")
	}
	retry = harness.retryState()
	*now = retry.next.Add(-time.Nanosecond)
	harness.sync()
	if dials != 2 {
		t.Fatal("portal failure was retried before its deadline")
	}

	*now = retry.next
	harness.sync()
	if dials != 3 || !harness.connected() || harness.sources() != 1 {
		t.Fatalf("portal recovery: dials %d connected %v sources %d", dials, harness.connected(), harness.sources())
	}
	if retry := harness.retryState(); retry.failures != 0 || !retry.next.IsZero() {
		t.Fatalf("successful connection retained retry state: %+v", retry)
	}
	if timeout := harness.pollTimeout(); timeout != -1 {
		t.Fatalf("connected idle poll timeout = %d, want -1", timeout)
	}

	first.receiveErr = errors.New("bus disconnected")
	harness.serve()
	if first.closes != 1 || harness.connected() || harness.sources() != 0 {
		t.Fatalf("disconnect cleanup: closes %d connected %v sources %d", first.closes, harness.connected(), harness.sources())
	}
	retry = harness.retryState()
	*now = retry.next
	harness.sync()
	if dials != 4 || !harness.connected() || harness.sources() != 1 {
		t.Fatalf("receive recovery: dials %d connected %v sources %d", dials, harness.connected(), harness.sources())
	}
	harness.close()
	if second.closes != 1 || harness.sources() != 0 {
		t.Fatalf("final cleanup: closes %d sources %d", second.closes, harness.sources())
	}
}

func TestX11DesktopWatchRetriesTemporaryFailures(t *testing.T) {
	prepareDesktopRetryTest(t)
	now := time.Unix(1000, 0)
	application := &linuxApplication{now: func() time.Time { return now }}
	harness := desktopRetryHarness{
		sync: application.syncDesktopWatch, retry: application.retryDesktopWatch,
		serve: application.serveDesktop, close: application.closeDesktop,
		connected:   func() bool { return application.desktop != nil },
		sources:     func() int { return len(application.pollFDs) },
		retryState:  func() desktopRetry { return application.desktopRetry },
		pollTimeout: application.pollTimeout,
	}
	t.Cleanup(harness.close)
	exerciseDesktopRetry(t, &now, harness)
}

func TestWaylandDesktopWatchRetriesTemporaryFailures(t *testing.T) {
	prepareDesktopRetryTest(t)
	now := time.Unix(1000, 0)
	app := &waylandApplication{loop: &waylandLoop{}, now: func() time.Time { return now }}
	harness := desktopRetryHarness{
		sync: app.syncDesktopWatch, retry: app.retryDesktopWatch,
		serve: app.serveDesktop, close: app.closeDesktop,
		connected:   func() bool { return app.desktop != nil },
		sources:     func() int { return len(app.loop.extraSources) },
		retryState:  func() desktopRetry { return app.desktopRetry },
		pollTimeout: func() int32 { return app.pollTimeout(now) },
	}
	t.Cleanup(harness.close)
	exerciseDesktopRetry(t, &now, harness)
}

func exercisePermanentDesktopFailure(t *testing.T, now *time.Time, harness desktopRetryHarness) {
	t.Helper()
	dials := 0
	conn := &fakeDesktopConnection{fd: 30, readErr: &dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}}
	dialDesktop = func() (desktopConnection, error) {
		dials++
		return conn, nil
	}
	harness.sync()
	if dials != 1 || conn.closes != 1 || !desktopPortalGone() {
		t.Fatalf("permanent failure: dials %d closes %d gone %v", dials, conn.closes, desktopPortalGone())
	}
	harness.retry()
	*now = now.Add(time.Hour)
	harness.sync()
	if dials != 1 {
		t.Fatal("a permanently unsupported portal was dialed again")
	}
	if timeout := harness.pollTimeout(); timeout != -1 {
		t.Fatalf("permanent failure poll timeout = %d, want -1", timeout)
	}
}

func TestX11DesktopWatchDoesNotRetryPermanentFailure(t *testing.T) {
	prepareDesktopRetryTest(t)
	now := time.Unix(1000, 0)
	application := &linuxApplication{now: func() time.Time { return now }}
	exercisePermanentDesktopFailure(t, &now, desktopRetryHarness{
		sync: application.syncDesktopWatch, retry: application.retryDesktopWatch,
		pollTimeout: application.pollTimeout,
	})
}

func TestWaylandDesktopWatchDoesNotRetryPermanentFailure(t *testing.T) {
	prepareDesktopRetryTest(t)
	now := time.Unix(1000, 0)
	app := &waylandApplication{now: func() time.Time { return now }}
	exercisePermanentDesktopFailure(t, &now, desktopRetryHarness{
		sync: app.syncDesktopWatch, retry: app.retryDesktopWatch,
		pollTimeout: func() int32 { return app.pollTimeout(now) },
	})
}

func TestDesktopRetryBackoffIsBounded(t *testing.T) {
	var retry desktopRetry
	now := time.Unix(1000, 0)
	for index, want := range []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond} {
		retry.fail(now)
		if got := retry.next.Sub(now); got != want {
			t.Fatalf("delay %d = %v, want %v", index, got, want)
		}
	}
	for range 20 {
		retry.fail(now)
		if got := retry.next.Sub(now); got > desktopRetryMax {
			t.Fatalf("retry delay exceeded its cap: %v", got)
		}
	}
	if got := retry.next.Sub(now); got != desktopRetryMax {
		t.Fatalf("saturated retry delay = %v, want %v", got, desktopRetryMax)
	}
	retry.reset()
	if retry.failures != 0 || !retry.ready(now) {
		t.Fatal("reset did not clear the retry backoff")
	}
}

func TestPortalReadFailureClassification(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want portalReadResult
	}{
		{name: "name changing owners", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.NameHasNoOwner"}, want: portalReadRetry},
		{name: "no reply", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.NoReply"}, want: portalReadRetry},
		{name: "transport", err: errors.New("connection closed"), want: portalReadRetry},
		{name: "unsupported", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}, want: portalReadUnsupported},
		{name: "old portal", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.UnknownMethod"}, want: portalReadOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, got := readPortalValues(&fakeDesktopConnection{readErr: test.err})
			if got != test.want {
				t.Fatalf("result = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDesktopFreshRetriesTransientPortalFailure(t *testing.T) {
	resetDesktopState(t)
	previous := dialDesktop
	t.Cleanup(func() { dialDesktop = previous })
	dials := 0
	dialDesktop = func() (desktopConnection, error) {
		dials++
		if dials == 1 {
			return &fakeDesktopConnection{fd: 40, readErr: &dbus.Error{Name: "org.freedesktop.DBus.Error.NoReply"}}, nil
		}
		return &fakeDesktopConnection{fd: 41}, nil
	}

	ensureDesktopFresh()
	ensureDesktopFresh()
	if dials != 1 {
		t.Fatal("a transient failure caused an unthrottled second portal query")
	}
	desktopState.Lock()
	gone := desktopState.portalGone
	desktopState.portalAt = time.Now().Add(-stale - time.Second)
	desktopState.Unlock()
	if gone {
		t.Fatal("a transient query failure latched portalGone")
	}
	ensureDesktopFresh()
	if dials != 2 {
		t.Fatalf("portal queries = %d, want recovery after the stale interval", dials)
	}
	desktopState.Lock()
	recovered := desktopState.portal.hasColorScheme
	desktopState.Unlock()
	if !recovered {
		t.Fatal("successful retry did not replace the portal values")
	}
}
