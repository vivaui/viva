//go:build windows

package windows

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Posted work is woken through a message-only window, not a thread message.
// A modal Win32 loop — a native menu, a dialog — retrieves messages and hands
// each one to DispatchMessage, but a thread message has no window to reach, so
// the modal loop consumes it and the event loop never sees the wake. The
// coalescing flag then stayed set and every later Post was accepted and never
// run: work waiting on the loop hung until restart while requests that never
// need the loop, such as a protocol handshake or a catalog listing, kept
// answering. A window message is dispatched to its window procedure even
// inside a modal loop, so the work runs there instead.
const (
	// hwndMessage is HWND_MESSAGE: the window is message-only and has no
	// presence on any desktop.
	hwndMessage = ^uintptr(2)
	// wakeClassName names the message-only window Post wakes.
	wakeClassName = "VivaNativeWake"
)

var (
	wakeClassOnce    sync.Once
	wakeClassNamePtr *uint16
	wakeClassErr     error
	wakeCallbackOnce sync.Once
	wakeCallback     uintptr
	// wakeOwner is the application whose loop drains the posted work. There is
	// one native application per process.
	wakeOwner atomic.Pointer[windowsApplication]
)

// wakeProc runs posted work for the wake window. A modal loop dispatches the
// wake here just as the application's own loop does.
func wakeProc(window, kind, first, second uintptr) (result uintptr) {
	if kind == wmNotificationActivation {
		activateSystemNotification(window, uint32(first), uint32(second))
		return 0
	}
	if kind == wmPost {
		if application := wakeOwner.Load(); application != nil {
			application.drainPosts()
		}
		return 0
	}
	result, _, _ = defaultWindowProc.Call(window, kind, first, second)
	return
}

// ensureWakeClass registers the wake window class once per process. A class
// registered here lives as long as the process; only the window is per loop.
func ensureWakeClass(instance uintptr) error {
	wakeClassOnce.Do(func() {
		wakeCallbackOnce.Do(func() { wakeCallback = syscall.NewCallback(wakeProc) })
		wakeClassNamePtr, _ = syscall.UTF16PtrFromString(wakeClassName)
		class := windowClass{
			Size:      uint32(unsafe.Sizeof(windowClass{})),
			Procedure: wakeCallback,
			Instance:  instance,
			Name:      wakeClassNamePtr,
		}
		if registered, _, err := registerClass.Call(uintptr(unsafe.Pointer(&class))); registered == 0 {
			wakeClassErr = fmt.Errorf("windows: register the wake class: %w", err)
		}
	})
	return wakeClassErr
}

// openWakeWindow creates the message-only window Post wakes. It must run on
// the event-loop thread.
func (application *windowsApplication) openWakeWindow() error {
	instance, _, _ := getModule.Call(0)
	if err := ensureWakeClass(instance); err != nil {
		return err
	}
	window, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(wakeClassNamePtr)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, instance, 0)
	if window == 0 {
		return fmt.Errorf("windows: create the wake window: %w", err)
	}
	application.wakeWindow.Store(window)
	wakeOwner.Store(application)
	return nil
}

// closeWakeWindow drops the wake window; no posted work runs after it.
func (application *windowsApplication) closeWakeWindow() {
	if window := application.wakeWindow.Swap(0); window != 0 {
		removeInteractiveNotifications(window)
		destroyWindow.Call(window)
	}
	wakeOwner.CompareAndSwap(application, nil)
}

// Post runs fn on the event-loop thread; it is safe from any goroutine.
func (application *windowsApplication) Post(fn func()) {
	if fn == nil {
		return
	}
	application.postMu.Lock()
	application.posts = append(application.posts, fn)
	application.postMu.Unlock()
	if application.postWake.CompareAndSwap(false, true) && !application.wake() {
		// No wake reached the loop. Clear the flag so the next Post posts one
		// instead of leaving the work queued forever.
		application.postWake.Store(false)
	}
}

// wake asks the loop to drain the work Post accepted. The window message is
// the reliable form: a modal loop dispatches it to wakeProc. The thread
// message is only a fallback for a Post before the window exists.
func (application *windowsApplication) wake() bool {
	if window := application.wakeWindow.Load(); window != 0 {
		posted, _, _ := postMessage.Call(window, wmPost, 0, 0)
		return posted != 0
	}
	posted, _, _ := postThreadMessage.Call(uintptr(application.thread), wmPost, 0, 0)
	return posted != 0
}

// drainPosts runs the functions accepted since the last drain, in order. A
// function that posts another is picked up by this same drain.
func (application *windowsApplication) drainPosts() {
	application.postWake.Store(false)
	for {
		application.postMu.Lock()
		if len(application.posts) == 0 {
			application.postMu.Unlock()
			return
		}
		fn := application.posts[0]
		application.posts = application.posts[1:]
		application.postMu.Unlock()
		fn()
	}
}
