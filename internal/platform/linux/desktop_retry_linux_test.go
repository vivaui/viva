//go:build linux

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
	application := &linuxApplication{}
	application.desktopServices.now = func() time.Time { return now }
	application.services()
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
	application := &linuxApplication{}
	application.desktopServices.now = func() time.Time { return now }
	application.services()
	exercisePermanentDesktopFailure(t, &now, desktopRetryHarness{
		sync: application.syncDesktopWatch, retry: application.retryDesktopWatch,
		pollTimeout: application.pollTimeout,
	})
	if !application.desktopServices.desktopUnsupported {
		t.Fatal("expected desktopUnsupported to be true")
	}
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
	if !retry.parked {
		t.Fatal("the retry did not park after the bounded series of failures")
	}
	if retry.ready(now.Add(time.Hour)) {
		t.Fatal("a parked retry became ready again on its own")
	}
	retry.reset()
	if retry.failures != 0 || retry.parked || !retry.ready(now) {
		t.Fatal("reset did not clear the retry backoff")
	}
}

// fakeBusHost is a loop the desktop services can belong to, with the commands
// posted to it recorded.
type fakeBusHost struct {
	sleeps bool
	posts  chan uint32
}

func (host *fakeBusHost) rebuildPoll()              {}
func (host *fakeBusHost) fail(error)                {}
func (host *fakeBusHost) atspiShells() []atspiShell { return nil }
func (host *fakeBusHost) refreshText()              {}
func (host *fakeBusHost) sleepsUntilWoken() bool    { return host.sleeps }
func (host *fakeBusHost) post(command uint32) {
	select {
	case host.posts <- command:
	default:
	}
}

// TestDesktopRetryWakesALoopThatSleepsUntilWoken runs without an injected
// clock, so the timer that wakes such a loop runs too.
func TestDesktopRetryWakesALoopThatSleepsUntilWoken(t *testing.T) {
	prepareDesktopRetryTest(t)
	dialDesktop = func() (desktopConnection, error) {
		return nil, errors.New("session bus is restarting")
	}
	host := &fakeBusHost{sleeps: true, posts: make(chan uint32, 4)}
	services := &desktopServices{host: host}
	services.syncDesktopWatch()
	select {
	case command := <-host.posts:
		if command != commandDesktop {
			t.Fatalf("posted command %d, want commandDesktop", command)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a loop that sleeps until woken was never woken to retry")
	}
}

func TestDesktopRetryDoesNotWakeALoopThatBoundsItsOwnSleep(t *testing.T) {
	prepareDesktopRetryTest(t)
	dialDesktop = func() (desktopConnection, error) {
		return nil, errors.New("session bus is restarting")
	}
	host := &fakeBusHost{sleeps: false, posts: make(chan uint32, 4)}
	services := &desktopServices{host: host}
	services.syncDesktopWatch()
	select {
	case command := <-host.posts:
		t.Fatalf("a loop that bounds its own sleep was posted command %d", command)
	case <-time.After(400 * time.Millisecond):
	}
}

func TestDesktopRetryParksUntilTheNextBypass(t *testing.T) {
	prepareDesktopRetryTest(t)
	now := time.Unix(1000, 0)
	dials := 0
	dialDesktop = func() (desktopConnection, error) {
		dials++
		return nil, errors.New("no session bus")
	}
	services := &desktopServices{host: &fakeBusHost{}, now: func() time.Time { return now }}
	services.syncDesktopWatch()
	for range desktopRetryParkFailures - 1 {
		now = services.desktopRetry.next
		services.syncDesktopWatch()
	}
	if dials != desktopRetryParkFailures {
		t.Fatalf("dials = %d, want %d", dials, desktopRetryParkFailures)
	}
	if !services.desktopRetry.parked {
		t.Fatal("the retry did not park after the bounded series of failures")
	}
	now = now.Add(time.Hour)
	services.syncDesktopWatch()
	if dials != desktopRetryParkFailures {
		t.Fatal("a parked retry dialed again")
	}
	if _, waiting := services.desktopRetry.deadline(true, false); waiting {
		t.Fatal("a parked retry still reported a wake-up deadline")
	}
	services.retryDesktopWatch()
	if dials != desktopRetryParkFailures+1 {
		t.Fatal("an observer re-registration did not unpark the retry")
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
		{name: "disconnected", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.Disconnected"}, want: portalReadRetry},
		{name: "timeout", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.Timeout"}, want: portalReadRetry},
		{name: "timed out", err: &dbus.Error{Name: "org.freedesktop.DBus.Error.TimedOut"}, want: portalReadRetry},
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
