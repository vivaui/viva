//go:build linux

package linux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	efdCloexec    = 0x80000
	efdNonblock   = 0x800
	pollIn        = 0x0001
	pollOut       = 0x0004
	pollErrorBits = 0x0008 | 0x0010 | 0x0020 // POLLERR | POLLHUP | POLLNVAL
)

type waylandCommand func()

// waylandEventSource is an auxiliary descriptor in the main loop. The source
// owner closes the descriptor after unregistering it; the loop never closes it.
type waylandEventSource interface {
	FD() int32
	Interest() uint16
	Dispatch(revents uint16) error
}

type waylandExtraSource struct {
	fd       int32
	interest uint16
	dispatch func(revents uint16) error
}

func (source *waylandExtraSource) FD() int32        { return source.fd }
func (source *waylandExtraSource) Interest() uint16 { return source.interest }
func (source *waylandExtraSource) Dispatch(revents uint16) error {
	return source.dispatch(revents)
}

type libcAPI struct {
	library       uintptr
	poll          func(*pollFD, uintptr, int32) int32
	eventfd       func(uint32, int32) int32
	read          func(int32, unsafe.Pointer, uintptr) int
	write         func(int32, unsafe.Pointer, uintptr) int
	close         func(int32) int32
	errnoLocation func() *int32
}

func loadLibc() (*libcAPI, error) {
	library, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("linux: failed to load libc.so.6: %w", err)
	}
	api := &libcAPI{library: library}
	purego.RegisterLibFunc(&api.poll, library, "poll")
	purego.RegisterLibFunc(&api.eventfd, library, "eventfd")
	purego.RegisterLibFunc(&api.read, library, "read")
	purego.RegisterLibFunc(&api.write, library, "write")
	purego.RegisterLibFunc(&api.close, library, "close")
	purego.RegisterLibFunc(&api.errnoLocation, library, "__errno_location")
	return api, nil
}

func (c *libcAPI) errno() syscall.Errno {
	if c == nil || c.errnoLocation == nil {
		return 0
	}
	return syscall.Errno(*c.errnoLocation())
}

func (c *libcAPI) teardown() {
	if c != nil && c.library != 0 {
		purego.Dlclose(c.library)
		c.library = 0
	}
}

// waylandLoop coordinates the Wayland display socket, D-Bus sockets, and cross-goroutine eventfd waker in a single poll loop.
type waylandLoop struct {
	libc      *libcAPI
	ownsLibc  bool
	wayland   *waylandAPI
	display   uintptr
	waylandFD int32
	wakeFD    int32

	cmdMu    sync.Mutex
	cmdQueue []waylandCommand

	running atomic.Bool
	err     error

	pollMu       sync.Mutex
	pollFDs      []pollFD
	extraSources []waylandEventSource
	flushPending bool
}

func newWaylandLoop(api *waylandAPI, display uintptr, libc *libcAPI) (*waylandLoop, error) {
	if api == nil || display == 0 {
		return nil, errors.New("linux: invalid wayland API or display")
	}
	ownsLibc := false
	if libc == nil {
		var err error
		libc, err = loadLibc()
		if err != nil {
			return nil, err
		}
		ownsLibc = true
	}
	wfd := api.displayGetFd(display)
	if wfd < 0 {
		if ownsLibc {
			libc.teardown()
		}
		return nil, fmt.Errorf("linux: failed to get wayland display fd: %d", wfd)
	}
	wakeFD := int32(-1)
	if libc.eventfd != nil {
		wakeFD = libc.eventfd(0, efdCloexec|efdNonblock)
	}
	if wakeFD < 0 {
		if ownsLibc {
			libc.teardown()
		}
		return nil, errors.New("linux: failed to create eventfd for wayland loop waker")
	}

	loop := &waylandLoop{
		libc:      libc,
		ownsLibc:  ownsLibc,
		wayland:   api,
		display:   display,
		waylandFD: wfd,
		wakeFD:    wakeFD,
	}
	loop.running.Store(true)
	loop.rebuildPollFDs()
	return loop, nil
}

func (l *waylandLoop) rebuildPollFDsLocked() {
	total := 2 + len(l.extraSources)
	if cap(l.pollFDs) >= total {
		l.pollFDs = l.pollFDs[:total]
	} else {
		l.pollFDs = make([]pollFD, total)
	}
	l.pollFDs[0] = pollFD{FD: l.waylandFD, Events: pollIn}
	if l.flushPending {
		l.pollFDs[0].Events |= pollOut
	}
	l.pollFDs[1] = pollFD{FD: l.wakeFD, Events: pollIn}
	for i, source := range l.extraSources {
		l.pollFDs[2+i] = pollFD{FD: source.FD(), Events: int16(source.Interest())}
	}
}

func (l *waylandLoop) rebuildPollFDs() {
	l.pollMu.Lock()
	defer l.pollMu.Unlock()
	l.rebuildPollFDsLocked()
}

func (l *waylandLoop) addEventSource(source waylandEventSource) {
	if source == nil || source.FD() < 0 {
		return
	}
	l.pollMu.Lock()
	defer l.pollMu.Unlock()
	l.extraSources = append(l.extraSources, source)
	l.rebuildPollFDsLocked()
}

func (l *waylandLoop) addExtraFD(fd int32, onReady func()) {
	l.addEventSource(&waylandExtraSource{
		fd:       fd,
		interest: pollIn,
		dispatch: func(uint16) error {
			if onReady != nil {
				onReady()
			}
			return nil
		},
	})
}

func (l *waylandLoop) removeExtraFD(fd int32) {
	if fd < 0 {
		return
	}
	l.pollMu.Lock()
	defer l.pollMu.Unlock()
	for i, source := range l.extraSources {
		if source.FD() == fd {
			l.extraSources = append(l.extraSources[:i], l.extraSources[i+1:]...)
			break
		}
	}
	l.rebuildPollFDsLocked()
}

func (l *waylandLoop) post(cmd waylandCommand) {
	if cmd == nil {
		return
	}
	l.cmdMu.Lock()
	defer l.cmdMu.Unlock()
	if l.wakeFD < 0 {
		return
	}
	l.cmdQueue = append(l.cmdQueue, cmd)

	if l.libc != nil && l.libc.write != nil {
		var val [8]byte
		binary.NativeEndian.PutUint64(val[:], 1)
		for {
			written := l.libc.write(l.wakeFD, unsafe.Pointer(&val[0]), 8)
			if written >= 0 || l.libc.errno() != syscall.EINTR {
				break
			}
		}
	}
}

func (l *waylandLoop) drainCommands() {
	l.cmdMu.Lock()
	cmds := l.cmdQueue
	l.cmdQueue = nil
	l.cmdMu.Unlock()

	for _, cmd := range cmds {
		if cmd != nil {
			cmd()
		}
	}
}

func (l *waylandLoop) setFlushPending(pending bool) {
	l.pollMu.Lock()
	l.flushPending = pending
	if len(l.pollFDs) > 0 {
		if pending {
			l.pollFDs[0].Events |= pollOut
		} else {
			l.pollFDs[0].Events &^= pollOut
		}
	}
	l.pollMu.Unlock()
}

func (l *waylandLoop) fail(err error) {
	if err == nil {
		return
	}
	if l.err == nil {
		l.err = err
	}
	l.running.Store(false)
}

func (l *waylandLoop) displayError(operation string) error {
	if l.wayland != nil && l.wayland.displayGetError != nil {
		if code := l.wayland.displayGetError(l.display); code != 0 {
			return fmt.Errorf("linux: Wayland %s failed (error %d)", operation, code)
		}
	}
	if errno := l.libc.errno(); errno != 0 {
		return fmt.Errorf("linux: Wayland %s failed: %w", operation, errno)
	}
	return fmt.Errorf("linux: Wayland %s failed", operation)
}

func (l *waylandLoop) flush() bool {
	if l.wayland == nil || l.wayland.displayFlush == nil {
		return true
	}
	for {
		if l.wayland.displayFlush(l.display) >= 0 {
			l.setFlushPending(false)
			return true
		}
		switch l.libc.errno() {
		case syscall.EINTR:
			continue
		case syscall.EAGAIN:
			l.setFlushPending(true)
			return true
		default:
			l.fail(l.displayError("flush"))
			return false
		}
	}
}

func (l *waylandLoop) step(timeoutMs int32) bool {
	return l.stepWithTimeout(func() int32 { return timeoutMs })
}

func (l *waylandLoop) dispatchPending() bool {
	if l.wayland == nil || l.wayland.displayDispatchPending == nil {
		return true
	}
	if l.wayland.displayDispatchPending(l.display) < 0 {
		l.fail(l.displayError("dispatch pending events"))
		return false
	}
	return true
}

func (l *waylandLoop) prepareRead() (prepared, ok bool) {
	if l.wayland == nil || l.wayland.displayPrepareRead == nil {
		l.fail(errors.New("linux: Wayland prepare read is unavailable"))
		return false, false
	}
	for {
		if l.wayland.displayPrepareRead(l.display) == 0 {
			return true, true
		}
		if l.libc.errno() != syscall.EAGAIN {
			l.fail(l.displayError("prepare read"))
			return false, false
		}
		if !l.dispatchPending() {
			return false, false
		}
	}
}

func (l *waylandLoop) cancelRead(prepared bool) {
	if prepared && l.wayland != nil && l.wayland.displayCancelRead != nil {
		l.wayland.displayCancelRead(l.display)
	}
}

func (l *waylandLoop) finishRead(prepared, readable bool) bool {
	if !prepared {
		return true
	}
	if readable {
		if l.wayland.displayReadEvents == nil {
			l.fail(errors.New("linux: Wayland read events is unavailable"))
			return false
		}
		if l.wayland.displayReadEvents(l.display) < 0 {
			l.fail(l.displayError("read events"))
			return false
		}
		return true
	}
	if l.wayland.displayCancelRead == nil {
		l.fail(errors.New("linux: Wayland cancel read is unavailable"))
		return false
	}
	l.wayland.displayCancelRead(l.display)
	return true
}

func (l *waylandLoop) stepWithTimeout(timeout func() int32) bool {
	if !l.running.Load() {
		return false
	}

	l.drainCommands()
	if !l.running.Load() {
		return false
	}

	// Empty the client-side queue before waiting, then pair every successful
	// prepare with exactly one read or cancel, including all poll error paths.
	if !l.dispatchPending() {
		return false
	}
	prepared, ok := l.prepareRead()
	if !ok {
		return false
	}
	if !l.flush() {
		l.cancelRead(prepared)
		return false
	}

	l.pollMu.Lock()
	nfds := uintptr(len(l.pollFDs))
	pfd := &l.pollFDs[0]
	l.pollMu.Unlock()

	if l.libc != nil && l.libc.poll != nil {
		timeoutMs := int32(-1)
		if timeout != nil {
			timeoutMs = timeout()
		}
		ret := l.libc.poll(pfd, nfds, timeoutMs)
		if ret < 0 {
			errno := l.libc.errno()
			l.cancelRead(prepared)
			if errno == syscall.EINTR {
				return l.running.Load()
			}
			l.fail(fmt.Errorf("linux: poll Wayland connections failed (errno %d)", errno))
			return false
		}
	}

	type readySource struct {
		source  waylandEventSource
		revents uint16
	}

	l.pollMu.Lock()
	woken := len(l.pollFDs) > 1 && l.pollFDs[1].REvents&pollIn != 0
	if woken {
		var val [8]byte
		if l.libc != nil && l.libc.read != nil && l.wakeFD >= 0 {
			l.libc.read(l.wakeFD, unsafe.Pointer(&val[0]), 8)
		}
	}

	var readySources []readySource
	var failedFDs []int32
	for i, source := range l.extraSources {
		if len(l.pollFDs) <= 2+i {
			continue
		}
		revents := l.pollFDs[2+i].REvents
		if revents&pollErrorBits != 0 {
			failedFDs = append(failedFDs, source.FD())
		}
		if revents != 0 {
			readySources = append(readySources, readySource{source: source, revents: uint16(revents)})
		}
	}

	waylandIn := len(l.pollFDs) > 0 && l.pollFDs[0].REvents&pollIn != 0
	waylandErr := len(l.pollFDs) > 0 && l.pollFDs[0].REvents&pollErrorBits != 0
	l.pollMu.Unlock()

	if !l.finishRead(prepared, waylandIn) {
		return false
	}

	// Failed sources are one-shot: unregister before dispatch so a closed FD
	// cannot immediately trigger poll again while the service cleans up.
	for _, fd := range failedFDs {
		l.removeExtraFD(fd)
	}
	if woken {
		l.drainCommands()
	}
	for _, ready := range readySources {
		if err := ready.source.Dispatch(ready.revents); err != nil {
			l.removeExtraFD(ready.source.FD())
			l.fail(fmt.Errorf("linux: Wayland event source %d failed: %w", ready.source.FD(), err))
			return false
		}
	}

	if !l.dispatchPending() {
		return false
	}
	if waylandErr {
		l.fail(errors.New("linux: Wayland socket error or hangup"))
		return false
	}

	return l.running.Load()
}

func (l *waylandLoop) stop() {
	l.running.Store(false)
	l.post(func() {})
}

func (l *waylandLoop) close() {
	l.stop()
	l.cmdMu.Lock()
	wakeFD := l.wakeFD
	l.wakeFD = -1
	l.cmdMu.Unlock()

	if l.libc != nil && l.libc.close != nil && wakeFD >= 0 {
		l.libc.close(wakeFD)
	}
	if l.ownsLibc && l.libc != nil {
		l.libc.teardown()
		l.libc = nil
	}
}
