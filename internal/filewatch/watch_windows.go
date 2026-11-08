//go:build windows

package filewatch

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procReadDirectoryChange = kernel32.NewProc("ReadDirectoryChangesW")
	procCancelIoEx          = kernel32.NewProc("CancelIoEx")
	procGetOverlappedResult = kernel32.NewProc("GetOverlappedResult")
	procCreateEvent         = kernel32.NewProc("CreateEventW")
	procSetEvent            = kernel32.NewProc("SetEvent")
	procWaitMultiple        = kernel32.NewProc("WaitForMultipleObjects")
)

const (
	fileListDirectory   = 0x0001
	fileFlagOverlapped  = 0x40000000
	filterChange        = 0x1 | 0x2 | 0x8 | 0x10 | 0x40 // file/dir name, attributes, size, last write, creation
	actionAdded         = 1
	actionRemoved       = 2
	actionModified      = 3
	actionRenamedOld    = 4
	actionRenamedNew    = 5
	errorOperationAbort = 995
	waitObject0         = 0
	infinite            = 0xFFFFFFFF
	// bufferSize holds many records of the longest names; the system reports an
	// overflow as a completion with zero bytes, which becomes a resync.
	bufferSize = 64 * 1024
)

// directoryBackend issues one overlapped ReadDirectoryChangesW at a time and
// waits on its event together with a stop event, so Close ends the wait without
// a race between issuing the read and cancelling it.
type directoryBackend struct {
	tree   bool
	handle syscall.Handle
	ioDone syscall.Handle
	stop   syscall.Handle
	done   chan struct{}
	once   sync.Once
	// overlapped and the read buffer are written by the system after the call
	// returns, so both live in this heap object, which Go never moves.
	overlapped syscall.Overlapped
	buf        [bufferSize]byte
}

func openBackend(dir string, w *Watch) (backend, error) {
	path, err := syscall.UTF16PtrFromString(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(path, fileListDirectory,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|fileFlagOverlapped, 0)
	if err != nil {
		return nil, &os.PathError{Op: "CreateFile", Path: dir, Err: err}
	}
	ioDone, err := newEvent()
	if err != nil {
		syscall.CloseHandle(handle)
		return nil, err
	}
	stop, err := newEvent()
	if err != nil {
		syscall.CloseHandle(ioDone)
		syscall.CloseHandle(handle)
		return nil, err
	}
	b := &directoryBackend{tree: w.tree, handle: handle, ioDone: ioDone, stop: stop, done: make(chan struct{})}
	if err := b.issue(); err != nil {
		syscall.CloseHandle(stop)
		syscall.CloseHandle(ioDone)
		syscall.CloseHandle(handle)
		return nil, &os.PathError{Op: "ReadDirectoryChangesW", Path: dir, Err: err}
	}
	go b.run(w)
	return b, nil
}

func newEvent() (syscall.Handle, error) {
	h, _, err := procCreateEvent.Call(0, 1, 0, 0) // manual reset, unsignalled
	if h == 0 {
		return 0, os.NewSyscallError("CreateEventW", err)
	}
	return syscall.Handle(h), nil
}

func (b *directoryBackend) close() {
	b.once.Do(func() {
		procSetEvent.Call(uintptr(b.stop))
		<-b.done
		syscall.CloseHandle(b.handle)
		syscall.CloseHandle(b.ioDone)
		syscall.CloseHandle(b.stop)
	})
}

// issue starts the next overlapped read. The first one is made before Open
// returns, because changes are only buffered by the system once a read has been
// issued; later ones follow a completion, and the system keeps what happened in
// between.
func (b *directoryBackend) issue() error {
	b.overlapped = syscall.Overlapped{HEvent: b.ioDone}
	subtree := uintptr(0)
	if b.tree {
		subtree = 1
	}
	r, _, err := procReadDirectoryChange.Call(uintptr(b.handle), uintptr(unsafe.Pointer(&b.buf[0])), uintptr(len(b.buf)),
		subtree, filterChange, 0, uintptr(unsafe.Pointer(&b.overlapped)), 0)
	if r == 0 {
		return err
	}
	return nil
}

func (b *directoryBackend) run(w *Watch) {
	defer close(b.done)
	for {
		handles := [2]syscall.Handle{b.ioDone, b.stop}
		which, _, _ := procWaitMultiple.Call(2, uintptr(unsafe.Pointer(&handles[0])), 0, infinite)
		var n uint32
		if which != waitObject0 {
			// Stopped (or the wait failed): abandon the read and wait for the system
			// to stop writing into the buffer before it is released.
			procCancelIoEx.Call(uintptr(b.handle), uintptr(unsafe.Pointer(&b.overlapped)))
			procGetOverlappedResult.Call(uintptr(b.handle), uintptr(unsafe.Pointer(&b.overlapped)), uintptr(unsafe.Pointer(&n)), 1)
			if which != waitObject0+1 {
				w.postResync()
			}
			return
		}
		r, _, err := procGetOverlappedResult.Call(uintptr(b.handle), uintptr(unsafe.Pointer(&b.overlapped)), uintptr(unsafe.Pointer(&n)), 0)
		if r == 0 {
			if errno, ok := err.(syscall.Errno); !ok || errno != errorOperationAbort {
				w.postResync()
			}
			return
		}
		if n == 0 {
			w.postResync() // the buffer overflowed
		} else {
			parseNotifications(b.buf[:n], w)
		}
		if b.issue() != nil {
			w.postResync()
			return
		}
	}
}

// parseNotifications turns FILE_NOTIFY_INFORMATION records into posts.
func parseNotifications(buf []byte, w *Watch) {
	for len(buf) >= 12 {
		next := int(binary.LittleEndian.Uint32(buf[0:4]))
		action := binary.LittleEndian.Uint32(buf[4:8])
		length := int(binary.LittleEndian.Uint32(buf[8:12]))
		if length%2 != 0 || 12+length > len(buf) {
			w.postResync()
			return
		}
		switch action {
		case actionAdded, actionRemoved, actionModified, actionRenamedOld, actionRenamedNew:
			name := make([]uint16, length/2)
			for i := range name {
				name[i] = binary.LittleEndian.Uint16(buf[12+2*i:])
			}
			if len(name) > 0 {
				w.post(strings.ReplaceAll(string(utf16.Decode(name)), `\`, "/"))
			}
		}
		if next == 0 || next > len(buf) {
			return
		}
		buf = buf[next:]
	}
}
