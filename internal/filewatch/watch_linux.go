//go:build linux

package filewatch

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const inotifyMask = syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MODIFY |
	syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_ATTRIB |
	syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_UNMOUNT | syscall.IN_ONLYDIR

// inotifyBackend reads one inotify descriptor through the runtime's poller, so
// the goroutine parks instead of holding a thread, and closing the file wakes it.
type inotifyBackend struct {
	file *os.File
	done chan struct{}
	once sync.Once
}

func openBackend(dir string, w *Watch) (backend, error) {
	dir = filepath.Clean(dir)
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, os.NewSyscallError("inotify_init1", err)
	}
	if _, err := syscall.InotifyAddWatch(fd, dir, inotifyMask); err != nil {
		syscall.Close(fd)
		return nil, &os.PathError{Op: "inotify_add_watch", Path: dir, Err: err}
	}
	b := &inotifyBackend{file: os.NewFile(uintptr(fd), "inotify"), done: make(chan struct{})}
	go b.run(w)
	return b, nil
}

func (b *inotifyBackend) close() {
	b.once.Do(func() { b.file.Close() })
	<-b.done
}

func (b *inotifyBackend) run(w *Watch) {
	defer close(b.done)
	// A read returns whole events; this holds several of the largest.
	buf := make([]byte, 16*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
	for {
		n, err := b.file.Read(buf)
		if err != nil {
			if !errors.Is(err, os.ErrClosed) {
				w.postResync()
			}
			return
		}
		parseInotify(buf[:n], w)
	}
}

// parseInotify turns a read's worth of inotify_event records into posts.
func parseInotify(buf []byte, w *Watch) {
	for len(buf) >= syscall.SizeofInotifyEvent {
		mask := binary.NativeEndian.Uint32(buf[4:8])
		length := int(binary.NativeEndian.Uint32(buf[12:16]))
		end := syscall.SizeofInotifyEvent + length
		if end > len(buf) {
			w.postResync()
			return
		}
		switch {
		case mask&(syscall.IN_Q_OVERFLOW|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_UNMOUNT|syscall.IN_IGNORED) != 0:
			w.postResync()
		case length > 0:
			name := buf[syscall.SizeofInotifyEvent:end]
			for i, c := range name {
				if c == 0 {
					name = name[:i]
					break
				}
			}
			if len(name) > 0 {
				w.post(string(name))
			}
		}
		buf = buf[end:]
	}
}
