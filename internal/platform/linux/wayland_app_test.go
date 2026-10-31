//go:build linux

package linux

import (
	"errors"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

type fakeWaylandInputMethod struct {
	fd          int
	err         error
	pending     int
	closeCalls  int
	deadline    time.Time
	deadlineSet bool
	expireCalls int
	expireErr   error
}

func (bus *fakeWaylandInputMethod) FD() int         { return bus.fd }
func (bus *fakeWaylandInputMethod) Err() error      { return bus.err }
func (bus *fakeWaylandInputMethod) Dispatch() error { return bus.err }
func (bus *fakeWaylandInputMethod) Deadline() (time.Time, bool) {
	return bus.deadline, bus.deadlineSet
}
func (bus *fakeWaylandInputMethod) Expire() {
	bus.expireCalls++
	if bus.expireErr != nil {
		bus.err = bus.expireErr
		bus.pending = 0
	}
}
func (bus *fakeWaylandInputMethod) Close() {
	bus.closeCalls++
	bus.pending = 0
	bus.fd = -1
}

func newWaylandInvokeTestApp(t *testing.T) (*waylandApplication, <-chan struct{}) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	wake := make(chan struct{}, 1)
	loop := &waylandLoop{
		wakeFD: 1,
		libc: &libcAPI{
			write: func(int32, unsafe.Pointer, uintptr) int {
				select {
				case wake <- struct{}{}:
				default:
				}
				return 8
			},
			close: func(int32) int32 { return 0 },
		},
	}
	app := &waylandApplication{
		loop:       loop,
		loopThread: syscall.Gettid(),
		done:       make(chan struct{}),
	}
	t.Cleanup(app.close)
	return app, wake
}

func waitWaylandInvokeSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command wake")
	}
}

func TestWaylandInvokeExecutesOnEventThread(t *testing.T) {
	app, _ := newWaylandInvokeTestApp(t)
	called := false
	if err := app.invoke(func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("invoke on event thread: %v", err)
	}
	if !called || len(app.loop.cmdQueue) != 0 {
		t.Fatalf("event-thread invoke called=%v queued=%d", called, len(app.loop.cmdQueue))
	}
}

func TestWaylandInvokeFromBackgroundUsesLoopThread(t *testing.T) {
	app, wake := newWaylandInvokeTestApp(t)
	expectedErr := errors.New("command result")
	commandThread := 0
	result := make(chan error, 1)
	go func() {
		result <- app.invoke(func() error {
			commandThread = syscall.Gettid()
			return expectedErr
		})
	}()
	waitWaylandInvokeSignal(t, wake)
	select {
	case err := <-result:
		t.Fatalf("background invoke bypassed the loop: %v", err)
	default:
	}
	app.loop.drainCommands()
	if commandThread != app.loopThread {
		t.Fatalf("command thread ID = %d, want event thread %d", commandThread, app.loopThread)
	}
	select {
	case err := <-result:
		if !errors.Is(err, expectedErr) {
			t.Fatalf("invoke error = %v, want %v", err, expectedErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for invoke result")
	}
}

func TestWaylandInvokeCloseRejectsAndUnblocks(t *testing.T) {
	app, wake := newWaylandInvokeTestApp(t)
	executed := false
	result := make(chan error, 1)
	go func() {
		result <- app.invoke(func() error {
			executed = true
			return nil
		})
	}()
	waitWaylandInvokeSignal(t, wake)
	app.close()
	select {
	case err := <-result:
		if !errors.Is(err, errWaylandApplicationClosed) {
			t.Fatalf("invoke close error = %v, want %v", err, errWaylandApplicationClosed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not release the invoke waiter")
	}
	if executed {
		t.Fatal("queued command executed while the application closed")
	}
	if err := app.invoke(func() error {
		executed = true
		return nil
	}); !errors.Is(err, errWaylandApplicationClosed) {
		t.Fatalf("invoke after close error = %v, want %v", err, errWaylandApplicationClosed)
	}
}

func TestWaylandSystemAPIsUseEventLoop(t *testing.T) {
	app, wake := newWaylandInvokeTestApp(t)
	win1 := &waylandWindow{app: app}
	state1 := &waylandWindowState{app: app, win: win1}
	win1.state = state1
	win2 := &waylandWindow{app: app}
	state2 := &waylandWindowState{app: app, win: win2}
	win2.state = state2
	app.windows = []*waylandWindowState{state1, state2}
	activeWaylandApp.Store(app)

	tests := []struct {
		name    string
		call    func() error
		wantErr bool
	}{
		{name: "cursor", call: func() error { return SetCursorShape(0) }},
		{name: "set clipboard", call: func() error { return SetClipboardText("text") }, wantErr: true},
		{name: "get clipboard", call: func() error { _, err := ClipboardText(); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := make(chan error, 1)
			go func() { result <- test.call() }()
			waitWaylandInvokeSignal(t, wake)
			select {
			case err := <-result:
				t.Fatalf("system call bypassed the loop: %v", err)
			default:
			}
			app.loop.drainCommands()
			select {
			case err := <-result:
				if (err != nil) != test.wantErr {
					t.Fatalf("system call error = %v, want error %v", err, test.wantErr)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for system call result")
			}
		})
	}
	win1.destroy()
	if !app.hasWindows() || len(app.windows) != 1 || app.windows[0] != state2 {
		t.Fatal("closing one window disturbed the remaining window")
	}
	if err := SetCursorShape(0); err != nil {
		t.Fatalf("system call with another window open: %v", err)
	}
}

func TestWaylandBackgroundClipboardReadDoesNotBlockLoop(t *testing.T) {
	app, wake := newWaylandInvokeTestApp(t)
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	win.state = state
	app.windows = []*waylandWindowState{state}
	activeWaylandApp.Store(app)

	var writer *os.File
	app.api = &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
			if opcode == wlDataOfferReceiveOpcode {
				dup, err := syscall.Dup(int(args[1].(int32)))
				if err != nil {
					t.Errorf("duplicate clipboard writer: %v", err)
					return 0
				}
				writer = os.NewFile(uintptr(dup), "wayland-clipboard-writer")
			}
			return 0
		},
	}
	app.clipboardOffer = &waylandDataOffer{proxy: 502, mimeTypes: []string{"UTF8_STRING"}}
	t.Cleanup(func() {
		if writer != nil {
			_ = writer.Close()
		}
	})

	result := make(chan error, 1)
	go func() {
		_, err := ClipboardText()
		result <- err
	}()
	waitWaylandInvokeSignal(t, wake)
	nextCommand := false
	app.post(func() { nextCommand = true })
	app.loop.drainCommands()
	if !nextCommand {
		t.Fatal("clipboard request blocked the following loop command")
	}
	if writer == nil {
		t.Fatal("clipboard request was not sent")
	}
	select {
	case err := <-result:
		t.Fatalf("clipboard read completed before its peer answered: %v", err)
	default:
	}

	app.close()
	select {
	case err := <-result:
		if !errors.Is(err, errWaylandApplicationClosed) {
			t.Fatalf("clipboard close error = %v, want %v", err, errWaylandApplicationClosed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel the background clipboard read")
	}
}

func TestWaylandSystemCallRejectedDuringRunExit(t *testing.T) {
	app, wake := newWaylandInvokeTestApp(t)
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	win.state = state
	app.windows = []*waylandWindowState{state}
	result := make(chan error, 1)

	err := app.Run(func(platform.Application) error {
		go func() { result <- SetCursorShape(2) }()
		waitWaylandInvokeSignal(t, wake)
		win.destroy()
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, errWaylandApplicationClosed) {
			t.Fatalf("system call after Run exit = %v, want %v", err, errWaylandApplicationClosed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run exit did not release the system call")
	}
	if app.currentCursorShape != 0 {
		t.Fatal("queued cursor command executed during Run exit")
	}
}

func TestWaylandAddStatusItemContract(t *testing.T) {
	app := &waylandApplication{}
	_, err := app.AddStatusItem(platform.StatusItemConfig{})
	if err == nil {
		t.Fatal("expected error from Wayland AddStatusItem, got nil")
	}
	if !errors.Is(err, platform.ErrStatusAreaUnavailable) {
		t.Fatalf("expected error wrapping platform.ErrStatusAreaUnavailable, got: %v", err)
	}
}

func TestWaylandQuitBypassesCloseRequestAndClosesAllWindows(t *testing.T) {
	app := &waylandApplication{}

	var closeRequestCalled atomic.Int32
	var win1Closed atomic.Int32
	var win2Closed atomic.Int32

	win1 := &waylandWindow{app: app}
	state1 := &waylandWindowState{
		app: app,
		win: win1,
		config: platform.Config{
			OnCloseRequest: func(closeWindow func()) {
				closeRequestCalled.Add(1)
				// deliberately refuse to close
			},
			OnClosed: func() {
				win1Closed.Add(1)
			},
		},
	}
	win1.state = state1

	win2 := &waylandWindow{app: app}
	state2 := &waylandWindowState{
		app: app,
		win: win2,
		config: platform.Config{
			OnCloseRequest: func(closeWindow func()) {
				closeRequestCalled.Add(1)
			},
			OnClosed: func() {
				win2Closed.Add(1)
			},
		},
	}
	win2.state = state2

	app.windows = []*waylandWindowState{state1, state2}

	// Quit must close all windows and trigger OnClosed without asking OnCloseRequest
	app.Quit()

	if req := closeRequestCalled.Load(); req != 0 {
		t.Errorf("expected 0 close requests called, got %d", req)
	}
	if c1 := win1Closed.Load(); c1 != 1 {
		t.Errorf("expected win1 OnClosed called 1 time, got %d", c1)
	}
	if c2 := win2Closed.Load(); c2 != 1 {
		t.Errorf("expected win2 OnClosed called 1 time, got %d", c2)
	}
	if len(app.windows) != 0 {
		t.Errorf("expected app.windows to be empty, got %d windows", len(app.windows))
	}
}

func TestWaylandStatusItemDrainRemovals(t *testing.T) {
	app := &waylandApplication{}
	notifier := &statusNotifier{
		items: map[string]*linuxStatusItem{
			"item1": {name: "item1"},
			"item2": {name: "item2"},
		},
		removals: []string{"item1"},
	}
	app.status = notifier

	app.drainStatusRemovals()

	if _, exists := notifier.items["item1"]; exists {
		t.Error("expected item1 to be removed from notifier")
	}
	if _, exists := notifier.items["item2"]; !exists {
		t.Error("expected item2 to remain in notifier")
	}
}

func TestWaylandWindowContentOrigin(t *testing.T) {
	win := &waylandWindow{
		margin: 48,
	}
	win.storeScale(2.0)

	originX, originY := win.contentOrigin()
	if originX != 24 || originY != 24 {
		t.Fatalf("expected content origin (24, 24), got (%d, %d)", originX, originY)
	}

	win.margin = 0
	win.storeScale(1.0)
	originX, originY = win.contentOrigin()
	if originX != 0 || originY != 0 {
		t.Fatalf("expected content origin (0, 0), got (%d, %d)", originX, originY)
	}
}

func TestWaylandAtspiReconnectFailureKeepsWatcherPolling(t *testing.T) {
	app := &waylandApplication{loop: &waylandLoop{}}
	host := &atspiHost{watcher: &dbus.Conn{}}
	app.atspi = host
	address := "unix:path=" + t.TempDir() + "/missing-atspi-bus"

	for range 2 {
		app.connectAtspi(host, address)
		if host.watcher == nil {
			t.Fatal("failed reconnect discarded the AT-SPI watcher")
		}
		if len(app.loop.extraSources) != 1 || app.loop.extraSources[0].FD() != int32(host.watcher.FD()) {
			t.Fatalf("watcher poll sources = %v, want one restored descriptor", app.loop.extraSources)
		}
		source, ok := app.loop.extraSources[0].(*waylandExtraSource)
		if !ok || source.dispatch == nil {
			t.Fatal("watcher dispatch callback was not restored")
		}
	}
}

func TestWaylandInputMethodFDRemovalUsesRegistration(t *testing.T) {
	loop := &waylandLoop{}
	app := &waylandApplication{loop: loop, ibusFD: 44, ibusFDSet: true}
	loop.addExtraFD(44, func() {})

	app.removeInputMethodFD()
	if len(loop.extraSources) != 0 || app.ibusFDSet {
		t.Fatalf("registered IBus FD survived removal: FDs=%v set=%v", loop.extraSources, app.ibusFDSet)
	}
	app.removeInputMethodFD()
	if len(loop.extraSources) != 0 {
		t.Fatal("repeated IBus removal changed the poll set")
	}
}

func TestWaylandCloseReleasesInputMethod(t *testing.T) {
	loop := &waylandLoop{}
	bus := &fakeWaylandInputMethod{fd: 44, pending: 2}
	app := &waylandApplication{loop: loop, ibus: bus, ibusFD: 44, ibusFDSet: true}
	loop.addExtraFD(44, func() {})

	app.close()
	if bus.closeCalls != 1 || bus.pending != 0 {
		t.Fatalf("IBus close calls=%d pending=%d", bus.closeCalls, bus.pending)
	}
	if len(loop.extraSources) != 0 || app.ibus != nil || app.ibusFDSet {
		t.Fatalf("IBus teardown left state: FDs=%v bus=%v FDSet=%v", loop.extraSources, app.ibus != nil, app.ibusFDSet)
	}
}

func TestWaylandPollTimeoutUsesEarliestDeadline(t *testing.T) {
	now := time.Unix(100, 0)
	app := &waylandApplication{}
	if timeout := app.pollTimeout(now); timeout != -1 {
		t.Fatalf("timeout without timers = %d, want -1", timeout)
	}

	bus := &fakeWaylandInputMethod{deadline: now.Add(2500 * time.Millisecond), deadlineSet: true}
	app.ibus = bus
	if timeout := app.pollTimeout(now); timeout != 2500 {
		t.Fatalf("IBus timeout = %d, want 2500", timeout)
	}
	app.repeatScancode = 30
	app.repeatDeadline = now.Add(1200 * time.Millisecond)
	if timeout := app.pollTimeout(now); timeout != 1200 {
		t.Fatalf("nearest timeout = %d, want 1200", timeout)
	}
	app.repeatDeadline = now.Add(1500 * time.Microsecond)
	if timeout := app.pollTimeout(now); timeout != 2 {
		t.Fatalf("fractional timeout = %d, want ceil to 2", timeout)
	}
	app.repeatDeadline = now.Add(-time.Millisecond)
	if timeout := app.pollTimeout(now); timeout != 0 {
		t.Fatalf("expired repeat timeout = %d, want 0", timeout)
	}
	app.repeatScancode = 0
	bus.deadline = now.Add(50 * 24 * time.Hour)
	if timeout := app.pollTimeout(now); timeout != int32(1<<31-1) {
		t.Fatalf("clamped timeout = %d, want %d", timeout, int32(1<<31-1))
	}
	bus.deadlineSet = false
	if timeout := app.pollTimeout(now); timeout != -1 {
		t.Fatalf("timeout after timers stopped = %d, want -1", timeout)
	}
}

func TestWaylandProcessDeadlinesExpiresInputMethod(t *testing.T) {
	expectedErr := errors.New("IBus key deadline expired")
	loop := &waylandLoop{}
	bus := &fakeWaylandInputMethod{fd: 44, expireErr: expectedErr}
	app := &waylandApplication{loop: loop, ibus: bus, ibusFD: 44, ibusFDSet: true}
	loop.addExtraFD(44, func() {})

	app.processDeadlines(time.Now())
	if bus.expireCalls != 1 {
		t.Fatalf("Expire calls = %d, want 1", bus.expireCalls)
	}
	if app.ibus != nil || app.ibusFDSet || len(loop.extraSources) != 0 {
		t.Fatalf("expired IBus remained registered: bus=%v FDSet=%v FDs=%v", app.ibus != nil, app.ibusFDSet, loop.extraSources)
	}
}

func TestWaylandRunUsesDeadlineAndProcessesExpiry(t *testing.T) {
	now := time.Unix(100, 0)
	bus := &fakeWaylandInputMethod{}
	var timeouts []int32
	loop := &waylandLoop{wayland: &waylandAPI{}, display: 1, waylandFD: 10, wakeFD: 11}
	installWaylandLoopReadTestCallbacks(loop.wayland)
	loop.libc = &libcAPI{poll: func(_ *pollFD, _ uintptr, timeout int32) int32 {
		timeouts = append(timeouts, timeout)
		if len(timeouts) == 2 {
			loop.running.Store(false)
		}
		return 0
	}}
	loop.running.Store(true)
	loop.rebuildPollFDs()
	loop.post(func() {
		bus.deadline = now.Add(125 * time.Millisecond)
		bus.deadlineSet = true
	})
	app := &waylandApplication{loop: loop, ibus: bus, now: func() time.Time { return now }}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{app: app, win: win}
	win.state = state
	app.windows = []*waylandWindowState{state}

	if err := app.Run(func(platform.Application) error { return nil }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(timeouts) != 2 || timeouts[0] != 125 || timeouts[1] != 125 {
		t.Fatalf("poll timeouts = %v, want [125 125]", timeouts)
	}
	if bus.expireCalls != 1 {
		t.Fatalf("Expire calls = %d, want one after the successful poll", bus.expireCalls)
	}
}

func TestWaylandPostDoesNotUseWindowMutex(t *testing.T) {
	app := &waylandApplication{loop: &waylandLoop{wakeFD: -1}}
	done := make(chan struct{})
	go func() {
		app.mu.Lock()
		defer app.mu.Unlock()
		app.post(func() {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("post deadlocked with the window callback mutex held")
	}
}

func TestWaylandPostAfterCloseDoesNotExecute(t *testing.T) {
	app := &waylandApplication{loop: &waylandLoop{wakeFD: -1}}
	app.close()
	executed := 0
	app.post(func() { executed++ })
	if executed != 0 {
		t.Fatal("post after application close executed on the calling goroutine")
	}
	if app.loop != nil {
		t.Fatal("application close retained its event loop")
	}
}

func TestWaylandAtspiServeAfterSelfClose(t *testing.T) {
	loop := &waylandLoop{}
	conn := &fakeAtspiTransport{fd: 42}
	bridge := newAtspiBridge(conn)
	host := &atspiHost{bridge: bridge}
	app := &waylandApplication{loop: loop, atspi: host}
	app.addAtspiFD(host)
	bridge.close()

	app.serveAtspi()
	if host.bridge != nil || len(loop.extraSources) != 0 || host.waylandFDSet {
		t.Fatalf("self-closed AT-SPI state bridge=%v FDs=%v FDSet=%v", host.bridge != nil, loop.extraSources, host.waylandFDSet)
	}
	// A second readiness notification is a no-op, not another failure dispatch.
	app.serveAtspi()
}

func TestWaylandAtspiReplyFailuresRemoveFDAndStopBatch(t *testing.T) {
	for _, test := range []struct {
		name          string
		path          string
		replyErr      error
		replyErrorErr error
	}{
		{name: "normal reply", path: atspiRootPath, replyErr: errors.New("reply write failed")},
		{name: "error reply", path: "/missing", replyErrorErr: errors.New("error reply write failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			loop := &waylandLoop{}
			call := &dbus.Message{Type: dbus.TypeCall, Path: test.path, Interface: atspiIntrospect, Member: "Introspect"}
			conn := &fakeAtspiTransport{
				messages: []*dbus.Message{call, call}, fd: 42,
				replyErr: test.replyErr, replyErrorErr: test.replyErrorErr,
			}
			host := &atspiHost{bridge: newAtspiBridge(conn)}
			app := &waylandApplication{loop: loop, atspi: host}
			app.addAtspiFD(host)

			app.serveAtspi()
			if conn.replyCalls+conn.replyErrorCalls != 1 {
				t.Fatalf("batch dispatched %d replies, want 1", conn.replyCalls+conn.replyErrorCalls)
			}
			if host.bridge != nil || len(loop.extraSources) != 0 || host.waylandFDSet {
				t.Fatalf("failed reply left AT-SPI registered: bridge=%v FDs=%v FDSet=%v", host.bridge != nil, loop.extraSources, host.waylandFDSet)
			}
		})
	}
}

func TestWaylandAtspiFDRemovalAfterConnectionClose(t *testing.T) {
	loop := &waylandLoop{}
	app := &waylandApplication{loop: loop}
	host := &atspiHost{watcher: &dbus.Conn{}}
	app.addAtspiFD(host)
	if !host.waylandFDSet || len(loop.extraSources) != 1 {
		t.Fatalf("AT-SPI registration FDs=%v set=%v", loop.extraSources, host.waylandFDSet)
	}

	// A failed service may clear its connection before the application cleans up.
	host.watcher = nil
	app.removeAtspiFD(host)
	if len(loop.extraSources) != 0 || host.waylandFDSet {
		t.Fatalf("closed AT-SPI FD survived removal: FDs=%v set=%v", loop.extraSources, host.waylandFDSet)
	}
}

func TestWaylandAtspiLifecycle(t *testing.T) {
	app := &waylandApplication{}
	win := &waylandWindow{app: app}
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			Title: "Wayland Atspi Window",
			Accessibility: []platform.AccessibilityNode{
				{Role: platform.A11yRoleButton, Label: "Click me"},
			},
		},
	}
	win.state = state

	app.atspiOpened(state)
	if app.atspi != nil {
		app.atspiFrame(state, []platform.AccessibilityNode{
			{Role: platform.A11yRoleButton, Label: "Clicked"},
		})
		app.atspiActive(state, true)
		app.atspiActive(state, false)
		app.atspiClosed(state)
	}
	app.atspiClose()
	if app.atspi != nil {
		t.Error("expected app.atspi to be nil after atspiClose")
	}
}

func TestWaylandRunReturnsFirstApplicationError(t *testing.T) {
	expectedErr := errors.New("first presentation failure")
	app := &waylandApplication{}
	app.fail(expectedErr)
	app.fail(errors.New("later failure"))

	err := app.Run(func(platform.Application) error { return nil })
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Run error = %v, want first error %v", err, expectedErr)
	}
}

func TestWaylandRunReturnsLoopError(t *testing.T) {
	expectedErr := errors.New("wayland transport failed")
	app := &waylandApplication{loop: &waylandLoop{err: expectedErr}}

	err := app.Run(func(platform.Application) error { return nil })
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Run error = %v, want %v", err, expectedErr)
	}
}

func TestWaylandRunClosesWindowsOnStartError(t *testing.T) {
	app := &waylandApplication{}

	var winClosed atomic.Int32

	expectedErr := errors.New("custom start failure")
	err := app.Run(func(a platform.Application) error {
		win := &waylandWindow{app: app}
		state := &waylandWindowState{
			app: app,
			win: win,
			config: platform.Config{
				OnClosed: func() {
					winClosed.Add(1)
				},
			},
		}
		win.state = state
		app.windows = append(app.windows, state)
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if winClosed.Load() != 1 {
		t.Errorf("expected OnClosed to be called on start error, got %d", winClosed.Load())
	}
	if len(app.windows) != 0 {
		t.Errorf("expected app.windows to be empty, got %d", len(app.windows))
	}
}
