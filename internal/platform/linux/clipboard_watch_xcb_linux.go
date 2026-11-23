//go:build linux && !vivagtk

package linux

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

var clipboardWatch struct {
	sync.Mutex
	current *x11ClipboardWatch
}

type x11ClipboardWatch struct {
	api                 *xcbAPI
	connection, library uintptr
	root, clipboard     uint32
	event               uint8
	wake                [2]int
	notify              func() // guarded by clipboardWatch
}

type xfixesVersionReply struct {
	Header [8]byte
	Major  uint32
}

// OnClipboardChange watches CLIPBOARD on a separate connection so selection
// reads and the window loop cannot consume these events. The callback must only
// hand the notification on, as on the other backends.
func OnClipboardChange(notify func()) error {
	if notify == nil {
		stopClipboardWatch()
		return nil
	}
	clipboardWatch.Lock()
	defer clipboardWatch.Unlock()
	if current := clipboardWatch.current; current != nil {
		current.notify = notify
		return nil
	}
	watch, err := newX11ClipboardWatch(notify)
	if err != nil {
		return err
	}
	clipboardWatch.current = watch
	go watch.run()
	return nil
}

func stopClipboardWatch() {
	clipboardWatch.Lock()
	defer clipboardWatch.Unlock()
	if current := clipboardWatch.current; current != nil {
		clipboardWatch.current = nil
		// EOF wakes the worker even when the server has no events. The
		// worker closes the X connection and the read end of the pipe.
		_ = unix.Close(current.wake[1])
	}
}

func newX11ClipboardWatch(notify func()) (*x11ClipboardWatch, error) {
	api, conn, screen, err := openSystemConnection()
	if err != nil {
		return nil, unsupported("clipboard change events need a usable X11 display")
	}
	w := &x11ClipboardWatch{api: api, connection: conn, root: screen.Root, notify: notify}
	if err := w.subscribe(); err != nil {
		w.closeX11()
		return nil, err
	}
	if err := unix.Pipe2(w.wake[:], unix.O_CLOEXEC); err != nil {
		w.closeX11()
		return nil, fmt.Errorf("linux: create clipboard watch wake pipe: %w", err)
	}
	return w, nil
}

func (w *x11ClipboardWatch) subscribe() error {
	library, err := purego.Dlopen("libxcb-xfixes.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return unsupported("clipboard change events need libxcb-xfixes")
	}
	w.library = library
	identity, err := purego.Dlsym(library, "xcb_xfixes_id")
	if err != nil {
		return unsupported("clipboard change events need XFixes")
	}
	var extensionData func(uintptr, uintptr) *xcbExtensionReply
	purego.RegisterLibFunc(&extensionData, w.api.library, "xcb_get_extension_data")
	extension := extensionData(w.connection, identity)
	if extension == nil || extension.Present == 0 {
		return unsupported("the X server does not provide XFixes")
	}
	w.event = extension.FirstEvent // XFixesSelectionNotify is extension event 0.
	var queryVersion func(uintptr, uint32, uint32) xcbVoidCookie
	var versionReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) *xfixesVersionReply
	purego.RegisterLibFunc(&queryVersion, library, "xcb_xfixes_query_version")
	purego.RegisterLibFunc(&versionReply, library, "xcb_xfixes_query_version_reply")
	var nativeError unsafe.Pointer
	version := versionReply(w.connection, queryVersion(w.connection, 2, 0), &nativeError)
	if nativeError != nil {
		w.api.free(nativeError)
	}
	if version == nil {
		return unsupported("clipboard change events could not negotiate XFixes")
	}
	major := version.Major
	w.api.free(unsafe.Pointer(version))
	if major < 2 {
		return unsupported("clipboard change events need XFixes 2.0")
	}
	if w.clipboard, err = w.api.atom(w.connection, "CLIPBOARD"); err != nil {
		return err
	}
	var selectInput func(uintptr, uint32, uint32, uint32) xcbVoidCookie
	purego.RegisterLibFunc(&selectInput, library, "xcb_xfixes_select_selection_input_checked")
	const setOwner, windowDestroy, clientClose = 1, 2, 4
	if failure := w.api.requestCheck(w.connection, selectInput(w.connection, w.root, w.clipboard, setOwner|windowDestroy|clientClose)); failure != nil {
		code := failure.ErrorCode
		w.api.free(unsafe.Pointer(failure))
		return fmt.Errorf("linux: subscribe to clipboard change events: X11 error %d", code)
	}
	return nil
}

func (w *x11ClipboardWatch) closeX11() {
	w.api.disconnect(w.connection)
	if w.library != 0 {
		_ = purego.Dlclose(w.library)
	}
}

func (w *x11ClipboardWatch) callback() func() {
	clipboardWatch.Lock()
	defer clipboardWatch.Unlock()
	if clipboardWatch.current != w {
		return nil
	}
	return w.notify
}

func (w *x11ClipboardWatch) run() {
	defer func() {
		clipboardWatch.Lock()
		if clipboardWatch.current == w {
			clipboardWatch.current = nil
			_ = unix.Close(w.wake[1])
		}
		clipboardWatch.Unlock()
		_ = unix.Close(w.wake[0])
		w.closeX11()
	}()
	fds := []unix.PollFd{{Fd: w.api.fileDescriptor(w.connection), Events: unix.POLLIN}, {Fd: int32(w.wake[0]), Events: unix.POLLIN}}
	for w.callback() != nil {
		for raw := w.api.pollForEvent(w.connection); raw != nil; raw = w.api.pollForEvent(w.connection) {
			event := (*xcbGenericEvent)(raw)
			// XFixes selection event: window, owner, selection. Ignore events
			// marked SendEvent and every other selection or extension event.
			changed := event.ResponseType == w.event && event.Data[0] == w.root && event.Data[2] == w.clipboard
			w.api.free(raw)
			callback := w.callback()
			if callback == nil {
				return
			}
			if changed {
				callback()
			}
		}
		if w.api.connectionHasError(w.connection) != 0 {
			return
		}
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil || fds[1].Revents != 0 || fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return
		}
	}
}
