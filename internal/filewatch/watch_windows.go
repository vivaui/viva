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
	procFindFirstChange     = kernel32.NewProc("FindFirstChangeNotificationW")
	procFindNextChange      = kernel32.NewProc("FindNextChangeNotification")
	procFindCloseChange     = kernel32.NewProc("FindCloseChangeNotification")
)

const (
	fileListDirectory   = 0x0001
	fileFlagOverlapped  = 0x40000000
	filterChange        = 0x1 | syscall.FILE_NOTIFY_CHANGE_DIR_NAME | 0x8 | 0x10 | 0x40 // file/dir name, attributes, size, last write, creation
	shareDirectory      = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE
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
	root   rootGuard
	done   chan struct{}
	once   sync.Once
	// overlapped and the read buffer are written by the system after the call
	// returns, so both live in this heap object, which Go never moves.
	overlapped syscall.Overlapped
	buf        [bufferSize]byte
}

func openBackend(dir string, w *Watch) (backend, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	path, err := syscall.UTF16PtrFromString(absolute)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(path, fileListDirectory, shareDirectory, nil,
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
	b.root = watchRootName(absolute, path, handle)
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

// ReadDirectoryChangesW follows the opened directory and never reports changes
// to that directory's own name. Watch direct directory names in its parent
// as well. This notification is only a signal: compare file IDs to distinguish
// a sibling change from a move or a different directory at the same path.
type rootGuard struct {
	event syscall.Handle
	path  *uint16
	info  syscall.ByHandleFileInformation
}

func watchRootName(dir string, path *uint16, handle syscall.Handle) rootGuard {
	parent := filepath.Dir(dir)
	if parent == dir {
		return rootGuard{} // volume root
	}
	var info syscall.ByHandleFileInformation
	if syscall.GetFileInformationByHandle(handle, &info) != nil {
		return rootGuard{}
	}
	parentPath, err := syscall.UTF16PtrFromString(parent)
	if err != nil {
		return rootGuard{}
	}
	event, _, _ := procFindFirstChange.Call(uintptr(unsafe.Pointer(parentPath)), 0, syscall.FILE_NOTIFY_CHANGE_DIR_NAME)
	if syscall.Handle(event) == syscall.InvalidHandle {
		// A caller can have access to the directory without being able to
		// list its parent. Keep delivering the directory's own changes.
		return rootGuard{}
	}
	return rootGuard{event: syscall.Handle(event), path: path, info: info}
}

func (g *rootGuard) sameDirectory() bool {
	h, err := syscall.CreateFile(g.path, 0, shareDirectory, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var current syscall.ByHandleFileInformation
	return syscall.GetFileInformationByHandle(h, &current) == nil &&
		current.VolumeSerialNumber == g.info.VolumeSerialNumber &&
		current.FileIndexHigh == g.info.FileIndexHigh && current.FileIndexLow == g.info.FileIndexLow
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
	pending := true
	defer func() {
		if pending {
			// Wait for the system to stop writing into the buffer before it
			// is released.
			procCancelIoEx.Call(uintptr(b.handle), uintptr(unsafe.Pointer(&b.overlapped)))
			var n uint32
			procGetOverlappedResult.Call(uintptr(b.handle), uintptr(unsafe.Pointer(&b.overlapped)), uintptr(unsafe.Pointer(&n)), 1)
		}
	}()
	// Stop takes priority over further changes, and root changes take
	// priority over events that may have come from the old directory.
	handles := []syscall.Handle{b.stop, b.ioDone}
	if b.root.event != 0 {
		defer procFindCloseChange.Call(uintptr(b.root.event))
		handles = []syscall.Handle{b.stop, b.root.event, b.ioDone}
		// The path may have changed before the parent notification was armed.
		if !b.root.sameDirectory() {
			w.postResync()
			return
		}
	}
	readReady := waitObject0 + uintptr(len(handles)-1)
	for {
		which, _, _ := procWaitMultiple.Call(uintptr(len(handles)), uintptr(unsafe.Pointer(&handles[0])), 0, infinite)
		switch {
		case which == waitObject0:
			return
		case which == readReady:
			pending = false
		case b.root.event != 0 && which == waitObject0+1:
			// Rearm before looking up the path, so a change during the lookup
			// either shows up in the lookup or signals the next wait.
			ok, _, _ := procFindNextChange.Call(uintptr(b.root.event))
			if ok == 0 || !b.root.sameDirectory() {
				w.postResync()
				return
			}
			continue
		default:
			w.postResync()
			return
		}
		var n uint32
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
		pending = true
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
