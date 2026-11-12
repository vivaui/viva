//go:build linux

package filewatch

import (
	"encoding/binary"
	"errors"
	"io/fs"
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
// A tree needs one registration per directory; wds maps each to its path below
// the root, and only the reading goroutine touches the map once it runs.
type inotifyBackend struct {
	fd    int
	file  *os.File
	root  string
	watch *Watch
	wds   map[int32]string // "" is the root
	done  chan struct{}
	once  sync.Once
}

func openBackend(dir string, w *Watch) (backend, error) {
	dir = filepath.Clean(dir)
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, os.NewSyscallError("inotify_init1", err)
	}
	b := &inotifyBackend{fd: fd, root: dir, watch: w, wds: map[int32]string{}, done: make(chan struct{})}
	root, err := syscall.InotifyAddWatch(fd, dir, inotifyMask)
	if err != nil {
		syscall.Close(fd)
		return nil, &os.PathError{Op: "inotify_add_watch", Path: dir, Err: err}
	}
	b.wds[int32(root)] = ""
	if w.tree {
		if err := b.addTree("", false); err != nil {
			syscall.Close(fd)
			return nil, err
		}
	}
	b.file = os.NewFile(uintptr(fd), "inotify")
	go b.run()
	return b, nil
}

// addTree registers every directory below rel. During Open a tree past
// MaxDirectories is refused; later it asks for a resync instead, since the
// watch is already in use. found: report what is there, because a directory that
// appeared may have been filled before its registration.
func (b *inotifyBackend) addTree(rel string, found bool) error {
	base := filepath.Join(b.root, filepath.FromSlash(rel))
	return filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == base {
				return err
			}
			return nil // gone or unreadable meanwhile: its parent's events say so
		}
		sub, relErr := filepath.Rel(b.root, path)
		if relErr != nil {
			return nil
		}
		sub = filepath.ToSlash(sub)
		if sub == "." {
			sub = ""
		}
		if path != base && found && sub != "" {
			b.watch.post(sub)
		}
		if !entry.IsDir() || path == base && rel == "" {
			return nil
		}
		if sub != "" && b.watch.skipped(sub) {
			return filepath.SkipDir
		}
		if len(b.wds) >= MaxDirectories {
			if found {
				b.watch.postResync()
				return filepath.SkipAll
			}
			return ErrTooLarge
		}
		wd, err := syscall.InotifyAddWatch(b.fd, path, inotifyMask)
		if err != nil {
			if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTDIR) {
				return nil
			}
			if found {
				b.watch.postResync()
				return filepath.SkipAll
			}
			return &os.PathError{Op: "inotify_add_watch", Path: path, Err: err}
		}
		b.wds[int32(wd)] = sub
		return nil
	})
}

func (b *inotifyBackend) close() {
	b.once.Do(func() { b.file.Close() })
	<-b.done
}

func (b *inotifyBackend) run() {
	defer close(b.done)
	// A read returns whole events; this holds several of the largest.
	buf := make([]byte, 16*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
	for {
		n, err := b.file.Read(buf)
		if err != nil {
			if !errors.Is(err, os.ErrClosed) {
				b.watch.postResync()
			}
			return
		}
		b.parse(buf[:n])
	}
}

// parse turns a read's worth of inotify_event records into posts.
func (b *inotifyBackend) parse(buf []byte) {
	for len(buf) >= syscall.SizeofInotifyEvent {
		wd := int32(binary.NativeEndian.Uint32(buf[0:4]))
		mask := binary.NativeEndian.Uint32(buf[4:8])
		length := int(binary.NativeEndian.Uint32(buf[12:16]))
		end := syscall.SizeofInotifyEvent + length
		if end > len(buf) {
			b.watch.postResync()
			return
		}
		dir, known := b.wds[wd]
		switch {
		case mask&syscall.IN_Q_OVERFLOW != 0:
			b.watch.postResync()
		case mask&syscall.IN_IGNORED != 0:
			// The registration ended with its directory. The root going away is news
			// of its own; a subdirectory's removal was already reported by the
			// parent's delete.
			if known && dir == "" {
				b.watch.postResync()
			}
			delete(b.wds, wd)
		case !known:
			// An event for a registration already dropped.
		case mask&(syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_UNMOUNT) != 0:
			if dir == "" {
				b.watch.postResync()
			}
		case length > 0:
			name := buf[syscall.SizeofInotifyEvent:end]
			for i, c := range name {
				if c == 0 {
					name = name[:i]
					break
				}
			}
			if len(name) > 0 {
				rel := string(name)
				if dir != "" {
					rel = dir + "/" + rel
				}
				if b.watch.tree {
					if mask&syscall.IN_ISDIR != 0 && mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 && !b.watch.skipped(rel) {
						b.addTree(rel, true)
					}
					b.watch.post(rel)
				} else if dir == "" {
					b.watch.post(rel)
				}
			}
		}
		buf = buf[end:]
	}
}
